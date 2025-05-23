// Quake代理收集插件 - 使用requests API的函数式实现
package main

import (
	"fmt"
	"github.com/overflow0verture/proxy_harvester/internal/logger"
	"github.com/overflow0verture/proxy_harvester/internal/requests"
)

// 插件配置
var config = struct {
	Switch      string
	APIURL      string
	Key         string
	QueryString string
	ResultSize  int
	Periodic    string
}{
	Switch:      "open",
	APIURL:      "https://quake.360.net/api/v3/search/quake_service",
	Key:         "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", // 在这里填入你的Quake API Key
	QueryString: "service:\"socks5\" AND location.country_cn:\"中国\" AND response:\"No authentication\"",
	ResultSize:  200,
	Periodic:    "0 0 * * 1", 
}

// Quake API响应结构
type QuakeAPIResponse struct {
	Code    interface{} `json:"code"`
	Message string      `json:"message"`
	Data    []struct {
		IP   string `json:"ip"`
		Port int    `json:"port"`
	} `json:"data"`
	Meta struct {
		Total      int `json:"total"`
		Count      int `json:"count"`
		CreditLeft int `json:"credit_left"`
	} `json:"meta"`
}

// PluginName 返回插件名称
func PluginName() string {
	return "Quake代理API爬虫(requests版本)"
}

// PluginCronSpec 返回定时任务表达式
func PluginCronSpec() string {
	return config.Periodic
}

// PluginFetchProxies 获取代理
func PluginFetchProxies(out chan<- string) error {
	
	if config.Switch != "open" {
		logger.Plugin("未开启Quake")
		return nil
	}
	
	logger.Plugin("已开启Quake,将根据配置条件从Quake中获取%d条数据", config.ResultSize)
	
	// 构建请求体
	requestBody := map[string]interface{}{
		"query": config.QueryString,
		"start": 0,
		"size":  config.ResultSize,
	}
	
	// 使用requests发送POST请求
	options := requests.RequestOptions{
		JSON: requestBody,
		Headers: map[string]string{
			"X-QuakeToken": config.Key,
			"Content-Type": "application/json",
			"User-Agent":   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		},
		Proxies: false,
		Timeout: 60,
	}
	
	resp, err := requests.Post(config.APIURL, options)
	if err != nil {
		logger.Error("requests请求Quake API失败: %v", err)
		return err
	}
	
	// 检查HTTP状态码
	if resp.StatusCode != 200 {
		logger.Error("Quake API返回错误状态码: %d", resp.StatusCode)
		return fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}
	
	logger.Plugin("Quake API请求成功，状态码: %d，响应大小: %d 字节", 
		resp.StatusCode, len(resp.Content))
	
	// 解析JSON响应
	var quakeResp QuakeAPIResponse
	err = resp.JSON(&quakeResp)
	if err != nil {
		logger.Error("解析Quake API响应JSON失败: %v", err)
		return fmt.Errorf("解析JSON失败: %v", err)
	}
	
	// 检查响应码
	var codeStr string
	switch v := quakeResp.Code.(type) {
	case string:
		codeStr = v
	case float64:
		codeStr = fmt.Sprintf("%.0f", v)
	case int:
		codeStr = fmt.Sprintf("%d", v)
	default:
		logger.Error("无法解析Quake响应码: %v", quakeResp.Code)
		return fmt.Errorf("无法解析响应码: %v", quakeResp.Code)
	}
	
	if codeStr != "0" {
		logger.Error("Quake API错误 [%s]: %s", codeStr, quakeResp.Message)
		return fmt.Errorf("quake error [%s]: %s", codeStr, quakeResp.Message)
	}
	
	logger.Plugin("Quake API响应解析成功，获取到 %d 个结果，剩余积分: %d", 
		len(quakeResp.Data), quakeResp.Meta.CreditLeft)
	
	// 处理结果
	count := 0
	for _, item := range quakeResp.Data {
		proxyAddr := fmt.Sprintf("socks5://%s:%d", item.IP, item.Port)
		out <- proxyAddr
		count++
		logger.Debug("添加代理: %s", proxyAddr)
	}
	
	logger.Success("Quake数据已取，共获取 %d 个代理", count)
	return nil
}

// 导出插件变量
var Plugin = map[string]interface{}{
	"Name":         PluginName,
	"CronSpec":     PluginCronSpec,
	"FetchProxies": PluginFetchProxies,
} 