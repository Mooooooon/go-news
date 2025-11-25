package service

import (
	"time"

	"go-news/internal/model"
	"gorm.io/gorm"
)

type StatusService struct {
	db *gorm.DB
}

type SystemStatus struct {
	// 文章统计
	TotalArticles     int64 `json:"total_articles"`
	PendingArticles   int64 `json:"pending_articles"`
	ProcessedArticles int64 `json:"processed_articles"`
	FilteredArticles  int64 `json:"filtered_articles"`

	// 订阅源统计
	TotalFeeds   int64 `json:"total_feeds"`
	EnabledFeeds int64 `json:"enabled_feeds"`

	// 定时任务信息
	NextFetchTime   time.Time `json:"next_fetch_time"`
	NextProcessTime time.Time `json:"next_process_time"`
}

func NewStatusService(db *gorm.DB) *StatusService {
	return &StatusService{db: db}
}

// GetSystemStatus 获取系统状态
func (s *StatusService) GetSystemStatus() (*SystemStatus, error) {
	status := &SystemStatus{}

	// 统计文章
	s.db.Model(&model.Article{}).Count(&status.TotalArticles)
	s.db.Model(&model.Article{}).Where("status = ?", model.StatusPending).Count(&status.PendingArticles)
	s.db.Model(&model.Article{}).Where("status = ?", model.StatusProcessed).Count(&status.ProcessedArticles)
	s.db.Model(&model.Article{}).Where("status = ?", model.StatusFiltered).Count(&status.FilteredArticles)

	// 统计订阅源
	s.db.Model(&model.Feed{}).Count(&status.TotalFeeds)
	s.db.Model(&model.Feed{}).Where("enabled = ?", true).Count(&status.EnabledFeeds)

	return status, nil
}
