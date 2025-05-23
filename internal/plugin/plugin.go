package plugin

import (
	"fmt"
	"github.com/overflow0verture/proxy_harvester/internal/globals"
	"github.com/overflow0verture/proxy_harvester/internal/logger"
	"github.com/overflow0verture/proxy_harvester/internal/pool"
	"github.com/overflow0verture/proxy_harvester/internal/requests"
	"github.com/overflow0verture/proxy_harvester/internal/symbols"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"github.com/robfig/cron/v3"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"time"
)

// ProxyProvider 是所有代理来源的统一接口
// out: 通过通道返回代理（带协议前缀的字符串）
type ProxyProvider interface {
	Name() string                     // 返回代理源名称
	CronSpec() string                 // 返回定时执行表达式
	FetchProxies(chan<- string) error // 获取代理并发送到通道
}

// PluginEntry 记录已加载插件的信息
// Provider: 插件实例，Interp: yaegi 解释器
// Path: 插件文件路径
// CronID: 定时任务ID（可选）
type PluginEntry struct {
	Provider ProxyProvider
	Interp   *interp.Interpreter
	Path     string
	CronID   string
}

// 插件注册表，key为插件名
var pluginRegistry = make(map[string]*PluginEntry)
var pluginMu sync.Mutex
var pluginCron = cron.New()

// 全局插件管理器实例
var globalPluginManager *PluginManager

// PluginManager 插件管理器，负责插件的生命周期管理
type PluginManager struct {
	proxyStore pool.ProxyStore
	registry   map[string]*PluginEntry
	cron       *cron.Cron
	mu         sync.Mutex
}

// NewPluginManager 创建新的插件管理器
func NewPluginManager(proxyStore pool.ProxyStore) *PluginManager {
	return &PluginManager{
		proxyStore: proxyStore,
		registry:   make(map[string]*PluginEntry),
		cron:       cron.New(),
	}
}

// SetGlobalPluginManager 设置全局插件管理器
func SetGlobalPluginManager(manager *PluginManager) {
	globalPluginManager = manager
}

// LoadPlugin 加载函数式插件文件，返回 ProxyProvider 实例
func LoadPlugin(path string, proxyStore pool.ProxyStore) (ProxyProvider, *interp.Interpreter, error) {
	// 设置yaegi的工作选项，特别是在Windows下
	opts := interp.Options{
		GoPath: "",  // 不使用GOPATH
		Env:    []string{}, // 清空环境变量，避免路径冲突
	}
	
	i := interp.New(opts)

	// 注册标准库符号
	i.Use(stdlib.Symbols)
	
	// 初始化requests包，注入proxyStore
	requests.InitRequests(proxyStore)
	
	// 使用symbols包中的符号表
	i.Use(symbols.Symbols)

	logger.Debug("加载函数式插件: %s", path)
	_, err := i.EvalPath(path)
	if err != nil {
		logger.Debug("插件文件评估失败: %v", err)
		return nil, nil, err
	}

	// 获取插件变量 - 只支持函数映射格式
	v, err := i.Eval("main.Plugin")
	if err != nil {
		logger.Debug("获取main.Plugin失败: %v", err)
		return nil, nil, fmt.Errorf("插件必须导出 var Plugin = map[string]interface{}")
	}

	pluginVal := v.Interface()

	// 只支持函数映射格式
	pluginMap, ok := pluginVal.(map[string]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("插件必须使用函数映射格式: var Plugin = map[string]interface{}")
	}

	logger.Debug("检测到函数式插件")

	// 检查必需的函数是否存在
	nameFunc, hasName := pluginMap["Name"]
	cronFunc, hasCron := pluginMap["CronSpec"]
	fetchFunc, hasFetch := pluginMap["FetchProxies"]

	if !hasName || !hasCron || !hasFetch {
		return nil, nil, fmt.Errorf("插件缺少必需函数: Name=%v, CronSpec=%v, FetchProxies=%v",
			hasName, hasCron, hasFetch)
	}

	// 创建函数适配器
	adapter := &functionMapAdapter{
		nameFunc:  reflect.ValueOf(nameFunc),
		cronFunc:  reflect.ValueOf(cronFunc),
		fetchFunc: reflect.ValueOf(fetchFunc),
	}

	logger.Debug("函数式插件加载成功: %s", adapter.Name())
	return adapter, i, nil
}

// 函数映射适配器 - 将函数映射转换为ProxyProvider接口
type functionMapAdapter struct {
	nameFunc  reflect.Value
	cronFunc  reflect.Value
	fetchFunc reflect.Value
}

func (a *functionMapAdapter) Name() string {
	result := a.nameFunc.Call(nil)
	return result[0].String()
}

func (a *functionMapAdapter) CronSpec() string {
	result := a.cronFunc.Call(nil)
	return result[0].String()
}

func (a *functionMapAdapter) FetchProxies(out chan<- string) error {
	args := []reflect.Value{reflect.ValueOf(out)}
	result := a.fetchFunc.Call(args)
	if result[0].IsNil() {
		return nil
	}
	return result[0].Interface().(error)
}

// RegisterPlugin 注册插件到注册表
func RegisterPlugin(name string, entry *PluginEntry) {
	pluginMu.Lock()
	defer pluginMu.Unlock()
	pluginRegistry[name] = entry
}

// UnregisterPlugin 注销插件
func UnregisterPlugin(name string) {
	pluginMu.Lock()
	defer pluginMu.Unlock()
	delete(pluginRegistry, name)
}

// StartAllPluginsWithCron 启动所有已注册插件的定时任务
func StartAllPluginsWithCron(proxyStore pool.ProxyStore) {
	pluginMu.Lock()
	defer pluginMu.Unlock()
	pluginCron.Stop()
	pluginCron = cron.New()

	logger.Info("插件系统开始注册定时任务，共有 %d 个插件", len(pluginRegistry))

	for name, entry := range pluginRegistry {
		spec := entry.Provider.CronSpec()
		provider := entry.Provider
		path := entry.Path
		if spec != "" {
			_, err := pluginCron.AddFunc(spec, func() {
				// 执行前获取代理池数量
				beforeCount, _ := proxyStore.Len()

				logger.Plugin("开始执行 %s 插件的代理收集任务，执行前代理池有 %d 个代理", name, beforeCount)

				// 创建有缓冲的通道接收代理
				out := make(chan string, 1000)

				// 启动收集器
				go func() {
					if err := provider.FetchProxies(out); err != nil {
						logger.Error("%s 插件执行失败: %v", name, err)
					}
					close(out)
				}()

				// 启动转发到全局存储的协程
				count := 0
				for proxy := range out {
					globals.ToCheckChan <- proxy
					count++
				}

				logger.Plugin("%s 插件已提交 %d 个代理到验证队列", name, count)
			})
			if err == nil {
				entry.CronID = path + ":" + spec // 记录cron id
				logger.Plugin("%s 定时任务已注册: %s", name, spec)
			} else {
				logger.Error("%s 定时任务注册失败: %v", name, err)
			}
		}
	}

	logger.Info("插件定时任务注册完成，启动定时器")
	pluginCron.Start()
}

// ReloadPlugin 重新加载插件（文件变更时调用）
func ReloadPlugin(path string, proxyStore pool.ProxyStore) error {
	provider, interp, err := LoadPlugin(path, proxyStore)
	if err != nil {
		return err
	}
	name := provider.Name()

	pluginMu.Lock()
	defer pluginMu.Unlock()

	// 如果插件已存在，停止其定时任务并清理资源
	if existingEntry, exists := pluginRegistry[name]; exists {
		logger.Plugin("插件 %s 已存在，正在替换", name)

		// 清理旧的解释器资源
		if existingEntry.Interp != nil {
			existingEntry.Interp = nil
		}
	}

	// 创建新的插件条目
	entry := &PluginEntry{
		Provider: provider,
		Interp:   interp,
		Path:     path,
	}

	// 直接替换插件
	pluginRegistry[name] = entry

	// 只为这个插件启动新的定时任务
	spec := provider.CronSpec()
	if spec != "" {
		cronID, err := pluginCron.AddFunc(spec, func() {
			// 创建有缓冲的通道接收代理
			out := make(chan string, 1000)

			// 启动收集器
			go func() {
				defer close(out)
				if err := provider.FetchProxies(out); err != nil {
					logger.Error("%s 插件执行失败: %v", name, err)
				}
			}()

			// 启动转发到全局存储的协程
			count := 0
			for proxy := range out {
				globals.ToCheckChan <- proxy
				count++
			}

			logger.Plugin("%s 插件已提交 %d 个代理到验证队列", name, count)
		})
		if err == nil {
			entry.CronID = fmt.Sprintf("%d", cronID)
			logger.Plugin("%s 定时任务已重新启动: %s", name, spec)
		} else {
			logger.Error("%s 定时任务启动失败: %v", name, err)
		}
	}

	logger.Plugin("%s 重载成功", name)

	// 立即执行一次该插件的抓取任务
	go func() {
		logger.Plugin("立即执行一次 %s 插件的代理收集任务", name)

		// 创建有缓冲的通道接收代理
		out := make(chan string, 1000)

		// 启动收集器
		go func() {
			defer close(out)
			if err := provider.FetchProxies(out); err != nil {
				logger.Error("%s 首次执行失败: %v", name, err)
			}
		}()

		// 转发到全局检测通道
		count := 0
		for proxy := range out {
			globals.ToCheckChan <- proxy
			count++
		}

		logger.Plugin("%s 首次已提交 %d 个代理到验证队列", name, count)
	}()

	return nil
}

// RemovePluginByPath 根据文件路径移除插件
func RemovePluginByPath(path string) {
	path = filepath.Clean(path)
	logger.Plugin("尝试移除插件: %s", filepath.Base(path))

	pluginMu.Lock()
	defer pluginMu.Unlock()

	var targetName string
	var targetEntry *PluginEntry

	// 查找匹配路径的插件
	for name, entry := range pluginRegistry {
		entryPath := filepath.Clean(entry.Path)
		if entryPath == path {
			targetName = name
			targetEntry = entry
			break
		}
	}

	// 如果没有精确匹配路径，尝试匹配文件名
	if targetName == "" {
		filename := filepath.Base(path)
		for name, entry := range pluginRegistry {
			entryFilename := filepath.Base(entry.Path)
			if entryFilename == filename {
				targetName = name
				targetEntry = entry
				logger.Plugin("按文件名匹配到插件: %s (路径: %s)", name, entry.Path)
				break
			}
		}
	}

	if targetName == "" {
		logger.Plugin("未找到要移除的插件: %s", filepath.Base(path))
		return
	}

	// 停止该插件的定时任务
	if targetEntry.CronID != "" {
		logger.Debug("停止插件 %s 的定时任务", targetName)
		targetEntry.CronID = ""
	}

	// 清理解释器资源
	if targetEntry.Interp != nil {
		targetEntry.Interp = nil
	}

	// 从注册表中移除
	delete(pluginRegistry, targetName)
	logger.Plugin("成功移除插件: %s", targetName)

	// 触发垃圾回收
	go func() {
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	}()
}

// InitPluginSystem 初始化插件系统
func InitPluginSystem() {
	// 启动全局cron调度器
	pluginCron.Start()

	logger.Info("插件系统初始化完成")
}
