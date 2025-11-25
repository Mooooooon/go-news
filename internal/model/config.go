package model

import "time"

type Config struct {
	ID        uint      `gorm:"primaryKey"`
	Key       string    `gorm:"size:100;uniqueIndex;not null"`
	Value     string    `gorm:"type:text"`
	UpdatedAt time.Time
}

// 预定义配置键
const (
	ConfigLLMProvider   = "llm_provider"
	ConfigLLMApiURL     = "llm_api_url"
	ConfigLLMApiKey     = "llm_api_key"
	ConfigLLMModel      = "llm_model"
	ConfigPromptFilter  = "prompt_filter"
	ConfigPromptSummary = "prompt_summary"
)
