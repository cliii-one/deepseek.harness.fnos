package main

import (
	"embed"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/gin-gonic/gin"
)

//go:embed frontend/dist/*
var embeddedWebFS embed.FS

var globalPkgVar string

func main() {
	pkgVar := os.Getenv("DATA_LIBRARY_PATH")
	if pkgVar == "" {
		LogFatal("缺少 DATA_LIBRARY_PATH 环境变量")
	}
	globalPkgVar = pkgVar

	appdest := os.Getenv("TRIM_APPDEST")
	if appdest == "" {
		LogFatal("缺少 TRIM_APPDEST 环境变量")
	}

	InitLogger(pkgVar)
	LogInfo("启动 deepseek.harness...")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		LogInfo("收到信号 %s，停止子进程...", sig)
		_ = Stop()
		os.Exit(0)
	}()

	InitConfig(pkgVar)
	InitHarness(pkgVar, appdest)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	WebFS = embeddedWebFS
	InitRoutes(r)
	StartWorkspaceWatch()

	_ = os.MkdirAll(appdest, 0755)
	socketPath := filepath.Join(appdest, "web.sock")
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		LogFatal("监听 Unix Socket 失败 %s: %s", socketPath, err)
	}
	defer listener.Close()

	if err := os.Chmod(socketPath, 0666); err != nil {
		LogWarning("设置 Socket 权限失败: %s", err)
	}

	LogInfo("监听 %s", socketPath)
	if err := r.RunListener(listener); err != nil {
		LogFatal("服务退出: %s", err)
	}
}
