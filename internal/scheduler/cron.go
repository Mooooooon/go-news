package scheduler

import (
	"context"
	"log"

	"github.com/robfig/cron/v3"
	"go-news/config"
	"go-news/internal/service"
)

type Scheduler struct {
	cron      *cron.Cron
	feed      *service.FeedService
	processor *service.ProcessorService
	config    config.CronConfig
}

func NewScheduler(feed *service.FeedService, processor *service.ProcessorService, cfg config.CronConfig) *Scheduler {
	return &Scheduler{
		cron:      cron.New(),
		feed:      feed,
		processor: processor,
		config:    cfg,
	}
}

func (s *Scheduler) Start() {
	// RSS抓取任务
	s.cron.AddFunc(s.config.FetchInterval, func() {
		log.Println("[Cron] Fetching feeds...")
		s.feed.FetchAllFeeds(context.Background())
	})

	// 文章处理任务
	s.cron.AddFunc(s.config.ProcessInterval, func() {
		log.Println("[Cron] Processing articles...")
		s.processor.ProcessPendingArticles(context.Background(), 5)
	})

	s.cron.Start()
	log.Printf("[Cron] Scheduler started (fetch: %s, process: %s)", s.config.FetchInterval, s.config.ProcessInterval)
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}
