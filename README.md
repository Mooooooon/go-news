# go-news 开发文档

## 项目概述

**项目名称**: go-news  
**开发语言**: Golang  
**项目定位**: RSS新闻订阅 + AI智能处理系统

### 核心功能流程
```
RSS订阅源 → 抓取文章 → 存储原文 → LLM处理 → 生成摘要 → 后台展示
```

---

## 技术栈

| 组件 | 选型 | 说明 |
|------|------|------|
| Web框架 | Gin | 轻量高效 |
| 数据库 | SQLite | 简单部署，单文件存储 |
| ORM | GORM | 数据库操作 |
| RSS解析 | gofeed | RSS/Atom解析 |
| 定时任务 | cron/v3 | 定时抓取 |
| 前端 | 内嵌模板 | 简洁后台 |

---

## 数据库设计

### 表结构

```sql
-- RSS订阅源
CREATE TABLE feeds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(255) NOT NULL,
    url VARCHAR(500) NOT NULL UNIQUE,
    enabled BOOLEAN DEFAULT true,
    created_at DATETIME,
    updated_at DATETIME
);

-- 文章表
CREATE TABLE articles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    feed_id INTEGER NOT NULL,
    title VARCHAR(500) NOT NULL,
    link VARCHAR(500) NOT NULL UNIQUE,
    content TEXT,
    pub_date DATETIME,
    -- AI处理相关
    status TINYINT DEFAULT 0,  -- 0:未处理 1:已处理 2:已过滤
    summary TEXT,              -- AI生成的摘要
    processed_at DATETIME,
    created_at DATETIME,
    FOREIGN KEY (feed_id) REFERENCES feeds(id)
);

-- 系统配置表
CREATE TABLE configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key VARCHAR(100) NOT NULL UNIQUE,
    value TEXT,
    updated_at DATETIME
);
```

### 配置项说明（configs表）

| key | 说明 | 示例值 |
|-----|------|--------|
| llm_provider | LLM提供商 | openai / ollama |
| llm_api_url | API地址 | https://api.openai.com/v1 |
| llm_api_key | API密钥 | sk-xxx |
| llm_model | 模型名称 | gpt-4o-mini |
| prompt_filter | 筛选提示词 | 判断文章是否值得阅读... |
| prompt_summary | 摘要提示词 | 请用中文总结这篇文章... |

---

## 项目结构

```
go-news/
├── main.go                 # 入口
├── go.mod
├── config/
│   └── config.go           # 配置加载
├── internal/
│   ├── model/              # 数据模型
│   │   ├── feed.go
│   │   ├── article.go
│   │   └── config.go
│   ├── service/            # 业务逻辑
│   │   ├── feed.go         # RSS抓取
│   │   ├── llm.go          # LLM对接
│   │   └── processor.go    # 文章处理
│   ├── handler/            # HTTP处理
│   │   ├── feed.go
│   │   ├── article.go
│   │   └── config.go
│   └── scheduler/          # 定时任务
│       └── cron.go
├── web/
│   ├── templates/          # HTML模板
│   │   ├── layout.html
│   │   ├── feeds.html
│   │   ├── articles.html
│   │   └── settings.html
│   └── static/             # 静态资源
│       └── style.css
└── data/
    └── news.db             # SQLite数据库
```

---

## 核心模块设计

### 1. 数据模型 (internal/model)

```go
// model/feed.go
package model

import "time"

type Feed struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    Name      string    `gorm:"size:255;not null" json:"name"`
    URL       string    `gorm:"size:500;uniqueIndex;not null" json:"url"`
    Enabled   bool      `gorm:"default:true" json:"enabled"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

```go
// model/article.go
package model

import "time"

type ArticleStatus int

const (
    StatusPending   ArticleStatus = 0 // 未处理
    StatusProcessed ArticleStatus = 1 // 已处理
    StatusFiltered  ArticleStatus = 2 // 已过滤（不重要）
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
```

```go
// model/config.go
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
    ConfigLLMProvider    = "llm_provider"
    ConfigLLMApiURL      = "llm_api_url"
    ConfigLLMApiKey      = "llm_api_key"
    ConfigLLMModel       = "llm_model"
    ConfigPromptFilter   = "prompt_filter"
    ConfigPromptSummary  = "prompt_summary"
)
```

### 2. RSS抓取服务 (internal/service/feed.go)

```go
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

// 抓取单个Feed
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

// 抓取所有启用的Feed
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
```

### 3. LLM服务 (internal/service/llm.go)

```go
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

// 获取LLM配置
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

// 调用LLM
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

// 获取提示词
func (s *LLMService) GetPrompt(key string) string {
    var config model.Config
    s.db.Where("key = ?", key).First(&config)
    return config.Value
}
```

### 4. 文章处理服务 (internal/service/processor.go)

```go
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

// 处理单篇文章
func (s *ProcessorService) ProcessArticle(ctx context.Context, article *model.Article) error {
    // 1. 筛选
    filterPrompt := s.llm.GetPrompt(model.ConfigPromptFilter)
    filterResp, err := s.llm.Chat(ctx, filterPrompt, article.Title+"\n\n"+article.Content)
    if err != nil {
        return err
    }
    
    var result FilterResult
    if err := json.Unmarshal([]byte(filterResp), &result); err != nil {
        // 简单处理：包含"不"或"no"认为不重要
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

// 批量处理未处理的文章
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
```

### 5. HTTP处理器 (internal/handler)

```go
// handler/handler.go
package handler

import (
    "net/http"
    "strconv"
    
    "github.com/gin-gonic/gin"
    "go-news/internal/model"
    "go-news/internal/service"
    "gorm.io/gorm"
)

type Handler struct {
    db        *gorm.DB
    feed      *service.FeedService
    llm       *service.LLMService
    processor *service.ProcessorService
}

func NewHandler(db *gorm.DB) *Handler {
    llm := service.NewLLMService(db)
    return &Handler{
        db:        db,
        feed:      service.NewFeedService(db),
        llm:       llm,
        processor: service.NewProcessorService(db, llm),
    }
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
    // 页面
    r.GET("/", h.IndexPage)
    r.GET("/feeds", h.FeedsPage)
    r.GET("/articles", h.ArticlesPage)
    r.GET("/settings", h.SettingsPage)
    
    // API
    api := r.Group("/api")
    {
        // Feeds
        api.GET("/feeds", h.ListFeeds)
        api.POST("/feeds", h.CreateFeed)
        api.DELETE("/feeds/:id", h.DeleteFeed)
        api.POST("/feeds/:id/fetch", h.FetchFeed)
        
        // Articles
        api.GET("/articles", h.ListArticles)
        api.POST("/articles/process", h.ProcessArticles)
        
        // Config
        api.GET("/config", h.GetConfig)
        api.POST("/config", h.SaveConfig)
    }
}

// ===== Feed相关 =====

func (h *Handler) ListFeeds(c *gin.Context) {
    var feeds []model.Feed
    h.db.Find(&feeds)
    c.JSON(http.StatusOK, feeds)
}

func (h *Handler) CreateFeed(c *gin.Context) {
    var feed model.Feed
    if err := c.ShouldBindJSON(&feed); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    if err := h.db.Create(&feed).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, feed)
}

func (h *Handler) DeleteFeed(c *gin.Context) {
    id := c.Param("id")
    h.db.Delete(&model.Feed{}, id)
    c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *Handler) FetchFeed(c *gin.Context) {
    id, _ := strconv.Atoi(c.Param("id"))
    var feed model.Feed
    if err := h.db.First(&feed, id).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "feed not found"})
        return
    }
    
    count, err := h.feed.FetchFeed(c.Request.Context(), &feed)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"new_articles": count})
}

// ===== Article相关 =====

func (h *Handler) ListArticles(c *gin.Context) {
    status := c.Query("status") // pending, processed, filtered
    page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
    pageSize := 20
    
    query := h.db.Model(&model.Article{}).Preload("Feed")
    
    switch status {
    case "pending":
        query = query.Where("status = ?", model.StatusPending)
    case "processed":
        query = query.Where("status = ?", model.StatusProcessed)
    case "filtered":
        query = query.Where("status = ?", model.StatusFiltered)
    }
    
    var total int64
    query.Count(&total)
    
    var articles []model.Article
    query.Order("pub_date DESC").
        Offset((page - 1) * pageSize).
        Limit(pageSize).
        Find(&articles)
    
    c.JSON(http.StatusOK, gin.H{
        "data":  articles,
        "total": total,
        "page":  page,
    })
}

func (h *Handler) ProcessArticles(c *gin.Context) {
    go h.processor.ProcessPendingArticles(c.Request.Context(), 10)
    c.JSON(http.StatusOK, gin.H{"message": "processing started"})
}

// ===== Config相关 =====

func (h *Handler) GetConfig(c *gin.Context) {
    var configs []model.Config
    h.db.Find(&configs)
    
    result := make(map[string]string)
    for _, cfg := range configs {
        result[cfg.Key] = cfg.Value
    }
    
    c.JSON(http.StatusOK, result)
}

func (h *Handler) SaveConfig(c *gin.Context) {
    var input map[string]string
    if err := c.ShouldBindJSON(&input); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    for key, value := range input {
        h.db.Where("key = ?", key).Assign(model.Config{Value: value}).FirstOrCreate(&model.Config{Key: key})
    }
    
    c.JSON(http.StatusOK, gin.H{"message": "saved"})
}

// ===== 页面 =====

func (h *Handler) IndexPage(c *gin.Context) {
    c.Redirect(http.StatusFound, "/articles")
}

func (h *Handler) FeedsPage(c *gin.Context) {
    var feeds []model.Feed
    h.db.Find(&feeds)
    c.HTML(http.StatusOK, "feeds.html", gin.H{"feeds": feeds})
}

func (h *Handler) ArticlesPage(c *gin.Context) {
    status := c.DefaultQuery("status", "processed")
    c.HTML(http.StatusOK, "articles.html", gin.H{"status": status})
}

func (h *Handler) SettingsPage(c *gin.Context) {
    var configs []model.Config
    h.db.Find(&configs)
    
    configMap := make(map[string]string)
    for _, cfg := range configs {
        configMap[cfg.Key] = cfg.Value
    }
    
    c.HTML(http.StatusOK, "settings.html", gin.H{"config": configMap})
}
```

### 6. 定时任务 (internal/scheduler/cron.go)

```go
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
```

### 7. 主程序入口 (main.go)

```go
package main

import (
    "html/template"
    "log"
    
    "github.com/gin-gonic/gin"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    
    "go-news/internal/handler"
    "go-news/internal/model"
    "go-news/internal/scheduler"
    "go-news/internal/service"
)

func main() {
    // 初始化数据库
    db, err := gorm.Open(sqlite.Open("data/news.db"), &gorm.Config{})
    if err != nil {
        log.Fatal("Failed to connect database:", err)
    }
    
    // 自动迁移
    db.AutoMigrate(&model.Feed{}, &model.Article{}, &model.Config{})
    
    // 初始化默认配置
    initDefaultConfig(db)
    
    // 初始化服务
    llmSvc := service.NewLLMService(db)
    feedSvc := service.NewFeedService(db)
    processorSvc := service.NewProcessorService(db, llmSvc)
    
    // 启动定时任务
    sched := scheduler.NewScheduler(feedSvc, processorSvc)
    sched.Start()
    defer sched.Stop()
    
    // 初始化Gin
    r := gin.Default()
    
    // 加载模板
    r.SetHTMLTemplate(template.Must(template.ParseGlob("web/templates/*.html")))
    r.Static("/static", "web/static")
    
    // 注册路由
    h := handler.NewHandler(db)
    h.RegisterRoutes(r)
    
    // 启动服务
    log.Println("Server starting on :8080")
    r.Run(":8080")
}

func initDefaultConfig(db *gorm.DB) {
    defaults := map[string]string{
        model.ConfigLLMProvider: "openai",
        model.ConfigLLMApiURL:   "https://api.openai.com/v1",
        model.ConfigLLMModel:    "gpt-4o-mini",
        model.ConfigPromptFilter: `你是一个新闻筛选助手。请判断以下文章是否值得阅读。
返回JSON格式：{"worth": true/false, "reason": "简短说明原因"}
只有重要的科技新闻、行业动态才值得阅读，广告、招聘信息、无意义内容不值得。`,
        model.ConfigPromptSummary: `请用中文总结以下文章的核心内容，要求：
1. 控制在200字以内
2. 突出关键信息
3. 语言简洁易懂`,
    }
    
    for key, value := range defaults {
        db.Where("key = ?", key).FirstOrCreate(&model.Config{Key: key, Value: value})
    }
}
```

---

## 前端模板

### layout.html
```html
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>go-news</title>
    <link rel="stylesheet" href="/static/style.css">
</head>
<body>
    <nav>
        <a href="/articles?status=processed">📰 文章</a>
        <a href="/feeds">📡 订阅源</a>
        <a href="/settings">⚙️ 设置</a>
    </nav>
    <main>
        {{template "content" .}}
    </main>
    <script src="/static/app.js"></script>
</body>
</html>
```

### articles.html
```html
{{define "content"}}
<div class="articles-page">
    <div class="tabs">
        <a href="?status=processed" class="{{if eq .status "processed"}}active{{end}}">已处理</a>
        <a href="?status=pending" class="{{if eq .status "pending"}}active{{end}}">待处理</a>
        <a href="?status=filtered" class="{{if eq .status "filtered"}}active{{end}}">已过滤</a>
    </div>
    
    <div class="actions">
        <button onclick="processArticles()">🤖 处理文章</button>
    </div>
    
    <div id="articles-list"></div>
</div>

<script>
const status = "{{.status}}";

async function loadArticles(page = 1) {
    const resp = await fetch(`/api/articles?status=${status}&page=${page}`);
    const data = await resp.json();
    
    const html = data.data.map(a => `
        <div class="article-card">
            <h3><a href="${a.link}" target="_blank">${a.title}</a></h3>
            <div class="meta">${a.feed?.name || ''} · ${new Date(a.pub_date).toLocaleDateString()}</div>
            ${a.summary ? `<p class="summary">${a.summary}</p>` : ''}
        </div>
    `).join('');
    
    document.getElementById('articles-list').innerHTML = html;
}

async function processArticles() {
    await fetch('/api/articles/process', {method: 'POST'});
    alert('开始处理，请稍后刷新页面');
}

loadArticles();
</script>
{{end}}
```

### feeds.html
```html
{{define "content"}}
<div class="feeds-page">
    <h2>订阅源管理</h2>
    
    <form id="add-feed-form" onsubmit="addFeed(event)">
        <input type="text" name="name" placeholder="名称" required>
        <input type="url" name="url" placeholder="RSS URL" required>
        <button type="submit">添加</button>
    </form>
    
    <div class="feeds-list">
        {{range .feeds}}
        <div class="feed-item" data-id="{{.ID}}">
            <span class="name">{{.Name}}</span>
            <span class="url">{{.URL}}</span>
            <button onclick="fetchFeed({{.ID}})">抓取</button>
            <button onclick="deleteFeed({{.ID}})">删除</button>
        </div>
        {{end}}
    </div>
</div>

<script>
async function addFeed(e) {
    e.preventDefault();
    const form = e.target;
    const data = {
        name: form.name.value,
        url: form.url.value
    };
    
    await fetch('/api/feeds', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(data)
    });
    
    location.reload();
}

async function fetchFeed(id) {
    const resp = await fetch(`/api/feeds/${id}/fetch`, {method: 'POST'});
    const data = await resp.json();
    alert(`抓取完成，新增 ${data.new_articles} 篇文章`);
}

async function deleteFeed(id) {
    if (!confirm('确定删除？')) return;
    await fetch(`/api/feeds/${id}`, {method: 'DELETE'});
    location.reload();
}
</script>
{{end}}
```

### settings.html
```html
{{define "content"}}
<div class="settings-page">
    <h2>系统设置</h2>
    
    <form id="settings-form" onsubmit="saveSettings(event)">
        <fieldset>
            <legend>LLM配置</legend>
            <label>
                提供商
                <select name="llm_provider">
                    <option value="openai" {{if eq .config.llm_provider "openai"}}selected{{end}}>OpenAI</option>
                    <option value="ollama" {{if eq .config.llm_provider "ollama"}}selected{{end}}>Ollama</option>
                </select>
            </label>
            <label>
                API地址
                <input type="url" name="llm_api_url" value="{{.config.llm_api_url}}">
            </label>
            <label>
                API密钥
                <input type="password" name="llm_api_key" value="{{.config.llm_api_key}}">
            </label>
            <label>
                模型
                <input type="text" name="llm_model" value="{{.config.llm_model}}">
            </label>
        </fieldset>
        
        <fieldset>
            <legend>提示词</legend>
            <label>
                筛选提示词
                <textarea name="prompt_filter" rows="5">{{.config.prompt_filter}}</textarea>
            </label>
            <label>
                摘要提示词
                <textarea name="prompt_summary" rows="5">{{.config.prompt_summary}}</textarea>
            </label>
        </fieldset>
        
        <button type="submit">保存设置</button>
    </form>
</div>

<script>
async function saveSettings(e) {
    e.preventDefault();
    const form = e.target;
    const data = {};
    
    new FormData(form).forEach((value, key) => {
        data[key] = value;
    });
    
    await fetch('/api/config', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(data)
    });
    
    alert('保存成功');
}
</script>
{{end}}
```

---

## API接口文档

| 方法 | 路径 | 说明 | 参数 |
|------|------|------|------|
| GET | /api/feeds | 获取所有订阅源 | - |
| POST | /api/feeds | 添加订阅源 | {name, url} |
| DELETE | /api/feeds/:id | 删除订阅源 | - |
| POST | /api/feeds/:id/fetch | 手动抓取 | - |
| GET | /api/articles | 获取文章列表 | status, page |
| POST | /api/articles/process | 处理待处理文章 | - |
| GET | /api/config | 获取配置 | - |
| POST | /api/config | 保存配置 | {key: value} |

---

## 部署说明

### 编译
```bash
go build -o go-news main.go
```

### 运行
```bash
mkdir -p data
./go-news
```

### Docker
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o go-news main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/go-news .
COPY web/ web/
EXPOSE 8080
CMD ["./go-news"]
```

---

## 开发计划

- [x] Phase 1: RSS抓取 + 存储
- [x] Phase 2: LLM对接 + 文章处理
- [x] Phase 3: 后台管理界面
- [ ] Phase 4: 支持更多LLM（Ollama本地模型）
- [ ] Phase 5: 文章导出/推送功能