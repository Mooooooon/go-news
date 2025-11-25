package scheduler

import (
	"context"
	"log"

	"github.com/robfig/cron/v3"
	"go-news/internal/service"
)

type Scheduler struct {
	cron      *cron.Cron
	feed      *service.FeedService
	processor *service.ProcessorService
}

func NewScheduler(feed *service.FeedService, processor *service.ProcessorService) *Scheduler {
	return &Scheduler{
		cron:      cron.New(),
		feed:      feed,
		processor: processor,
	}
}

func (s *Scheduler) Start() {
	// 每30分钟抓取一次RSS
	s.cron.AddFunc("*/30 * * * *", func() {
		log.Println("[Cron] Fetching feeds...")
		s.feed.FetchAllFeeds(context.Background())
	})

	// 每10分钟处理一次文章
	s.cron.AddFunc("*/10 * * * *", func() {
		log.Println("[Cron] Processing articles...")
		s.processor.ProcessPendingArticles(context.Background(), 5)
	})

	s.cron.Start()
	log.Println("[Cron] Scheduler started")
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}
