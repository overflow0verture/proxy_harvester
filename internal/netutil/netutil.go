package netutil

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"encoding/base64"
	"github.com/overflow0verture/proxy_harvester/internal/pool"
	"github.com/overflow0verture/proxy_harvester/internal/logger"
	"golang.org/x/net/proxy"
)

// 支持socks5/http/https认证代理的转发
func TransmitReqFromClient(network string, address string, proxyStore pool.ProxyStore, timeout int) (net.Conn, error) {
	proxyAddr, err := proxyStore.GetNext()
	if err != nil {
		return nil, fmt.Errorf("无可用代理")
	}
	// fmt.Println(time.Now().Format("2006-01-02 15:04:05") + "\t" + proxyAddr)
	timeoutDur := time.Duration(timeout) * time.Second

	if strings.HasPrefix(proxyAddr, "socks5://") {
		u, err := url.Parse(proxyAddr)
		if err != nil {
			proxyStore.MarkInvalid(proxyAddr)
			return TransmitReqFromClient(network, address, proxyStore, timeout)
		}
		host := u.Host
		var auth *proxy.Auth
		if u.User != nil {
			user := u.User.Username()
			pass, _ := u.User.Password()
			auth = &proxy.Auth{User: user, Password: pass}
		}
		dialer := &net.Dialer{Timeout: timeoutDur}
		socksDialer, err := proxy.SOCKS5(network, host, auth, dialer)
		if err != nil {
			if rec, ok := proxyStore.(pool.ResultRecorder); ok {
				rec.RecordResult(proxyAddr, false)
			}
			proxyStore.MarkInvalid(proxyAddr)
			logger.Info("%s无效，自动切换下一个......\n", proxyAddr)
			return TransmitReqFromClient(network, address, proxyStore, timeout)
		}
		conn, err := socksDialer.Dial(network, address)
		if err != nil {
			if rec, ok := proxyStore.(pool.ResultRecorder); ok {
				rec.RecordResult(proxyAddr, false)
			}
			proxyStore.MarkInvalid(proxyAddr)
			logger.Info("%s无效，自动切换下一个......\n", proxyAddr)
			return TransmitReqFromClient(network, address, proxyStore, timeout)
		}
		if rec, ok := proxyStore.(pool.ResultRecorder); ok {
			rec.RecordResult(proxyAddr, true)
		}
		return conn, nil
	} else if strings.HasPrefix(proxyAddr, "http://") || strings.HasPrefix(proxyAddr, "https://") {
		proxyURL, err := url.Parse(proxyAddr)
		if err != nil {
			if rec, ok := proxyStore.(pool.ResultRecorder); ok {
				rec.RecordResult(proxyAddr, false)
			}
			proxyStore.MarkInvalid(proxyAddr)
			return TransmitReqFromClient(network, address, proxyStore, timeout)
		}
		dialer := &net.Dialer{Timeout: timeoutDur}
		if network != "tcp" {
			return nil, fmt.Errorf("http/https代理仅支持tcp网络")
		}
		conn, err := dialer.Dial("tcp", proxyURL.Host)
		if err != nil {
			proxyStore.MarkInvalid(proxyAddr)
			logger.Info("%s无效，自动切换下一个......\n", proxyAddr)
			return TransmitReqFromClient(network, address, proxyStore, timeout)
		}
		target := address
		if !strings.Contains(target, ":") {
			target += ":80"
		}
		req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", target, target)
		if proxyURL.User != nil {
			user := proxyURL.User.Username()
			pass, _ := proxyURL.User.Password()
			auth := user + ":" + pass
			b64 := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
			req += "Proxy-Authorization: " + b64 + "\r\n"
		}
		req += "\r\n"
		_, err = conn.Write([]byte(req))
		if err != nil {
			if rec, ok := proxyStore.(pool.ResultRecorder); ok {
				rec.RecordResult(proxyAddr, false)
			}
			conn.Close()
			proxyStore.MarkInvalid(proxyAddr)
			return TransmitReqFromClient(network, address, proxyStore, timeout)
		}
		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil || !strings.Contains(string(buf[:n]), "200 Connection established") {
			if rec, ok := proxyStore.(pool.ResultRecorder); ok {
				rec.RecordResult(proxyAddr, false)
			}
			conn.Close()
			proxyStore.MarkInvalid(proxyAddr)
			return TransmitReqFromClient(network, address, proxyStore, timeout)
		}
		if rec, ok := proxyStore.(pool.ResultRecorder); ok {
			rec.RecordResult(proxyAddr, true)
		}
		return conn, nil
	}
	return nil, fmt.Errorf("未知代理类型")
}
