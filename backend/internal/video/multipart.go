package video

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"backend/internal/mq"
	"backend/internal/storage"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	MultipartPartSize  int64 = 10 << 20
	MaxVideoUploadSize int64 = 500 << 20
	multipartLifetime        = 2 * time.Hour
	maxActiveUploads         = 3
)

var multipartCompleteSlots = make(chan struct{}, 4)

type MultipartInitRequest struct {
	FileID     string `json:"file_id"`
	FileName   string `json:"file_name"`
	FileSize   int64  `json:"file_size"`
	TotalParts int    `json:"total_parts"`
}

type MultipartInitResponse struct {
	Mode          string    `json:"mode"`
	TaskID        string    `json:"task_id,omitempty"`
	PartSize      int64     `json:"part_size"`
	TotalParts    int       `json:"total_parts"`
	UploadedParts []int     `json:"uploaded_parts"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
}

type MultipartTaskRequest struct {
	TaskID string `json:"task_id"`
}

type MultipartSignPartsRequest struct {
	TaskID      string `json:"task_id"`
	PartNumbers []int  `json:"part_numbers"`
}

type MultipartSignedPart struct {
	PartNumber int    `json:"part_number"`
	URL        string `json:"url"`
}

type MultipartPartCompletedRequest struct {
	TaskID     string `json:"task_id"`
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
	Size       int64  `json:"size"`
}

type MultipartCompleteRequest struct {
	TaskID         string `json:"task_id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	CoverObjectKey string `json:"cover_object_key"`
}

type MultipartTaskView struct {
	ID            string    `json:"id"`
	FileID        string    `json:"file_id"`
	FileName      string    `json:"file_name"`
	FileSize      int64     `json:"file_size"`
	TotalParts    int       `json:"total_parts"`
	Status        string    `json:"status"`
	VideoID       uint      `json:"video_id,omitempty"`
	ExpiresAt     time.Time `json:"expires_at"`
	FailureReason string    `json:"failure_reason,omitempty"`
}

func multipartTaskID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func (r *Repository) FindResumableMultipart(ctx context.Context, accountID uint, fileID string) (*MultipartUpload, error) {
	var task MultipartUpload
	err := r.db.WithContext(ctx).Where("account_id = ? AND file_id = ? AND status = ? AND expires_at > ?", accountID, fileID, MultipartStatusUploading, time.Now()).Order("create_time DESC").First(&task).Error
	return &task, err
}

func (r *Repository) CountActiveMultiparts(ctx context.Context, accountID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&MultipartUpload{}).
		Where("account_id = ? AND (status = ? AND expires_at > ? OR status = ?)", accountID, MultipartStatusUploading, time.Now(), MultipartStatusCompleting).
		Count(&count).Error
	return count, err
}

func (r *Repository) CreateMultipart(ctx context.Context, task *MultipartUpload) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *Repository) FindOwnedMultipart(ctx context.Context, accountID uint, taskID string) (*MultipartUpload, error) {
	var task MultipartUpload
	err := r.db.WithContext(ctx).Where("id = ? AND account_id = ?", taskID, accountID).First(&task).Error
	return &task, err
}

func (r *Repository) ReplaceMultipartParts(ctx context.Context, taskID string, parts []storage.MultipartPart) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("upload_task_id = ?", taskID).Delete(&MultipartUploadPart{}).Error; err != nil {
			return err
		}
		now := time.Now()
		for _, part := range parts {
			row := MultipartUploadPart{UploadTaskID: taskID, PartNumber: part.PartNumber, ETag: part.ETag, Size: part.Size, CompletedAt: now}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) UpsertMultipartPart(ctx context.Context, taskID string, part storage.MultipartPart) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := MultipartUploadPart{UploadTaskID: taskID, PartNumber: part.PartNumber, ETag: part.ETag, Size: part.Size, CompletedAt: now}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "upload_task_id"}, {Name: "part_number"}},
			DoUpdates: clause.Assignments(map[string]any{"etag": part.ETag, "size": part.Size, "completed_at": now}),
		}).Create(&row).Error; err != nil {
			return err
		}
		result := tx.Model(&MultipartUpload{}).Where("id = ? AND status = ?", taskID, MultipartStatusUploading).Updates(map[string]any{
			"last_activity_at": now, "expires_at": now.Add(multipartLifetime), "failure_reason": "",
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("upload task is not active")
		}
		return nil
	})
}

func (r *Repository) TryMarkMultipartCompleting(ctx context.Context, taskID string) (bool, error) {
	result := r.db.WithContext(ctx).Model(&MultipartUpload{}).Where("id = ? AND status = ? AND expires_at > ?", taskID, MultipartStatusUploading, time.Now()).Update("status", MultipartStatusCompleting)
	return result.RowsAffected == 1, result.Error
}

func (r *Repository) RestoreMultipartUploading(ctx context.Context, taskID string, cause error) error {
	reason := errorText(cause)
	return r.db.WithContext(ctx).Model(&MultipartUpload{}).Where("id = ? AND status = ?", taskID, MultipartStatusCompleting).Updates(map[string]any{"status": MultipartStatusUploading, "failure_reason": reason}).Error
}

func (r *Repository) CompleteMultipartWithVideo(ctx context.Context, taskID string, video *Video) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(video).Error; err != nil {
			return err
		}
		eventID := newEventID("video_processing_requested")
		event := mq.VideoProcessingEvent{EventID: eventID, EventType: "video_processing_requested", VideoID: video.ID, AuthorID: video.AuthorID, OriginalObjectKey: video.OriginalObjectKey}
		msg, err := newOutboxMsg(mq.VideoProcessingQueueName, eventID, event, event.EventType, video.ID, video.AuthorID, video.Title)
		if err != nil {
			return err
		}
		if err := tx.Create(msg).Error; err != nil {
			return err
		}
		result := tx.Model(&MultipartUpload{}).Where("id = ? AND status = ?", taskID, MultipartStatusCompleting).Updates(map[string]any{
			"status": MultipartStatusCompleted, "video_id": video.ID, "failure_reason": "",
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("upload task completion lost ownership")
		}
		return nil
	})
}

func (r *Repository) ListMultipartTasks(ctx context.Context, accountID uint, limit int) ([]MultipartUpload, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var tasks []MultipartUpload
	err := r.db.WithContext(ctx).Where("account_id = ?", accountID).Order("create_time DESC").Limit(limit).Find(&tasks).Error
	return tasks, err
}

func (r *Repository) TryMarkMultipartAborting(ctx context.Context, taskID string, accountID uint, requireExpired bool) (bool, error) {
	query := r.db.WithContext(ctx).Model(&MultipartUpload{}).Where("id = ? AND status = ?", taskID, MultipartStatusUploading)
	if accountID != 0 {
		query = query.Where("account_id = ?", accountID)
	}
	if requireExpired {
		query = query.Where("expires_at < ?", time.Now())
	}
	result := query.Update("status", MultipartStatusAborting)
	return result.RowsAffected == 1, result.Error
}

func (r *Repository) MarkMultipartExpired(ctx context.Context, taskID, reason string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("upload_task_id = ?", taskID).Delete(&MultipartUploadPart{}).Error; err != nil {
			return err
		}
		return tx.Model(&MultipartUpload{}).Where("id = ? AND status = ?", taskID, MultipartStatusAborting).Updates(map[string]any{"status": MultipartStatusExpired, "failure_reason": reason}).Error
	})
}

func (r *Repository) RestoreMultipartAfterAbortFailure(ctx context.Context, taskID string, cause error) error {
	return r.db.WithContext(ctx).Model(&MultipartUpload{}).Where("id = ? AND status = ?", taskID, MultipartStatusAborting).Updates(map[string]any{"status": MultipartStatusUploading, "failure_reason": errorText(cause)}).Error
}

func (r *Repository) ListExpiredMultiparts(ctx context.Context, limit int) ([]MultipartUpload, error) {
	var tasks []MultipartUpload
	err := r.db.WithContext(ctx).Where("status = ? AND expires_at < ?", MultipartStatusUploading, time.Now()).Order("expires_at").Limit(limit).Find(&tasks).Error
	return tasks, err
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	text := strings.TrimSpace(err.Error())
	if len(text) > 500 {
		return text[:500]
	}
	return text
}

func (s *Service) multipartStore() (storage.MultipartStorage, bool) {
	store, ok := s.fileStorage.(storage.MultipartStorage)
	return store, ok
}

func (s *Service) InitMultipart(ctx context.Context, accountID uint, req MultipartInitRequest) (*MultipartInitResponse, error) {
	store, ok := s.multipartStore()
	if !ok {
		return &MultipartInitResponse{Mode: "local", PartSize: MultipartPartSize}, nil
	}
	if err := validateMultipartInit(req); err != nil {
		return nil, err
	}
	if task, err := s.repo.FindResumableMultipart(ctx, accountID, req.FileID); err == nil {
		if task.FileSize != req.FileSize || task.TotalParts != req.TotalParts || task.FileName != req.FileName {
			return nil, errors.New("file information does not match existing upload task")
		}
		parts, listErr := store.ListMultipartParts(ctx, task.ObjectKey, task.UploadID)
		if listErr != nil {
			return nil, listErr
		}
		if err := s.repo.ReplaceMultipartParts(ctx, task.ID, parts); err != nil {
			return nil, err
		}
		return multipartInitResponse(task, parts), nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	count, err := s.repo.CountActiveMultiparts(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if count >= maxActiveUploads {
		return nil, fmt.Errorf("at most %d active uploads are allowed", maxActiveUploads)
	}
	taskID, err := multipartTaskID()
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(path.Ext(req.FileName))
	ext, err = normalizeVideoExtension(ext)
	if err != nil {
		return nil, err
	}
	objectKey := path.Join("videos", fmt.Sprint(accountID), time.Now().Format("20060102"), "original-"+taskID+ext)
	uploadID, err := store.InitiateMultipart(ctx, objectKey)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	task := &MultipartUpload{ID: taskID, AccountID: accountID, FileID: req.FileID, FileName: req.FileName, FileSize: req.FileSize, ObjectKey: objectKey, UploadID: uploadID, TotalParts: req.TotalParts, Status: MultipartStatusUploading, LastActivityAt: now, ExpiresAt: now.Add(multipartLifetime)}
	if err := s.repo.CreateMultipart(ctx, task); err != nil {
		_ = store.AbortMultipart(context.Background(), objectKey, uploadID)
		return nil, err
	}
	return multipartInitResponse(task, nil), nil
}

func multipartInitResponse(task *MultipartUpload, parts []storage.MultipartPart) *MultipartInitResponse {
	uploaded := make([]int, 0, len(parts))
	for _, part := range parts {
		uploaded = append(uploaded, part.PartNumber)
	}
	sort.Ints(uploaded)
	return &MultipartInitResponse{Mode: "oss", TaskID: task.ID, PartSize: MultipartPartSize, TotalParts: task.TotalParts, UploadedParts: uploaded, ExpiresAt: task.ExpiresAt}
}

func validateMultipartInit(req MultipartInitRequest) error {
	if err := validateUploadFileID(req.FileID); err != nil {
		return err
	}
	if strings.TrimSpace(req.FileName) == "" || len(req.FileName) > 255 {
		return errors.New("invalid file name")
	}
	if req.FileSize <= 0 || req.FileSize > MaxVideoUploadSize {
		return errors.New("video file must be between 1 byte and 500 MiB")
	}
	expected := int((req.FileSize + MultipartPartSize - 1) / MultipartPartSize)
	if req.TotalParts != expected || req.TotalParts < 1 || req.TotalParts > 50 {
		return errors.New("invalid total parts")
	}
	_, err := normalizeVideoExtension(strings.ToLower(path.Ext(req.FileName)))
	return err
}

func (s *Service) SignMultipartParts(ctx context.Context, accountID uint, req MultipartSignPartsRequest) ([]MultipartSignedPart, error) {
	store, ok := s.multipartStore()
	if !ok {
		return nil, errors.New("multipart upload requires OSS storage")
	}
	task, err := s.repo.FindOwnedMultipart(ctx, accountID, req.TaskID)
	if err != nil {
		return nil, err
	}
	if task.Status != MultipartStatusUploading || !task.ExpiresAt.After(time.Now()) {
		return nil, errors.New("upload task is not active")
	}
	if len(req.PartNumbers) == 0 || len(req.PartNumbers) > 50 {
		return nil, errors.New("invalid part numbers")
	}
	seen := map[int]struct{}{}
	result := make([]MultipartSignedPart, 0, len(req.PartNumbers))
	for _, number := range req.PartNumbers {
		if number < 1 || number > task.TotalParts {
			return nil, errors.New("part number out of range")
		}
		if _, exists := seen[number]; exists {
			continue
		}
		seen[number] = struct{}{}
		url, signErr := store.SignMultipartPart(ctx, task.ObjectKey, task.UploadID, number, 15*time.Minute)
		if signErr != nil {
			return nil, signErr
		}
		result = append(result, MultipartSignedPart{PartNumber: number, URL: url})
	}
	return result, nil
}

func (s *Service) ConfirmMultipartPart(ctx context.Context, accountID uint, req MultipartPartCompletedRequest) error {
	task, err := s.repo.FindOwnedMultipart(ctx, accountID, req.TaskID)
	if err != nil {
		return err
	}
	if task.Status != MultipartStatusUploading || !task.ExpiresAt.After(time.Now()) {
		return errors.New("upload task is not active")
	}
	if req.PartNumber < 1 || req.PartNumber > task.TotalParts || strings.TrimSpace(req.ETag) == "" {
		return errors.New("invalid completed part")
	}
	expected := MultipartPartSize
	if req.PartNumber == task.TotalParts {
		expected = task.FileSize - int64(task.TotalParts-1)*MultipartPartSize
	}
	if req.Size != expected {
		return errors.New("part size does not match upload task")
	}
	return s.repo.UpsertMultipartPart(ctx, task.ID, storage.MultipartPart{PartNumber: req.PartNumber, ETag: req.ETag, Size: req.Size})
}

func (s *Service) MultipartStatus(ctx context.Context, accountID uint, taskID string) (*MultipartInitResponse, error) {
	store, ok := s.multipartStore()
	if !ok {
		return nil, errors.New("multipart upload requires OSS storage")
	}
	task, err := s.repo.FindOwnedMultipart(ctx, accountID, taskID)
	if err != nil {
		return nil, err
	}
	var parts []storage.MultipartPart
	if task.Status == MultipartStatusUploading {
		parts, err = store.ListMultipartParts(ctx, task.ObjectKey, task.UploadID)
		if err != nil {
			return nil, err
		}
		if err = s.repo.ReplaceMultipartParts(ctx, task.ID, parts); err != nil {
			return nil, err
		}
	}
	response := multipartInitResponse(task, parts)
	response.Mode = task.Status
	return response, nil
}

func (s *Service) CompleteMultipart(ctx context.Context, accountID uint, username string, req MultipartCompleteRequest) (*Video, error) {
	store, ok := s.multipartStore()
	if !ok {
		return nil, errors.New("multipart upload requires OSS storage")
	}
	task, err := s.repo.FindOwnedMultipart(ctx, accountID, req.TaskID)
	if err != nil {
		return nil, err
	}
	if task.Status == MultipartStatusCompleted && task.VideoID != 0 {
		return s.repo.FindByID(ctx, task.VideoID)
	}
	if strings.TrimSpace(req.Title) == "" {
		return nil, errors.New("title is required")
	}
	if req.CoverObjectKey != "" && !strings.HasPrefix(req.CoverObjectKey, fmt.Sprintf("covers/%d/", accountID)) {
		return nil, errors.New("cover_object_key does not belong to current account")
	}
	claimed, err := s.repo.TryMarkMultipartCompleting(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, errors.New("upload task is being completed or is no longer active")
	}

	select {
	case multipartCompleteSlots <- struct{}{}:
		defer func() { <-multipartCompleteSlots }()
	case <-ctx.Done():
		_ = s.repo.RestoreMultipartUploading(context.Background(), task.ID, ctx.Err())
		return nil, ctx.Err()
	}
	parts, err := store.ListMultipartParts(ctx, task.ObjectKey, task.UploadID)
	if err == nil {
		err = validateCompletedParts(task, parts)
	}
	if err == nil {
		err = store.CompleteMultipart(ctx, task.ObjectKey, task.UploadID, parts)
	}
	if err != nil {
		_ = s.repo.RestoreMultipartUploading(context.Background(), task.ID, err)
		return nil, err
	}

	video := &Video{AuthorID: accountID, Username: username, Title: strings.TrimSpace(req.Title), Description: strings.TrimSpace(req.Description), OriginalObjectKey: task.ObjectKey, CoverObjectKey: req.CoverObjectKey, Status: VideoStatusProcessing, CreateTime: time.Now()}
	if err := s.repo.CompleteMultipartWithVideo(ctx, task.ID, video); err != nil {
		return nil, err
	}
	_ = SaveProcessingProgress(ctx, s.cache, video.ID, "queued", 0)
	return video, nil
}

func validateCompletedParts(task *MultipartUpload, parts []storage.MultipartPart) error {
	if len(parts) != task.TotalParts {
		return fmt.Errorf("uploaded part count is %d, expected %d", len(parts), task.TotalParts)
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })
	var size int64
	for index, part := range parts {
		if part.PartNumber != index+1 || strings.TrimSpace(part.ETag) == "" {
			return errors.New("uploaded parts are incomplete")
		}
		expected := MultipartPartSize
		if part.PartNumber == task.TotalParts {
			expected = task.FileSize - int64(task.TotalParts-1)*MultipartPartSize
		}
		if part.Size != expected {
			return fmt.Errorf("part %d size mismatch", part.PartNumber)
		}
		size += part.Size
	}
	if size != task.FileSize {
		return errors.New("uploaded file size mismatch")
	}
	return nil
}

func (s *Service) AbortMultipart(ctx context.Context, accountID uint, taskID string) error {
	store, ok := s.multipartStore()
	if !ok {
		return errors.New("multipart upload requires OSS storage")
	}
	task, err := s.repo.FindOwnedMultipart(ctx, accountID, taskID)
	if err != nil {
		return err
	}
	if task.Status == MultipartStatusExpired {
		return nil
	}
	claimed, err := s.repo.TryMarkMultipartAborting(ctx, task.ID, accountID, false)
	if err != nil {
		return err
	}
	if !claimed {
		return errors.New("upload task cannot be aborted")
	}
	if err := store.AbortMultipart(ctx, task.ObjectKey, task.UploadID); err != nil {
		_ = s.repo.RestoreMultipartAfterAbortFailure(context.Background(), task.ID, err)
		return err
	}
	return s.repo.MarkMultipartExpired(ctx, task.ID, "upload cancelled and temporary parts removed")
}

func (s *Service) ListMultipartTasks(ctx context.Context, accountID uint) ([]MultipartTaskView, error) {
	tasks, err := s.repo.ListMultipartTasks(ctx, accountID, 20)
	if err != nil {
		return nil, err
	}
	result := make([]MultipartTaskView, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, MultipartTaskView{ID: task.ID, FileID: task.FileID, FileName: task.FileName, FileSize: task.FileSize, TotalParts: task.TotalParts, Status: task.Status, VideoID: task.VideoID, ExpiresAt: task.ExpiresAt, FailureReason: task.FailureReason})
	}
	return result, nil
}
