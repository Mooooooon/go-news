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
返回JSON格式:{"worth": true/false, "reason": "简短说明原因"}
只有重要的科技新闻、行业动态才值得阅读,广告、招聘信息、无意义内容不值得。`,
		model.ConfigPromptSummary: `请用中文总结以下文章的核心内容,要求:
1. 控制在200字以内
2. 突出关键信息
3. 语言简洁易懂`,
	}

	for key, value := range defaults {
		db.Where("key = ?", key).FirstOrCreate(&model.Config{Key: key, Value: value})
	}
}
