package video

import (
	"context"
	"time"

	"backend/internal/storage"
)

// RefreshAccessURLs 根据稳定 ObjectKey 刷新视频和封面的访问地址。
// 私有 OSS 会生成新的签名 URL，本地存储会返回固定静态地址。
func RefreshAccessURLs(ctx context.Context, fileStorage storage.Storage, video *Video) error {
	if fileStorage == nil || video == nil {
		return nil
	}

	if video.PlayObjectKey != "" {
		playURL, err := fileStorage.URL(ctx, video.PlayObjectKey, time.Hour)
		if err != nil {
			return err
		}
		video.PlayURL = playURL
	}

	if video.CoverObjectKey != "" {
		coverURL, err := fileStorage.URL(ctx, video.CoverObjectKey, time.Hour)
		if err != nil {
			return err
		}
		video.CoverURL = coverURL
	}

	return nil
}
