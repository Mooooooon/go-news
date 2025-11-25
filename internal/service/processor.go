package service

import (
	"context"
	"encoding/json"
	"strings"
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

// ProcessPendingArticles 批量处理未处理的文章
func (s *ProcessorService) ProcessPendingArticles(ctx context.Context, limit int) error {
	var articles []model.Article
	s.db.Where("status = ?", model.StatusPending).
		Order("pub_date DESC").
		Limit(limit).
		Find(&articles)

	for _, article := range articles {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			s.ProcessArticle(ctx, &article)
		}
	}

	return nil
}
