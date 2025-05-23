package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/overflow0verture/proxy_harvester/internal/logger"
	"github.com/overflow0verture/proxy_harvester/internal/pool"
)

// APIServer API服务器
type APIServer struct {
	proxyStore pool.ProxyStore
	token      string
	port       int
	server     *http.Server
}

// ProxyResponse 代理响应结构
type ProxyResponse struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Data    []string `json:"data"`
	Count   int      `json:"count"`
	Total   int      `json:"total"`
}

// ErrorResponse 错误响应结构
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewAPIServer 创建新的API服务器
func NewAPIServer(proxyStore pool.ProxyStore, token string, port int) *APIServer {
	return &APIServer{
		proxyStore: proxyStore,
		token:      token,
		port:       port,
	}
}

// Start 启动API服务器
func (s *APIServer) Start() error {
	mux := http.NewServeMux()
	
	// 注册路由
	mux.HandleFunc("/api/proxies", s.handleGetProxies)
	mux.HandleFunc("/api/status", s.handleGetStatus)
	// mux.HandleFunc("/", s.handleIndex)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      s.loggingMiddleware(s.authMiddleware(mux)),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logger.Info("API服务器启动在端口 %d", s.port)
	return s.server.ListenAndServe()
}

// Stop 停止API服务器
func (s *APIServer) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// authMiddleware 认证中间件
func (s *APIServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 首页不需要认证
		if r.URL.Path == "/" {
			next.ServeHTTP(w, r)
			return
		}

		token := r.URL.Query().Get("token")
		if token == "" {
			s.writeError(w, 401, "缺少token参数")
			return
		}

		if token != s.token {
			s.writeError(w, 401, "token无效")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware 日志中间件
func (s *APIServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Debug("API请求: %s %s - %v", r.Method, r.URL.Path, time.Since(start))
	})
}

// handleGetProxies 获取代理接口
func (s *APIServer) handleGetProxies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, 405, "只支持GET方法")
		return
	}

	// 解析参数
	countStr := r.URL.Query().Get("count")
	proxyType := r.URL.Query().Get("type")

	// 默认值
	count := 10
	if countStr != "" {
		var err error
		count, err = strconv.Atoi(countStr)
		if err != nil || count <= 0 || count > 100 {
			s.writeError(w, 400, "count参数无效，必须是1-100之间的整数")
			return
		}
	}

	// 获取代理池总数
	total, err := s.proxyStore.Len()
	if err != nil {
		s.writeError(w, 500, "获取代理池状态失败")
		return
	}

	if total == 0 {
		s.writeJSON(w, ProxyResponse{
			Code:    200,
			Message: "成功，但代理池为空",
			Data:    []string{},
			Count:   0,
			Total:   0,
		})
		return
	}

	// 如果代理池中的数量小于请求数量，则调整count为实际可用数量
	// 但仍然不能超过100个的限制
	if total < count {
		count = total
		if count > 100 {
			count = 100
		}
	}

	// 获取代理
	proxies := make([]string, 0, count)
	attemptCount := 0
	maxAttempts := count * 3 // 尝试最多3倍数量，以防类型过滤导致获取不足

	for len(proxies) < count && attemptCount < maxAttempts {
		proxy, err := s.proxyStore.GetNext()
		if err != nil {
			break // 没有更多代理了
		}
		attemptCount++

		// 如果指定了代理类型，进行过滤
		if proxyType != "" {
			if !strings.HasPrefix(proxy, proxyType+"://") {
				// 不匹配类型，尝试下一个
				continue
			}
		}

		proxies = append(proxies, proxy)
	}

	s.writeJSON(w, ProxyResponse{
		Code:    200,
		Message: "获取成功",
		Data:    proxies,
		Count:   len(proxies),
		Total:   total,
	})
}

// handleGetStatus 获取状态接口
func (s *APIServer) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, 405, "只支持GET方法")
		return
	}

	total, err := s.proxyStore.Len()
	if err != nil {
		s.writeError(w, 500, "获取代理池状态失败")
		return
	}

	status := map[string]interface{}{
		"code":      200,
		"message":   "状态正常",
		"total":     total,
		"timestamp": time.Now().Unix(),
	}

	s.writeJSON(w, status)
}

// handleIndex 首页
// func (s *APIServer) handleIndex(w http.ResponseWriter, r *http.Request) {
// 	html := `<!DOCTYPE html>
// <html>
// <head>
//     <title>Proxy Harvester API</title>
//     <meta charset="utf-8">
//     <style>
//         body { font-family: Arial, sans-serif; margin: 40px; }
//         .container { max-width: 800px; margin: 0 auto; }
//         h1 { color: #333; }
//         .endpoint { background: #f5f5f5; padding: 15px; margin: 10px 0; border-radius: 5px; }
//         .method { color: #4CAF50; font-weight: bold; }
//         .note { background: #e3f2fd; padding: 10px; margin: 10px 0; border-radius: 5px; border-left: 4px solid #2196F3; }
//         code { background: #e0e0e0; padding: 2px 4px; border-radius: 3px; }
//     </style>
// </head>
// <body>
//     <div class="container">
//         <h1>🚀 Proxy Harvester API</h1>
//         <p>代理池访问API服务</p>
        
//         <h2>📋 API接口</h2>
        
//         <div class="endpoint">
//             <h3><span class="method">GET</span> /api/proxies</h3>
//             <p><strong>功能：</strong>获取代理列表</p>
//             <p><strong>参数：</strong></p>
//             <ul>
//                 <li><code>token</code> - 必需，认证令牌</li>
//                 <li><code>count</code> - 可选，获取数量(1-100)，默认10</li>
//                 <li><code>type</code> - 可选，代理类型(socks5/http/https)</li>
//             </ul>
//             <div class="note">
//                 <strong>智能数量处理：</strong>
//                 <ul>
//                     <li>如果代理池数量少于请求数量，将返回所有可用代理</li>
//                     <li>单次请求最多返回100个代理</li>
//                     <li>支持按类型过滤，过滤可能导致实际返回数量小于请求数量</li>
//                 </ul>
//             </div>
//             <p><strong>示例：</strong></p>
//             <ul>
//                 <li><code>/api/proxies?token=atoken&count=5&type=socks5</code></li>
//                 <li><code>/api/proxies?token=atoken&count=1000</code> (最多返回100个)</li>
//             </ul>
//         </div>
        
//         <div class="endpoint">
//             <h3><span class="method">GET</span> /api/status</h3>
//             <p><strong>功能：</strong>获取代理池状态</p>
//             <p><strong>参数：</strong></p>
//             <ul>
//                 <li><code>token</code> - 必需，认证令牌</li>
//             </ul>
//             <p><strong>示例：</strong><code>/api/status?token=atoken</code></p>
//         </div>

//         <h2>📝 响应格式</h2>
//         <pre>{
//   "code": 200,
//   "message": "获取成功",
//   "data": ["socks5://ip:port", ...],
//   "count": 5,      // 实际返回的代理数量
//   "total": 100     // 代理池总数
// }</pre>

//         <h2>💡 使用建议</h2>
//         <div class="note">
//             <ul>
//                 <li>建议先调用 <code>/api/status</code> 了解代理池总数</li>
//                 <li>如需大量代理，可多次调用API接口</li>
//                 <li>代理可能随时失效，请做好重试机制</li>
//                 <li>按类型过滤时，实际返回数量可能小于请求数量</li>
//             </ul>
//         </div>
//     </div>
// </body>
// </html>`
	
// 	w.Header().Set("Content-Type", "text/html; charset=utf-8")
// 	w.WriteHeader(http.StatusOK)
// 	w.Write([]byte(html))
// }

// writeJSON 写入JSON响应
func (s *APIServer) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应
func (s *APIServer) writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{
		Code:    code,
		Message: message,
	})
} 