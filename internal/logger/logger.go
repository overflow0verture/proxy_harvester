package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gookit/color"
)

var (
	infoLogger  *log.Logger
	errorLogger *log.Logger
	stdLogger   *log.Logger
	
	infoFile  *os.File
	errorFile *os.File
	
	enabled bool
	logMutex sync.Mutex
	
	// 记录最后一次汇总IP数量的时间
	lastIPSummary time.Time
	
	// IP汇总间隔时间，默认5分钟
	IPSummaryInterval = 5 * time.Minute
	
	// 颜色样式定义
	infoStyle    = color.New(color.FgLightBlue, color.Bold)
	errorStyle   = color.New(color.FgLightRed, color.Bold)
	debugStyle   = color.New(color.FgGray)
	pluginStyle  = color.New(color.FgLightGreen, color.Bold)
	poolStyle    = color.New(color.FgLightCyan, color.Bold)
	summaryStyle = color.New(color.FgLightYellow, color.Bold)
	successStyle = color.New(color.FgLightGreen, color.Bold)
	warningStyle = color.New(color.FgLightYellow, color.Bold)
)

// Setup 初始化日志系统
func Setup(logEnabled bool, logDir string) error {
	logMutex.Lock()
	defer logMutex.Unlock()
	
	// 设置日志开关
	enabled = logEnabled
	
	// 标准输出日志
	stdLogger = log.New(os.Stdout, "", log.LstdFlags)
	
	// 如果不启用文件日志，直接返回
	if !enabled {
		infoLogger = stdLogger
		errorLogger = stdLogger
		return nil
	}
	
	// 确保日志目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %v", err)
	}
	
	// 关闭之前可能存在的文件
	if infoFile != nil {
		infoFile.Close()
	}
	if errorFile != nil {
		errorFile.Close()
	}
	
	// 打开日志文件
	var err error
	infoFile, err = os.OpenFile(filepath.Join(logDir, "info.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("打开info日志文件失败: %v", err)
	}
	
	errorFile, err = os.OpenFile(filepath.Join(logDir, "error.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		infoFile.Close()
		return fmt.Errorf("打开error日志文件失败: %v", err)
	}
	
	// 创建日志器
	infoLogger = log.New(infoFile, "", log.LstdFlags)
	errorLogger = log.New(errorFile, "", log.LstdFlags)
	
	// 初始化IP汇总时间
	lastIPSummary = time.Now()
	
	return nil
}

// Info 记录一般信息
func Info(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	
	// 控制台输出带颜色
	coloredMessage := infoStyle.Sprintf("[INFO]") + " " + message
	stdLogger.Printf("%s", coloredMessage)
	
	// 文件输出不带颜色
	if enabled {
		logMutex.Lock()
		defer logMutex.Unlock()
		infoLogger.Printf("%s", message)
	}
}

// Error 记录错误信息
func Error(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	
	// 控制台输出带颜色（错误信息和标签都用红色）
	coloredMessage := errorStyle.Sprintf("[ERROR]") + " " + color.FgRed.Sprintf(message)
	stdLogger.Printf("%s", coloredMessage)
	
	// 文件输出不带颜色
	if enabled {
		logMutex.Lock()
		defer logMutex.Unlock()
		errorLogger.Printf("%s", message)
	}
}

// Debug 调试信息，仅在控制台显示，不写入文件
func Debug(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	
	// 控制台输出带颜色（调试信息用灰色）
	coloredMessage := debugStyle.Sprintf("[DEBUG]") + " " + debugStyle.Sprintf(message)
	stdLogger.Printf("%s", coloredMessage)
}

// Plugin 插件相关日志
func Plugin(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	
	// 控制台输出带颜色
	coloredMessage := pluginStyle.Sprintf("[插件]") + " " + color.FgGreen.Sprintf(message)
	stdLogger.Printf("%s", coloredMessage)
	
	// 文件输出不带颜色
	if enabled {
		logMutex.Lock()
		defer logMutex.Unlock()
		infoLogger.Printf("[插件] %s", message)
	}
}

// ProxyPool 代理池相关日志
func ProxyPool(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	
	// 控制台输出带颜色
	coloredMessage := poolStyle.Sprintf("[代理池]") + " " + color.FgCyan.Sprintf(message)
	stdLogger.Printf("%s", coloredMessage)
	
	// 文件输出不带颜色
	if enabled {
		logMutex.Lock()
		defer logMutex.Unlock()
		infoLogger.Printf("[代理池] %s", message)
	}
}

// IPSummary 汇总代理池IP数量
// count: 当前可用IP数量
// force: 是否强制输出，不考虑时间间隔
func IPSummary(count int, force bool) {
	now := time.Now()
	
	// 如果不是强制输出，且距离上次汇总时间未达到设定间隔，则不输出
	if !force && now.Sub(lastIPSummary) < IPSummaryInterval {
		return
	}
	
	// 更新最后汇总时间
	lastIPSummary = now
	
	message := fmt.Sprintf("当前代理池中共有 %d 个可用代理", count)
	
	// 控制台输出带颜色
	coloredMessage := summaryStyle.Sprintf("[代理池汇总]") + " " + color.FgYellow.Sprintf(message)
	stdLogger.Printf("%s", coloredMessage)
	
	// 文件输出不带颜色
	if enabled {
		logMutex.Lock()
		defer logMutex.Unlock()
		infoLogger.Printf("[代理池汇总] %s", message)
	}
}

// Success 成功信息日志
func Success(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	
	// 控制台输出带颜色
	coloredMessage := successStyle.Sprintf("[SUCCESS]") + " " + color.FgGreen.Sprintf(message)
	stdLogger.Printf("%s", coloredMessage)
	
	// 文件输出不带颜色
	if enabled {
		logMutex.Lock()
		defer logMutex.Unlock()
		infoLogger.Printf("[SUCCESS] %s", message)
	}
}

// Warning 警告信息日志
func Warning(format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)
	
	// 控制台输出带颜色
	coloredMessage := warningStyle.Sprintf("[WARNING]") + " " + color.FgYellow.Sprintf(message)
	stdLogger.Printf("%s", coloredMessage)
	
	// 文件输出不带颜色
	if enabled {
		logMutex.Lock()
		defer logMutex.Unlock()
		infoLogger.Printf("[WARNING] %s", message)
	}
}

// GetColorSupport 返回当前是否支持颜色
func GetColorSupport() bool {
	return color.SupportColor()
}

// SetColorSupport 手动设置颜色支持
func SetColorSupport(support bool) {
	if support {
		color.ForceOpenColor()
	} else {
		color.Disable()
	}
}

// GetColorLevel 获取颜色支持级别
func GetColorLevel() int {
	level := color.TermColorLevel()
	switch level {
	case color.LevelNo:
		return 0 // 不支持颜色
	case color.Level16:
		return 16 // 16色
	case color.Level256:
		return 256 // 256色
	case color.LevelRgb:
		return 16777216 // RGB真彩色
	default:
		return 0
	}
}

// EnableTrueColor 启用真彩色模式
func EnableTrueColor() {
	color.ForceOpenColor()
}

// CreateCustomStyle 创建自定义颜色样式
func CreateCustomStyle(fg, bg color.Color, opts ...color.Color) color.Style {
	colors := []color.Color{fg}
	if bg != 0 {
		colors = append(colors, bg)
	}
	colors = append(colors, opts...)
	return color.New(colors...)
}

// PrintColored 打印彩色文本（用于测试）
func PrintColored() {
	Info("这是一条信息日志")
	Error("这是一条错误日志")
	Debug("这是一条调试日志")
	Plugin("这是一条插件日志")
	ProxyPool("这是一条代理池日志")
	Success("这是一条成功日志")
	Warning("这是一条警告日志")
	IPSummary(100, true)
	
	// 显示颜色支持信息
	Info("颜色支持: %v", GetColorSupport())
	Info("颜色级别: %d", GetColorLevel())
	
	// 展示不同颜色
	fmt.Println()
	color.Red.Println("🔴 红色文本")
	color.Green.Println("🟢 绿色文本")
	color.Blue.Println("🔵 蓝色文本")
	color.Yellow.Println("🟡 黄色文本")
	color.Cyan.Println("🟦 青色文本")
	color.Magenta.Println("🟣 紫色文本")
	
	// 展示样式组合
	fmt.Println()
	color.Bold.Println("粗体文本")
	color.OpItalic.Println("斜体文本")
	color.OpUnderscore.Println("下划线文本")
	
	// 展示RGB颜色
	fmt.Println()
	color.RGB(255, 100, 50).Println("🎨 RGB自定义颜色")
	color.HEX("#00ff88").Println("�� HEX颜色 #00ff88")
}

// Close 关闭日志文件
func Close() {
	logMutex.Lock()
	defer logMutex.Unlock()
	
	if infoFile != nil {
		infoFile.Close()
		infoFile = nil
	}
	
	if errorFile != nil {
		errorFile.Close()
		errorFile = nil
	}
} 