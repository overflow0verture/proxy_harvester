package main

import (
	"fmt"
	"github.com/overflow0verture/proxy_harvester/internal/apiserver"
	"github.com/overflow0verture/proxy_harvester/internal/check"
	"github.com/overflow0verture/proxy_harvester/internal/config"
	"github.com/overflow0verture/proxy_harvester/internal/globals"
	"github.com/overflow0verture/proxy_harvester/internal/logger"
	"github.com/overflow0verture/proxy_harvester/internal/plugin"
	"github.com/overflow0verture/proxy_harvester/internal/pool"
	"github.com/overflow0verture/proxy_harvester/internal/server"
	"os"
	"strings"
	"time"
)

func main() {
	// 1. 加载配置
	cfg, err := config.LoadConfig("configs/config.toml")
	if err != nil {
		fmt.Println("配置加载失败:", err)
		os.Exit(1)
	}

	// 2. 初始化日志系统
	if err := logger.Setup(cfg.Log.Enabled, cfg.Log.LogDir); err != nil {
		fmt.Printf("初始化日志系统失败: %v\n", err)
		os.Exit(1)
	}
	// 设置IP汇总间隔
	if cfg.Log.IPSummaryInterval > 0 {
		logger.IPSummaryInterval = time.Duration(cfg.Log.IPSummaryInterval) * time.Minute
	}

	// 3. 初始化全局通道
	globals.InitFetchChannel(1000)

	// 4. 初始化代理池
	proxyStore := pool.InitProxyStore(cfg.Storage, 10) // 10为速率，可根据配置调整

	// 5. 启动检测worker
	check.StartCheckWorkers(cfg.CheckSocks.MaxConcurrentReq, cfg.CheckSocks, proxyStore)

	// 6. 启动 socks5 监听服务
	go socks5server.StartServer(proxyStore, cfg.Listener, cfg.CheckSocks.Timeout)

	// 7. 启动API服务器（如果启用）
	if strings.ToLower(cfg.APIServer.Switch) == "open" {
		apiServer := apiserver.NewAPIServer(proxyStore, cfg.APIServer.Token, cfg.APIServer.Port)
		go func() {
			if err := apiServer.Start(); err != nil {
				logger.Error("API服务器启动失败: %v", err)
			}
		}()
	}

	// 8. 初始化插件系统
	plugin.InitPluginSystem()

	// 9. 创建插件管理器并设置为全局管理器
	pluginManager := plugin.NewPluginManager(proxyStore)
	plugin.SetGlobalPluginManager(pluginManager)

	// 10. 启动插件监控
	plugin.WatchPluginFolder(cfg.Plugin, proxyStore)

	// 11. 定期汇总代理池状态
	go func() {
		for {
			time.Sleep(logger.IPSummaryInterval / 2) // 使用一半的汇总间隔检查
			count, _ := proxyStore.Len()
			logger.IPSummary(count, false)
		}
	}()

	// 12. 业务主循环/监听/定时任务等
	logger.Info("proxy harvester 启动完成")

	// 程序退出前关闭日志
	defer logger.Close()

	select {} // 阻塞主线程
}
