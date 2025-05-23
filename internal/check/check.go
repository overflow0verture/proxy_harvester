package check

import (
	"github.com/overflow0verture/proxy_harvester/internal/config"
	"github.com/overflow0verture/proxy_harvester/internal/globals"
	"github.com/overflow0verture/proxy_harvester/internal/logger"
	"github.com/overflow0verture/proxy_harvester/internal/pool"
	"context"
	"crypto/tls"
	"golang.org/x/net/proxy"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type checkJob struct {
	Proxy string
}

type checkResult struct {
	Proxy string
	Alive bool
}

// Worker pool高并发检测
func CheckSocks(checkSocks config.CheckSocksConfig, socksListParam []string, proxyStore pool.ProxyStore) {
	startTime := time.Now()
	maxWorkers := checkSocks.MaxConcurrentReq
	timeout := checkSocks.Timeout

	checkRspKeywords := checkSocks.CheckRspKeywords
	checkGeolocateConfig := checkSocks.CheckGeolocate
	checkGeolocateSwitch := checkGeolocateConfig.Switch
	isOpenGeolocateSwitch := false
	reqUrl := checkSocks.CheckURL
	if checkGeolocateSwitch == "open" {
		isOpenGeolocateSwitch = true
		reqUrl = checkGeolocateConfig.CheckURL
	}

	logger.Info("开始批量检测代理，并发: %v, 超时标准: %vs", maxWorkers, timeout)

	jobs := make(chan checkJob, len(socksListParam))
	results := make(chan checkResult, len(socksListParam))

	// 启动worker
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				alive := checkProxyAlive(job.Proxy, reqUrl, timeout, checkRspKeywords, isOpenGeolocateSwitch, checkGeolocateConfig)
				results <- checkResult{Proxy: job.Proxy, Alive: alive}
			}
		}()
	}

	// 投递任务
	for _, proxy := range socksListParam {
		jobs <- checkJob{Proxy: proxy}
	}
	close(jobs)

	total := len(socksListParam)
	valid := 0
	for i := 0; i < total; i++ {
		res := <-results
		if res.Alive {
			proxyStore.Add(res.Proxy)
			valid++
		} else {
			proxyStore.MarkInvalid(res.Proxy)
		}
	}

	wg.Wait()

	cnt, _ := proxyStore.Len()
	sec := int(time.Since(startTime).Seconds())
	if sec == 0 {
		sec = 1
	}
	logger.Info("批量检测完成，用时 %vs，发现 %v 个可用代理，总代理池数量: %v", sec, valid, cnt)
}

// 检测单个代理是否可用，支持socks5/http/https认证代理
func checkProxyAlive(proxyAddr, reqUrl string, timeout int, checkRspKeywords string, isOpenGeolocateSwitch bool, checkGeolocateConfig config.CheckGeolocateConfig) bool {
	var client *http.Client
	var transport *http.Transport

	if strings.HasPrefix(proxyAddr, "socks5://") {
		u, err := url.Parse(proxyAddr)
		if err != nil {
			return false
		}
		host := u.Host
		var auth *proxy.Auth
		if u.User != nil {
			user := u.User.Username()
			pass, _ := u.User.Password()
			auth = &proxy.Auth{User: user, Password: pass}
		}
		dialer := &net.Dialer{Timeout: time.Duration(timeout) * time.Second}
		socksDialer, err := proxy.SOCKS5("tcp", host, auth, dialer)
		if err != nil {
			return false
		}
		tr := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return socksDialer.Dial(network, addr)
			},
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client = &http.Client{
			Transport: tr,
			Timeout:   time.Duration(timeout) * time.Second,
		}
	} else if strings.HasPrefix(proxyAddr, "http://") || strings.HasPrefix(proxyAddr, "https://") {
		proxyURL, err := url.Parse(proxyAddr)
		if err != nil {
			return false
		}
		transport = &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client = &http.Client{
			Transport: transport,
			Timeout:   time.Duration(timeout) * time.Second,
		}
	} else {
		return false
	}

	req, err := http.NewRequest("GET", reqUrl, nil)
	if err != nil {
		return false
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36 Edg/112.0.1722.17")
	req.Header.Add("referer", "https://www.baidu.com/s?ie=utf-8&f=8&rsv_bp=1&rsv_idx=1&tn=baidu&wd=ip&fenlei=256&rsv_pq=0xc23dafcc00076e78&rsv_t=6743gNBuwGYWrgBnSC7Yl62e52x3CKQWYiI10NeKs73cFjFpwmqJH%2FOI%2FSRG&rqlang=en&rsv_dl=tb&rsv_enter=1&rsv_sug3=5&rsv_sug1=5&rsv_sug7=101&rsv_sug2=0&rsv_btype=i&prefixsug=ip&rsp=4&inputT=2165&rsv_sug4=2719")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}
	stringBody := string(body)
	if !isOpenGeolocateSwitch {
		if !strings.Contains(stringBody, checkRspKeywords) {
			return false
		}
	} else {
		for _, keyword := range checkGeolocateConfig.ExcludeKeywords {
			if strings.Contains(stringBody, keyword) {
				return false
			}
		}
		for _, keyword := range checkGeolocateConfig.IncludeKeywords {
			if !strings.Contains(stringBody, keyword) {
				return false
			}
		}
	}
	return true
}

func StartCheckWorkers(workerNum int, checkCfg config.CheckSocksConfig, proxyStore pool.ProxyStore) {
	logger.Info("启动 %d 个代理检测工作线程", workerNum)
	for i := 0; i < workerNum; i++ {
		go checkWorker(checkCfg, proxyStore)
	}
}

// 检测worker，从ToCheckChan取代理，检测通过才入库
func checkWorker(checkCfg config.CheckSocksConfig, proxyStore pool.ProxyStore) {
	for proxy := range globals.ToCheckChan {
		if checkProxyAlive(proxy, checkCfg.CheckURL, checkCfg.Timeout, checkCfg.CheckRspKeywords, checkCfg.CheckGeolocate.Switch == "open", checkCfg.CheckGeolocate) {
			proxyStore.Add(proxy)
		}
		// 检测失败自动丢弃
	}
}
