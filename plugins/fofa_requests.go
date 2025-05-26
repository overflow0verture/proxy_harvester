// FOFA代理收集插件 - 使用requests API的函数式实现
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
	Email       string
	Key         string
	QueryString string
	ResultSize  int
	Periodic    string
}{
	Switch:      "open",
	APIURL:      "https://fofa.info/api/v1/search/all",
	Email:       "xxxxxxxxxxxxxxxxxxxxxxxxxxx", // 在这里填入你的FOFA邮箱
	Key:         "xxxxxxxxxxxxxxxxxxxxxxxxxxx", // 在这里填入你的FOFA API Key
	QueryString: "protocol==\"socks5\" && country==\"CN\" && banner=\"Method:No Authentication\"",
	ResultSize:  50,
	Periodic:    "0 0 * * 1", 
}

// FOFA API响应结构
type FOFAAPIResponse struct {
	Error   bool        `json:"error"`
	Mode    string      `json:"mode"`
	Page    int         `json:"page"`
	Size    int         `json:"size"`
	Query   string      `json:"query"`
	Results [][]string  `json:"results"` // FOFA返回二维数组，每个内部数组有两个元素：IP和端口
	Total   int         `json:"total"`
}

// PluginName 返回插件名称
func PluginName() string {
	return "FOFA代理API爬虫(requests版本)"
}

// PluginCronSpec 返回定时任务表达式
func PluginCronSpec() string {
	return config.Periodic
}

// PluginFetchProxies 获取代理
func PluginFetchProxies(out chan<- string) error {
	
	if config.Switch != "open" {
		logger.Plugin("未开启FOFA")
		return nil
	}
	
	logger.Plugin("已开启FOFA,将根据配置条件从FOFA中获取%d条数据", config.ResultSize)
	
	// 将查询字符串进行Base64编码
	queryBase64 := base64.StdEncoding.EncodeToString([]byte(config.QueryString))
	
	// 构建请求参数
	params := map[string]string{
		"email":   config.Email,
		"key":     config.Key,
		"qbase64": queryBase64,
		"size":    fmt.Sprintf("%d", config.ResultSize),
		"fields":  "ip,port",
	}
	
	// 使用requests发送GET请求
	options := requests.RequestOptions{
		Params: params,
		Headers: map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		},
		Proxies: false,
	}
	
	resp, err := requests.Get(config.APIURL, options)
	if err != nil {
		logger.Error("requests请求FOFA API失败: %v", err)
		return err
	}
	
	// 检查HTTP状态码
	if resp.StatusCode != 200 {
		logger.Error("FOFA API返回错误状态码: %d", resp.StatusCode)
		return fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}
	
	logger.Plugin("FOFA API请求成功，状态码: %d，响应大小: %d 字节", 
		resp.StatusCode, len(resp.Content))
	
	// 解析JSON响应
	var fofaResp FOFAAPIResponse
	err = resp.JSON(&fofaResp)
	if err != nil {
		logger.Error("解析FOFA API响应JSON失败: %v", err)
		return fmt.Errorf("解析JSON失败: %v", err)
	}
	
	// 检查是否有错误
	if fofaResp.Error {
		logger.Error("FOFA API返回错误")
		return fmt.Errorf("FOFA API返回错误")
	}
	
	logger.Plugin("FOFA API响应解析成功，获取到 %d 个结果", len(fofaResp.Results))
	
	// 处理结果
	count := 0
	for _, result := range fofaResp.Results {
		if len(result) >= 2 {
			ip := result[0]
			port := result[1]
			proxyAddr := fmt.Sprintf("socks5://%s:%s", ip, port)
			out <- proxyAddr
			count++
			logger.Debug("添加代理: %s", proxyAddr)
		}
	}
	
	logger.Plugin("FOFA数据已取，共获取 %d 个代理", count)
	return nil
}

// 导出插件变量
var Plugin = map[string]interface{}{
	"Name":         PluginName,
	"CronSpec":     PluginCronSpec,
	"FetchProxies": PluginFetchProxies,
} 