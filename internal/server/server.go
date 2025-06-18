package socks5server

import (
	"github.com/overflow0verture/proxy_harvester/internal/config"
	"github.com/overflow0verture/proxy_harvester/internal/logger"
	"github.com/overflow0verture/proxy_harvester/internal/netutil"
	"github.com/overflow0verture/proxy_harvester/internal/pool"
	"context"
	"github.com/armon/go-socks5"
	"io"
	"log"
	"net"
	"strconv"
)

// StartServer 启动socks5监听服务（增强版，提供更多日志）
func StartServer(proxyStore pool.ProxyStore, cfg config.ListenerConfig, timeout int) {
	// 获取代理池信息
	proxyCount, _ := proxyStore.Len()
	storeType := "Redis存储"
	logger.Info("Socks5服务启动中，监听地址: %s:%d，使用%s代理池，当前有 %d 个代理", 
		cfg.IP, cfg.Port, storeType, proxyCount)
	
	conf := &socks5.Config{
		Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return netutil.TransmitReqFromClient(network, addr, proxyStore, timeout)
		},
		Logger: log.New(io.Discard, "", log.LstdFlags),
	}
	
	// 如果配置了用户名和密码，添加认证
	userName := cfg.UserName
	password := cfg.Password
	if userName != "" && password != "" {
		cator := socks5.UserPassAuthenticator{Credentials: socks5.StaticCredentials{
			userName: password,
		}}
		conf.AuthMethods = []socks5.Authenticator{cator}
		logger.Info("Socks5服务已启用认证，用户名: %s", userName)
	}
	
	server, err := socks5.New(conf)
	if err != nil {
		logger.Error("Socks5服务初始化失败: %v", err)
		return
	}
	
	listener := cfg.IP + ":" + strconv.Itoa(cfg.Port)
	
	if err := server.ListenAndServe("tcp", listener); err != nil {
		logger.Error("Socks5服务启动失败: %v", err)
	}
} 