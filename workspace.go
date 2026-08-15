package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type WorkspaceItem struct {
	WorkspaceID string   `json:"workspaceId"`
	Path        string   `json:"path"`
	Title       string   `json:"title"`
	SessionIDs  []string `json:"sessionIds"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

type WorkspaceValue struct {
	Items              []WorkspaceItem `json:"items"`
	ArchivedSessionIDs []string        `json:"archivedSessionIds"`
}

var (
	workspaceValue = WorkspaceValue{
		Items:              []WorkspaceItem{},
		ArchivedSessionIDs: []string{},
	}
	workspaceMu     sync.RWMutex
	workspaceSubs   = make(map[chan struct{}]struct{})
	workspaceSubsMu sync.Mutex
)

func GetWorkspaces() WorkspaceValue {
	workspaceMu.RLock()
	defer workspaceMu.RUnlock()
	v := workspaceValue
	if v.Items == nil {
		v.Items = []WorkspaceItem{}
	}
	if v.ArchivedSessionIDs == nil {
		v.ArchivedSessionIDs = []string{}
	}
	return v
}

func clearWorkspaceValue() {
	workspaceMu.Lock()
	workspaceValue = WorkspaceValue{
		Items:              []WorkspaceItem{},
		ArchivedSessionIDs: []string{},
	}
	workspaceMu.Unlock()
	notifyWorkspace()
}

func SubscribeWorkspace(buf int) (<-chan struct{}, func()) {
	workspaceSubsMu.Lock()
	defer workspaceSubsMu.Unlock()
	ch := make(chan struct{}, buf)
	workspaceSubs[ch] = struct{}{}
	return ch, func() {
		workspaceSubsMu.Lock()
		delete(workspaceSubs, ch)
		workspaceSubsMu.Unlock()
	}
}

func notifyWorkspace() {
	workspaceSubsMu.Lock()
	subs := make([]chan struct{}, 0, len(workspaceSubs))
	for ch := range workspaceSubs {
		subs = append(subs, ch)
	}
	workspaceSubsMu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func fetchWorkspaces() error {
	cfg := GetConfig()
	port := cfg.ServerPort
	if port <= 0 {
		port = 3080
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/api/workspace.list", port)

	reqBody := map[string]any{
		"type":   "client-request",
		"rpcId":  "workspace-fetch",
		"method": "workspace.list",
		"payload": map[string]any{},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	if cfg.AccessPassword != "" {
		req.AddCookie(&http.Cookie{
			Name:  authCookieName,
			Value: cfg.AccessPassword,
		})
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		clearWorkspaceValue()
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		clearWorkspaceValue()
		return err
	}

	var rpcResp struct {
		Result struct {
			OK    bool           `json:"ok"`
			Value WorkspaceValue `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		clearWorkspaceValue()
		return err
	}
	if !rpcResp.Result.OK {
		clearWorkspaceValue()
		return fmt.Errorf("harness returned ok=false")
	}

	workspaceMu.Lock()
	workspaceValue = rpcResp.Result.Value
	workspaceMu.Unlock()

	notifyWorkspace()
	return nil
}

func StartWorkspacePolling() {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			_ = fetchWorkspaces()
		}
	}()
}

func handleGetWorkspaces(c *gin.Context) {
	if err := fetchWorkspaces(); err != nil {
		Fail(c, http.StatusBadGateway, "获取工作区失败: "+err.Error())
		return
	}
	OK(c, GetWorkspaces())
}