package service

import (
	"context"
	"time"

	"github.com/mmcdole/gofeed"
	"go-news/internal/model"
	"gorm.io/gorm"
)

type FeedService struct {
	db     *gorm.DB
	parser *gofeed.Parser
}

func NewFeedService(db *gorm.DB) *FeedService {
	return &FeedService{
		db:     db,
		parser: gofeed.NewParser(),
	}
}

// FetchFeed 抓取单个Feed
func (s *FeedService) FetchFeed(ctx context.Context, feed *model.Feed) (int, error) {
	parsed, err := s.parser.ParseURLWithContext(feed.URL, ctx)
	if err != nil {
		return 0, err
	}

	var count int
	for _, item := range parsed.Items {
		article := model.Article{
			FeedID:  feed.ID,
			Title:   item.Title,
			Link:    item.Link,
			Content: item.Description,
			PubDate: s.parseTime(item),
		}

		// 使用Link去重
		result := s.db.Where("link = ?", article.Link).FirstOrCreate(&article)
		if result.RowsAffected > 0 {
			count++
		}
	}

	return count, nil
}

// FetchAllFeeds 抓取所有启用的Feed
func (s *FeedService) FetchAllFeeds(ctx context.Context) error {
	var feeds []model.Feed
	s.db.Where("enabled = ?", true).Find(&feeds)

	for _, feed := range feeds {
		s.FetchFeed(ctx, &feed)
	}
	return nil
}

func (s *FeedService) parseTime(item *gofeed.Item) time.Time {
	if item.PublishedParsed != nil {
		return *item.PublishedParsed
	}
	return time.Now()
}
