package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Duration 自定义Duration类型用于JSON解析
type Duration time.Duration

// UsageError 表示需要显示使用帮助的错误
type UsageError struct{}

func (e *UsageError) Error() string {
	return "usage"
}

// UnmarshalJSON 实现JSON解析
func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	duration, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(duration)
	return nil
}

// MarshalJSON 实现JSON序列化
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// Config 应用程序配置
type Config struct {
	Target     string            `json:"target"`
	Wordlist   string            `json:"wordlist"`
	Threads    int               `json:"threads"`
	Timeout    Duration          `json:"timeout"`
	Output     OutputConfig      `json:"output"`
	Scanner    ScannerConfig     `json:"scanner"`
	RateLimit  RateLimitConfig   `json:"rate_limit"`
	Filters    FilterConfig      `json:"filters"`
	Headers    map[string]string `json:"headers"`
	UserAgent  string            `json:"user_agent"`
	Recursive  bool              `json:"recursive"`
	MaxDepth   int               `json:"max_depth"`
	RetryCount int               `json:"retry_count"`
	RetryDelay Duration          `json:"retry_delay"`
}

// OutputConfig 输出配置
type OutputConfig struct {
	Format     string `json:"format"`      // console, json, csv
	File       string `json:"file"`        // 输出文件路径
	Verbose    bool   `json:"verbose"`     // 详细输出
	ShowErrors bool   `json:"show_errors"` // 显示错误信息
}

// ScannerConfig 扫描器配置
type ScannerConfig struct {
	Methods         []string `json:"methods"`          // HTTP 方法
	Extensions      []string `json:"extensions"`       // 文件扩展名
	SkipSSLVerify   bool     `json:"skip_ssl_verify"`  // 跳过SSL验证
	FollowRedirects bool     `json:"follow_redirects"` // 跟随重定向
	MaxRedirects    int      `json:"max_redirects"`    // 最大重定向次数
}

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	Enabled           bool     `json:"enabled"`             // 启用速率限制
	RequestsPerSecond int      `json:"requests_per_second"` // 每秒请求数
	Delay             Duration `json:"delay"`               // 请求间延迟
}

// FilterConfig 过滤配置
type FilterConfig struct {
	StatusCodes   []int    `json:"status_codes"`   // 包���的状态码
	ExcludeStatus []int    `json:"exclude_status"` // 排除的状态码
	MinSize       int64    `json:"min_size"`       // 最小响应大小
	MaxSize       int64    `json:"max_size"`       // 最大响应大小
	IncludeRegex  string   `json:"include_regex"`  // 包含的正则表达式
	ExcludeRegex  string   `json:"exclude_regex"`  // 排除的正则表达式
	IncludeWords  []string `json:"include_words"`  // 包含的关键词
	ExcludeWords  []string `json:"exclude_words"`  // 排除的关键词
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Threads:   20,
		Timeout:   Duration(10 * time.Second),
		UserAgent: "dirsearch-go/0.01",
		Wordlist:  "dicc.txt",
		Output: OutputConfig{
			Format:     "console",
			Verbose:    false,
			ShowErrors: false,
		},
		Scanner: ScannerConfig{
			Methods:         []string{"GET"},
			Extensions:      []string{"php", "html", "js", "txt"},
			SkipSSLVerify:   true,
			FollowRedirects: false,
			MaxRedirects:    3,
		},
		RateLimit: RateLimitConfig{
			Enabled:           false,
			RequestsPerSecond: 10,
			Delay:             Duration(0),
		},
		Filters: FilterConfig{
			ExcludeStatus: []int{404, 400, 403},
			MinSize:       0,
			MaxSize:       0,
		},
		Headers:    make(map[string]string),
		Recursive:  false,
		MaxDepth:   3,
		RetryCount: 3,
		RetryDelay: Duration(1 * time.Second),
	}
}

// LoadFromFile 从配置文件加载配置
func LoadFromFile(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("打开配置文件失败: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	config := DefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return config, nil
}

// SaveToFile 保存配置到文件
func (c *Config) SaveToFile(filename string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// ParseFlags 解析命令行参数
func ParseFlags() (*Config, string, error) {
	config := DefaultConfig()
	var configFile string
	var timeout time.Duration
	var retryDelay time.Duration
	var extensions string
	var showHelp bool

	flag.StringVar(&config.Target, "u", "", "目标URL (例如: http://example.com)")
	flag.StringVar(&config.Wordlist, "w", config.Wordlist, "词典文件路径")
	flag.IntVar(&config.Threads, "t", config.Threads, "并发线程数")
	flag.DurationVar(&timeout, "timeout", time.Duration(config.Timeout), "请求超时时间")
	flag.StringVar(&config.Output.Format, "format", config.Output.Format, "输出格式 (console, json, csv)")
	flag.StringVar(&config.Output.File, "o", "", "输出文件路径")
	flag.BoolVar(&config.Output.Verbose, "v", config.Output.Verbose, "详细输出")
	flag.BoolVar(&config.Recursive, "r", config.Recursive, "递归扫描")
	flag.IntVar(&config.MaxDepth, "depth", config.MaxDepth, "递归最大深度")
	flag.IntVar(&config.RetryCount, "retry", config.RetryCount, "重试次数")
	flag.DurationVar(&retryDelay, "retry-delay", time.Duration(config.RetryDelay), "重试延迟")
	flag.StringVar(&config.UserAgent, "user-agent", config.UserAgent, "用户代理")
	flag.BoolVar(&config.RateLimit.Enabled, "rate-limit", config.RateLimit.Enabled, "启用速率限制")
	flag.IntVar(&config.RateLimit.RequestsPerSecond, "rps", config.RateLimit.RequestsPerSecond, "每秒请求数")
	flag.StringVar(&configFile, "config", "", "配置文件路径")
	flag.StringVar(&extensions, "e", "", "要测试的文件扩展名列表 (逗号分隔)")
	flag.BoolVar(&showHelp, "h", false, "显示帮助信息")
	flag.BoolVar(&showHelp, "help", false, "显示帮助信息")

	flag.Parse()

	// 检查是否需要显示帮助信息
	if showHelp || (len(os.Args) == 1) {
		return nil, "", &UsageError{}
	}

	// 转换time.Duration到自定义Duration类型
	config.Timeout = Duration(timeout)
	config.RetryDelay = Duration(retryDelay)

	// 解析扩展名
	if extensions != "" {
		config.Scanner.Extensions = strings.Split(extensions, ",")
		for i, ext := range config.Scanner.Extensions {
			ext = strings.TrimSpace(ext)
			// 去掉前导点号，确保扩展名格式一致
			if strings.HasPrefix(ext, ".") {
				ext = ext[1:]
			}
			config.Scanner.Extensions[i] = ext
		}
	}

	return config, configFile, nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Target == "" {
		return fmt.Errorf("目标URL不能为空")
	}

	if c.Threads <= 0 {
		return fmt.Errorf("线程数必须大于0")
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("超时时间必须大于0")
	}

	if c.MaxDepth < 0 {
		return fmt.Errorf("递归深度不能为负数")
	}

	if c.RetryCount < 0 {
		return fmt.Errorf("重试次数不能为负数")
	}

	return nil
}

// PrintUsage 打印使用帮助信息
func PrintUsage() {
	fmt.Fprintf(os.Stderr, `Dirsearch-Go - 高性能目录扫描工具

用法:
  %s [选项] -u <目标URL> [词典文件]

必需参数:
  -u string          目标URL (例如: http://example.com)

可选参数:
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
  -user-agent string 用户代理 (默认: dirsearch-go/0.01)
  -rate-limit        启用速率限制
  -rps int           每秒请求数 (默认: 10)
  -e string          要测试的文件扩展名列表 (逗号分隔)
  -config string     配置文件路径
  -h, -help          显示此帮助信息

示例:
  # 基本扫描 (使用默认词典文件 dicc.txt)
  %s -u https://example.com

  # 指定词典文件
  %s -u https://example.com -w custom_dict.txt

  # 使用配置文件
  %s -config config.json

  # 输出到JSON文件
  %s -u https://example.com -format json -o results.json

  # 递归扫描
  %s -u https://example.com -r -depth 3

  # 启用速率限制
  %s -u https://example.com -rate-limit -rps 5

更多信息请访问: https://github.com/KPF888/dirsearch-go
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}
