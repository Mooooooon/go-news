# go-news

基于 Go 开发的 RSS 新闻订阅系统，集成 AI 智能处理功能，自动筛选和总结文章内容。

## 功能特性

- 📡 **RSS订阅管理** - 支持添加、删除、抓取多个RSS源
- 🤖 **AI智能处理** - 自动筛选重要文章并生成中文摘要
- 📊 **实时状态监控** - 查看系统运行状态和处理进度
- ⚙️ **灵活配置** - 支持 OpenAI/Ollama 等多种 LLM 提供商
- ⏰ **自动化任务** - 定时抓取RSS和处理文章
- 🔄 **并发处理** - 多线程并发提升处理速度

## 快速开始

### 1. 安装依赖

```bash
go mod download
```

### 2. 配置文件

复制配置示例并修改:

```bash
cp config.example.yaml config.yaml
```

编辑 `config.yaml` 设置端口、数据库路径等参数:

```yaml
server:
  port: "3000"
  mode: "debug"

database:
  path: "data/news.db"

cron:
  fetch_interval: "*/30 * * * *"    # 每30分钟抓取RSS
  process_interval: "*/10 * * * *"  # 每10分钟处理文章
```

### 3. 运行程序

```bash
# 创建数据目录
mkdir -p data

# 编译运行
go build -o go-news
./go-news
```

访问 http://localhost:3000 开始使用。

## 使用说明

### 初次配置

1. 访问 **设置页面** 配置 LLM:
   - 选择提供商 (OpenAI/Ollama)
   - 填写 API 地址和密钥
   - 选择或填写模型名称
   - 测试连接确保配置正确

2. 在 **订阅源页面** 添加 RSS 源:
   ```
   名称: 技术博客
   URL: https://example.com/feed.xml
   ```

3. 点击"抓取"按钮获取文章

4. 在 **文章页面** 点击"处理文章"开始AI处理

### 页面说明

- **📰 文章** - 查看已处理/待处理/已过滤的文章
- **📡 订阅源** - 管理 RSS 订阅源
- **⚙️ 设置** - 配置 LLM 和提示词
- **📊 状态** - 查看系统运行状态和处理进度

## 技术架构

### 技术栈

- **后端框架**: Gin
- **数据库**: SQLite + GORM
- **RSS解析**: gofeed
- **定时任务**: robfig/cron
- **配置管理**: gopkg.in/yaml.v3

### 项目结构

```
go-news/
├── main.go                    # 程序入口
├── config/                    # 配置加载
├── internal/
│   ├── model/                # 数据模型
│   ├── service/              # 业务逻辑
│   │   ├── feed.go          # RSS 抓取
│   │   ├── llm.go           # LLM 调用
│   │   ├── processor.go     # 文章处理
│   │   └── status.go        # 状态统计
│   ├── handler/             # HTTP 处理器
│   └── scheduler/           # 定时任务
└── web/
    ├── templates/           # HTML 模板
    └── static/              # 静态资源
```

### 数据库表

#### feeds - 订阅源
- `id`, `name`, `url`, `enabled`, `created_at`, `updated_at`

#### articles - 文章
- `id`, `feed_id`, `title`, `link`, `content`, `pub_date`
- `status` - 0:待处理 1:已处理 2:已过滤
- `summary` - AI生成的摘要
- `processed_at`, `created_at`

#### configs - 系统配置
- `id`, `key`, `value`, `updated_at`

## 核心功能

### 文章处理流程

```
RSS抓取 → 存储原文 → AI筛选 → 生成摘要 → 展示结果
                     ↓
                 不重要 → 标记过滤
```

### AI处理策略

1. **第一步: 筛选** - 判断文章是否值得阅读
   - LLM分析文章标题和内容
   - 返回 JSON: `{worth: true/false, reason: "..."}`
   - 不重要的文章标记为"已过滤"

2. **第二步: 摘要** - 为重要文章生成摘要
   - 提取核心信息
   - 生成200字以内的中文摘要
   - 保存到数据库

### 并发处理

- 默认3个并发 goroutine 同时处理文章
- 使用信号量控制并发数,避免 API 限流
- 循环处理直到所有待处理文章完成
- 详细的进度日志输出

## 配置选项

### LLM 配置

支持任何 OpenAI 兼容 API:

```yaml
# OpenAI
llm_provider: openai
llm_api_url: https://api.openai.com/v1
llm_api_key: sk-xxx
llm_model: gpt-4o-mini

# Ollama (本地)
llm_provider: ollama
llm_api_url: http://localhost:11434/v1
llm_api_key: ollama
llm_model: qwen2.5:7b
```

### Cron 表达式

```
*/5 * * * *     每5分钟
*/30 * * * *    每30分钟
0 * * * *       每小时
0 */2 * * *     每2小时
0 0 * * *       每天凌晨
```

## API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/feeds` | 获取订阅源列表 |
| POST | `/api/feeds` | 添加订阅源 |
| DELETE | `/api/feeds/:id` | 删除订阅源 |
| POST | `/api/feeds/:id/fetch` | 手动抓取 |
| GET | `/api/articles` | 获取文章列表 |
| POST | `/api/articles/process` | 处理文章 |
| GET | `/api/config` | 获取配置 |
| POST | `/api/config` | 保存配置 |
| GET | `/api/llm/models` | 获取模型列表 |
| POST | `/api/llm/test` | 测试连接 |
| GET | `/api/status` | 获取系统状态 |

## 部署

### Docker

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o go-news

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/go-news .
COPY --from=builder /app/web ./web
COPY --from=builder /app/config.example.yaml ./config.yaml
RUN mkdir -p data
EXPOSE 3000
CMD ["./go-news"]
```

### Systemd 服务

```ini
[Unit]
Description=go-news RSS Service
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/go-news
ExecStart=/opt/go-news/go-news
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

## 常见问题

### 1. 处理速度慢怎么办?

- 调整 `processor.go` 中的 `concurrency` 参数增加并发数
- 缩短定时任务间隔
- 使用更快的 LLM API

### 2. 如何使用本地 Ollama?

1. 安装 Ollama: https://ollama.ai
2. 下载模型: `ollama pull qwen2.5:7b`
3. 在设置中填写:
   - API地址: `http://localhost:11434/v1`
   - 模型: `qwen2.5:7b`

### 3. 端口被占用?

修改 `config.yaml` 中的 `server.port` 配置。

## 开发路线

- [x] RSS 订阅和抓取
- [x] LLM 智能处理
- [x] Web 管理界面
- [x] 状态监控页面
- [x] 并发处理优化
- [x] 配置文件支持
- [ ] 文章导出功能
- [ ] 推送通知 (邮件/Webhook)
- [ ] 多用户支持

## License

MIT

## 贡献

欢迎提交 Issue 和 Pull Request!
