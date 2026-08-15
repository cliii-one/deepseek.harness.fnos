package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
	// lastWorkspaceMod 上次成功解析的文件修改时间，用于跳过未变化的重复解析
	lastWorkspaceMod time.Time
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

// workspaceFilePath dsh 持久化的工作区存储（$DSH_HOME/storages/workspace.json）
func workspaceFilePath() string {
	return filepath.Join(pkgVarDir, "dsh-data", "storages", "workspace.json")
}

// workspaceFile 对应 dsh workspace domain 的持久化结构（unit.version=2）
type workspaceFile struct {
	Unit struct {
		Name    string `json:"name"`
		Version int    `json:"version"`
	} `json:"unit"`
	Global struct {
		Initialized        bool     `json:"initialized"`
		WorkspaceIDs       []string `json:"workspaceIds"`
		ArchivedSessionIDs []string `json:"archivedSessionIds"`
	} `json:"global"`
	Tables struct {
		Workspaces map[string]workspaceFileRecord `json:"workspaces"`
	} `json:"tables"`
}

type workspaceFileRecord struct {
	Path       string   `json:"path"`
	Title      string   `json:"title"`
	SessionIDs []string `json:"sessionIds"`
	CreatedAt  string   `json:"createdAt"`
	UpdatedAt  string   `json:"updatedAt"`
}

func workspaceItemFromFile(id string, rec workspaceFileRecord) WorkspaceItem {
	return WorkspaceItem{
		WorkspaceID: id,
		Path:        rec.Path,
		Title:       rec.Title,
		SessionIDs:  rec.SessionIDs,
		CreatedAt:   rec.CreatedAt,
		UpdatedAt:   rec.UpdatedAt,
	}
}

// convertWorkspaceFile 转换 workspace.json 为展示结构；按 workspaceIds 顺序，
// 未列出记录按更新时间倒序兜底
func convertWorkspaceFile(data []byte) (WorkspaceValue, error) {
	var wf workspaceFile
	if err := json.Unmarshal(data, &wf); err != nil {
		return WorkspaceValue{}, fmt.Errorf("解析 workspace.json 失败: %s", err)
	}
	if wf.Unit.Name != "workspace" {
		return WorkspaceValue{}, fmt.Errorf("不支持的存储单元: %s", wf.Unit.Name)
	}
	// 版本兜底：当前解析 v2 结构，未来存储格式变更时在此兼容
	if wf.Unit.Version > 2 {
		return WorkspaceValue{}, fmt.Errorf("workspace.json 版本 %d 暂不支持", wf.Unit.Version)
	}
	if wf.Tables.Workspaces == nil {
		wf.Tables.Workspaces = map[string]workspaceFileRecord{}
	}

	val := WorkspaceValue{Items: []WorkspaceItem{}, ArchivedSessionIDs: []string{}}
	seen := make(map[string]bool, len(wf.Tables.Workspaces))
	for _, id := range wf.Global.WorkspaceIDs {
		rec, ok := wf.Tables.Workspaces[id]
		if !ok {
			continue
		}
		seen[id] = true
		val.Items = append(val.Items, workspaceItemFromFile(id, rec))
	}

	// 兜底：registry 未列出的记录（异常状态）按更新时间倒序追加
	var leftoverIDs []string
	var leftovers []workspaceFileRecord
	for id, rec := range wf.Tables.Workspaces {
		if !seen[id] {
			leftoverIDs = append(leftoverIDs, id)
			leftovers = append(leftovers, rec)
		}
	}
	sort.Slice(leftovers, func(i, j int) bool { return leftovers[i].UpdatedAt > leftovers[j].UpdatedAt })
	for i, rec := range leftovers {
		val.Items = append(val.Items, workspaceItemFromFile(leftoverIDs[i], rec))
	}

	val.ArchivedSessionIDs = wf.Global.ArchivedSessionIDs
	if val.ArchivedSessionIDs == nil {
		val.ArchivedSessionIDs = []string{}
	}
	return val, nil
}

// fetchWorkspaces 读取 workspace.json（不依赖服务运行），失败保留上次数据
func fetchWorkspaces() error {
	path := workspaceFilePath()
	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// dsh 尚未初始化工作区存储：保持现状
			return nil
		}
		return err
	}
	// 文件未变化则跳过重复解析
	if !st.ModTime().After(lastWorkspaceMod) && len(GetWorkspaces().Items) > 0 {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	val, err := convertWorkspaceFile(data)
	if err != nil {
		return err
	}
	lastWorkspaceMod = st.ModTime()
	workspaceMu.Lock()
	workspaceValue = val
	workspaceMu.Unlock()
	notifyWorkspace()
	return nil
}

// StartWorkspaceWatch 每秒检查 workspace.json mtime，变化即解析并通过 WebSocket 推送
func StartWorkspaceWatch() {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		var lastErr string
		for range ticker.C {
			if err := fetchWorkspaces(); err != nil {
				// 持续错误只记录一次，避免每秒刷屏
				if msg := err.Error(); msg != lastErr {
					LogWarning("读取工作区数据失败: %s", msg)
					lastErr = msg
				}
				continue
			}
			lastErr = ""
		}
	}()
}

func handleGetWorkspaces(c *gin.Context) {
	if err := fetchWorkspaces(); err != nil {
		Fail(c, http.StatusInternalServerError, "读取工作区失败: "+err.Error())
		return
	}
	OK(c, GetWorkspaces())
}
