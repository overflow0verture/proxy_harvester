// IP测试插件 - 验证代理是否工作
package main

import (
	"fmt"
	"strings"
	"github.com/overflow0verture/proxy_harvester/internal/logger"
	"github.com/overflow0verture/proxy_harvester/internal/requests"
)

// 插件配置
var config = struct {
	Switch   string
	TestURL  string
	Periodic string
}{
	Switch:   "open",
	TestURL:  "https://icanhazip.com/",
	Periodic: "0 0 31 2 * *", 
}

// PluginName 返回插件名称
func PluginName() string {
	return "IP测试插件(验证代理工作)"
}

// PluginCronSpec 返回定时任务表达式
func PluginCronSpec() string {
	return config.Periodic
}

// PluginFetchProxies 测试代理并打印IP
func PluginFetchProxies(out chan<- string) error {
	logger.Plugin("开始测试代理IP - 访问 %s", config.TestURL)
	
	if config.Switch != "open" {
		logger.Plugin("IP测试插件未开启")
		return nil
	}
	
	// 测试不使用代理的情况
	logger.Plugin("测试直连（不使用代理）")
	respDirect, err := requests.Get(config.TestURL, requests.RequestOptions{
		Proxies: false, // 不使用代理
		Headers: map[string]string{
			"User-Agent": "ProxyTester/1.0",
		},
		Timeout: 10,
	})
	
	if err != nil {
		logger.Error("直连请求失败: %v", err)
	} else {
		directIP := strings.TrimSpace(respDirect.Text)
		logger.Plugin("直连IP: %s", directIP)
	}
	
	// 测试使用代理的情况
	logger.Plugin("测试代理连接")
	respProxy, err := requests.Get(config.TestURL, requests.RequestOptions{
		Proxies: true, // 使用代理池
		Headers: map[string]string{
			"User-Agent": "ProxyTester/1.0",
		},
		Timeout: 10,
	})
	
	if err != nil {
		logger.Error("代理请求失败: %v", err)
		logger.Plugin("代理可能不可用或代理池为空")
	} else {
		proxyIP := strings.TrimSpace(respProxy.Text)
		logger.Plugin("代理IP: %s", proxyIP)
		
		// 比较IP是否不同
		if respDirect != nil {
			directIP := strings.TrimSpace(respDirect.Text)
			if proxyIP != directIP {
				logger.Plugin("代理工作正常！IP已改变: %s -> %s", directIP, proxyIP)
			} else {
				logger.Plugin("代理可能未生效，IP相同: %s", proxyIP)
			}
		}
	}
	
	// 多次测试，查看代理轮换
	logger.Plugin("测试代理轮换（连续3次请求）")
	for i := 1; i <= 3; i++ {
		resp, err := requests.Get(config.TestURL, requests.RequestOptions{
			Proxies: true,
			Timeout: 8,
		})
		
		if err != nil {
			logger.Plugin("第%d次代理请求失败: %v", i, err)
		} else {
			ip := strings.TrimSpace(resp.Text)
			logger.Plugin("第%d次代理IP: %s", i, ip)
		}
	}
	
	// 不添加任何代理到池中，这只是测试插件
	logger.Plugin("IP测试完成")
	return nil
}

// 导出插件变量
var Plugin = map[string]interface{}{
	"Name":         PluginName,
	"CronSpec":     PluginCronSpec,
	"FetchProxies": PluginFetchProxies,
}