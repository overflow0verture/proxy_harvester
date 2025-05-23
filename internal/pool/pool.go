package pool

import (
	"github.com/overflow0verture/proxy_harvester/internal/config"
	"github.com/overflow0verture/proxy_harvester/internal/logger"
	"bufio"
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"os"
	"strings"
	"sync"
	"time"
)

// ProxyStore 代理池统一接口，支持本地文件和Redis两种实现
// 所有代理操作都通过该接口，便于热切换和扩展
type ProxyStore interface {
	Add(proxy string) error         // 添加代理
	Remove(proxy string) error      // 删除代理
	GetAll() ([]string, error)      // 获取所有代理
	GetNext() (string, error)       // 获取下一个可用代理（带令牌桶限速）
	MarkInvalid(proxy string) error // 标记代理无效并剔除
	Len() (int, error)              // 获取代理池长度
}

// 本地文件实现（内存+锁，适合小规模）
type FileProxyStore struct {
	proxies  []string
	index    int
	mu       sync.Mutex
	tokenMap map[string]chan struct{}
	rate     int
	filename string // 新增字段
}

// 新建本地文件代理池，rate为每个代理每秒最大使用次数
func NewFileProxyStore(filename string, rate int) *FileProxyStore {
	store := &FileProxyStore{
		proxies:  make([]string, 0),
		index:    0,
		tokenMap: make(map[string]chan struct{}),
		filename: filename,
		rate:     rate,
	}
	
	// 尝试从文件加载现有代理
	if err := store.loadFromFile(); err != nil {
		logger.Error("无法从文件加载代理: %v", err)
	}
	
	return store
}

// 从文件加载代理列表
func (s *FileProxyStore) loadFromFile() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	file, err := os.Open(s.filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在不是错误
		}
		return err
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	count := 0
	
	// 清空现有代理列表
	s.proxies = make([]string, 0)
	
	for scanner.Scan() {
		proxy := strings.TrimSpace(scanner.Text())
		if proxy != "" {
			s.proxies = append(s.proxies, proxy)
			count++
		}
	}
	
	if err := scanner.Err(); err != nil {
		return err
	}
	
	return nil
}

// 保存代理列表到文件
func (s *FileProxyStore) saveToFile() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	file, err := os.Create(s.filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	writer := bufio.NewWriter(file)
	for _, proxy := range s.proxies {
		fmt.Fprintln(writer, proxy)
	}
	
	return writer.Flush()
}

// Add 添加代理并保存到文件
func (s *FileProxyStore) Add(proxy string) error {
	proxy = strings.TrimSpace(proxy)
	if proxy == "" {
		return nil
	}
	
	s.mu.Lock()
	
	// 检查重复
	for _, p := range s.proxies {
		if p == proxy {
			s.mu.Unlock()
			return nil
		}
	}
	
	s.proxies = append(s.proxies, proxy)
	s.mu.Unlock()
	
	// 每次添加都保存文件可能性能差，可以考虑延迟批量保存
	return s.saveToFile()
}

// Remove 删除代理并更新文件
func (s *FileProxyStore) Remove(proxy string) error {
	s.mu.Lock()
	
	found := false
	for i, p := range s.proxies {
		if p == proxy {
			// 移除找到的元素
			s.proxies = append(s.proxies[:i], s.proxies[i+1:]...)
			found = true
			break
		}
	}
	
	s.mu.Unlock()
	
	if found {
		return s.saveToFile()
	}
	
	return nil
}

// GetAll 获取所有代理
func (s *FileProxyStore) GetAll() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	result := make([]string, len(s.proxies))
	copy(result, s.proxies)
	
	return result, nil
}

// GetNext 获取下一个可用代理（带令牌桶限速）
func (s *FileProxyStore) GetNext() (string, error) {
	s.mu.Lock()
	
	if len(s.proxies) == 0 {
		s.mu.Unlock()
		return "", fmt.Errorf("代理池为空")
	}
	
	// 轮询选择下一个代理
	s.index = (s.index + 1) % len(s.proxies)
	proxy := s.proxies[s.index]
	
	s.mu.Unlock()
	
	// 限速控制：确保每个代理的使用不超过指定速率
	tokenCh, exists := s.tokenMap[proxy]
	if !exists {
		// 为此代理创建令牌桶
		tokenCh = make(chan struct{}, s.rate)
		s.tokenMap[proxy] = tokenCh
		
		// 初始填充令牌
		for i := 0; i < s.rate; i++ {
			tokenCh <- struct{}{}
		}
		
		// 启动令牌生成器
		go func(ch chan struct{}) {
			ticker := time.NewTicker(time.Second / time.Duration(s.rate))
			defer ticker.Stop()
			
			for range ticker.C {
				select {
				case ch <- struct{}{}:
					// 添加成功
				default:
					// 桶已满，跳过
				}
			}
		}(tokenCh)
	}
	
	// 获取令牌
	<-tokenCh
	
	return proxy, nil
}

// MarkInvalid 标记代理无效并剔除
func (s *FileProxyStore) MarkInvalid(proxy string) error {
	return s.Remove(proxy)
}

// Len 获取代理池长度
func (s *FileProxyStore) Len() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	return len(s.proxies), nil
}

// Redis实现
type RedisProxyStore struct {
	client  *redis.Client
	key     string
	ctx     context.Context
	index   int
	mu      sync.Mutex
	proxies []string // 缓存
	rate    int
}

// 新建Redis代理池
func NewRedisProxyStore(host string, port int, password string, rate int) *RedisProxyStore {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", host, port),
		Password: password,
		DB:       0,
	})
	
	store := &RedisProxyStore{
		client:  client,
		key:     "proxy_pool",
		ctx:     context.Background(),
		index:   0,
		proxies: make([]string, 0),
		rate:    rate,
	}
	
	// 初始加载缓存
	if members, err := client.SMembers(store.ctx, store.key).Result(); err == nil {
		store.proxies = members
		logger.ProxyPool("已从Redis加载 %d 个代理", len(members))
	} else {
		logger.Error("从Redis加载代理失败: %v", err)
	}
	
	return store
}

// Add 添加代理到Redis
func (s *RedisProxyStore) Add(proxy string) error {
	proxy = strings.TrimSpace(proxy)
	if proxy == "" {
		return nil
	}
	
	// 先检查是否已存在
	if s.client.SIsMember(s.ctx, s.key, proxy).Val() {
		return nil
	}
	
	// 添加到Redis
	err := s.client.SAdd(s.ctx, s.key, proxy).Err()
	if err != nil {
		return err
	}
	
	// 更新本地缓存
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for _, p := range s.proxies {
		if p == proxy {
			return nil // 已在缓存中
		}
	}
	
	s.proxies = append(s.proxies, proxy)
	return nil
}

// Remove 从Redis删除代理
func (s *RedisProxyStore) Remove(proxy string) error {
	// 从Redis删除
	err := s.client.SRem(s.ctx, s.key, proxy).Err()
	
	// 更新本地缓存
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for i, p := range s.proxies {
		if p == proxy {
			s.proxies = append(s.proxies[:i], s.proxies[i+1:]...)
			break
		}
	}
	
	return err
}

// GetAll 获取所有Redis代理
func (s *RedisProxyStore) GetAll() ([]string, error) {
	return s.client.SMembers(s.ctx, s.key).Result()
}

// GetNext 获取下一个Redis代理
func (s *RedisProxyStore) GetNext() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if len(s.proxies) == 0 {
		// 尝试重新加载
		members, err := s.client.SMembers(s.ctx, s.key).Result()
		if err != nil {
			return "", fmt.Errorf("代理池为空且无法重新加载: %v", err)
		}
		
		if len(members) == 0 {
			return "", fmt.Errorf("代理池为空")
		}
		
		s.proxies = members
	}
	
	// 轮询选择下一个代理
	s.index = (s.index + 1) % len(s.proxies)
	return s.proxies[s.index], nil
}

// MarkInvalid 标记Redis代理无效并剔除
func (s *RedisProxyStore) MarkInvalid(proxy string) error {
	return s.Remove(proxy)
}

// Len 获取Redis代理池长度
func (s *RedisProxyStore) Len() (int, error) {
	return int(s.client.SCard(s.ctx, s.key).Val()), nil
}

func InitProxyStore(cfg config.StorageConfig, rate int) ProxyStore {
	if cfg.Type == "redis" {
		logger.Info("使用Redis作为代理池存储")
		return NewRedisProxyStore(cfg.RedisHost, cfg.RedisPort, cfg.RedisPassword, rate)
	} else {
		logger.Info("使用本地文件作为代理池存储")
		return NewFileProxyStore(cfg.FileName, rate)
	}
}
