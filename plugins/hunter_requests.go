// Hunter代理收集插件 - 使用requests API的函数式实现
package main

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"time"
	"github.com/overflow0verture/proxy_harvester/internal/logger"
	"github.com/overflow0verture/proxy_harvester/internal/requests"
)

// 插件配置
var config = struct {
	Switch      string
	APIURL      string
	APIKey      string
	QueryString string
	ResultSize  int
	Periodic    string
}{
	Switch:      "open",
	APIURL:      "https://hunter.qianxin.com/openApi/search",
	APIKey:      "xxxxxxxxxxxxxxxxxxxxx", // 在这里填入你的Hunter API Key
	QueryString: `protocol=="socks5"&&protocol.banner="No authentication"&&ip.country="CN"`,
	ResultSize:  100, // 每次获取的结果数量
	Periodic:    "0 0 0/2 * *", // 每2小时执行一次
}

// Hunter API响应结构
type HunterAPIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Total        int    `json:"total"`
		Time         int    `json:"time"`
		ConsumeQuota string `json:"consume_quota"`
		RestQuota    string `json:"rest_quota"`
		Arr          []struct {
			WebTitle        string `json:"web_title"`
			IP              string `json:"ip"`
			Port            int    `json:"port"`
			BaseProtocol    string `json:"base_protocol"`
			Protocol        string `json:"protocol"`
			Domain          string `json:"domain"`
			Component       []struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"component"`
			URL             string `json:"url"`
			Os              string `json:"os"`
			Country         string `json:"country"`
			Province        string `json:"province"`
			City            string `json:"city"`
			UpdatedAt       string `json:"updated_at"`
			StatusCode      int    `json:"status_code"`
			Number          string `json:"number"`
			Company         string `json:"company"`
			IsWeb           string `json:"is_web"`
			IsRisk          string `json:"is_risk"`
			IsRiskProtocol  string `json:"is_risk_protocol"`
			AsOrg           string `json:"as_org"`
			Isp             string `json:"isp"`
			Banner          string `json:"banner"`
			Header          string `json:"header"`
		} `json:"arr"`
	} `json:"data"`
}

// PluginName 返回插件名称
func PluginName() string {
	return "Hunter代理API爬虫(requests版本)"
}

// PluginCronSpec 返回定时任务表达式
func PluginCronSpec() string {
	return config.Periodic
}

// PluginFetchProxies 获取代理
func PluginFetchProxies(out chan<- string) error {
	
	if config.Switch != "open" {
		logger.Plugin("未开启Hunter")
		return nil
	}
	
	logger.Plugin("已开启Hunter,将根据配置条件从Hunter中获取%d条数据", config.ResultSize)
	logger.Plugin("搜索条件: %s", config.QueryString)
	
	// Base64编码搜索条件
	searchBase64 := base64.URLEncoding.EncodeToString([]byte(config.QueryString))
	
	var totalProcessed int // 记录处理了几条数据
	pageSize := 100        // 每页100条
	totalPages := (config.ResultSize + pageSize - 1) / pageSize // 计算总页数
	
	logger.Plugin("Hunter查询计划: 每页%d条，共查询%d页", pageSize, totalPages)
	
	for page := 1; page <= totalPages; page++ {
		logger.Plugin("Hunter: 正在查询第%d页", page)
		
		// 构建请求参数
		params := map[string]string{
			"api-key":   config.APIKey,
			"search":    searchBase64,
			"page":      strconv.Itoa(page),
			"page_size": strconv.Itoa(pageSize),
		}
		
		// 使用requests发送GET请求
		options := requests.RequestOptions{
			Params: params,
			Headers: map[string]string{
				"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			},
			Proxies: false, // 不使用代理发送API请求
			Timeout: 60,
		}
		
		resp, err := requests.Get(config.APIURL, options)
		if err != nil {
			logger.Error("requests请求Hunter API失败: %v", err)
			return err
		}
		
		// 检查HTTP状态码
		if resp.StatusCode != 200 {
			logger.Error("Hunter API返回错误状态码: %d", resp.StatusCode)
			return fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
		}
		
		logger.Plugin("Hunter API请求成功，状态码: %d，响应大小: %d 字节", 
			resp.StatusCode, len(resp.Content))
		
		// 解析JSON响应
		var hunterResp HunterAPIResponse
		err = resp.JSON(&hunterResp)
		if err != nil {
			logger.Error("解析Hunter API响应JSON失败: %v", err)
			return fmt.Errorf("解析JSON失败: %v", err)
		}
		
		// 检查API响应状态
		if hunterResp.Code != 200 {
			logger.Error("Hunter API返回错误代码: %d, 消息: %s", hunterResp.Code, hunterResp.Message)
			return fmt.Errorf("API错误代码: %d, 消息: %s", hunterResp.Code, hunterResp.Message)
		}
		
		// 检查是否有数据
		if hunterResp.Data.Total == 0 {
			logger.Plugin("Hunter: 根据配置语法，未取到数据")
			break
		}
		
		logger.Plugin("Hunter第%d页响应解析成功，本页获取到 %d 个结果，总共 %d 个可用代理", 
			page, len(hunterResp.Data.Arr), hunterResp.Data.Total)
		logger.Plugin("Hunter配额信息: %s, %s", hunterResp.Data.ConsumeQuota, hunterResp.Data.RestQuota)
		
		// 处理当前页结果
		pageCount := 0
		for _, result := range hunterResp.Data.Arr {
			// 只处理SOCKS5代理
			if result.Protocol == "socks5" {
				proxyAddr := fmt.Sprintf("socks5://%s:%d", result.IP, result.Port)
				out <- proxyAddr
				pageCount++
				totalProcessed++
				logger.Debug("添加代理: %s (ISP: %s, 位置: %s %s %s)", 
					proxyAddr, result.Isp, result.Country, result.Province, result.City)
			}
		}
		
		logger.Plugin("Hunter第%d页处理完成，获取到 %d 个SOCKS5代理", page, pageCount)
		
		// 检查是否已处理完所有数据
		if totalProcessed >= hunterResp.Data.Total {
			logger.Plugin("Hunter已处理完所有可用数据")
			break
		}
		
		// 检查是否达到设定的获取数量
		if totalProcessed >= config.ResultSize {
			logger.Plugin("Hunter已达到设定的获取数量: %d", config.ResultSize)
			break
		}
		
		// 防止访问过快，添加延时（除了最后一页）
		if page < totalPages && page > 1 {
			logger.Plugin("Hunter延时3秒，防止访问过快...")
			time.Sleep(3 * time.Second)
		}
	}
	
	logger.Plugin("Hunter数据已取，共获取 %d 个SOCKS5代理", totalProcessed)
	return nil
}

// 导出插件变量
var Plugin = map[string]interface{}{
	"Name":         PluginName,
	"CronSpec":     PluginCronSpec,
	"FetchProxies": PluginFetchProxies,
} 