# Proxy Harvester API 服务器

## 概述

Proxy Harvester API 提供了一个简单的 HTTP 接口，允许其他工具通过 REST API 访问代理池。支持获取指定数量和类型的代理，并提供 token 认证机制。

## 配置

在 `configs/config.toml` 文件中配置 API 服务器：

```toml
[apiserver]
switch = 'open'     # 开启API服务器 (open/close)
token = "atoken"    # API访问令牌
port = 10087        # 服务端口
```

## API 接口

### 1. 获取代理列表

**请求方式：** `GET`  
**路径：** `/api/proxies`

**参数：**
- `token` (必需) - 认证令牌
- `count` (可选) - 获取数量，默认10，范围1-100
- `type` (可选) - 代理类型过滤，支持 `socks5`、`http`、`https`

**逻辑说明：**
- 如果代理池中的代理数量少于请求数量，将返回所有可用的代理
- 单次请求最多返回100个代理
- 支持按代理类型过滤，过滤可能导致实际返回数量小于请求数量

**示例请求：**
```bash
# 获取10个代理（默认）
curl "http://localhost:10087/api/proxies?token=atoken"

# 获取5个socks5代理
curl "http://localhost:10087/api/proxies?token=atoken&count=5&type=socks5"

# 获取20个http代理
curl "http://localhost:10087/api/proxies?token=atoken&count=20&type=http"

# 尝试获取1000个代理（实际最多返回100个或代理池总数）
curl "http://localhost:10087/api/proxies?token=atoken&count=1000"
```

**响应格式：**
```json
{
  "code": 200,
  "message": "获取成功",
  "data": [
    "socks5://1.2.3.4:1080",
    "socks5://5.6.7.8:1080"
  ],
  "count": 2,
  "total": 150
}
```

### 2. 获取代理池状态

**请求方式：** `GET`  
**路径：** `/api/status`

**参数：**
- `token` (必需) - 认证令牌

**示例请求：**
```bash
curl "http://localhost:10087/api/status?token=atoken"
```

**响应格式：**
```json
{
  "code": 200,
  "message": "状态正常",
  "total": 150,
  "timestamp": 1703123456
}
```

### 3. 首页文档

**请求方式：** `GET`  
**路径：** `/`

访问 `http://localhost:10087/` 可查看在线API文档。

## 错误响应

当请求失败时，API 会返回相应的错误信息：

```json
{
  "code": 401,
  "message": "token无效"
}
```

**常见错误代码：**
- `400` - 参数错误
- `401` - 认证失败
- `405` - 请求方法不支持
- `500` - 服务器内部错误

## 使用示例

### Python 示例

```python
import requests

API_BASE = "http://localhost:10087"
TOKEN = "atoken"

def get_proxies(count=10, proxy_type=None):
    """获取代理列表"""
    params = {"token": TOKEN, "count": count}
    if proxy_type:
        params["type"] = proxy_type
    
    response = requests.get(f"{API_BASE}/api/proxies", params=params)
    return response.json()

def get_status():
    """获取状态"""
    response = requests.get(f"{API_BASE}/api/status", params={"token": TOKEN})
    return response.json()

# 使用示例
status = get_status()
print(f"代理池总数: {status['total']}")

# 请求5个socks5代理
proxies = get_proxies(count=5, proxy_type="socks5")
print(f"请求5个，实际获取到 {proxies['count']} 个代理")

# 请求大量代理（测试限制）
large_request = get_proxies(count=500)
print(f"请求500个，实际获取到 {large_request['count']} 个代理（最多100个）")

# 当代理池较小时的处理
if status['total'] < 20:
    all_proxies = get_proxies(count=50)  # 请求50个，但可能只返回总数
    print(f"代理池只有 {status['total']} 个，实际返回 {all_proxies['count']} 个")

for proxy in proxies['data']:
    print(f"代理: {proxy}")
```


## 安全说明

1. **Token 认证**：所有 API 请求都需要有效的 token
2. **速率限制**：建议不要过于频繁地请求 API
3. **代理使用**：获取的代理可能随时失效，请做好重试机制
4. **内网访问**：默认只监听本地，如需外网访问请谨慎配置防火墙

## 故障排除

### 常见问题

1. **连接被拒绝**
   - 检查 API 服务器是否启动
   - 确认端口配置是否正确
   - 检查防火墙设置

2. **401 认证失败**
   - 检查 token 是否正确
   - 确认 token 参数是否包含在请求中

3. **代理池为空**
   - 检查插件是否正常工作
   - 查看日志确认代理收集情况
   - 等待插件收集足够的代理

4. **获取的代理无效**
   - 代理可能已失效，这是正常现象
   - 实现重试机制，多获取几个代理
   - 使用前可以先测试代理连通性

### 日志查看

API 服务器的日志会记录在主程序日志中，可以通过日志查看请求情况和错误信息。

## 许可证

与主项目相同的许可证。 