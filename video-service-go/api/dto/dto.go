// Package dto 定义服务对外/对内传输的数据结构。
//
// 放在独立包是为了让 handler 与 service 都能引用，同时避免二者互相 import。
package dto

import "time"

// ---------------------------------------------------------------- 请求

// CreateVideoRequest 是创建视频草稿的请求体。
type CreateVideoRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	CategoryID  *uint64  `json:"categoryId"`
	Tags        []string `json:"tags"`
}

// UpdateVideoRequest 是修改视频的请求体。
// 指针字段用于区分「未传」与「传了空值」：nil 表示不修改。
type UpdateVideoRequest struct {
	Title       *string   `json:"title"`
	Description *string   `json:"description"`
	CategoryID  *uint64   `json:"categoryId"`
	Tags        *[]string `json:"tags"`
}

// ListQuery 是视频列表查询条件（由 handler 从 query string 解析）。
type ListQuery struct {
	Page       int
	PageSize   int
	Keyword    string
	CategoryID *uint64
	OwnerID    string
	Status     string
	Mine       bool
}

// CreateCategoryRequest 是新建分类的请求体。
type CreateCategoryRequest struct {
	Name string `json:"name"`
	Sort int    `json:"sort"`
}

// ---------------------------------------------------------------- 响应

// CreateVideoResult 是创建草稿的返回。
type CreateVideoResult struct {
	VideoID uint64 `json:"videoId"`
	Status  string `json:"status"`
}

// UpdateVideoResult 是修改视频的返回。
type UpdateVideoResult struct {
	VideoID uint64 `json:"videoId"`
	Updated bool   `json:"updated"`
}

// DeleteVideoResult 是删除视频的返回。
type DeleteVideoResult struct {
	VideoID uint64 `json:"videoId"`
	Deleted bool   `json:"deleted"`
}

// UploadResult 是上传原片的返回。
type UploadResult struct {
	VideoID   uint64 `json:"videoId"`
	Status    string `json:"status"`
	TaskID    uint64 `json:"taskId"`
	SizeBytes int64  `json:"sizeBytes"`
}

// PublishResult 是发布 / 下架的返回。
type PublishResult struct {
	VideoID     uint64     `json:"videoId"`
	Status      string     `json:"status"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
}

// VideoBrief 是列表项。
type VideoBrief struct {
	ID           uint64     `json:"id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	OwnerID      string     `json:"ownerId"`
	CategoryID   *uint64    `json:"categoryId"`
	CategoryName string     `json:"categoryName"`
	Tags         []string   `json:"tags"`
	CoverURL     string     `json:"coverUrl"`
	Duration     int        `json:"duration"`
	Status       string     `json:"status"`
	ViewCount    int64      `json:"viewCount"`
	PublishedAt  *time.Time `json:"publishedAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// VideoDetail 是详情（比列表多出播放地址等字段）。
type VideoDetail struct {
	VideoBrief
	PlaybackURL  string `json:"playbackUrl"`
	SizeBytes    int64  `json:"sizeBytes"`
	SourceObject string `json:"sourceObject,omitempty"` // 仅所有者 / ADMIN 可见
}

// PlaySource 是单个码率的播放地址。
type PlaySource struct {
	Profile   string `json:"profile"`
	URL       string `json:"url"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Bandwidth int    `json:"bandwidth"`
}

// PlayInfo 是播放信息。
type PlayInfo struct {
	VideoID     uint64       `json:"videoId"`
	Title       string       `json:"title"`
	CoverURL    string       `json:"coverUrl"`
	Duration    int          `json:"duration"`
	Format      string       `json:"format"`
	PlaybackURL string       `json:"playbackUrl"`
	Sources     []PlaySource `json:"sources"`
}

// TranscodeInfo 是转码任务状态。
type TranscodeInfo struct {
	VideoID    uint64     `json:"videoId"`
	TaskID     uint64     `json:"taskId"`
	Status     string     `json:"status"`
	Progress   int        `json:"progress"`
	Profiles   []string   `json:"profiles"`
	Message    string     `json:"message"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}

// CategoryDTO 是分类项。
type CategoryDTO struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
	Sort int    `json:"sort"`
}

// CategoryList 是分类列表返回。
type CategoryList struct {
	Items []CategoryDTO `json:"items"`
}

// ---------------------------------------------------------------- 事件载荷

// VideoUploadedPayload 是 video.uploaded 事件的载荷。
type VideoUploadedPayload struct {
	VideoID      uint64 `json:"videoId"`
	OwnerID      string `json:"ownerId"`
	SourceObject string `json:"sourceObject"`
	Ext          string `json:"ext"`
	SizeBytes    int64  `json:"sizeBytes"`
	TaskID       uint64 `json:"taskId"`
}

// VideoTranscodedPayload 是 video.transcoded 事件的载荷。
type VideoTranscodedPayload struct {
	VideoID     uint64 `json:"videoId"`
	TaskID      uint64 `json:"taskId"`
	Success     bool   `json:"success"`
	PlaybackURL string `json:"playbackUrl"`
	Duration    int    `json:"duration"`
	Message     string `json:"message"`
}
