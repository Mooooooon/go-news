package handler

import (
	"net/http"
	"strconv"
	"time"

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
	status    *service.StatusService
	scheduler interface {
		GetNextFetchTime() time.Time
		GetNextProcessTime() time.Time
	}
}

func NewHandler(db *gorm.DB) *Handler {
	llm := service.NewLLMService(db)
	return &Handler{
		db:        db,
		feed:      service.NewFeedService(db),
		llm:       llm,
		processor: service.NewProcessorService(db, llm),
		status:    service.NewStatusService(db),
	}
}

// SetScheduler 设置调度器引用
func (h *Handler) SetScheduler(scheduler interface {
	GetNextFetchTime() time.Time
	GetNextProcessTime() time.Time
}) {
	h.scheduler = scheduler
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// 页面
	r.GET("/", h.IndexPage)
	r.GET("/feeds", h.FeedsPage)
	r.GET("/articles", h.ArticlesPage)
	r.GET("/settings", h.SettingsPage)
	r.GET("/status", h.StatusPage)

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

		// LLM
		api.GET("/llm/models", h.GetLLMModels)
		api.POST("/llm/test", h.TestLLMConnection)

		// Status
		api.GET("/status", h.GetStatus)
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

// ===== LLM相关 =====

func (h *Handler) GetLLMModels(c *gin.Context) {
	models, err := h.llm.GetModels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"models": models})
}

func (h *Handler) TestLLMConnection(c *gin.Context) {
	response, err := h.llm.TestConnection(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "连接成功",
		"response": response,
	})
}

// ===== Status相关 =====

func (h *Handler) StatusPage(c *gin.Context) {
	c.HTML(http.StatusOK, "status.html", nil)
}

func (h *Handler) GetStatus(c *gin.Context) {
	status, err := h.status.GetSystemStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 添加定时任务信息
	if h.scheduler != nil {
		status.NextFetchTime = h.scheduler.GetNextFetchTime()
		status.NextProcessTime = h.scheduler.GetNextProcessTime()
	}

	c.JSON(http.StatusOK, status)
}
