package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var (
	logFile   *os.File
	logger    *log.Logger
	logOutput io.Writer

	logSubMu sync.Mutex
	logSubs  = map[chan string]struct{}{}
)

func InitLogger(pkgVar string) {
	_ = os.MkdirAll(pkgVar, 0755)
	logPath := filepath.Join(pkgVar, "harness.log")

	var err error
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		logOutput = io.MultiWriter(os.Stdout, broadcastWriter{})
	} else {
		logOutput = io.MultiWriter(os.Stdout, logFile, broadcastWriter{})
	}
	logger = log.New(logOutput, "", log.LstdFlags)
}

type broadcastWriter struct{}

func (broadcastWriter) Write(p []byte) (int, error) {
	s := string(p)
	logSubMu.Lock()
	for ch := range logSubs {
		select {
		case ch <- s:
		default:
		}
	}
	logSubMu.Unlock()
	return len(p), nil
}

// SubscribeLog 订阅日志增量，返回的函数用于取消订阅
func SubscribeLog(buf int) (<-chan string, func()) {
	ch := make(chan string, buf)
	logSubMu.Lock()
	logSubs[ch] = struct{}{}
	logSubMu.Unlock()
	return ch, func() {
		logSubMu.Lock()
		delete(logSubs, ch)
		logSubMu.Unlock()
	}
}

func LogInfo(format string, args ...any) {
	logger.Printf("[INFO]  "+format, args...)
}

func LogWarning(format string, args ...any) {
	logger.Printf("[WARN]  "+format, args...)
}

func LogFatal(format string, args ...any) {
	logger.Printf("[FATAL] "+format, args...)
	os.Exit(1)
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\([a-zA-Z]`)

func cleanLogLine(b []byte) string {
	s := string(b)
	s = ansiRegex.ReplaceAllString(s, "")
	s = strings.TrimRight(s, "\r\n ")
	s = strings.TrimLeft(s, "\r\n")
	return s
}

// LineLogWriter 行缓冲写入器，将外部子进程流转换为标准逐行日志
type LineLogWriter struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	logFunc func(format string, args ...any)
}

// NewLogWriterInfo 创建 INFO 级别行缓冲写入器
func NewLogWriterInfo() *LineLogWriter {
	return &LineLogWriter{logFunc: LogInfo}
}

// NewLogWriterWarn 创建 WARN 级别行缓冲写入器
func NewLogWriterWarn() *LineLogWriter {
	return &LineLogWriter{logFunc: LogWarning}
}

// NewLogWriter 根据级别创建行缓冲写入器
func NewLogWriter(level string) *LineLogWriter {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "WARN", "WARNING":
		return NewLogWriterWarn()
	default:
		return NewLogWriterInfo()
	}
}

func (w *LineLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := len(p)
	w.buf.Write(p)
	for {
		b := w.buf.Bytes()
		idx := bytes.IndexByte(b, '\n')
		if idx < 0 {
			break
		}
		line := b[:idx]
		clean := cleanLogLine(line)
		if clean != "" {
			w.logFunc("%s", clean)
		}
		w.buf.Next(idx + 1)
	}
	return n, nil
}

// Flush 刷出缓冲区残留内容
func (w *LineLogWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() > 0 {
		clean := cleanLogLine(w.buf.Bytes())
		w.buf.Reset()
		if clean != "" {
			w.logFunc("%s", clean)
		}
	}
}

// Close 实现 io.Closer 接口
func (w *LineLogWriter) Close() error {
	w.Flush()
	return nil
}
