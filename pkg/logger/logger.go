package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// Level 日志级别
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var levelNames = map[Level]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
}

// Logger 日志记录器
type Logger struct {
	level  Level
	logger *log.Logger
	file   *os.File
}

// New 创建新的日志记录器
func New(level Level, filename string) (*Logger, error) {
	logger := &Logger{level: level}

	var writer io.Writer = os.Stderr

	// 如果指定了文件名，创建日志文件
	if filename != "" {
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("创建日志文件失败: %w", err)
		}
		logger.file = file
		writer = io.MultiWriter(os.Stderr, file)
	}

	logger.logger = log.New(writer, "", 0)
	return logger, nil
}

// Debug 记录调试信息
func (l *Logger) Debug(msg string, keyvals ...interface{}) {
	if l.level <= LevelDebug {
		l.log(LevelDebug, msg, keyvals...)
	}
}

// Info 记录信息
func (l *Logger) Info(msg string, keyvals ...interface{}) {
	if l.level <= LevelInfo {
		l.log(LevelInfo, msg, keyvals...)
	}
}

// Warn 记录警告
func (l *Logger) Warn(msg string, keyvals ...interface{}) {
	if l.level <= LevelWarn {
		l.log(LevelWarn, msg, keyvals...)
	}
}

// Error 记录错误
func (l *Logger) Error(msg string, keyvals ...interface{}) {
	if l.level <= LevelError {
		l.log(LevelError, msg, keyvals...)
	}
}

// log 内部日志记录方法
func (l *Logger) log(level Level, msg string, keyvals ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelName := levelNames[level]
	
	logMsg := fmt.Sprintf("[%s] [%s] %s", timestamp, levelName, msg)
	
	// 添加键值对
	if len(keyvals) > 0 {
		for i := 0; i < len(keyvals); i += 2 {
			if i+1 < len(keyvals) {
				logMsg += fmt.Sprintf(" %v=%v", keyvals[i], keyvals[i+1])
			}
		}
	}
	
	l.logger.Println(logMsg)
}

// Close 关闭日志记录器
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// SetLevel 设置日志级别
func (l *Logger) SetLevel(level Level) {
	l.level = level
}