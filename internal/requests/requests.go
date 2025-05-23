package requests

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/overflow0verture/proxy_harvester/internal/netutil"
	"github.com/overflow0verture/proxy_harvester/internal/pool"
)

// 全局ProxyStore实例，由插件系统初始化
var globalProxyStore pool.ProxyStore

// InitRequests 初始化requests包，注入ProxyStore
func InitRequests(proxyStore pool.ProxyStore) {
	globalProxyStore = proxyStore
}

// Response HTTP响应结构
type Response struct {
	*http.Response
	Text    string            // 响应文本内容
	Content []byte            // 响应二进制内容
	JSON    func(v interface{}) error // 解析JSON的便捷方法
	URL     string            // 最终的URL（处理重定向后）
	Headers map[string]string // 响应头
}

// RequestOptions 请求选项
type RequestOptions struct {
	Headers map[string]string // 请求头
	Timeout int               // 超时时间（秒），默认30秒
	Proxies bool              // 是否使用代理，默认true
	Data    interface{}       // POST数据
	JSON    interface{}       // JSON数据
	Params  map[string]string // URL参数
}

// Get 发送GET请求
func Get(url string, options ...RequestOptions) (*Response, error) {
	return request("GET", url, options...)
}

// Post 发送POST请求
func Post(url string, options ...RequestOptions) (*Response, error) {
	return request("POST", url, options...)
}

// Put 发送PUT请求
func Put(url string, options ...RequestOptions) (*Response, error) {
	return request("PUT", url, options...)
}

// Delete 发送DELETE请求
func Delete(url string, options ...RequestOptions) (*Response, error) {
	return request("DELETE", url, options...)
}

// Head 发送HEAD请求
func Head(url string, options ...RequestOptions) (*Response, error) {
	return request("HEAD", url, options...)
}

// request 通用请求方法
func request(method, reqURL string, options ...RequestOptions) (*Response, error) {
	// 合并选项
	opts := RequestOptions{
		Headers: make(map[string]string),
		Timeout: 30,
		Proxies: true,
	}
	if len(options) > 0 {
		if options[0].Headers != nil {
			for k, v := range options[0].Headers {
				opts.Headers[k] = v
			}
		}
		if options[0].Timeout > 0 {
			opts.Timeout = options[0].Timeout
		}
		if options[0].Data != nil {
			opts.Data = options[0].Data
		}
		if options[0].JSON != nil {
			opts.JSON = options[0].JSON
		}
		if options[0].Params != nil {
			opts.Params = options[0].Params
		}
		opts.Proxies = options[0].Proxies
	}

	// 处理URL参数
	if opts.Params != nil {
		reqURL = addURLParams(reqURL, opts.Params)
	}

	// 创建HTTP客户端
	client := createHTTPClient(opts.Timeout, opts.Proxies)

	// 准备请求体
	var body io.Reader
	if opts.JSON != nil {
		jsonData, err := json.Marshal(opts.JSON)
		if err != nil {
			return nil, fmt.Errorf("序列化JSON失败: %v", err)
		}
		body = bytes.NewBuffer(jsonData)
		if opts.Headers["Content-Type"] == "" {
			opts.Headers["Content-Type"] = "application/json"
		}
	} else if opts.Data != nil {
		switch data := opts.Data.(type) {
		case string:
			body = strings.NewReader(data)
		case []byte:
			body = bytes.NewReader(data)
		case io.Reader:
			body = data
		case map[string]string:
			// 表单数据
			formData := url.Values{}
			for k, v := range data {
				formData.Set(k, v)
			}
			body = strings.NewReader(formData.Encode())
			if opts.Headers["Content-Type"] == "" {
				opts.Headers["Content-Type"] = "application/x-www-form-urlencoded"
			}
		default:
			return nil, fmt.Errorf("不支持的数据类型: %T", data)
		}
	}

	// 创建请求
	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置默认请求头
	setDefaultHeaders(req, opts.Headers)

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}

	// 创建Response对象
	return newResponse(resp)
}

// createHTTPClient 创建HTTP客户端
func createHTTPClient(timeout int, useProxies bool) *http.Client {
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// 如果启用代理且有可用的ProxyStore
	if useProxies && globalProxyStore != nil {
		client.Transport = &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return netutil.TransmitReqFromClient(network, addr, globalProxyStore, timeout)
			},
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	return client
}

// setDefaultHeaders 设置默认请求头
func setDefaultHeaders(req *http.Request, headers map[string]string) {
	// 设置默认User-Agent
	if req.Header.Get("User-Agent") == "" && headers["User-Agent"] == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	}

	// 设置自定义请求头
	for key, value := range headers {
		req.Header.Set(key, value)
	}
}

// addURLParams 添加URL参数
func addURLParams(reqURL string, params map[string]string) string {
	u, err := url.Parse(reqURL)
	if err != nil {
		return reqURL
	}

	q := u.Query()
	for key, value := range params {
		q.Set(key, value)
	}
	u.RawQuery = q.Encode()

	return u.String()
}

// newResponse 创建Response对象
func newResponse(resp *http.Response) (*Response, error) {
	defer resp.Body.Close()

	// 读取响应体
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %v", err)
	}

	// 转换响应头
	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	response := &Response{
		Response: resp,
		Text:     string(content),
		Content:  content,
		URL:      resp.Request.URL.String(),
		Headers:  headers,
		JSON: func(v interface{}) error {
			return json.Unmarshal(content, v)
		},
	}

	return response, nil
}

// Session HTTP会话，支持cookie持久化
type Session struct {
	client  *http.Client
	headers map[string]string
	cookies []*http.Cookie
	timeout int
	proxies bool
}

// NewSession 创建新的HTTP会话
func NewSession() *Session {
	return &Session{
		headers: make(map[string]string),
		timeout: 30,
		proxies: true,
	}
}

// Get Session的GET方法
func (s *Session) Get(url string, options ...RequestOptions) (*Response, error) {
	return s.request("GET", url, options...)
}

// Post Session的POST方法
func (s *Session) Post(url string, options ...RequestOptions) (*Response, error) {
	return s.request("POST", url, options...)
}

// SetHeaders 设置会话默认请求头
func (s *Session) SetHeaders(headers map[string]string) {
	for k, v := range headers {
		s.headers[k] = v
	}
}

// SetTimeout 设置会话超时
func (s *Session) SetTimeout(timeout int) {
	s.timeout = timeout
}

// SetProxies 设置是否使用代理
func (s *Session) SetProxies(enable bool) {
	s.proxies = enable
}

// request Session的请求方法
func (s *Session) request(method, url string, options ...RequestOptions) (*Response, error) {
	// 合并会话和请求选项
	opts := RequestOptions{
		Headers: make(map[string]string),
		Timeout: s.timeout,
		Proxies: s.proxies,
	}

	// 复制会话头
	for k, v := range s.headers {
		opts.Headers[k] = v
	}

	// 合并请求选项
	if len(options) > 0 {
		if options[0].Headers != nil {
			for k, v := range options[0].Headers {
				opts.Headers[k] = v
			}
		}
		if options[0].Timeout > 0 {
			opts.Timeout = options[0].Timeout
		}
		if options[0].Data != nil {
			opts.Data = options[0].Data
		}
		if options[0].JSON != nil {
			opts.JSON = options[0].JSON
		}
		if options[0].Params != nil {
			opts.Params = options[0].Params
		}
	}

	return request(method, url, opts)
} 