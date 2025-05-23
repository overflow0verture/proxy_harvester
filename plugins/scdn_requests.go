// SCDN代理收集插件 - 使用requests API的函数式实现
package main

import (
	"encoding/json"
	"fmt"
	"github.com/overflow0verture/proxy_harvester/internal/logger"
	"github.com/overflow0verture/proxy_harvester/internal/requests"
)

// 插件配置
var config = struct {
	Switch      string
	APIURL      string
	Protocol    string
	Count       int
	Periodic    string
}{
	Switch:   "open",
	APIURL:   "https://proxy.scdn.io/api/get_proxy.php",
	Protocol: "socks5",
	Count:    5,
	Periodic: "0 0 * * 1", 
}

// SCDN API响应结构
type SCDNAPIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Proxies []string `json:"proxies"`
		Count   int      `json:"count"`
	} `json:"data"`
}

// PluginName 返回插件名称
func PluginName() string {
	return "SCDN代理API爬虫(requests版本)"
}

// PluginCronSpec 返回定时任务表达式
func PluginCronSpec() string {
	return config.Periodic
}

// PluginFetchProxies 获取代理
func PluginFetchProxies(out chan<- string) error {
	logger.Plugin("开始从SCDN API爬取%s代理(使用requests)", config.Protocol)
	
	if config.Switch != "open" {
		logger.Plugin("SCDN插件未开启")
		return nil
	}
	
	// 构建请求URL和参数
	url := config.APIURL
	params := map[string]string{
		"protocol": config.Protocol,
		"count":    fmt.Sprintf("%d", config.Count),
	}
	
	// 使用requests发送GET请求
	options := requests.RequestOptions{
		Params: params,
		Headers: map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		},
		Proxies: true, 
	}
	
	resp, err := requests.Get(url, options)
	if err != nil {
		logger.Error("requests请求SCDN API失败: %v", err)
		return err
	}
	
	// 检查HTTP状态码
	if resp.StatusCode != 200 {
		logger.Error("SCDN API返回错误状态码: %d", resp.StatusCode)
		return fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}
	
	logger.Plugin("SCDN API请求成功，状态码: %d，响应大小: %d 字节", 
		resp.StatusCode, len(resp.Content))
	
	// 解析JSON响应
	var apiResp SCDNAPIResponse
	err = resp.JSON(&apiResp)
	if err != nil {
		logger.Error("解析SCDN API响应JSON失败: %v", err)
		return fmt.Errorf("解析JSON失败: %v", err)
	}
	
	// 检查API响应状态
	if apiResp.Code != 200 {
		logger.Error("SCDN API返回错误: %s (代码: %d)", apiResp.Message, apiResp.Code)
		return fmt.Errorf("API错误: %s", apiResp.Message)
	}
	
	logger.Plugin("SCDN API响应解析成功，获取到 %d 个代理", apiResp.Data.Count)
	
	// 处理代理列表
	count := 0
	for _, proxy := range apiResp.Data.Proxies {
		if proxy != "" {
			formattedProxy := fmt.Sprintf("%s://%s", config.Protocol, proxy)
			out <- formattedProxy
			count++
			logger.Debug("添加代理: %s", formattedProxy)
		}
	}
	
	logger.Plugin("SCDN API完成，成功处理了 %d 个%s代理", count, config.Protocol)
	return nil
}

// 导出插件变量
var Plugin = map[string]interface{}{
	"Name":         PluginName,
	"CronSpec":     PluginCronSpec,
	"FetchProxies": PluginFetchProxies,
} 