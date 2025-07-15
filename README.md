# Dirsearch-Go

一个用Go语言编写的高性能目录扫描工具，用于安全测试和渗透测试中发现网站的隐藏目录和文件。

## 功能特性

### 核心功能
- ✅ 高性能多线程并发扫描
- ✅ 支持多种输出格式 (控制台、JSON、CSV)
- ✅ 灵活的配置文件支持
- ✅ 请求重试机制
- ✅ 优雅停止机制 (Ctrl+C)
- ✅ 递归扫描功能
- ✅ 智能过滤系统

### 高级功能
- ✅ 响应内容过滤 (正则表达式、关键词、大小)
- ✅ 速率限制避免被封禁
- ✅ 详细日志记录
- ✅ 自定义HTTP头和用户代理
- ✅ SSL证书验证跳过
- ✅ 进度条显示

## 安装

### 从源码构建
```bash
go build -o dirsearch-go cmd/
```

### 使用预编译二进制文件
从[Releases](releases)页面下载适合您平台的预编译二进制文件。

## 使用方法

### 基本用法
```bash
# 基本扫描 (使用默认词典文件 dicc.txt)
./dirsearch-go -u https://www.baidu.com

# 指定词典文件
./dirsearch-go -u https://www.baidu.com -w custom_dict.txt

# 使用配置文件
./dirsearch-go -config config.json

# 输出到JSON文件
./dirsearch-go -u https://www.baidu.com -format json -o results.json

# 递归扫描
./dirsearch-go -u https://www.baidu.com -r -depth 3

# 启用速率限制
./dirsearch-go -u https://www.baidu.com -rate-limit -rps 5
```

### 命令行参数
```
-u string          目标URL (例如: https://www.baidu.com)
-w string          词典文件路径 (默认: dicc.txt)
-t int             并发线程数 (默认: 20)
-timeout duration  请求超时时间 (默认: 10s)
-format string     输出格式 (console, json, csv) (默认: console)
-o string          输出文件路径
-v                 详细输出
-r                 递归扫描
-depth int         递归最大深度 (默认: 3)
-retry int         重试次数 (默认: 3)
-retry-delay duration  重试延迟 (默认: 1s)
-user-agent string 用户代理 (默认: dirsearch-go/2.0)
-rate-limit        启用速率限制
-rps int           每秒请求数 (默认: 10)
-config string     配置文件路径
```

### 配置文件

您可以使用JSON配置文件来管理复杂的扫描设置：

```bash
# 生成示例配置文件
cp config.example.json config.json

# 使用配置文件
./dirsearch-go -config config.json
```

配置文件示例：
```json
{
  "target": "https://www.baidu.com",
  "wordlist": "dicc.txt",
  "threads": 20,
  "timeout": "10s",
  "output": {
    "format": "json",
    "file": "results.json",
    "verbose": true
  },
  "scanner": {
    "methods": ["GET", "POST"],
    "skip_ssl_verify": true,
    "follow_redirects": false
  },
  "rate_limit": {
    "enabled": true,
    "requests_per_second": 10
  },
  "filters": {
    "exclude_status": [404, 400],
    "min_size": 100,
    "include_regex": "admin|config|backup"
  },
  "headers": {
    "Accept": "text/html,application/xhtml+xml",
    "Authorization": "Bearer token"
  },
  "recursive": true,
  "max_depth": 3,
  "retry_count": 3
}
```

## 过滤选项

### 状态码过滤
```json
{
  "filters": {
    "status_codes": [200, 301, 302],     // 只包含这些状态码
    "exclude_status": [404, 400, 403]    // 排除这些状态码
  }
}
```

### 响应大小过滤
```json
{
  "filters": {
    "min_size": 100,    // 最小响应大小
    "max_size": 10000   // 最大响应大小
  }
}
```

### 内容过滤
```json
{
  "filters": {
    "include_regex": "admin|config|backup",  // 包含的正则表达式
    "exclude_regex": "error|not found",     // 排除的正则表达式
    "include_words": ["dashboard", "admin"], // 包含的关键词
    "exclude_words": ["404", "error"]       // 排除的关键词
  }
}
```

## 输出格式

### 控制台输出
默认彩色控制台输出，不同状态码使用不同颜色：
- 绿色: 2xx (成功)
- 黄色: 3xx (重定向)
- 红色: 4xx (客户端错误)
- 紫色: 5xx (服务器错误)

### JSON输出
```json
[
  {
    "url": "https://www.baidu.com/admin",
    "status_code": 200,
    "size": 1024,
    "method": "GET",
    "depth": 0,
    "timestamp": "2024-01-01T12:00:00Z"
  }
]
```

### CSV输出
```csv
URL,StatusCode,Size,Method,Depth,Timestamp,Error
https://www.baidu.com/admin,200,1024,GET,0,2024-01-01T12:00:00Z,
```

## 词典文件

工具使用文本文件作为词典，每行一个路径：
```
admin
config
backup
login
dashboard
api
```

支持的路径格式：
- 简单路径: `admin`
- 带扩展名: `config.php`
- 子目录: `admin/login`
- 参数化路径: `api/v1/users`

## 性能优化

### 线程调优
```bash
# 高性能扫描
./dirsearch-go -u https://www.baidu.com -t 50

# 谨慎扫描
./dirsearch-go -u https://www.baidu.com -t 5 -rate-limit -rps 2
```

### 内存使用
- 使用流式读取词典文件，内存使用恒定
- 连接池复用，减少连接开销
- 智能缓冲，平衡性能和内存

## 安全考虑

### 负责任的使用
- 仅在授权的目标上使用
- 遵守目标网站的robots.txt
- 使用适当的速率限制
- 避免对生产环境造成影响

### 法律声明
此工具仅用于授权的安全测试。用户需要确保在使用前获得适当的授权。开发者不对误用此工具承担责任。

## 贡献

欢迎提交问题和拉取请求。在贡献代码之前，请确保：

1. 代码通过所有测试
2. 遵循Go代码规范
3. 添加适当的文档
4. 更新相关的测试

## 许可证

本项目采用MIT许可证。详情请参阅[LICENSE](LICENSE)文件。

## 更新日志

### v2.0.0
- 完全重构代码架构
- 添加配置文件支持
- 实现多种输出格式
- 添加递归扫描功能
- 实现高级过滤系统
- 添加速率限制功能
- 改进错误处理和日志记录

### v1.0.0
- 初始版本
- 基本目录扫描功能
- 多线程支持
- 进度条显示