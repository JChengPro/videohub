package feed

// ---- FeedVideoItem（前端 Feed 卡片数据格式）----

type FeedAuthor struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

type FeedVideoItem struct {
	ID          uint       `json:"id"`
	Author      FeedAuthor `json:"author"` // 嵌套对象，不是平铺字段
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	PlayURL     string     `json:"play_url"`
	CoverURL    string     `json:"cover_url"`
	CreateTime  int64      `json:"create_time"` // Unix 毫秒时间戳，不是 time.Time
	LikesCount  int64      `json:"likes_count"`
	IsLiked     bool       `json:"is_liked"` // 当前用户是否点赞
}

// ---- listLatest ----

type ListLatestRequest struct {
	Limit      int   `json:"limit"`
	LatestTime int64 `json:"latest_time"`
}

type ListLatestResponse struct {
	VideoList []FeedVideoItem `json:"video_list"`
	HasMore   bool            `json:"has_more"`
	NextTime  int64           `json:"next_time"`
}

// ---- listByFollowing ----

type ListByFollowingRequest struct {
	Limit      int   `json:"limit"`
	LatestTime int64 `json:"latest_time"`
}

type ListByFollowingResponse struct {
	VideoList []FeedVideoItem `json:"video_list"`
	HasMore   bool            `json:"has_more"`
	NextTime  int64           `json:"next_time"`
}

// ---- listByPopularity ----

type ListByPopularityRequest struct {
	Limit  int   `json:"limit"`
	AsOf   int64 `json:"as_of"`
	Offset int   `json:"offset"`
}

type ListByPopularityResponse struct {
	VideoList  []FeedVideoItem `json:"video_list"`
	AsOf       int64           `json:"as_of"`
	NextOffset int             `json:"next_offset"`
	HasMore    bool            `json:"has_more"`
}

type ListLikesCountRequest struct {
	Limit            int   `json:"limit"`
	LikesCountBefore int64 `json:"likes_count_before"`
	IDBefore         uint  `json:"id_before"`
}

type ListLikesCountResponse struct {
	VideoList            []FeedVideoItem `json:"video_list"`
	NextLikesCountBefore *int64          `json:"next_likes_count_before,omitempty"`
	NextIDBefore         *uint           `json:"next_id_before,omitempty"`
	HasMore              bool            `json:"has_more"`
}
