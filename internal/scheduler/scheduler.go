package scheduler

import (
	"github.com/overflow0verture/proxy_harvester/internal/check"
	"github.com/overflow0verture/proxy_harvester/internal/config"
	"github.com/overflow0verture/proxy_harvester/internal/pool"
	"github.com/overflow0verture/proxy_harvester/internal/logger"
	"github.com/robfig/cron/v3"
	"strings"
)

// Start 启动所有定时任务
func Start(cfg config.Config, proxyStore pool.ProxyStore) {
	cronJob := cron.New()
	cronFlag := false

	if periodicChecking := strings.TrimSpace(cfg.Task.PeriodicChecking); periodicChecking != "" {
		cronFlag = true
		cronJob.AddFunc(periodicChecking, func() {
			logger.Info("\n代理存活自检 开始\n\n")
			allProxies, _ := proxyStore.GetAll()
			check.CheckSocks(cfg.CheckSocks, allProxies, proxyStore)
			logger.Info("\n代理存活自检 结束\n\n")
		})
	}

	if cronFlag {
		cronJob.Start()
	}
}

// 启动所有 Provider 的定时任务
//func StartProvidersWithCron(providers []provider.ProxyProvider) {
//	cronJob := cron.New()
//	out := make(chan string, 1000)
//
//	for _, p := range providers {
//		spec := p.CronSpec()
//		if spec != "" {
//			// 闭包捕获 p，防止循环变量问题
//			cronJob.AddFunc(spec, func(p provider.ProxyProvider) func() {
//				return func() {
//					go p.FetchProxies(out)
//				}
//			}(p))
//		}
//	}
//
//	// 消费 out 通道，写入待检测通道
//	go func() {
//		for proxy := range out {
//			globals.ToCheckChan <- proxy
//		}
//	}()
//
//	cronJob.Start()
//}
