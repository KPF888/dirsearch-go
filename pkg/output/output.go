package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"dirsearch-go/pkg/scanner"

	"github.com/fatih/color"
)

// Writer 输出写入器接口
type Writer interface {
	Write(result *scanner.Result) error
	Close() error
}

// BufferedWriter 缓冲写入器接口，支持批量刷新
type BufferedWriter interface {
	Writer
	Flush() error // 将缓冲的结果写入文件
}

// ConsoleWriter 控制台输出
type ConsoleWriter struct {
	color200     *color.Color
	color301     *color.Color
	color403     *color.Color
	color500     *color.Color
	colorDefault *color.Color
	verbose      bool
}

// JSONWriter JSON文件输出
type JSONWriter struct {
	file    *os.File
	encoder *json.Encoder
	first   bool
}

// CSVWriter CSV文件输出
type CSVWriter struct {
	file   *os.File
	writer *csv.Writer
	header bool
}

// NewConsoleWriter 创建控制台输出器
func NewConsoleWriter(verbose bool) *ConsoleWriter {
	return &ConsoleWriter{
		color200:     color.New(color.FgGreen),
		color301:     color.New(color.FgYellow),
		color403:     color.New(color.FgRed),
		color500:     color.New(color.FgMagenta),
		colorDefault: color.New(color.FgWhite),
		verbose:      verbose,
	}
}

// Write 写入结果到控制台
func (w *ConsoleWriter) Write(result *scanner.Result) error {
	if result.Error != "" {
		if w.verbose {
			// 错误信息输出到 stdout，保持一致性
			fmt.Fprintf(os.Stdout, "[ERROR] %s: %s\n", result.URL, result.Error)
		}
		return nil
	}

	var output string
	if w.verbose {
		output = fmt.Sprintf("[%d] %s [%s] [%d bytes] [%s]",
			result.StatusCode, result.URL, result.Method, result.Size,
			result.Timestamp.Format("15:04:05"))
	} else {
		output = fmt.Sprintf("[%d] %s", result.StatusCode, result.URL)
	}

	// 确保所有输出都到 stdout
	switch {
	case result.StatusCode >= 200 && result.StatusCode < 300:
		w.color200.Fprint(os.Stdout, output+"\n")
	case result.StatusCode >= 300 && result.StatusCode < 400:
		w.color301.Fprint(os.Stdout, output+"\n")
	case result.StatusCode >= 400 && result.StatusCode < 500:
		w.color403.Fprint(os.Stdout, output+"\n")
	case result.StatusCode >= 500:
		w.color500.Fprint(os.Stdout, output+"\n")
	default:
		w.colorDefault.Fprint(os.Stdout, output+"\n")
	}

	return nil
}

// Close 关闭控制台输出器
func (w *ConsoleWriter) Close() error {
	return nil
}

// NewJSONWriter 创建JSON输出器
func NewJSONWriter(filename string) (*JSONWriter, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("创建JSON文件失败: %w", err)
	}

	// 写入JSON数组开始
	if _, err := file.WriteString("[\n"); err != nil {
		file.Close()
		return nil, fmt.Errorf("写入JSON开始失败: %w", err)
	}

	return &JSONWriter{
		file:    file,
		encoder: json.NewEncoder(file),
		first:   true,
	}, nil
}

// Write 写入结果到JSON文件
func (w *JSONWriter) Write(result *scanner.Result) error {
	if !w.first {
		if _, err := w.file.WriteString(",\n"); err != nil {
			return fmt.Errorf("写入JSON分隔符失败: %w", err)
		}
	} else {
		w.first = false
	}

	// 使用手动缩进
	if _, err := w.file.WriteString("  "); err != nil {
		return fmt.Errorf("写入缩进失败: %w", err)
	}

	// 不使用自动缩进，因为我们手动控制格式
	w.encoder.SetIndent("", "")
	if err := w.encoder.Encode(result); err != nil {
		return fmt.Errorf("JSON编码失败: %w", err)
	}

	return nil
}

// Close 关闭JSON输出器
func (w *JSONWriter) Close() error {
	if w.file != nil {
		// 写入JSON数组结束
		w.file.WriteString("\n]")
		return w.file.Close()
	}
	return nil
}

// BufferedJSONWriter 缓冲JSON文件输出
type BufferedJSONWriter struct {
	filename string
	results  []*scanner.Result
}

// NewBufferedJSONWriter 创建缓冲JSON输出器
func NewBufferedJSONWriter(filename string) *BufferedJSONWriter {
	return &BufferedJSONWriter{
		filename: filename,
		results:  make([]*scanner.Result, 0),
	}
}

// Write 将结果添加到缓冲区
func (w *BufferedJSONWriter) Write(result *scanner.Result) error {
	w.results = append(w.results, result)
	return nil
}

// Flush 将所有缓冲的结果写入JSON文件
func (w *BufferedJSONWriter) Flush() error {
	if len(w.results) == 0 {
		return nil
	}

	file, err := os.Create(w.filename)
	if err != nil {
		return fmt.Errorf("创建JSON文件失败: %w", err)
	}
	defer file.Close()

	// 写入JSON数组
	if _, err := file.WriteString("[\n"); err != nil {
		return fmt.Errorf("写入JSON开始失败: %w", err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "")

	for i, result := range w.results {
		if i > 0 {
			if _, err := file.WriteString(",\n"); err != nil {
				return fmt.Errorf("写入JSON分隔符失败: %w", err)
			}
		}

		// 写入缩进
		if _, err := file.WriteString("  "); err != nil {
			return fmt.Errorf("写入缩进失败: %w", err)
		}

		if err := encoder.Encode(result); err != nil {
			return fmt.Errorf("JSON编码失败: %w", err)
		}
	}

	if _, err := file.WriteString("\n]"); err != nil {
		return fmt.Errorf("写入JSON结束失败: %w", err)
	}

	return nil
}

// Close 关闭缓冲JSON输出器
func (w *BufferedJSONWriter) Close() error {
	return w.Flush()
}

// NewCSVWriter 创建CSV输出器
func NewCSVWriter(filename string) (*CSVWriter, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("创建CSV文件失败: %w", err)
	}

	return &CSVWriter{
		file:   file,
		writer: csv.NewWriter(file),
		header: false,
	}, nil
}

// Write 写入结果到CSV文件
func (w *CSVWriter) Write(result *scanner.Result) error {
	// 写入表头
	if !w.header {
		header := []string{"URL", "StatusCode", "Size", "Method", "Depth", "Timestamp", "Error"}
		if err := w.writer.Write(header); err != nil {
			return fmt.Errorf("写入CSV表头失败: %w", err)
		}
		w.header = true
	}

	// 写入数据行
	record := []string{
		result.URL,
		strconv.Itoa(result.StatusCode),
		strconv.FormatInt(result.Size, 10),
		result.Method,
		strconv.Itoa(result.Depth),
		result.Timestamp.Format(time.RFC3339),
		result.Error,
	}

	if err := w.writer.Write(record); err != nil {
		return fmt.Errorf("写入CSV数据失败: %w", err)
	}

	w.writer.Flush()
	return w.writer.Error()
}

// Close 关闭CSV输出器
func (w *CSVWriter) Close() error {
	if w.writer != nil {
		w.writer.Flush()
	}
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// BufferedCSVWriter 缓冲CSV文件输出
type BufferedCSVWriter struct {
	filename string
	results  []*scanner.Result
}

// NewBufferedCSVWriter 创建缓冲CSV输出器
func NewBufferedCSVWriter(filename string) *BufferedCSVWriter {
	return &BufferedCSVWriter{
		filename: filename,
		results:  make([]*scanner.Result, 0),
	}
}

// Write 将结果添加到缓冲区
func (w *BufferedCSVWriter) Write(result *scanner.Result) error {
	w.results = append(w.results, result)
	return nil
}

// Flush 将所有缓冲的结果写入CSV文件
func (w *BufferedCSVWriter) Flush() error {
	if len(w.results) == 0 {
		return nil
	}

	file, err := os.Create(w.filename)
	if err != nil {
		return fmt.Errorf("创建CSV文件失败: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入表头
	header := []string{"URL", "StatusCode", "Size", "Method", "Depth", "Timestamp", "Error"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("写入CSV表头失败: %w", err)
	}

	// 写入所有数据行
	for _, result := range w.results {
		record := []string{
			result.URL,
			strconv.Itoa(result.StatusCode),
			strconv.FormatInt(result.Size, 10),
			result.Method,
			strconv.Itoa(result.Depth),
			result.Timestamp.Format(time.RFC3339),
			result.Error,
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("写入CSV数据失败: %w", err)
		}
	}

	return writer.Error()
}

// Close 关闭缓冲CSV输出器
func (w *BufferedCSVWriter) Close() error {
	return w.Flush()
}

// MultiWriter 多重输出器
type MultiWriter struct {
	writers []Writer
}

// NewMultiWriter 创建多重输出器
func NewMultiWriter(writers ...Writer) *MultiWriter {
	return &MultiWriter{writers: writers}
}

// Write 写入到所有输出器
func (w *MultiWriter) Write(result *scanner.Result) error {
	for _, writer := range w.writers {
		if err := writer.Write(result); err != nil {
			return err
		}
	}
	return nil
}

// Flush 刷新所有缓冲写入器
func (w *MultiWriter) Flush() error {
	for _, writer := range w.writers {
		if bufferedWriter, ok := writer.(BufferedWriter); ok {
			if err := bufferedWriter.Flush(); err != nil {
				return err
			}
		}
	}
	return nil
}

// Close 关闭所有输出器
func (w *MultiWriter) Close() error {
	for _, writer := range w.writers {
		if err := writer.Close(); err != nil {
			return err
		}
	}
	return nil
}

// CreateWriter 根据配置创建输出器
func CreateWriter(format, filename string, verbose bool) (Writer, error) {
	switch format {
	case "console":
		return NewConsoleWriter(verbose), nil
	case "json":
		if filename == "" {
			return nil, fmt.Errorf("JSON格式需要指定输出文件")
		}
		return NewJSONWriter(filename)
	case "csv":
		if filename == "" {
			return nil, fmt.Errorf("CSV格式需要指定输出文件")
		}
		return NewCSVWriter(filename)
	default:
		return nil, fmt.Errorf("不支持的输出格式: %s", format)
	}
}

// CreateBufferedWriter 根据配置创建缓冲输出器
func CreateBufferedWriter(format, filename string, verbose bool) (Writer, error) {
	switch format {
	case "console":
		return NewConsoleWriter(verbose), nil
	case "json":
		if filename == "" {
			return nil, fmt.Errorf("JSON格式需要指定输出文件")
		}
		return NewBufferedJSONWriter(filename), nil
	case "csv":
		if filename == "" {
			return nil, fmt.Errorf("CSV格式需要指定输出文件")
		}
		return NewBufferedCSVWriter(filename), nil
	default:
		return nil, fmt.Errorf("不支持的输出格式: %s", format)
	}
}
