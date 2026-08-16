package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var WebFS embed.FS

const basePath = "/app/deepseek-harness"

func InitRoutes(r *gin.Engine) {
	base := r.Group(basePath)

	api := base.Group("/api")
	{
		api.GET("/ws", handleWS)
		api.POST("/action", handleAction)
		api.GET("/logs", handleGetLogs)
		api.DELETE("/logs", handleDeleteLogs)
		api.GET("/logs/download", handleDownloadLogs)
		api.GET("/config", handleGetConfig)
		api.POST("/config", handleSaveConfig)
		api.GET("/workspace/list", handleGetWorkspaces)
		api.GET("/plugins", handleListPlugins)
		api.GET("/plugins/status", handlePluginStatus)
		api.POST("/plugins/preview", handlePluginPreview)
		api.POST("/plugins/run", handlePluginRun)
		api.POST("/plugins/upload", handlePluginUpload)
		api.POST("/plugins/toggle", handlePluginToggle)
	}

	sub, err := fs.Sub(WebFS, "frontend/dist")
	if err != nil {
		LogFatal("静态前端资源装载失败: %s", err)
	}
	fileServer := http.FileServer(http.FS(sub))

	base.GET("/", func(c *gin.Context) {
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})

	r.NoRoute(func(c *gin.Context) {
		// 未知 API 路径返回 404 JSON，不回退首页
		if strings.HasPrefix(c.Request.URL.Path, basePath+"/api/") {
			c.JSON(http.StatusNotFound, gin.H{"message": "接口不存在"})
			return
		}
		fp := strings.TrimPrefix(c.Request.URL.Path, basePath)
		if fp == "" {
			fp = "/"
		}
		f, err := sub.Open(strings.TrimPrefix(fp, "/"))
		if err == nil {
			f.Close()
			c.Request.URL.Path = fp
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}

// ApiResponse 统一 API 响应契约
type ApiResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// ResultOK 成功响应：HTTP 200 + code 0 + 数据
func ResultOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, ApiResponse{
		Code:      0,
		Message:   "success",
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	})
}

// ResultSuccess 成功响应携带自定义信息
func ResultSuccess(c *gin.Context, msg string, data any) {
	c.JSON(http.StatusOK, ApiResponse{
		Code:      0,
		Message:   msg,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	})
}

// ResultFail 失败响应：HTTP 状态码 + 业务错误码 + 错误信息
func ResultFail(c *gin.Context, httpStatus int, code int, msg string) {
	c.JSON(httpStatus, ApiResponse{
		Code:      code,
		Message:   msg,
		Timestamp: time.Now().UnixMilli(),
	})
}

// OK 快捷成功响应
func OK(c *gin.Context, data any) {
	ResultOK(c, data)
}

// Fail 快捷失败响应
func Fail(c *gin.Context, status int, msg string) {
	ResultFail(c, status, status, msg)
}

func statusPayload() gin.H {
	status, uptime, lastMsg, commit, version, buildTime, startedAt := state.Snapshot()
	cfg := GetConfig()

	port := cfg.ProxyPort
	if port <= 0 {
		port = 2299
	}
	appURL := ":" + strconv.Itoa(port) + "/"

	serverPort := cfg.ServerPort
	if serverPort <= 0 {
		serverPort = 2298
	}

	return gin.H{
		"name":         "DeepSeek Harness",
		"version":      version,
		"commit":       commit,
		"status":       status,
		"uptime":       uptime,
		"started_at":   startedAt,
		"server_port":  serverPort,
		"server_time":  time.Now().Unix(),
		"build_time":   buildTime,
		"app_url":      appURL,
		"last_message": lastMsg,
	}
}

// wsUpgrader WebSocket 升级器（允许任意 Origin）
var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// wsMsg WebSocket 消息：统一信封结构
type wsMsg struct {
	Type      string `json:"type"`
	Data      any    `json:"data"`
	Timestamp int64  `json:"timestamp"`
}

// handleWS WebSocket：状态与日志实时推送及双向心跳
func handleWS(c *gin.Context) {
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		LogWarning("WebSocket 连接升级失败: %s", err)
		return
	}
	defer conn.Close()

	// gorilla/websocket 不允许多协程并发写
	var writeMu sync.Mutex
	sendMsg := func(msgType string, data any) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.WriteJSON(wsMsg{
			Type:      msgType,
			Data:      data,
			Timestamp: time.Now().UnixMilli(),
		})
	}

	// 连接即推送最新快照
	sendMsg("status", statusPayload())
	sendMsg("workspace", GetWorkspaces())
	sendMsg("plugin", pluginStatusPayload())

	// 事件驱动：状态与日志变更即时推送
	stateCh, unsubscribeState := state.SubscribeState(16)
	defer unsubscribeState()
	logCh, unsubscribeLog := SubscribeLog(256)
	defer unsubscribeLog()
	wsCh, unsubscribeWs := SubscribeWorkspace(16)
	defer unsubscribeWs()
	pluginCh, unsubscribePlugin := SubscribePlugin(16)
	defer unsubscribePlugin()

	// 读循环：消费客户端 ping 等应用层控制帧并检测断开
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			var inMsg struct {
				Type string `json:"type"`
			}
			if err := conn.ReadJSON(&inMsg); err != nil {
				return
			}
			if inMsg.Type == "ping" {
				sendMsg("pong", gin.H{"server_time": time.Now().Unix()})
			}
		}
	}()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-done:
			return
		case <-stateCh:
			sendMsg("status", statusPayload())
		case chunk := <-logCh:
			sendMsg("log", chunk)
		case <-wsCh:
			sendMsg("workspace", GetWorkspaces())
		case <-pluginCh:
			sendMsg("plugin", pluginStatusPayload())
		case <-heartbeat.C:
			writeMu.Lock()
			_ = conn.WriteMessage(websocket.PingMessage, nil)
			writeMu.Unlock()
		}
	}
}

func handleAction(c *gin.Context) {
	var req struct {
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}

	switch req.Action {
	case "start", "stop", "restart":
		if state.Status() == StatusBuilding {
			Fail(c, http.StatusConflict, "正在构建中，请稍候再试")
			return
		}
		var err error
		switch req.Action {
		case "start":
			err = Start()
		case "stop":
			err = Stop()
		case "restart":
			err = Restart()
		}
		if err != nil {
			Fail(c, actionErrStatus(err), err.Error())
			return
		}
	case "upgrade", "rebuild":
		if state.Status() == StatusBuilding {
			Fail(c, http.StatusConflict, "正在构建中，请稍候再试")
			return
		}
		if req.Action == "upgrade" {
			Upgrade()
		} else {
			Rebuild()
		}
	default:
		Fail(c, http.StatusBadRequest, "未知操作: "+req.Action)
		return
	}

	OK(c, statusPayload())
}

// actionErrStatus 将动作前置错误映射为合适的 HTTP 状态码
func actionErrStatus(err error) int {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "源码不存在"):
		return http.StatusNotFound
	case strings.Contains(msg, "构建中"), strings.Contains(msg, "运行中"), strings.Contains(msg, "依赖未安装"):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

type LogPayload struct {
	Lines   []string `json:"lines"`
	Content string   `json:"content"`
}

func handleGetLogs(c *gin.Context) {
	data, err := os.ReadFile(logFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			OK(c, LogPayload{Lines: []string{}, Content: ""})
			return
		}
		Fail(c, http.StatusInternalServerError, "读取日志失败: "+err.Error())
		return
	}
	content := string(data)
	var lines []string
	if len(content) > 0 {
		rawLines := strings.Split(content, "\n")
		for _, l := range rawLines {
			if l != "" {
				lines = append(lines, l+"\n")
			}
		}
	}
	if lines == nil {
		lines = []string{}
	}
	OK(c, LogPayload{Lines: lines, Content: content})
}

func handleDeleteLogs(c *gin.Context) {
	err := os.Truncate(logFilePath(), 0)
	if err != nil && !os.IsNotExist(err) {
		Fail(c, http.StatusInternalServerError, "清空日志失败: "+err.Error())
		return
	}
	OK(c, true)
}

func handleDownloadLogs(c *gin.Context) {
	c.FileAttachment(logFilePath(), "harness.log")
}

func handleGetConfig(c *gin.Context) {
	OK(c, GetConfig())
}

// checkPortAvailable 检测 TCP 端口是否可用
func checkPortAvailable(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	_ = ln.Close()
	return nil
}

func handleSaveConfig(c *gin.Context) {
	var cfg Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	if cfg.ServerPort < 1 || cfg.ServerPort > 65535 || cfg.ProxyPort < 1 || cfg.ProxyPort > 65535 {
		Fail(c, http.StatusBadRequest, "端口号必须在 1 ~ 65535 之间")
		return
	}

	if cfg.ServerPort == cfg.ProxyPort {
		Fail(c, http.StatusBadRequest, fmt.Sprintf("内部监听端口与反向代理端口不能相同 (%d)", cfg.ServerPort))
		return
	}

	oldCfg := GetConfig()
	serverPortChanged := oldCfg.ServerPort != cfg.ServerPort
	proxyPortChanged := oldCfg.ProxyPort != cfg.ProxyPort

	if serverPortChanged {
		if err := checkPortAvailable(cfg.ServerPort); err != nil {
			Fail(c, http.StatusBadRequest, fmt.Sprintf("内部监听端口 %d 已被占用，请更换端口", cfg.ServerPort))
			return
		}
	}

	if proxyPortChanged {
		if err := checkPortAvailable(cfg.ProxyPort); err != nil {
			Fail(c, http.StatusBadRequest, fmt.Sprintf("反向代理端口 %d 已被占用，请更换端口", cfg.ProxyPort))
			return
		}
	}

	cfg.BuildTime = GetBuildTime()
	cfg.Version = GetVersion()
	cfg.Commit = GetCommit()
	if err := SaveConfig(cfg); err != nil {
		Fail(c, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}

	if serverPortChanged {
		if state.Status() == StatusRunning {
			LogInfo("服务端口已变更 (%d → %d)，正在自动重启服务", oldCfg.ServerPort, cfg.ServerPort)
			go func() {
				_ = Restart()
			}()
		} else {
			restartReverseProxy()
		}
	} else if proxyPortChanged {
		restartReverseProxy()
	}

	state.Poke()
	OK(c, cfg)
}

func logFilePath() string {
	return filepath.Join(globalPkgVar, "harness.log")
}
