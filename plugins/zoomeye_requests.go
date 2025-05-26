// ZoomEye代理收集插件 - 使用requests API的函数式实现
package main

import (
	"encoding/base64"
	"fmt"
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
	MaxPages    int
	Periodic    string
}{
	Switch:      "open",
	APIURL:      "https://api.zoomeye.org/v2/search",
	APIKey:      "xxxxxxxxxxxxxxxxxxxxx", // 在这里填入你的ZoomEye API Key
	QueryString: `service="socks5" && country="CN" && banner_hash=67fba3cf0f19bc1818d406feb8364e90`,
	ResultSize:  200, // 每次获取的结果数量
	MaxPages:    5,   // 最大分页数
	Periodic:    "0 0 */2 * *", 
}

// ZoomEye API响应结构
type ZoomEyeAPIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Total   int    `json:"total"`
	Query   string `json:"query"`
	Data    []struct {
		URL           string   `json:"url"`
		IP            string   `json:"ip"`
		Domain        string   `json:"domain"`
		Hostname      string   `json:"hostname"`
		OS            string   `json:"os"`
		Port          int      `json:"port"`
		Service       string   `json:"service"`
		Title         []string `json:"title"`
		Version       string   `json:"version"`
		Device        string   `json:"device"`
		Rdns          string   `json:"rdns"`
		Product       string   `json:"product"`
		Header        string   `json:"header"`
		HeaderHash    string   `json:"header_hash"`
		Body          string   `json:"body"`
		BodyHash      string   `json:"body_hash"`
		Banner        string   `json:"banner"`
		UpdateTime    string   `json:"update_time"`
		HeaderServer  struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"header.server"`
		Continent struct {
			Name string `json:"name"`
		} `json:"continent"`
		Country struct {
			Name string `json:"name"`
		} `json:"country"`
		Province struct {
			Name string `json:"name"`
		} `json:"province"`
		City struct {
			Name string `json:"name"`
		} `json:"city"`
		Lon string `json:"lon"`
		Lat string `json:"lat"`
		Isp struct {
			Name string `json:"name"`
		} `json:"isp"`
		Organization struct {
			Name string `json:"name"`
		} `json:"organization"`
		Zipcode        string `json:"zipcode"`
		Idc            int    `json:"idc"`
		Honeypot       int    `json:"honeypot"`
		Asn            int    `json:"asn"`
		Protocol       string `json:"protocol"`
		SSL            string `json:"ssl"`
		PrimaryIndustry string `json:"primary_industry"`
		SubIndustry    string `json:"sub_industry"`
		Rank           int    `json:"rank"`
	} `json:"data"`
}

// PluginName 返回插件名称
func PluginName() string {
	return "ZoomEye代理API爬虫(requests版本)"
}

// PluginCronSpec 返回定时任务表达式
func PluginCronSpec() string {
	return config.Periodic
}

// PluginFetchProxies 获取代理
func PluginFetchProxies(out chan<- string) error {
	
	if config.Switch != "open" {
		logger.Plugin("未开启ZoomEye")
		return nil
	}
	
	logger.Plugin("已开启ZoomEye,将根据配置条件从ZoomEye中获取%d条数据", config.ResultSize)
	logger.Plugin("搜索条件: %s", config.QueryString)
	
	// Base64编码搜索条件
	queryBase64 := base64.StdEncoding.EncodeToString([]byte(config.QueryString))
	
	var totalProcessed int // 记录处理了几条数据
	pageSize := 20         // ZoomEye每页通常20条
	totalPages := config.MaxPages
	if config.ResultSize > 0 {
		calculatedPages := (config.ResultSize + pageSize - 1) / pageSize
		if calculatedPages < totalPages {
			totalPages = calculatedPages
		}
	}
	
	logger.Plugin("ZoomEye查询计划: 每页%d条，最多查询%d页", pageSize, totalPages)
	
	for page := 1; page <= totalPages; page++ {
		logger.Plugin("ZoomEye: 正在查询第%d页", page)
		
		// 构建请求体
		requestData := map[string]interface{}{
			"qbase64": queryBase64,
			"page":    page,
		}
		
		// 使用requests发送POST请求
		options := requests.RequestOptions{
			JSON: requestData,
			Headers: map[string]string{
				"API-KEY":      config.APIKey,
				"Content-Type": "application/json",
				"User-Agent":   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			},
			Proxies: false, // 不使用代理发送API请求
			Timeout: 60,
		}
		
		resp, err := requests.Post(config.APIURL, options)
		if err != nil {
			logger.Error("requests请求ZoomEye API失败: %v", err)
			return err
		}
		
		// 检查HTTP状态码
		if resp.StatusCode != 200 {
			logger.Error("ZoomEye API返回错误状态码: %d, 响应内容: %s", resp.StatusCode, resp.Text)
			return fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
		}
		
		logger.Plugin("ZoomEye API请求成功，状态码: %d，响应大小: %d 字节", 
			resp.StatusCode, len(resp.Content))
		
		// 解析JSON响应
		var zoomeyeResp ZoomEyeAPIResponse
		err = resp.JSON(&zoomeyeResp)
		if err != nil {
			logger.Error("解析ZoomEye API响应JSON失败: %v", err)
			return fmt.Errorf("解析JSON失败: %v", err)
		}
		
		// 检查API响应状态
		if zoomeyeResp.Code != 60000 {
			logger.Error("ZoomEye API返回错误代码: %d, 消息: %s", zoomeyeResp.Code, zoomeyeResp.Message)
			return fmt.Errorf("API错误代码: %d, 消息: %s", zoomeyeResp.Code, zoomeyeResp.Message)
		}
		
		// 检查是否有数据
		if len(zoomeyeResp.Data) == 0 {
			logger.Plugin("ZoomEye: 第%d页无数据，停止查询", page)
			break
		}
		
		logger.Plugin("ZoomEye第%d页响应解析成功，本页获取到 %d 个结果，总共 %d 个可用资源", 
			page, len(zoomeyeResp.Data), zoomeyeResp.Total)
		
		// 处理当前页结果
		pageCount := 0
		for _, result := range zoomeyeResp.Data {
			// 添加调试信息，查看实际字段值
			logger.Debug("ZoomEye数据项: IP=%s, Port=%d, Service='%s', Domain='%s'", 
				result.IP, result.Port, result.Service, result.Domain)
			
		
			if result.IP != "" && result.Port > 0 {
				proxyAddr := fmt.Sprintf("socks5://%s:%d", result.IP, result.Port)
				out <- proxyAddr
				pageCount++
				totalProcessed++
				
				// 构建位置信息
				location := ""
				if result.Country.Name != "" {
					location += result.Country.Name
				}
				if result.Province.Name != "" {
					location += " " + result.Province.Name
				}
				if result.City.Name != "" {
					location += " " + result.City.Name
				}
				
				logger.Debug("添加代理: %s (ISP: %s, 位置: %s, 组织: %s)", 
					proxyAddr, result.Isp.Name, location, result.Organization.Name)
			}
		}
		
		logger.Plugin("ZoomEye第%d页处理完成，获取到 %d 个SOCKS5代理", page, pageCount)
		
		// 检查是否达到设定的获取数量
		if config.ResultSize > 0 && totalProcessed >= config.ResultSize {
			logger.Plugin("ZoomEye已达到设定的获取数量: %d", config.ResultSize)
			break
		}
		
		// 防止访问过快，添加延时（除了最后一页）
		if page < totalPages {
			logger.Plugin("ZoomEye延时2秒，防止访问过快...")
			time.Sleep(2 * time.Second)
		}
	}
	
	logger.Plugin("ZoomEye数据已取，共获取 %d 个SOCKS5代理", totalProcessed)
	return nil
}

// 导出插件变量
var Plugin = map[string]interface{}{
	"Name":         PluginName,
	"CronSpec":     PluginCronSpec,
	"FetchProxies": PluginFetchProxies,
} 