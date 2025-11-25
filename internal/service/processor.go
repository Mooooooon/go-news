package service

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"go-news/internal/model"
	"gorm.io/gorm"
)

type ProcessorService struct {
	db  *gorm.DB
	llm *LLMService
}

func NewProcessorService(db *gorm.DB, llm *LLMService) *ProcessorService {
	return &ProcessorService{db: db, llm: llm}
}

// FilterResult 筛选结果
type FilterResult struct {
	Worth  bool   `json:"worth"`
	Reason string `json:"reason"`
}

// ProcessArticle 处理单篇文章
func (s *ProcessorService) ProcessArticle(ctx context.Context, article *model.Article) error {
	// 1. 筛选
	filterPrompt := s.llm.GetPrompt(model.ConfigPromptFilter)
	filterResp, err := s.llm.Chat(ctx, filterPrompt, article.Title+"\n\n"+article.Content)
	if err != nil {
		return err
	}

	var result FilterResult
	if err := json.Unmarshal([]byte(filterResp), &result); err != nil {
		// 简单处理:包含"不"或"no"认为不重要
		result.Worth = !strings.Contains(strings.ToLower(filterResp), "不值得") &&
			!strings.Contains(strings.ToLower(filterResp), "no")
	}

	now := time.Now()

	if !result.Worth {
		// 标记为已过滤
		article.Status = model.StatusFiltered
		article.Summary = result.Reason
		article.ProcessedAt = &now
		return s.db.Save(article).Error
	}

	// 2. 生成摘要
	summaryPrompt := s.llm.GetPrompt(model.ConfigPromptSummary)
	summary, err := s.llm.Chat(ctx, summaryPrompt, article.Title+"\n\n"+article.Content)
	if err != nil {
		return err
	}

	article.Status = model.StatusProcessed
	article.Summary = summary
	article.ProcessedAt = &now

	return s.db.Save(article).Error
}

// ProcessPendingArticles 批量处理未处理的文章,直到全部处理完成
func (s *ProcessorService) ProcessPendingArticles(ctx context.Context, limit int) error {
	// 获取待处理文章总数
	var total int64
	s.db.Model(&model.Article{}).Where("status = ?", model.StatusPending).Count(&total)

	if total == 0 {
		log.Println("[Processor] 没有待处理的文章")
		return nil
	}

	log.Printf("[Processor] 开始处理,共 %d 篇待处理文章", total)

	// 使用并发处理,最多同时处理的文章数
	concurrency := 3
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	processed := 0
	failed := 0

	// 分批循环处理,直到没有待处理文章
	for {
		var articles []model.Article
		s.db.Where("status = ?", model.StatusPending).
			Order("pub_date DESC").
			Limit(limit).
			Find(&articles)

		if len(articles) == 0 {
			break
		}

		for _, article := range articles {
			select {
			case <-ctx.Done():
				wg.Wait()
				log.Printf("[Processor] 处理中断: 已处理 %d 篇, 失败 %d 篇", processed, failed)
				return ctx.Err()
			default:
				wg.Add(1)
				semaphore <- struct{}{} // 获取信号量

				go func(art model.Article) {
					defer wg.Done()
					defer func() { <-semaphore }() // 释放信号量

					if err := s.ProcessArticle(ctx, &art); err != nil {
						log.Printf("[Processor] 处理文章失败 [%s]: %v", art.Title, err)
						mu.Lock()
						failed++
						mu.Unlock()
					} else {
						mu.Lock()
						processed++
						current := processed + failed
						if current%10 == 0 || current == int(total) {
							log.Printf("[Processor] 进度: %d/%d (已处理:%d, 失败:%d)", current, total, processed, failed)
						}
						mu.Unlock()
					}
				}(article)
			}
		}

		// 等待当前批次处理完成
		wg.Wait()
	}

	log.Printf("[Processor] 处理完成! 总计: %d 篇, 成功: %d 篇, 失败: %d 篇", total, processed, failed)
	return nil
}
