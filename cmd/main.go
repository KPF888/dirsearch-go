package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"dirsearch-go/pkg/config"
	"dirsearch-go/pkg/logger"
	"dirsearch-go/pkg/logo"
	"dirsearch-go/pkg/output"
	"dirsearch-go/pkg/scanner"
	"strings"

	"github.com/schollz/progressbar/v3"
)

// 版本信息，会在构建时通过 -ldflags 注入
var version = "v0.01"

// 为输出通道定义消息类型
type progressIncrement int
type progressMaxChange int
type statusMessage struct {
	message  string
	toStderr bool // true 表示输出到 stderr，false 表示输出到 stdout
}

// App 主应用程序
type App struct {
	config     *config.Config
	logger     *logger.Logger
	scanner    *scanner.Scanner
	writer     output.Writer
	progress   *progressbar.ProgressBar
	ctx        context.Context
	cancel     context.CancelFunc
	outputChan chan interface{} // 用于结果和进度更新的统一通道
}

// NewApp 创建新的应用程序实例
func NewApp() (*App, error) {
	// 解析命令行参数
	cfg, configFile, err := config.ParseFlags()
	if err != nil {
		// 如果是使用帮助错误，直接返回不包装
		if _, isUsageError := err.(*config.UsageError); isUsageError {
			return nil, err
		}
		return nil, fmt.Errorf("解析命令行参数失败: %w", err)
	}

	// 如果指定了配置文件，加载配置
	if configFile != "" {
		fileCfg, err := config.LoadFromFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("加载配置文件失败: %w", err)
		}
		// 合并配置：命令行 > 配置文件 > 默认值
		if cfg.Target == "" {
			cfg.Target = fileCfg.Target
		}
		if cfg.Wordlist == "" {
			cfg.Wordlist = fileCfg.Wordlist
		}
		// ... 其他配置项的合并
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	// 注意：这里不再需要 color.Output = os.Stderr
	// 因为进度条和结果输出将通过 outputManager 协调
	// 并且 color 库默认写入 stdout，这正是我们想要的

	// 创建日志记录器
	logLevel := logger.LevelInfo
	if cfg.Output.Verbose {
		logLevel = logger.LevelDebug
	}
	logFile := ""
	if cfg.Output.ShowErrors {
		logFile = "dirsearch.log"
	}
	log, err := logger.New(logLevel, logFile)
	if err != nil {
		return nil, fmt.Errorf("创建日志记录器失败: %w", err)
	}

	// 创建扫描器
	scan, err := scanner.New(cfg, log)
	if err != nil {
		return nil, fmt.Errorf("创建扫描器失败: %w", err)
	}

	// 创建输出器
	var writers []output.Writer

	// 始终添加控制台输出器，以便用户能看到实时结果
	consoleWriter := output.NewConsoleWriter(cfg.Output.Verbose)
	writers = append(writers, consoleWriter)

	// 如果指定了文件输出，添加缓冲文件写入器
	if cfg.Output.File != "" {
		fileWriter, err := output.CreateBufferedWriter(cfg.Output.Format, cfg.Output.File, cfg.Output.Verbose)
		if err != nil {
			return nil, fmt.Errorf("创建文件输出器失败: %w", err)
		}
		writers = append(writers, fileWriter)
	}

	var writer output.Writer
	if len(writers) == 1 {
		writer = writers[0]
	} else {
		writer = output.NewMultiWriter(writers...)
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		config:     cfg,
		logger:     log,
		scanner:    scan,
		writer:     writer,
		ctx:        ctx,
		cancel:     cancel,
		outputChan: make(chan interface{}, cfg.Threads*2), // 带缓冲的通道
	}

	return app, nil
}

// Run 运行应用程序
func (a *App) Run() error {
	a.setupSignalHandling()

	totalJobs, err := a.calculateTotalJobs(a.config.Wordlist)
	if err != nil {
		return fmt.Errorf("计算总任务数失败: %w", err)
	}

	a.progress = progressbar.NewOptions(totalJobs,
		progressbar.OptionSetDescription("扫描进度"),
		progressbar.OptionSetWriter(os.Stderr), // 进度条写入 stderr
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(50*time.Millisecond), // 更频繁的更新 - 从100ms调整到50ms
		progressbar.OptionOnCompletion(func() {
			// 完成时清除进度条并换行
			fmt.Fprint(os.Stderr, "\r\033[K\n")
		}),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionClearOnFinish(), // 完成时清除进度条
	)

	if err := a.scan(); err != nil {
		// 避免在上下文取消时报告错误
		if a.ctx.Err() == nil {
			return fmt.Errorf("扫描失败: %w", err)
		}
	}

	// 刷新缓冲的输出数据
	if err := a.flushBufferedOutput(); err != nil {
		a.logger.Error("刷新缓冲输出失败", "error", err)
	}

	if a.writer != nil {
		if err := a.writer.Close(); err != nil {
			a.logger.Error("关闭输出器失败", "error", err)
		}
	}

	a.logger.Info("扫描完成")
	return nil
}

// setupSignalHandling 设置信号处理
func (a *App) setupSignalHandling() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		a.logger.Info("收到停止信号，正在优雅关闭...")
		// 清除进度条并输出停止消息到 stderr
		fmt.Fprint(os.Stderr, "\r\033[K")

		// 刷新缓冲的输出数据
		if err := a.flushBufferedOutput(); err != nil {
			a.logger.Error("刷新缓冲输出失败", "error", err)
		}

		a.cancel()
	}()
}

// outputManager 是唯一写入控制台的 goroutine
func (a *App) outputManager(wg *sync.WaitGroup) {
	defer wg.Done()
	for msg := range a.outputChan {
		select {
		case <-a.ctx.Done():
			// 清理通道中剩余的消息，但不处理
			continue
		default:
			switch v := msg.(type) {
			case *scanner.Result:
				// 在输出结果前清除进度条
				a.clearProgressBar()

				// 输出结果到 stdout
				if err := a.writer.Write(v); err != nil {
					a.logger.Error("写入结果失败", "error", err)
				}

				// 输出结果后重新显示进度条
				a.restoreProgressBar()
			case progressIncrement:
				a.progress.Add(int(v))
			case progressMaxChange:
				a.progress.ChangeMax(a.progress.GetMax() + int(v))
			case statusMessage:
				// 清除进度条
				a.clearProgressBar()

				// 输出状态消息
				if v.toStderr {
					fmt.Fprintln(os.Stderr, v.message)
				} else {
					fmt.Fprintln(os.Stdout, v.message)
				}

				// 恢复进度条
				a.restoreProgressBar()
			}
		}
	}
}

// scan 执行扫描
func (a *App) scan() error {
	file, err := os.Open(a.config.Wordlist)
	if err != nil {
		return fmt.Errorf("打开词典文件失败: %w", err)
	}
	defer file.Close()

	jobs := make(chan string, a.config.Threads*2)
	var workerWg sync.WaitGroup
	var outputWg sync.WaitGroup

	// 启动 outputManager
	outputWg.Add(1)
	go a.outputManager(&outputWg)

	// 启动工作线程
	for i := 0; i < a.config.Threads; i++ {
		workerWg.Add(1)
		go a.worker(jobs, &workerWg)
	}

	// 读取词典并发送任务
	fileScanner := bufio.NewScanner(file)
	for fileScanner.Scan() {
		select {
		case <-a.ctx.Done():
			goto cleanup
		default:
			word := fileScanner.Text()
			if strings.Contains(word, "%EXT%") {
				for _, ext := range a.config.Scanner.Extensions {
					// 智能处理扩展名替换，避免双点号
					var extToUse string
					if strings.Contains(word, ".%EXT%") {
						// 如果占位符前已经有点号，直接使用扩展名（不加点号）
						extToUse = ext
					} else {
						// 如果占位符前没有点号，添加点号
						if !strings.HasPrefix(ext, ".") {
							extToUse = "." + ext
						} else {
							extToUse = ext
						}
					}
					newWord := strings.ReplaceAll(word, "%EXT%", extToUse)
					jobs <- newWord
				}
			} else {
				jobs <- word
			}
		}
	}

cleanup:
	if err := fileScanner.Err(); err != nil {
		a.logger.Error("读取词典文件失败", "error", err)
	}

	close(jobs)
	workerWg.Wait()
	close(a.outputChan)
	outputWg.Wait()

	return a.ctx.Err()
}

// worker 工作线程
func (a *App) worker(jobs <-chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	for word := range jobs {
		select {
		case <-a.ctx.Done():
			return
		default:
			a.outputChan <- progressIncrement(1)
			result, err := a.scanner.ScanURL(a.ctx, a.config.Target, word, 0)
			if err != nil {
				a.logger.Error("扫描URL失败", "word", word, "error", err)
				continue
			}
			if result != nil {
				a.outputChan <- result
				if a.config.Recursive && result.StatusCode >= 200 && result.StatusCode < 400 {
					a.recursiveScan(result, 1)
				}
			}
		}
	}
}

// recursiveScan 递归扫描
func (a *App) recursiveScan(parentResult *scanner.Result, depth int) {
	if depth > a.config.MaxDepth {
		return
	}

	paths := a.scanner.ExtractPaths(parentResult)
	if len(paths) > 0 {
		a.outputChan <- progressMaxChange(len(paths))
	}

	for _, path := range paths {
		select {
		case <-a.ctx.Done():
			return
		default:
			a.outputChan <- progressIncrement(1)
			result, err := a.scanner.ScanURL(a.ctx, a.config.Target, path, depth)
			if err != nil {
				a.logger.Error("递归扫描失败", "path", path, "depth", depth, "error", err)
				continue
			}
			if result != nil {
				a.outputChan <- result
				if result.StatusCode >= 200 && result.StatusCode < 400 {
					a.recursiveScan(result, depth+1)
				}
			}
		}
	}
}

// clearProgressBar 清除进度条显示
func (a *App) clearProgressBar() {
	if a.progress != nil {
		// 移动到行首并清除当前行
		fmt.Fprint(os.Stderr, "\r\033[K")
		// 确保输出被刷新
		os.Stderr.Sync()
	}
}

// restoreProgressBar 恢复进度条显示
func (a *App) restoreProgressBar() {
	if a.progress != nil {
		// 重新渲染进度条到当前状态
		a.progress.RenderBlank()
		// 确保输出被刷新
		os.Stderr.Sync()
	}
}

// flushBufferedOutput 刷新缓冲的输出数据
func (a *App) flushBufferedOutput() error {
	if a.writer == nil {
		return nil
	}

	// 检查是否是 MultiWriter
	if multiWriter, ok := a.writer.(*output.MultiWriter); ok {
		return a.flushMultiWriter(multiWriter)
	}

	// 检查是否是 BufferedWriter
	if bufferedWriter, ok := a.writer.(output.BufferedWriter); ok {
		return bufferedWriter.Flush()
	}

	return nil
}

// flushMultiWriter 刷新 MultiWriter 中的缓冲写入器
func (a *App) flushMultiWriter(multiWriter *output.MultiWriter) error {
	return multiWriter.Flush()
}

// calculateTotalJobs 计算总任务数
func (a *App) calculateTotalJobs(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	numExtensions := len(a.config.Scanner.Extensions)

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "%EXT%") {
			count += numExtensions
		} else {
			count++
		}
	}
	return count, scanner.Err()
}

// Close 关闭应用程序
func (a *App) Close() {
	a.cancel()
	if a.scanner != nil {
		a.scanner.Close()
	}
	if a.logger != nil {
		a.logger.Close()
	}
}

func main() {
	// 设置logo版本信息
	logo.Version = version

	// Display startup logo
	logo.Display()

	app, err := NewApp()
	if err != nil {
		// 检查是否是使用帮助错误
		if _, isUsageError := err.(*config.UsageError); isUsageError {
			config.PrintUsage()
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "创建应用程序失败: %v\n", err)
		os.Exit(1)
	}
	defer app.Close()

	if err := app.Run(); err != nil {
		// 在 main 函数中，我们仍然希望在发生严重错误时看到输出
		if err != context.Canceled {
			fmt.Fprintf(os.Stderr, "运行应用程序失败: %v\n", err)
		}
		os.Exit(1)
	}
}
