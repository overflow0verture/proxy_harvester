// config.go
// 配置加载与类型定义
package config

import (
	toml "github.com/pelletier/go-toml/v2"
	"os"
)

// BruteForceConfig 暴力破解配置
type BruteForceConfig struct {
	Enable      bool   `toml:"enable"`
	Credentials string `toml:"credentials"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type          string `toml:"type"`
	RedisHost     string `toml:"redis_host"`
	RedisPort     int    `toml:"redis_port"`
	RedisPassword string `toml:"redis_password"`
	FileName      string `toml:"file_name"`
}

// ListenerConfig 本地监听配置
type ListenerConfig struct {
	IP       string `toml:"IP"`
	Port     int    `toml:"PORT"`
	UserName string `toml:"userName"`
	Password string `toml:"password"`
}

// TaskConfig 定时任务配置
type TaskConfig struct {
	PeriodicChecking string `toml:"periodicChecking"`
}

// LogConfig 日志配置
type LogConfig struct {
	Enabled bool   `toml:"enabled"`
	LogDir  string `toml:"log_dir"`
	// 代理IP汇总间隔（分钟）
	IPSummaryInterval int `toml:"ip_summary_interval"`
}

// CheckGeolocateConfig 地理位置检测配置
type CheckGeolocateConfig struct {
	Switch          string   `toml:"switch"`
	CheckURL        string   `toml:"checkURL"`
	ExcludeKeywords []string `toml:"excludeKeywords"`
	IncludeKeywords []string `toml:"includeKeywords"`
}

// CheckSocksConfig 代理检测配置
type CheckSocksConfig struct {
	CheckURL         string               `toml:"checkURL"`
	CheckRspKeywords string               `toml:"checkRspKeywords"`
	MaxConcurrentReq int                  `toml:"maxConcurrentReq"`
	Timeout          int                  `toml:"timeout"`
	CheckGeolocate   CheckGeolocateConfig `toml:"checkGeolocate"`
}

// PluginConfig 插件相关配置
// PluginFolder 插件目录
// toml: plugin_folder
// 示例: plugin_folder = "plugins"
type PluginConfig struct {
	PluginFolder string `toml:"plugin_folder"`
}

// APIServerConfig API服务器配置
type APIServerConfig struct {
	Switch string `toml:"switch"`
	Token  string `toml:"token"`
	Port   int    `toml:"port"`
}

// Config 是全局配置结构体
type Config struct {
	Listener   ListenerConfig   `toml:"listener"`
	Task       TaskConfig       `toml:"task"`
	CheckSocks CheckSocksConfig `toml:"checkSocks"`
	Storage    StorageConfig    `toml:"storage"`
	Plugin     PluginConfig     `toml:"plugin"`
	Log        LogConfig        `toml:"log"`
	APIServer  APIServerConfig  `toml:"apiserver"`
}

// LoadConfig 负责加载 TOML 配置文件
func LoadConfig(path string) (Config, error) {
	var config Config
	// 读取并解析 TOML 文件
	data, err := os.ReadFile(path)
	if err != nil {
		return config, err
	}

	err = toml.Unmarshal(data, &config)

	return config, err
}
