// IP3366免费代理收集插件 - 使用requests API的网页爬虫实现
package main

import (
	"fmt"
	"regexp"
	"strings"
	"github.com/overflow0verture/proxy_harvester/internal/logger"
	"github.com/overflow0verture/proxy_harvester/internal/requests"
)

// 插件配置
var config = struct {
	Switch    string
	BaseURL   string
	MaxPages  int
	UserAgent string
	Periodic  string
}{
	Switch:    "open",
	BaseURL:   "http://www.ip3366.net/free/",
	MaxPages:  3, // 爬取前3页
	UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	Periodic:  "0 0 * * 1", // 每20分钟执行一次
}

// 代理信息结构
type ProxyInfo struct {
	IP       string
	Port     string
	Protocol string
	Location string
	Speed    string
}

// PluginName 返回插件名称
func PluginName() string {
	return "IP3366免费代理爬虫(requests版本)"
}

// PluginCronSpec 返回定时任务表达式
func PluginCronSpec() string {
	return config.Periodic
}

// PluginFetchProxies 获取代理
func PluginFetchProxies(out chan<- string) error {
	
	if config.Switch != "open" {
		logger.Plugin("未开启IP3366")
		return nil
	}
	
	logger.Plugin("已开启IP3366,开始爬取免费代理数据，最多爬取%d页", config.MaxPages)
	
	totalCount := 0
	
	// 爬取多页数据
	for page := 1; page <= config.MaxPages; page++ {
		logger.Plugin("正在爬取第 %d 页", page)
		
		pageURL := config.BaseURL
		if page > 1 {
			pageURL = fmt.Sprintf("%s?stype=1&page=%d", config.BaseURL, page)
		}
		
		// 尝试多种请求方式
		proxies, err := fetchPageProxiesWithRetry(pageURL)
		if err != nil {
			logger.Error("爬取第 %d 页失败: %v", page, err)
			continue
		}
		
		// 处理当前页的代理
		pageCount := 0
		for _, proxy := range proxies {
			proxyURL := formatProxyURL(proxy)
			if proxyURL != "" {
				out <- proxyURL
				totalCount++
				pageCount++
				logger.Debug("添加代理: %s", proxyURL)
			}
		}
		
		logger.Plugin("第 %d 页爬取完成，获取 %d 个代理", page, pageCount)
		
		// 页面间隔，避免过于频繁请求
		if page < config.MaxPages {
			logger.Debug("等待2秒后继续下一页...")
			// 这里可以用time.Sleep，但为了插件简洁性，我们跳过
		}
	}
	
	logger.Success("IP3366数据爬取完成，共获取 %d 个代理", totalCount)
	return nil
}

// fetchPageProxiesWithRetry 带重试的页面获取
func fetchPageProxiesWithRetry(pageURL string) ([]ProxyInfo, error) {
	// 尝试不同的请求配置
	requestConfigs := []map[string]string{
		// 配置1：标准浏览器请求（不指定编码）
		{
			"User-Agent": config.UserAgent,
			"Accept":     "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
			"Connection": "keep-alive",
			"Cache-Control": "no-cache",
		},
		// 配置2：简单请求
		{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		},
		// 配置3：模拟旧浏览器
		{
			"User-Agent": "Mozilla/4.0 (compatible; MSIE 8.0; Windows NT 6.1)",
			"Accept": "*/*",
		},
	}
	
	for i, headers := range requestConfigs {
		logger.Debug("尝试请求配置 %d", i+1)
		
		options := requests.RequestOptions{
			Headers: headers,
			Proxies: false,
			Timeout: 30,
		}
		
		resp, err := requests.Get(pageURL, options)
		if err != nil {
			logger.Debug("请求配置 %d 失败: %v", i+1, err)
			continue
		}
		
		// 检查HTTP状态码
		if resp.StatusCode != 200 {
			logger.Debug("请求配置 %d 状态码错误: %d", i+1, resp.StatusCode)
			continue
		}
		
		// 检查响应内容
		if len(resp.Content) == 0 {
			logger.Debug("请求配置 %d 响应内容为空", i+1)
			continue
		}
		
		htmlContent := string(resp.Content)
		
		// 检查是否为有效的HTML内容
		if !strings.Contains(htmlContent, "<") {
			logger.Debug("请求配置 %d 响应非HTML内容", i+1)
			continue
		}
		
		logger.Plugin("请求配置 %d 成功，状态码: %d，响应大小: %d 字节", 
			i+1, resp.StatusCode, len(resp.Content))
		
		// 解析HTML内容
		proxies, err := parseHTMLContent(htmlContent)
		if err != nil {
			logger.Debug("请求配置 %d 解析HTML失败: %v", i+1, err)
			continue
		}
		
		// 如果成功解析到代理，返回结果
		if len(proxies) > 0 {
			return proxies, nil
		}
		
		logger.Debug("请求配置 %d 未解析到代理，尝试下一个配置", i+1)
	}
	
	return nil, fmt.Errorf("所有请求配置都失败")
}

// parseHTMLContent 解析HTML内容提取代理信息
func parseHTMLContent(html string) ([]ProxyInfo, error) {
	var proxies []ProxyInfo
	
	// 安全的HTML片段输出
	debugSnippet := ""
	if len(html) > 0 {
		snippetLen := 500
		if len(html) < snippetLen {
			snippetLen = len(html)
		}
		
		// 检查是否包含有效的HTML标签
		if strings.Contains(html, "<") {
			debugSnippet = html[:snippetLen]
		} else {
			debugSnippet = "非HTML内容或编码问题"
		}
	} else {
		debugSnippet = "HTML内容为空"
	}
	
	
	// 多种正则表达式尝试匹配
	regexPatterns := []string{
		// 完整表格行匹配
		`<tr[^>]*>\s*<td[^>]*>(\d+\.\d+\.\d+\.\d+)</td>\s*<td[^>]*>(\d+)</td>\s*<td[^>]*>[^<]*</td>\s*<td[^>]*>(HTTPS?)</td>\s*<td[^>]*>([^<]*)</td>\s*<td[^>]*>([^<]*)</td>\s*<td[^>]*>[^<]*</td>\s*</tr>`,
		
		// 简化的表格行匹配
		`<tr[^>]*>.*?(\d+\.\d+\.\d+\.\d+).*?(\d+).*?(HTTP[S]?).*?</tr>`,
		
		// 更宽松的匹配
		`(\d+\.\d+\.\d+\.\d+)[^0-9]*(\d+)[^a-zA-Z]*(HTTP[S]?)`,
		
		// 最简单的IP端口匹配
		`(\d+\.\d+\.\d+\.\d+)[^\d]*(\d{2,5})`,
		
		// 非贪婪匹配
		`(\d+\.\d+\.\d+\.\d+).*?(\d{2,5})`,
	}
	
	for i, pattern := range regexPatterns {
		patternDesc := pattern
		if len(pattern) > 50 {
			patternDesc = pattern[:50] + "..."
		}
		// logger.Debug("尝试正则模式 %d: %s", i+1, patternDesc)zz
		
		regex := regexp.MustCompile(pattern)
		matches := regex.FindAllStringSubmatch(html, -1)
		
		logger.Debug("正则模式 %d 匹配到 %d 个结果", i+1, len(matches))
		
		if len(matches) > 0 {
			for _, match := range matches {
				if len(match) >= 3 {
					proxy := ProxyInfo{
						IP:       strings.TrimSpace(match[1]),
						Port:     strings.TrimSpace(match[2]),
						Protocol: "HTTP", // 默认协议
						Location: "未知",
						Speed:    "未知",
					}
					
					// 如果有协议信息，使用它
					if len(match) >= 4 && match[3] != "" {
						proxy.Protocol = strings.TrimSpace(match[3])
					}
					
					// 验证IP和端口格式
					if isValidIP(proxy.IP) && isValidPort(proxy.Port) {
						// 检查是否已存在（去重）
						found := false
						for _, existing := range proxies {
							if existing.IP == proxy.IP && existing.Port == proxy.Port {
								found = true
								break
							}
						}
						
						if !found {
							proxies = append(proxies, proxy)
							logger.Debug("找到有效代理: %s:%s (%s)", proxy.IP, proxy.Port, proxy.Protocol)
						}
					} else {
						logger.Debug("无效代理格式: IP=%s, Port=%s", proxy.IP, proxy.Port)
					}
				}
			}
			
			// 如果找到了代理，就不再尝试其他模式
			if len(proxies) > 0 {
				break
			}
		}
	}
	
	// 如果所有正则都失败，尝试最后的备用方案
	if len(proxies) == 0 {
		logger.Warning("所有正则表达式都匹配失败，尝试最后的备用解析")
		return parseHTMLFallback(html)
	}
	
	logger.Plugin("从HTML中解析出 %d 个有效代理", len(proxies))
	return proxies, nil
}

// parseHTMLFallback 备用HTML解析方法
func parseHTMLFallback(html string) ([]ProxyInfo, error) {
	var proxies []ProxyInfo
	
	// 查找所有IP地址
	ipRegex := regexp.MustCompile(`\b(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\b`)
	ipMatches := ipRegex.FindAllString(html, -1)
	
	// 查找所有可能的端口号
	portRegex := regexp.MustCompile(`\b(\d{2,5})\b`)
	portMatches := portRegex.FindAllString(html, -1)
	
	logger.Debug("备用解析: 找到 %d 个IP地址, %d 个端口号", len(ipMatches), len(portMatches))
	
	// 尝试配对IP和端口
	for i, ip := range ipMatches {
		if !isValidIP(ip) {
			continue
		}
		
		// 寻找距离最近的端口号
		for j, port := range portMatches {
			if !isValidPort(port) {
				continue
			}
			
			// 简单的距离判断（可以改进）
			if abs(i-j) <= 2 {
				proxy := ProxyInfo{
					IP:       ip,
					Port:     port,
					Protocol: "HTTP",
					Location: "未知",
					Speed:    "未知",
				}
				
				// 检查是否已存在
				found := false
				for _, existing := range proxies {
					if existing.IP == proxy.IP && existing.Port == proxy.Port {
						found = true
						break
					}
				}
				
				if !found {
					proxies = append(proxies, proxy)
					logger.Debug("备用解析找到代理: %s:%s", proxy.IP, proxy.Port)
				}
				break
			}
		}
	}
	
	logger.Plugin("备用解析完成，找到 %d 个代理", len(proxies))
	return proxies, nil
}

// parseHTMLSimple 简单模式解析HTML
func parseHTMLSimple(html string) ([]ProxyInfo, error) {
	var proxies []ProxyInfo
	
	// 查找包含IP地址的行
	ipRegex := regexp.MustCompile(`(\d+\.\d+\.\d+\.\d+).*?(\d+).*?(HTTPS?)`)
	matches := ipRegex.FindAllStringSubmatch(html, -1)
	
	for _, match := range matches {
		if len(match) >= 4 {
			proxy := ProxyInfo{
				IP:       strings.TrimSpace(match[1]),
				Port:     strings.TrimSpace(match[2]),
				Protocol: strings.TrimSpace(match[3]),
				Location: "未知",
				Speed:    "未知",
			}
			
			if isValidIP(proxy.IP) && isValidPort(proxy.Port) {
				proxies = append(proxies, proxy)
			}
		}
	}
	
	return proxies, nil
}

// formatProxyURL 格式化代理URL
func formatProxyURL(proxy ProxyInfo) string {
	protocol := strings.ToLower(proxy.Protocol)
	
	switch protocol {
	case "http":
		return fmt.Sprintf("http://%s:%s", proxy.IP, proxy.Port)
	case "https":
		return fmt.Sprintf("https://%s:%s", proxy.IP, proxy.Port)
	default:
		// 默认当作HTTP处理
		return fmt.Sprintf("http://%s:%s", proxy.IP, proxy.Port)
	}
}

// isValidIP 验证IP地址格式
func isValidIP(ip string) bool {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}
	
	for _, part := range parts {
		if len(part) == 0 || len(part) > 3 {
			return false
		}
		
		// 简单验证，确保是数字且在有效范围内
		num := 0
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
			num = num*10 + int(char-'0')
		}
		
		if num > 255 {
			return false
		}
	}
	
	return true
}

// isValidPort 验证端口号
func isValidPort(port string) bool {
	if len(port) == 0 || len(port) > 5 {
		return false
	}
	
	// 确保是数字
	num := 0
	for _, char := range port {
		if char < '0' || char > '9' {
			return false
		}
		num = num*10 + int(char-'0')
	}
	
	// 端口范围检查
	return num >= 80 && num <= 65535
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// abs 辅助函数
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// 导出插件变量
var Plugin = map[string]interface{}{
	"Name":         PluginName,
	"CronSpec":     PluginCronSpec,
	"FetchProxies": PluginFetchProxies,
} 