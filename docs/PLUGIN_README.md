# 插件开发示例

ip_test.go 测试requests能否正常使用

## 推荐格式：函数式插件

```go
// 推荐：函数式插件格式
package main

import (
    "github.com/overflow0verture/proxy_harvester/internal/logger"
    "github.com/overflow0verture/proxy_harvester/internal/requests"
)

// PluginName 返回插件名称
func PluginName() string {
    return "我的代理爬虫"
}

// PluginCronSpec 返回定时任务表达式
func PluginCronSpec() string {
    return "0 */5 * * * *" // 每5分钟执行一次
}

// PluginFetchProxies 获取代理
func PluginFetchProxies(out chan<- string) error {
    logger.Plugin("开始执行代理收集")
    
    // 使用requests API发送请求
    resp, err := requests.Get("https://api.example.com/proxies", requests.RequestOptions{
        Proxies: true,
        Headers: map[string]string{
            "User-Agent": "MyBot/1.0",
        },
    })
    
    if err != nil {
        return err
    }
    
    // 解析响应并发送代理返回一定要以 协议://用户名:密码@ip:port 或 协议://@ip:port
    // ...

    // 解析到代理后发送到out中
    // out <- proxy
    
    return nil
}

// 导出插件变量 - 必须使用这种格式
var Plugin = map[string]interface{}{
    "Name":         PluginName,
    "CronSpec":     PluginCronSpec,
    "FetchProxies": PluginFetchProxies,
}
```


## 使用requests API

### 1. 自动代理管理
```go
// 自动使用代理池
resp, err := requests.Get(url, requests.RequestOptions{
    Proxies: true, // 自动从代理池选择代理
})
```

### 2. 简化的API
```go
// GET请求
resp, err := requests.Get(url, options)

// POST请求 
resp, err := requests.Post(url, requests.RequestOptions{
    JSON: map[string]interface{}{
        "key": "value",
    },
})

// 解析JSON响应
var result MyStruct
err = resp.JSON(&result)
```

### 3. 会话支持
```go
session := requests.NewSession()
resp1, _ := session.Get("https://example.com/login")
resp2, _ := session.Post("https://example.com/api", options)
```

### 4. RequestOptions 完整字段说明
```go
options := requests.RequestOptions{
    Headers: map[string]string{        // 自定义请求头
        "User-Agent": "MyBot/1.0",
        "Authorization": "Bearer token",
    },
    Timeout: 30,                       // 超时时间（秒），默认30秒
    Proxies: true,                     // 是否使用代理池，默认true
    Params: map[string]string{         // URL参数
        "page": "1",
        "size": "10",
    },
    JSON: map[string]interface{}{      // JSON数据（POST/PUT请求）
        "key": "value",
    },
    Data: "form data",                 // 表单数据或原始数据
}
```

## 完整插件示例

### SCDN API爬虫（使用requests）
参见：`scdn_requests.go`

### FOFA API爬虫（使用requests）
参见：`fofa_requests.go`

## 插件推荐配置

1. **使用配置结构体**：统一管理插件配置
2. **开关控制**：添加Switch字段控制插件启用
3. **合理的执行频率**：不要设置过高的执行频率
4. **错误处理**：妥善处理网络请求错误
5. **日志记录**：使用logger记录关键信息

## 测试插件

1. 将插件文件放入`plugins/`目录
2. 重启程序，观察日志输出
3. 检查代理池是否有新增代理

## 故障排除

1. **插件无法加载**：检查函数名是否正确，确保使用函数映射格式
2. **网络请求失败**：检查URL和参数是否正确
3. **JSON解析失败**：检查API响应格式是否匹配结构体定义
4. **字段错误**：确保使用正确的RequestOptions字段名（Proxies而非UseProxy） 