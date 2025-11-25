package model

import "time"

type ArticleStatus int

const (
	StatusPending   ArticleStatus = 0 // 未处理
	StatusProcessed ArticleStatus = 1 // 已处理
	StatusFiltered  ArticleStatus = 2 // 已过滤(不重要)
)

type Article struct {
	ID          uint          `gorm:"primaryKey" json:"id"`
	FeedID      uint          `gorm:"not null" json:"feed_id"`
	Feed        Feed          `gorm:"foreignKey:FeedID" json:"feed,omitempty"`
	Title       string        `gorm:"size:500;not null" json:"title"`
	Link        string        `gorm:"size:500;uniqueIndex;not null" json:"link"`
	Content     string        `gorm:"type:text" json:"content"`
	PubDate     time.Time     `json:"pub_date"`
	Status      ArticleStatus `gorm:"default:0" json:"status"`
	Summary     string        `gorm:"type:text" json:"summary"`
	ProcessedAt *time.Time    `json:"processed_at,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
}
