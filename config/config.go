package config

import (
	"log"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Cron     CronConfig     `yaml:"cron"`
}

type ServerConfig struct {
	Port string `yaml:"port"`
	Mode string `yaml:"mode"` // debug, release
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type CronConfig struct {
	FetchInterval   string `yaml:"fetch_interval"`   // RSS抓取间隔
	ProcessInterval string `yaml:"process_interval"` // 文章处理间隔
}

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	// 默认配置
	cfg := &Config{
		Server: ServerConfig{
			Port: "3000",
			Mode: "debug",
		},
		Database: DatabaseConfig{
			Path: "data/news.db",
		},
		Cron: CronConfig{
			FetchInterval:   "*/30 * * * *", // 每30分钟
			ProcessInterval: "*/10 * * * *", // 每10分钟
		},
	}

	// 如果配置文件存在,读取配置
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}

		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	} else {
		log.Printf("配置文件不存在: %s, 使用默认配置", configPath)
	}

	// 环境变量覆盖配置
	if port := os.Getenv("PORT"); port != "" {
		cfg.Server.Port = port
	}

	if mode := os.Getenv("GIN_MODE"); mode != "" {
		cfg.Server.Mode = mode
	}

	if dbPath := os.Getenv("DB_PATH"); dbPath != "" {
		cfg.Database.Path = dbPath
	}

	return cfg, nil
}

// GetServerAddress 获取服务器监听地址
func (c *Config) GetServerAddress() string {
	// 如果端口是纯数字,加上冒号前缀
	if _, err := strconv.Atoi(c.Server.Port); err == nil {
		return ":" + c.Server.Port
	}
	return c.Server.Port
}
