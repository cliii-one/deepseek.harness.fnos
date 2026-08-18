package main

import (
	"embed"
	"io"
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

	// 注册飞牛网关直连 WebUI 代理路由
	InitFnGateway(base)

	api := base.Group("/api")
	{
		api.GET("/ws", handleWS)
		api.POST("/action", handleAction)
		api.GET("/logs", handleGetLogs)
		api.DELETE("/logs", handleDeleteLogs)
		api.GET("/logs/download", handleDownloadLogs)
		api.GET("/config", handleGetConfig)
		api.GET("/workspace/list", handleGetWorkspaces)
		api.GET("/plugins", handleListPlugins)
		api.GET("/plugins/status", handlePluginStatus)
		api.POST("/plugins/preview", handlePluginPreview)
		api.POST("/plugins/run", handlePluginRun)
		api.POST("/plugins/toggle", handlePluginToggle)
		api.POST("/plugins/disable-broken", handlePluginDisableAllBroken)
		api.POST("/plugins/cancel", handlePluginCancel)
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
		// 飞牛网关直连 WebUI 请求交由网关代理处理
		if strings.HasPrefix(c.Request.URL.Path, fnGatewayPrefix) {
			handleFnGateway(c)
			return
		}

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

func OK(c *gin.Context, data any) {
	OKMsg(c, "success", data)
}

func OKMsg(c *gin.Context, msg string, data any) {
	c.JSON(http.StatusOK, ApiResponse{
		Code:      0,
		Message:   msg,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	})
}

func Fail(c *gin.Context, status int, msg string) {
	c.JSON(status, ApiResponse{
		Code:      status,
		Message:   msg,
		Timestamp: time.Now().UnixMilli(),
	})
}

func statusPayload() gin.H {
	status, uptime, lastMsg, commit, version, buildTime, targetCommit, startedAt := state.Snapshot()
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

	var pidVal any
	if (status == StatusRunning || status == StatusStarting) {
		if data, err := os.ReadFile(pidFilePath()); err == nil {
			if p, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && p > 0 && isProcessAlive(p) {
				pidVal = p
			}
		}
	}

	var verVal any
	if version != "" {
		verVal = version
	}

	var commitVal any
	if commit != "" {
		commitVal = commit
	}

	var buildTimeVal any
	if buildTime != "" {
		buildTimeVal = buildTime
	}

	var uptimeVal any
	if uptime != "" {
		uptimeVal = uptime
	}

	return gin.H{
		"name":          "DeepSeek Harness",
		"version":       verVal,
		"commit":        commitVal,
		"target_commit": targetCommit,
		"status":        status,
		"uptime":        uptimeVal,
		"started_at":    startedAt,
		"server_port":   serverPort,
		"server_time":   time.Now().Unix(),
		"build_time":    buildTimeVal,
		"app_url":       appURL,
		"pid":           pidVal,
		"last_message":  lastMsg,
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
		if state.Status() == StatusStarting && req.Action == "start" {
			Fail(c, http.StatusConflict, "服务正在启动中，请稍候")
			return
		}
		var err error
		switch req.Action {
		case "start":
			err = Start()
		case "stop":
			err = Stop()
			if err == nil {
				SetLastRunState(StatusStopped)
			}
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

	var msg string
	switch req.Action {
	case "start":
		msg = "启动指令已发送，正在等待服务就绪…"
	case "stop":
		msg = "服务已停止"
	case "restart":
		msg = "重启指令已发送，正在等待服务就绪…"
	case "upgrade":
		msg = "开始拉取远程更新并构建…"
	case "rebuild":
		msg = "开始强制重建源码…"
	}

	OKMsg(c, msg, statusPayload())
}

// actionErrStatus 将动作前置错误映射为合适的 HTTP 状态码
func actionErrStatus(err error) int {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "源码不存在"):
		return http.StatusNotFound
	case strings.Contains(msg, "构建中"), strings.Contains(msg, "启动中"), strings.Contains(msg, "运行中"), strings.Contains(msg, "依赖未安装"):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

type LogPayload struct {
	Lines   []string `json:"lines"`
	Content string   `json:"content"`
}

// readLastNLines 高效获取日志文件末尾的最新 N 行
func readLastNLines(path string, maxLines int) ([]string, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, "", err
	}
	size := stat.Size()
	if size == 0 {
		return []string{}, "", nil
	}

	// 若文件较小（<= 512KB），直接全量读取并截取后 N 行
	if size <= 512*1024 {
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, "", err
		}
		rawLines := strings.Split(string(data), "\n")
		var lines []string
		for _, l := range rawLines {
			if l != "" {
				lines = append(lines, l+"\n")
			}
		}
		if len(lines) > maxLines {
			lines = lines[len(lines)-maxLines:]
		}
		return lines, strings.Join(lines, ""), nil
	}

	// 若文件较大，从尾部逆向读取最多 256KB
	readSize := int64(256 * 1024)
	if readSize > size {
		readSize = size
	}
	buf := make([]byte, readSize)
	_, err = file.ReadAt(buf, size-readSize)
	if err != nil && err != io.EOF {
		return nil, "", err
	}

	raw := string(buf)
	if readSize < size {
		// 丢弃第一条可能截断的不完整半行
		if idx := strings.IndexByte(raw, '\n'); idx >= 0 {
			raw = raw[idx+1:]
		}
	}

	rawLines := strings.Split(raw, "\n")
	var lines []string
	for _, l := range rawLines {
		if l != "" {
			lines = append(lines, l+"\n")
		}
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, strings.Join(lines, ""), nil
}

func handleGetLogs(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "150")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 150
	}
	lines, content, err := readLastNLines(logFilePath(), limit)
	if err != nil {
		if os.IsNotExist(err) {
			OK(c, LogPayload{Lines: []string{}, Content: ""})
			return
		}
		Fail(c, http.StatusInternalServerError, "读取日志失败: "+err.Error())
		return
	}
	OK(c, LogPayload{Lines: lines, Content: content})
}

func handleDeleteLogs(c *gin.Context) {
	if err := ClearLogs(); err != nil {
		Fail(c, http.StatusInternalServerError, "清空日志失败: "+err.Error())
		return
	}
	OKMsg(c, "运行日志已清空", true)
}

func handleDownloadLogs(c *gin.Context) {
	c.FileAttachment(logFilePath(), "harness.log")
}

func handleGetConfig(c *gin.Context) {
	OK(c, GetConfig())
}

func logFilePath() string {
	return filepath.Join(globalPkgVar, "harness.log")
}
