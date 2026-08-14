package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
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

// AppendToLog 将子进程输出写入日志
func AppendToLog(data []byte) {
	if logOutput != nil {
		_, _ = logOutput.Write(data)
	}
}
