package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go-news/internal/model"
	"gorm.io/gorm"
)

type LLMService struct {
	db     *gorm.DB
	client *http.Client
}

type LLMConfig struct {
	Provider string
	ApiURL   string
	ApiKey   string
	Model    string
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

func NewLLMService(db *gorm.DB) *LLMService {
	return &LLMService{
		db:     db,
		client: &http.Client{},
	}
}

// GetConfig 获取LLM配置
func (s *LLMService) GetConfig() (*LLMConfig, error) {
	configs := make(map[string]string)
	var items []model.Config
	s.db.Find(&items)

	for _, item := range items {
		configs[item.Key] = item.Value
	}

	return &LLMConfig{
		Provider: configs[model.ConfigLLMProvider],
		ApiURL:   configs[model.ConfigLLMApiURL],
		ApiKey:   configs[model.ConfigLLMApiKey],
		Model:    configs[model.ConfigLLMModel],
	}, nil
}

// Chat 调用LLM
func (s *LLMService) Chat(ctx context.Context, prompt, content string) (string, error) {
	cfg, err := s.GetConfig()
	if err != nil {
		return "", err
	}

	reqBody := ChatRequest{
		Model: cfg.Model,
		Messages: []Message{
			{Role: "system", Content: prompt},
			{Role: "user", Content: content},
		},
	}

	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST",
		cfg.ApiURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.ApiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// GetPrompt 获取提示词
func (s *LLMService) GetPrompt(key string) string {
	var config model.Config
	s.db.Where("key = ?", key).First(&config)
	return config.Value
}
