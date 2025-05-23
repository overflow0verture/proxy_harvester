package plugin

import (
	"github.com/fsnotify/fsnotify"
	"github.com/overflow0verture/proxy_harvester/internal/config"
	"github.com/overflow0verture/proxy_harvester/internal/logger"
	"github.com/overflow0verture/proxy_harvester/internal/pool"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// 用于防止过于频繁的文件变更事件触发
var lastEvents = struct {
	sync.Mutex
	timestamps map[string]time.Time
}{
	timestamps: make(map[string]time.Time),
}

// 事件节流，确保同一个文件的事件处理间隔至少为指定的时间
func shouldProcessEvent(path string, minInterval time.Duration) bool {
	lastEvents.Lock()
	defer lastEvents.Unlock()

	now := time.Now()
	lastTime, exists := lastEvents.timestamps[path]

	if !exists || now.Sub(lastTime) > minInterval {
		lastEvents.timestamps[path] = now
		return true
	}

	return false
}

// 启动插件目录监控
func WatchPluginFolder(cfg config.PluginConfig, proxyStore pool.ProxyStore) error {
	pluginDir := cfg.PluginFolder

	// 创建插件目录(如果不存在)
	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		logger.Plugin("插件目录不存在，创建目录: %s", pluginDir)
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			logger.Error("创建插件目录失败: %v", err)
		}
	}

	logger.Plugin("开始监控插件目录: %s", pluginDir)

	// 启动时先加载所有已有插件
	LoadAllPluginsOnStart(pluginDir, proxyStore)

	// 启动文件监控
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	err = watcher.Add(pluginDir)
	if err != nil {
		watcher.Close()
		return err
	}

	logger.Plugin("文件监控已启动，将监控插件文件的变化")

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// 只处理Go文件
				if !strings.HasSuffix(event.Name, ".go") {
					continue
				}

				// 事件节流，防止过于频繁的处理
				if !shouldProcessEvent(event.Name, 2*time.Second) {
					continue
				}

				switch {
				case event.Op&fsnotify.Create == fsnotify.Create:
					logger.Plugin("检测到新增文件: %s", filepath.Base(event.Name))
					if err := ReloadPlugin(event.Name, proxyStore); err != nil {
						logger.Error("新增插件加载失败: %v", err)
					}

				case event.Op&fsnotify.Write == fsnotify.Write:
					logger.Plugin("检测到文件修改: %s", filepath.Base(event.Name))
					if err := ReloadPlugin(event.Name, proxyStore); err != nil {
						logger.Error("修改插件重载失败: %v", err)
					}

				case event.Op&fsnotify.Remove == fsnotify.Remove,
					event.Op&fsnotify.Rename == fsnotify.Rename:
					logger.Plugin("检测到文件删除: %s", filepath.Base(event.Name))
					RemovePluginByPath(event.Name)
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Error("插件监控错误: %v", err)
			}
		}
	}()
	return nil
}

// 启动时加载所有插件
func LoadAllPluginsOnStart(pluginDir string, proxyStore pool.ProxyStore) {
	files, err := os.ReadDir(pluginDir)
	if err != nil {
		logger.Error("启动时读取插件目录失败: %v", err)
		return
	}

	goFiles := 0
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".go") {
			goFiles++
		}
	}

	if goFiles == 0 {
		logger.Plugin("插件目录中未发现Go文件: %s", pluginDir)
		return
	}

	logger.Plugin("启动时加载插件目录: %s，发现 %d 个Go文件", pluginDir, goFiles)

	// 清空现有插件注册表，确保干净启动
	pluginMu.Lock()
	pluginRegistry = make(map[string]*PluginEntry)
	pluginMu.Unlock()

	successCount := 0
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".go") {
			fullPath := filepath.Join(pluginDir, f.Name())

			// 直接使用ReloadPlugin，简化逻辑
			if err := ReloadPlugin(fullPath, proxyStore); err != nil {
				logger.Error("启动加载 %s 失败: %v", f.Name(), err)
			} else {
				successCount++
			}

			// 记录插件文件的最后修改时间
			if fileInfo, err := os.Stat(fullPath); err == nil {
				lastEvents.Lock()
				lastEvents.timestamps[fullPath] = fileInfo.ModTime()
				lastEvents.Unlock()
			}
		}
	}

	logger.Plugin("启动时成功加载 %d/%d 个插件", successCount, goFiles)
}

// 更新插件文件的最后修改时间，强制下次检测重新加载
//func UpdatePluginTimestamp(path string) error {
//	// 打开文件
//	file, err := os.OpenFile(path, os.O_RDWR, 0644)
//	if err != nil {
//		return fmt.Errorf("无法打开插件文件: %w", err)
//	}
//	defer file.Close()
//
//	// 获取当前内容
//	stat, err := file.Stat()
//	if err != nil {
//		return fmt.Errorf("无法获取文件信息: %w", err)
//	}
//
//	// 只需修改一下文件的访问时间和修改时间
//	now := time.Now()
//	err = os.Chtimes(path, now, now)
//	if err != nil {
//		return fmt.Errorf("无法更新文件时间戳: %w", err)
//	}
//
//	logger.Debug("已更新文件时间戳: %s, 大小: %d 字节", path, stat.Size())
//	return nil
//}
