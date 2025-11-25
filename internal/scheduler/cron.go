package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"go-news/config"
	"go-news/internal/service"
)

type Scheduler struct {
	cron          *cron.Cron
	feed          *service.FeedService
	processor     *service.ProcessorService
	config        config.CronConfig
	fetchEntryID  cron.EntryID
	processEntryID cron.EntryID
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
	s.fetchEntryID, _ = s.cron.AddFunc(s.config.FetchInterval, func() {
		log.Println("[Cron] Fetching feeds...")
		s.feed.FetchAllFeeds(context.Background())
	})

	// 文章处理任务
	s.processEntryID, _ = s.cron.AddFunc(s.config.ProcessInterval, func() {
		log.Println("[Cron] Processing articles...")
		s.processor.ProcessPendingArticles(context.Background(), 5)
	})

	s.cron.Start()
	log.Printf("[Cron] Scheduler started (fetch: %s, process: %s)", s.config.FetchInterval, s.config.ProcessInterval)
}

// GetNextFetchTime 获取下次抓取时间
func (s *Scheduler) GetNextFetchTime() time.Time {
	entry := s.cron.Entry(s.fetchEntryID)
	return entry.Next
}

// GetNextProcessTime 获取下次处理时间
func (s *Scheduler) GetNextProcessTime() time.Time {
	entry := s.cron.Entry(s.processEntryID)
	return entry.Next
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}
