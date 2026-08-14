package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var WebFS embed.FS

const basePath = "/app/deepseek-harness"

func InitRoutes(r *gin.Engine) {
	base := r.Group(basePath)

	api := base.Group("/api")
	{
		api.GET("/events", handleEvents)
		api.POST("/action", handleAction)
		api.GET("/logs", handleGetLogs)
		api.DELETE("/logs", handleDeleteLogs)
		api.GET("/logs/download", handleDownloadLogs)
		api.GET("/config", handleGetConfig)
		api.POST("/config", handleSaveConfig)
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

func R(c *gin.Context, data any) {
	c.JSON(200, gin.H{"code": 0, "data": data})
}

func RE(c *gin.Context, code int, msg string) {
	c.JSON(200, gin.H{"code": code, "message": msg})
}

func statusPayload() gin.H {
	status, uptime, lastMsg, commit, version, buildTime := state.Snapshot()
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
		"build_time":   buildTime,
		"app_url":      appURL,
		"last_message": lastMsg,
	}
}

func handleStatus(c *gin.Context) {
	R(c, statusPayload())
}

// handleEvents SSE：状态变更与日志增量实时推送
func handleEvents(c *gin.Context) {
	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	writeSSE(w, "status", statusPayload())
	w.Flush()

	logCh, unsubscribe := SubscribeLog(256)
	defer unsubscribe()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	lastStatus, _ := json.Marshal(statusPayload())

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case chunk := <-logCh:
			writeSSE(w, "log", chunk)
			w.Flush()
		case <-ticker.C:
			cur, _ := json.Marshal(statusPayload())
			if !bytes.Equal(cur, lastStatus) {
				lastStatus = cur
				writeSSE(w, "status", statusPayload())
				w.Flush()
			}
		case <-heartbeat.C:
			_, _ = fmt.Fprint(w, ": hb\n\n")
			w.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, event string, data any) {
	payload, err := json.Marshal(data)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
}

func handleAction(c *gin.Context) {
	var req struct {
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RE(c, 1, "参数错误")
		return
	}

	var err error
	switch req.Action {
	case "start", "stop", "restart":
		if state.Status() == StatusBuilding {
			RE(c, 1, "正在构建中，请稍候再试")
			return
		}
		switch req.Action {
		case "start":
			err = Start()
		case "stop":
			err = Stop()
		case "restart":
			err = Restart()
		}
	case "upgrade":
		if state.Status() == StatusBuilding {
			RE(c, 1, "正在构建中，请稍候再试")
			return
		}
		Upgrade()
	case "rebuild":
		if state.Status() == StatusBuilding {
			RE(c, 1, "正在构建中，请稍候再试")
			return
		}
		Rebuild()
	default:
		RE(c, 1, "未知操作: "+req.Action)
		return
	}

	if err != nil {
		RE(c, 1, err.Error())
		return
	}
	handleStatus(c)
}

func handleGetLogs(c *gin.Context) {
	data, err := os.ReadFile(logFilePath())
	if err != nil {
		R(c, "")
		return
	}
	R(c, string(data))
}

func handleDeleteLogs(c *gin.Context) {
	_ = os.Truncate(logFilePath(), 0)
	R(c, nil)
}

func handleDownloadLogs(c *gin.Context) {
	c.FileAttachment(logFilePath(), "harness.log")
}

func handleGetConfig(c *gin.Context) {
	R(c, GetConfig())
}

func handleSaveConfig(c *gin.Context) {
	var cfg Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		RE(c, 1, "参数错误: "+err.Error())
		return
	}
	cfg.BuildTime = GetBuildTime()
	cfg.Version = GetVersion()
	cfg.Commit = GetCommit()
	if err := SaveConfig(cfg); err != nil {
		RE(c, 1, "保存失败: "+err.Error())
		return
	}
	R(c, cfg)
}

func logFilePath() string {
	return filepath.Join(globalPkgVar, "harness.log")
}