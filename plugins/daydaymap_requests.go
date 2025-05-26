// DayDayMap代理收集插件 - 使用requests API的函数式实现
package main

import (
	"encoding/base64"
	"fmt"
	"github.com/overflow0verture/proxy_harvester/internal/logger"
	"github.com/overflow0verture/proxy_harvester/internal/requests"
)

// 插件配置
var config = struct {
	Switch      string
	APIURL      string
	APIKey      string
	QueryBase64 string
	ResultSize  int
	Periodic    string
}{
	Switch:      "open",
	APIURL:      "https://www.daydaymap.com/api/v1/raymap/search/all",
	APIKey:      "xxxxxxxxxxxxxxxxxxxxx", // 在这里填入你的DayDayMap API Key
	QueryBase64: "c2VydmljZT0iU09DS1M1IiAmJiBiYW5uZXI9Ilx4MDVceDAwIiAmJiBpcC5jb3VudHJ5PSJDTiI=", // 搜索中国SOCKS5代理的base64编码
	ResultSize:  50,
	Periodic:    "0 0 * * 1", // 每周一凌晨执行
}

// DayDayMap API响应结构
type DayDayMapAPIResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		List []struct {
			ASN         string   `json:"asn"`
			ASNOrg      string   `json:"asn_org"`
			Banner      string   `json:"banner"`
			Cert        *string  `json:"cert"`
			City        string   `json:"city"`
			Country     string   `json:"country"`
			Device      *string  `json:"device"`
			DeviceType  *string  `json:"device_type"`
			Domain      *string  `json:"domain"`
			Header      string   `json:"header"`
			IP          string   `json:"ip"`
			IsIPv6      bool     `json:"is_ipv6"`
			IsWebsite   bool     `json:"is_website"`
			ISP         string   `json:"isp"`
			Lang        *string  `json:"lang"`
			OS          *string  `json:"os"`
			Port        int      `json:"port"`
			Product     []string `json:"product"`
			Protocol    string   `json:"protocol"`
			Province    string   `json:"province"`
			Server      *string  `json:"server"`
			Service     string   `json:"service"`
			Tags        []string `json:"tags"`
			TimeStamp   string   `json:"time_stamp"`
			Title       *string  `json:"title"`
		} `json:"list"`
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
		Total    int `json:"total"`
		UseTime  string `json:"use_time"`
	} `json:"data"`
}

// PluginName 返回插件名称
func PluginName() string {
	return "DayDayMap代理API爬虫(requests版本)"
}

// PluginCronSpec 返回定时任务表达式
func PluginCronSpec() string {
	return config.Periodic
}

// PluginFetchProxies 获取代理
func PluginFetchProxies(out chan<- string) error {
	
	if config.Switch != "open" {
		logger.Plugin("未开启DayDayMap")
		return nil
	}
	
	logger.Plugin("已开启DayDayMap,将根据配置条件从DayDayMap中获取%d条数据", config.ResultSize)
	
	// 解码查询字符串以记录日志
	queryBytes, err := base64.StdEncoding.DecodeString(config.QueryBase64)
	if err != nil {
		logger.Error("解码查询字符串失败: %v", err)
	} else {
		logger.Plugin("搜索条件: %s", string(queryBytes))
	}
	
	// 构建请求体
	requestData := map[string]interface{}{
		"page":      1,
		"page_size": config.ResultSize,
		"keyword":   config.QueryBase64,
	}
	
	// 使用requests发送POST请求
	options := requests.RequestOptions{
		JSON: requestData,
		Headers: map[string]string{
			"api-key":      config.APIKey,
			"Content-Type": "application/json",
			"User-Agent":   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		},
		Proxies: false, // 不使用代理发送API请求
		Timeout: 30,
	}
	
	resp, err := requests.Post(config.APIURL, options)
	if err != nil {
		logger.Error("requests请求DayDayMap API失败: %v", err)
		return err
	}
	
	// 检查HTTP状态码
	if resp.StatusCode != 200 {
		logger.Error("DayDayMap API返回错误状态码: %d", resp.StatusCode)
		return fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}
	
	logger.Plugin("DayDayMap API请求成功，状态码: %d，响应大小: %d 字节", 
		resp.StatusCode, len(resp.Content))
	
	// 解析JSON响应
	var dayDayMapResp DayDayMapAPIResponse
	err = resp.JSON(&dayDayMapResp)
	if err != nil {
		logger.Error("解析DayDayMap API响应JSON失败: %v", err)
		return fmt.Errorf("解析JSON失败: %v", err)
	}
	
	// 检查API响应状态
	if dayDayMapResp.Code != 200 {
		logger.Error("DayDayMap API返回错误代码: %d, 消息: %s", dayDayMapResp.Code, dayDayMapResp.Msg)
		return fmt.Errorf("API错误代码: %d, 消息: %s", dayDayMapResp.Code, dayDayMapResp.Msg)
	}
	
	logger.Plugin("DayDayMap API响应解析成功，获取到 %d 个结果，总共 %d 个可用代理", 
		len(dayDayMapResp.Data.List), dayDayMapResp.Data.Total)
	
	// 处理结果
	count := 0
	for _, result := range dayDayMapResp.Data.List {
		// 只处理SOCKS5代理
		if result.Service == "socks5" {
			proxyAddr := fmt.Sprintf("socks5://%s:%d", result.IP, result.Port)
			out <- proxyAddr
			count++
			logger.Debug("添加代理: %s (ISP: %s, 位置: %s %s)", 
				proxyAddr, result.ISP, result.Province, result.City)
		}
	}
	
	logger.Plugin("DayDayMap数据已取，共获取 %d 个SOCKS5代理", count)
	return nil
}

// 导出插件变量
var Plugin = map[string]interface{}{
	"Name":         PluginName,
	"CronSpec":     PluginCronSpec,
	"FetchProxies": PluginFetchProxies,
} 