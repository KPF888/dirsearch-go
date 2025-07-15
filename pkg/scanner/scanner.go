package scanner

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"dirsearch-go/pkg/config"
	"dirsearch-go/pkg/logger"
)

// Result 扫描结果
type Result struct {
	URL         string            `json:"url"`
	StatusCode  int               `json:"status_code"`
	Size        int64             `json:"size"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
	Error       string            `json:"error,omitempty"`
	Depth       int               `json:"depth"`
	Method      string            `json:"method"`
	Timestamp   time.Time         `json:"timestamp"`
}

// Scanner 扫描器
type Scanner struct {
	config        *config.Config
	client        *http.Client
	logger        *logger.Logger
	includeRegex  *regexp.Regexp
	excludeRegex  *regexp.Regexp
	rateLimiter   chan struct{}
}

// New 创建新的扫描器
func New(cfg *config.Config, log *logger.Logger) (*Scanner, error) {
	scanner := &Scanner{
		config: cfg,
		logger: log,
	}

	// 创建HTTP客户端
	scanner.client = &http.Client{
		Timeout: time.Duration(cfg.Timeout),
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.Scanner.SkipSSLVerify,
			},
			MaxIdleConns:        cfg.Threads,
			MaxIdleConnsPerHost: cfg.Threads,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	// 设置重定向策略
	if !cfg.Scanner.FollowRedirects {
		scanner.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	// 编译正则表达式
	if cfg.Filters.IncludeRegex != "" {
		var err error
		scanner.includeRegex, err = regexp.Compile(cfg.Filters.IncludeRegex)
		if err != nil {
			return nil, fmt.Errorf("编译包含正则表达式失败: %w", err)
		}
	}

	if cfg.Filters.ExcludeRegex != "" {
		var err error
		scanner.excludeRegex, err = regexp.Compile(cfg.Filters.ExcludeRegex)
		if err != nil {
			return nil, fmt.Errorf("编译排除正则表达式失败: %w", err)
		}
	}

	// 创建速率限制器
	if cfg.RateLimit.Enabled {
		scanner.rateLimiter = make(chan struct{}, cfg.RateLimit.RequestsPerSecond)
		go scanner.fillRateLimiter()
	}

	return scanner, nil
}

// fillRateLimiter 填充速率限制器
func (s *Scanner) fillRateLimiter() {
	ticker := time.NewTicker(time.Second / time.Duration(s.config.RateLimit.RequestsPerSecond))
	defer ticker.Stop()

	for range ticker.C {
		select {
		case s.rateLimiter <- struct{}{}:
		default:
		}
	}
}

// ScanURL 扫描单个URL
func (s *Scanner) ScanURL(ctx context.Context, targetURL, path string, depth int) (*Result, error) {
	// 速率限制
	if s.rateLimiter != nil {
		select {
		case <-s.rateLimiter:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// 跳过包含占位符的路径
	if strings.Contains(path, "%FUZZ%") {
		return nil, nil
	}

	// 构建完整URL
	fullURL := strings.TrimRight(targetURL, "/") + "/" + strings.TrimLeft(path, "/")

	// 尝试多种HTTP方法
	for _, method := range s.config.Scanner.Methods {
		result, err := s.makeRequest(ctx, method, fullURL, depth)
		if err != nil {
			// 只记录非URL解析错误
			if !strings.Contains(err.Error(), "invalid URL escape") {
				s.logger.Debug("请求失败", "url", fullURL, "method", method, "error", err)
			}
			continue
		}

		if result != nil && s.shouldIncludeResult(result) {
			return result, nil
		}
	}

	return nil, nil
}

// makeRequest 发送HTTP请求
func (s *Scanner) makeRequest(ctx context.Context, method, url string, depth int) (*Result, error) {
	var err error
	var resp *http.Response

	// 重试机制
	for i := 0; i <= s.config.RetryCount; i++ {
		req, err := http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			return nil, fmt.Errorf("创建请求失败: %w", err)
		}

		// 设置请求头
		req.Header.Set("User-Agent", s.config.UserAgent)
		for key, value := range s.config.Headers {
			req.Header.Set(key, value)
		}

		resp, err = s.client.Do(req)
		if err == nil {
			break
		}

		if i < s.config.RetryCount {
			s.logger.Debug("请求重试", "url", url, "attempt", i+1, "error", err)
			time.Sleep(time.Duration(s.config.RetryDelay))
		}
	}

	if err != nil {
		return &Result{
			URL:       url,
			Method:    method,
			Error:     err.Error(),
			Depth:     depth,
			Timestamp: time.Now(),
		}, nil
	}

	if resp == nil {
		return &Result{
			URL:       url,
			Method:    method,
			Error:     "响应为空",
			Depth:     depth,
			Timestamp: time.Now(),
		}, nil
	}

	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("读取响应体失败", "url", url, "error", err)
		body = []byte{}
	}

	// 构建结果
	result := &Result{
		URL:        url,
		StatusCode: resp.StatusCode,
		Size:       int64(len(body)),
		Method:     method,
		Depth:      depth,
		Timestamp:  time.Now(),
	}

	// 如果需要详细输出，包含响应头和体
	if s.config.Output.Verbose {
		result.Headers = make(map[string]string)
		for key, values := range resp.Header {
			result.Headers[key] = strings.Join(values, ", ")
		}
		result.Body = string(body)
	}

	return result, nil
}

// shouldIncludeResult 判断是否应该包含结果
func (s *Scanner) shouldIncludeResult(result *Result) bool {
	// 状态码过滤
	if len(s.config.Filters.StatusCodes) > 0 {
		included := false
		for _, code := range s.config.Filters.StatusCodes {
			if result.StatusCode == code {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	// 排除状态码
	for _, code := range s.config.Filters.ExcludeStatus {
		if result.StatusCode == code {
			return false
		}
	}

	// 大小过滤
	if s.config.Filters.MinSize > 0 && result.Size < s.config.Filters.MinSize {
		return false
	}

	if s.config.Filters.MaxSize > 0 && result.Size > s.config.Filters.MaxSize {
		return false
	}

	// 正则表达式过滤
	if s.includeRegex != nil && !s.includeRegex.MatchString(result.Body) {
		return false
	}

	if s.excludeRegex != nil && s.excludeRegex.MatchString(result.Body) {
		return false
	}

	// 关键词过滤
	if len(s.config.Filters.IncludeWords) > 0 {
		included := false
		for _, word := range s.config.Filters.IncludeWords {
			if strings.Contains(result.Body, word) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	// 排除关键词
	for _, word := range s.config.Filters.ExcludeWords {
		if strings.Contains(result.Body, word) {
			return false
		}
	}

	return true
}

// ExtractPaths 从响应中提取路径（用于递归扫描）
func (s *Scanner) ExtractPaths(result *Result) []string {
	if result.Body == "" {
		return nil
	}

	// 简单的路径提取正则表达式
	pathRegex := regexp.MustCompile(`href=["']([^"']+)["']`)
	matches := pathRegex.FindAllStringSubmatch(result.Body, -1)

	var paths []string
	for _, match := range matches {
		if len(match) > 1 {
			path := match[1]
			// 过滤掉外部链接和特殊路径
			if !strings.HasPrefix(path, "http") && 
			   !strings.HasPrefix(path, "mailto:") && 
			   !strings.HasPrefix(path, "#") &&
			   !strings.HasPrefix(path, "javascript:") {
				
				// 解析URL
				u, err := url.Parse(path)
				if err == nil && u.Path != "" {
					paths = append(paths, strings.TrimPrefix(u.Path, "/"))
				}
			}
		}
	}

	return paths
}

// Close 关闭扫描器
func (s *Scanner) Close() {
	if s.rateLimiter != nil {
		close(s.rateLimiter)
	}
}