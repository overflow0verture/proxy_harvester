	// Shodan代理收集插件 - 使用requests API的函数式实现
package main

import (
	"fmt"
	"strings"
	"github.com/overflow0verture/proxy_harvester/internal/logger"
	"github.com/overflow0verture/proxy_harvester/internal/requests"
)

// 插件配置
var shodanConfig = struct {
	Switch      string
	APIURL      string
	APIKey      string
	QueryString string
	Periodic    string
}{
	Switch:      "open",
	APIURL:      "https://api.shodan.io/shodan/host/search",
	APIKey:      "xxxxxxxxxxxxxxxxxxx", // 你的Shodan API Key
	QueryString: "hash:1121074672 country:CN", 
	Periodic:    "0 0 */2 * *", // 
}

// Shodan API响应结构
type ShodanAPIResponse struct {
	Matches []struct {
		Product   string `json:"product"`
		Hash      int64  `json:"hash"`
		OS        string `json:"os"`
		Timestamp string `json:"timestamp"`
		ISP       string `json:"isp"`
		Transport string `json:"transport"`
		ASN       string `json:"asn"`
		Hostnames []string `json:"hostnames"`
		Location  struct {
			City        string  `json:"city"`
			RegionCode  string  `json:"region_code"`
			AreaCode    string  `json:"area_code"`
			Longitude   float64 `json:"longitude"`
			Latitude    float64 `json:"latitude"`
			CountryCode string  `json:"country_code"`
			CountryName string  `json:"country_name"`
		} `json:"location"`
		IP      int64    `json:"ip"`
		Domains []string `json:"domains"`
		Org     string   `json:"org"`
		Data    string   `json:"data"`
		Port    int      `json:"port"`
		IPStr   string   `json:"ip_str"`
		Tags    []string `json:"tags"`
	} `json:"matches"`
	Total int `json:"total"`
	Took  int `json:"took"`
}

// PluginName 返回插件名称
func PluginName() string {
	return "Shodan代理API爬虫(requests版本)"
}

// PluginCronSpec 返回定时任务表达式
func PluginCronSpec() string {
	return shodanConfig.Periodic
}

// PluginFetchProxies 获取代理
func PluginFetchProxies(out chan<- string) error {
	
	if shodanConfig.Switch != "open" {
		logger.Plugin("未开启Shodan")
		return nil
	}
	
	logger.Plugin("已开启Shodan,将从Shodan获取SOCKS5代理数据")
	logger.Plugin("搜索条件: %s", shodanConfig.QueryString)
	
	var totalProcessed int // 记录处理了几条数据
	
	logger.Plugin("Shodan: 开始查询代理数据")
	
	// 简单的参数构建：只把空格替换成+号
	query := strings.ReplaceAll(shodanConfig.QueryString, " ", "+")
	
	// 直接拼接URL，不进行复杂编码
	fullURL := fmt.Sprintf("%s?query=%s&key=%s", shodanConfig.APIURL, query, shodanConfig.APIKey)
	logger.Debug("Shodan请求URL: %s", fullURL)
	
	// 使用requests发送GET请求
	options := requests.RequestOptions{
		Headers: map[string]string{
			"User-Agent": "curl/7.81.0",
			"Accept":     "*/*",
		},
		Proxies: false, // 不使用代理发送API请求
		Timeout: 60,
	}
	
	resp, err := requests.Get(fullURL, options)
	if err != nil {
		logger.Error("requests请求Shodan API失败: %v", err)
		return err
	}
	
	// 检查HTTP状态码
	if resp.StatusCode != 200 {
		logger.Error("Shodan API返回错误状态码: %d, 响应内容: %s", resp.StatusCode, resp.Text)
		return fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}
	
	logger.Plugin("Shodan API请求成功，状态码: %d，响应大小: %d 字节", 
		resp.StatusCode, len(resp.Content))
	
	// 解析JSON响应
	var shodanResp ShodanAPIResponse
	err = resp.JSON(&shodanResp)
	if err != nil {
		logger.Error("解析Shodan API响应JSON失败: %v", err)
		return fmt.Errorf("解析JSON失败: %v", err)
	}
	
	// 检查是否有数据
	if len(shodanResp.Matches) == 0 {
		logger.Plugin("Shodan: 无数据，查询结束")
		return nil
	}
	
	logger.Plugin("Shodan响应解析成功，获取到 %d 个结果，总共 %d 个可用资源", 
		len(shodanResp.Matches), shodanResp.Total)
	
	// 处理结果
	for _, result := range shodanResp.Matches {
		// 添加调试信息，查看实际字段值
		logger.Debug("Shodan数据项: IP=%s, Port=%d, Product='%s', Transport='%s'", 
			result.IPStr, result.Port, result.Product, result.Transport)
		
		// 检查是否为SOCKS5代理
		if result.Product == "SOCKS5 Proxy" && result.IPStr != "" && result.Port > 0 {
			proxyAddr := fmt.Sprintf("socks5://%s:%d", result.IPStr, result.Port)
			out <- proxyAddr
			totalProcessed++
			
			// 构建位置信息
			location := ""
			if result.Location.CountryName != "" {
				location += result.Location.CountryName
			}
			if result.Location.City != "" {
				location += " " + result.Location.City
			}
			
			logger.Debug("添加代理: %s (ISP: %s, 位置: %s, 组织: %s, ASN: %s)", 
				proxyAddr, result.ISP, location, result.Org, result.ASN)
		}
	}
	
	logger.Plugin("Shodan数据获取完成，共获取 %d 个SOCKS5代理", totalProcessed)
	return nil
}

// 导出插件变量
var Plugin = map[string]interface{}{
	"Name":         PluginName,
	"CronSpec":     PluginCronSpec,
	"FetchProxies": PluginFetchProxies,
} 