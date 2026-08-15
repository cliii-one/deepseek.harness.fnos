package main

import (
	"embed"
	"io/fs"
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
		LogFatal("内嵌前端文件系统错误: %s", err)
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

// OK 成功响应：HTTP 200 + 裸数据
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, data)
}

// Fail 失败响应：HTTP 状态码 + {message}
func Fail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"message": msg})
}

func statusPayload() gin.H {
	status, uptime, lastMsg, commit, version, buildTime, startedAt := state.Snapshot()
	cfg := GetConfig()

	port := cfg.ProxyPort
	if port <= 0 {
		port = 2277
	}
	appURL := ":" + strconv.Itoa(port) + "/"

	return gin.H{
		"name":         "DeepSeek Harness",
		"version":      version,
		"commit":       commit,
		"status":       status,
		"uptime":       uptime,
		"started_at":   startedAt,
		"build_time":   buildTime,
		"app_url":      appURL,
		"last_message": lastMsg,
	}
}

// wsUpgrader WebSocket 升级器（允许任意 Origin）
var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// wsMsg WebSocket 消息：type 为 status/log，data 为负载
type wsMsg struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// handleWS WebSocket：状态与日志实时推送
func handleWS(c *gin.Context) {
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		LogWarning("WebSocket 升级失败: %s", err)
		return
	}
	defer conn.Close()

	// gorilla/websocket 不允许多协程并发写
	var writeMu sync.Mutex
	writeJSON := func(v any) {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.WriteJSON(v)
	}

	// 连接即快照
	writeJSON(wsMsg{Type: "status", Data: statusPayload()})
	writeJSON(wsMsg{Type: "workspace", Data: GetWorkspaces()})
	writeJSON(wsMsg{Type: "plugin", Data: pluginStatusPayload()})

	// 事件驱动：状态与日志变更即时推送
	stateCh, unsubscribeState := state.SubscribeState(16)
	defer unsubscribeState()
	logCh, unsubscribeLog := SubscribeLog(256)
	defer unsubscribeLog()
	wsCh, unsubscribeWs := SubscribeWorkspace(16)
	defer unsubscribeWs()
	pluginCh, unsubscribePlugin := SubscribePlugin(16)
	defer unsubscribePlugin()

	// 读循环：消费控制帧并检测断开
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
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
			writeJSON(wsMsg{Type: "status", Data: statusPayload()})
		case chunk := <-logCh:
			writeJSON(wsMsg{Type: "log", Data: chunk})
		case <-wsCh:
			writeJSON(wsMsg{Type: "workspace", Data: GetWorkspaces()})
		case <-pluginCh:
			writeJSON(wsMsg{Type: "plugin", Data: pluginStatusPayload()})
		case <-heartbeat.C:
			// 心跳 ping，防代理空闲断连
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

func handleGetLogs(c *gin.Context) {
	data, err := os.ReadFile(logFilePath())
	if err != nil {
		// 日志文件不存在属正常空态
		if os.IsNotExist(err) {
			OK(c, "")
			return
		}
		Fail(c, http.StatusInternalServerError, "读取日志失败: "+err.Error())
		return
	}
	OK(c, string(data))
}

func handleDeleteLogs(c *gin.Context) {
	err := os.Truncate(logFilePath(), 0)
	if err != nil && !os.IsNotExist(err) {
		// 文件不存在视为无需清空
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

func handleSaveConfig(c *gin.Context) {
	var cfg Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	// 比较配置是否真正变化
	oldCfg := GetConfig()
	proxyChanged := oldCfg.ServerPort != cfg.ServerPort || oldCfg.ProxyPort != cfg.ProxyPort

	cfg.BuildTime = GetBuildTime()
	cfg.Version = GetVersion()
	cfg.Commit = GetCommit()
	if err := SaveConfig(cfg); err != nil {
		Fail(c, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}

	// 当代理配置变化时重启反向代理
	if proxyChanged {
		restartReverseProxy()
	}
	state.Poke()
	OK(c, cfg)
}

func logFilePath() string {
	return filepath.Join(globalPkgVar, "harness.log")
}
