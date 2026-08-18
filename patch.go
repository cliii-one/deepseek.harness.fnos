package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

var patchFileMu sync.Mutex

// 核心基础设施受保护模块正则（禁止被禁用或删除）
var protectedModulePatterns = []*regexp.Regexp{
	regexp.MustCompile(`^cordis:`),
	regexp.MustCompile(`^@deepseek-ai/cordis-plugin-`),
	regexp.MustCompile(`^@deepseek-ai/dsh-host-`),
	regexp.MustCompile(`^@deepseek-ai/dsh-client-`),
	regexp.MustCompile(`^@deepseek-ai/dsh-web`),
	regexp.MustCompile(`^@deepseek-ai/dsh-settings`),
	regexp.MustCompile(`^@deepseek-ai/dsh-credentials`),
	regexp.MustCompile(`^@deepseek-ai/dsh-session`),
	regexp.MustCompile(`^@deepseek-ai/dsh-storage`),
	regexp.MustCompile(`^@deepseek-ai/dsh-tools`),
	regexp.MustCompile(`^@deepseek-ai/dsh-system-prompt`),
	regexp.MustCompile(`^@deepseek-ai/dsh-agent`),
	regexp.MustCompile(`^@deepseek-ai/dsh-llm`),
	regexp.MustCompile(`^@deepseek-ai/dsh-shell`),
	regexp.MustCompile(`^@deepseek-ai/dsh-fs`),
	regexp.MustCompile(`^@deepseek-ai/dsh-sandbox`),
	regexp.MustCompile(`^@deepseek-ai/dsh-jobs`),
	regexp.MustCompile(`^@deepseek-ai/dsh-base`),
	regexp.MustCompile(`^@deepseek-ai/dsh-web-app`),
}

// IsProtectedPlugin 检查插件名或 ID 是否属于受保护的宿主基础设施
func IsProtectedPlugin(name string) bool {
	if name == "" {
		return false
	}
	for _, p := range protectedModulePatterns {
		if p.MatchString(name) {
			return true
		}
	}
	return false
}

// ProfileUserPatchPath 获取 profile 自身 cordis.patch.yml 绝对路径
func ProfileUserPatchPath(profile string) string {
	if profile == "" {
		profile = "web"
	}
	return filepath.Join(profileDirFor(profile), "cordis.patch.yml")
}

// CordisPatchRow 表示 cordis.patch.yml 中的单个 patch 条目
type CordisPatchRow struct {
	ID       string                 `yaml:"id,omitempty"`
	Name     string                 `yaml:"name,omitempty"`
	Disabled *bool                  `yaml:"disabled,omitempty"`
	Config   map[string]interface{} `yaml:"config,omitempty"`
	Insert   []CordisPatchRow       `yaml:"insert,omitempty"`
}

// ReadProfileUserPatch 读取并解析 profile 的 cordis.patch.yml
func ReadProfileUserPatch(profile string) ([]CordisPatchRow, error) {
	patchPath := ProfileUserPatchPath(profile)
	data, err := os.ReadFile(patchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []CordisPatchRow{}, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return []CordisPatchRow{}, nil
	}

	var rows []CordisPatchRow
	if err := yaml.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("解析 %s 失败: %w", patchPath, err)
	}
	return rows, nil
}

// WriteProfileUserPatch 写入 profile 的 cordis.patch.yml
func WriteProfileUserPatch(profile string, rows []CordisPatchRow) error {
	patchPath := ProfileUserPatchPath(profile)
	if err := os.MkdirAll(filepath.Dir(patchPath), 0755); err != nil {
		return err
	}

	if len(rows) == 0 {
		// 清空文件或写空列表
		return os.WriteFile(patchPath, []byte("[]\n"), 0644)
	}

	data, err := yaml.Marshal(rows)
	if err != nil {
		return fmt.Errorf("序列化 patch 失败: %w", err)
	}
	return os.WriteFile(patchPath, data, 0644)
}

// ExtractPluginEntryIDs 解析已安装插件的 Bundle Patch，提取其 Loader Entry ID 列表
func ExtractPluginEntryIDs(profile, packageName string) []string {
	var candidates []string

	// 1. 优先读插件 package.json 中声明的 dsh.bundle.patch
	pkgJsonPath := filepath.Join(profileDirFor(profile), "node_modules", packageName, "package.json")
	data, err := os.ReadFile(pkgJsonPath)
	if err == nil {
		var meta struct {
			Dsh *struct {
				Bundle *struct {
					Patch string `json:"patch"`
				} `json:"bundle"`
			} `json:"dsh"`
		}
		if jsonErr := json.Unmarshal(data, &meta); jsonErr == nil && meta.Dsh != nil && meta.Dsh.Bundle != nil && meta.Dsh.Bundle.Patch != "" {
			patchFile := filepath.Join(filepath.Dir(pkgJsonPath), filepath.FromSlash(meta.Dsh.Bundle.Patch))
			patchData, readErr := os.ReadFile(patchFile)
			if readErr == nil {
				var rows []CordisPatchRow
				if yamlErr := yaml.Unmarshal(patchData, &rows); yamlErr == nil {
					for _, r := range rows {
						if len(r.Insert) > 0 {
							for _, ins := range r.Insert {
								if ins.ID != "" {
									candidates = append(candidates, ins.ID)
								}
							}
						} else if r.ID != "" {
							candidates = append(candidates, r.ID)
						}
					}
				}
			}
		}
	}

	if len(candidates) == 0 {
		candidates = append(candidates, packageName)
	}
	return candidates
}

// ReadDisabledEntryMap 读取指定 profile 中已被标记为 disabled 的 entry id 集合
func ReadDisabledEntryMap(profile string) (map[string]bool, error) {
	patchFileMu.Lock()
	defer patchFileMu.Unlock()

	rows, err := ReadProfileUserPatch(profile)
	if err != nil {
		return nil, err
	}

	res := make(map[string]bool)
	for _, r := range rows {
		if r.ID != "" && r.Disabled != nil && *r.Disabled {
			res[r.ID] = true
		}
	}
	return res, nil
}

// SetPluginDisabled 在 profile 的 cordis.patch.yml 中设置插件 entry 的禁用/启用状态
func SetPluginDisabled(profile, packageName string, disabled bool) error {
	if IsProtectedPlugin(packageName) {
		return fmt.Errorf("核心基础设施插件 %q 受到保护，禁止更改启停状态", packageName)
	}

	patchFileMu.Lock()
	defer patchFileMu.Unlock()

	entryIDs := ExtractPluginEntryIDs(profile, packageName)
	if len(entryIDs) == 0 {
		entryIDs = []string{packageName}
	}

	rows, err := ReadProfileUserPatch(profile)
	if err != nil {
		return err
	}

	for _, targetID := range entryIDs {
		found := false
		var newRows []CordisPatchRow

		for _, r := range rows {
			if r.ID == targetID {
				found = true
				if disabled {
					val := true
					r.Disabled = &val
					newRows = append(newRows, r)
				} else {
					// 启用：若除 disabled 外没有其他自定义字段，则直接移除此 override 行以保持配置干净
					if len(r.Config) == 0 && r.Name == "" && len(r.Insert) == 0 {
						continue
					}
					r.Disabled = nil
					newRows = append(newRows, r)
				}
			} else {
				newRows = append(newRows, r)
			}
		}

		if !found && disabled {
			val := true
			newRows = append(newRows, CordisPatchRow{
				ID:       targetID,
				Disabled: &val,
			})
		}
		rows = newRows
	}

	if err := WriteProfileUserPatch(profile, rows); err != nil {
		return err
	}

	// 触发状态更新并记录日志
	stateAction := "启用"
	if disabled {
		stateAction = "禁用"
	}
	LogInfo("[Cordis Patch] 已通过 user patch %s 插件 %s (Entry IDs: %v)", stateAction, packageName, entryIDs)
	return nil
}

// RemovePluginFromProfileUserPatch 彻底从 cordis.patch.yml 中物理剔除该插件的所有条目
func RemovePluginFromProfileUserPatch(profile, packageName string, entryIDs ...string) error {
	patchFileMu.Lock()
	defer patchFileMu.Unlock()

	rows, err := ReadProfileUserPatch(profile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	idSet := make(map[string]bool)
	idSet[packageName] = true
	for _, id := range entryIDs {
		if id != "" {
			idSet[id] = true
		}
	}

	var newRows []CordisPatchRow
	for _, r := range rows {
		if idSet[r.ID] || idSet[r.Name] {
			continue
		}
		newRows = append(newRows, r)
	}

	return WriteProfileUserPatch(profile, newRows)
}

// 插件故障诊断记录器
type PluginFailureInfo struct {
	Target    string    `json:"target"`
	EntryID   string    `json:"entryId,omitempty"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

var (
	diagMu        sync.RWMutex
	failedPlugins = make(map[string]PluginFailureInfo)

	// 状态机：跟踪上一行是否为 "Failed to load plugins"
	lastLineWasFailedToLoad bool
)

var (
	// 匹配 failed to apply loader entry 8ac07933 (dsh-vision-router): ...
	loaderEntryErrRe = regexp.MustCompile(`(?i)failed to apply loader entry\s*([0-9a-fA-F_-]+)?\s*(?:\(([^)]+)\))?:\s*(.+)`)
	// 匹配 [ERROR] failed to load plugin dsh-xxx / Plugin 'xxx' failed to load
	genericPluginErrRe = regexp.MustCompile(`(?i)(?:plugin\s+['"]?([@a-zA-Z0-9/._-]+)['"]?\s+failed|failed to load plugin\s+['"]?([@a-zA-Z0-9/._-]+)['"]?):\s*(.+)`)
	// 匹配包名格式（用于多行 Failed to load plugins 下一行的包名提取）
	pkgNamePatternRe = regexp.MustCompile(`^[@a-zA-Z0-9/._-]+$`)
	// 匹配服务依赖死锁挂起: @deepseek-ai/dsh-host-apiproxy: pending (waiting for service: attachments)
	servicePendingErrRe = regexp.MustCompile(`(?i)([@a-zA-Z0-9/._-]+):\s*pending\s*\(waiting for service:\s*([^)]+)\)`)
)

// RecordPluginFailureRecord 记录插件加载失败信息
func RecordPluginFailureRecord(pkgName, entryID, reason string) {
	pkgName = strings.TrimSpace(pkgName)
	entryID = strings.TrimSpace(entryID)
	reason = strings.TrimSpace(reason)

	if pkgName == "" && entryID == "" {
		return
	}

	diagMu.Lock()
	defer diagMu.Unlock()

	key := pkgName
	if key == "" {
		key = entryID
	}

	failedPlugins[key] = PluginFailureInfo{
		Target:    pkgName,
		EntryID:   entryID,
		Reason:    reason,
		Timestamp: time.Now(),
	}
	LogWarning("[插件故障捕获] 成功捕获插件崩溃: %s (Entry: %s) -> %s", pkgName, entryID, reason)
}

// ClearPluginFailure 清理特定插件的故障记录
func ClearPluginFailure(pkgName string) {
	diagMu.Lock()
	defer diagMu.Unlock()
	delete(failedPlugins, pkgName)
}

// ClearAllPluginFailures 清理全部故障记录（如每次主服务发起新一轮启动前）
func ClearAllPluginFailures() {
	diagMu.Lock()
	defer diagMu.Unlock()
	lastLineWasFailedToLoad = false
	if len(failedPlugins) > 0 {
		failedPlugins = make(map[string]PluginFailureInfo)
	}
}

// GetFailedPlugins 获取当前所有故障插件映射快照
func GetFailedPlugins() map[string]PluginFailureInfo {
	diagMu.RLock()
	defer diagMu.RUnlock()
	res := make(map[string]PluginFailureInfo, len(failedPlugins))
	for k, v := range failedPlugins {
		res[k] = v
	}
	return res
}

// DisableAllBrokenPlugins 一键禁用所有当前记录的故障插件
func DisableAllBrokenPlugins(profile string) ([]string, error) {
	failed := GetFailedPlugins()
	if len(failed) == 0 {
		return nil, nil
	}

	var disabledNames []string
	for name := range failed {
		if IsProtectedPlugin(name) {
			continue
		}
		if err := SetPluginDisabled(profile, name, true); err != nil {
			LogWarning("[一键自愈] 禁用故障插件 %s 失败: %s", name, err)
			continue
		}
		ClearPluginFailure(name)
		disabledNames = append(disabledNames, name)
	}

	LogInfo("[一键自愈] 已批量禁用故障插件: %v", disabledNames)
	return disabledNames, nil
}

// ParseAndRecordStderrDiagnostics 从 stderr/stdout 输出行中解析并记录插件崩溃信息
func ParseAndRecordStderrDiagnostics(text string) {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return
	}

	// 1. 匹配 loader entry 报错
	if m := loaderEntryErrRe.FindStringSubmatch(clean); len(m) >= 4 {
		entryID := strings.TrimSpace(m[1])
		pkgName := strings.TrimSpace(m[2])
		reason := strings.TrimSpace(m[3])
		if pkgName == "" {
			pkgName = entryID
		}
		RecordPluginFailureRecord(pkgName, entryID, reason)
		lastLineWasFailedToLoad = false
		return
	}

	// 2. 匹配通用插件失败行
	if m := genericPluginErrRe.FindStringSubmatch(clean); len(m) >= 4 {
		pkgName := strings.TrimSpace(m[1])
		if pkgName == "" {
			pkgName = strings.TrimSpace(m[2])
		}
		reason := strings.TrimSpace(m[3])
		RecordPluginFailureRecord(pkgName, "", reason)
		lastLineWasFailedToLoad = false
		return
	}

	// 3. 匹配服务死锁挂起行
	if m := servicePendingErrRe.FindStringSubmatch(clean); len(m) >= 3 {
		target := strings.TrimSpace(m[1])
		svc := strings.TrimSpace(m[2])
		RecordPluginFailureRecord(target, "", fmt.Sprintf("服务挂起: 等待 %s 超时 (前置插件崩溃引发连锁中断)", svc))
		lastLineWasFailedToLoad = false
		return
	}

	// 4. 状态机：处理多行 "Failed to load plugins" 结构
	if strings.EqualFold(clean, "Failed to load plugins") || strings.EqualFold(clean, "Failed to load plugin") {
		lastLineWasFailedToLoad = true
		return
	}

	if lastLineWasFailedToLoad {
		if pkgNamePatternRe.MatchString(clean) {
			RecordPluginFailureRecord(clean, "", "启动时加载失败 (Failed to load plugin)")
		}
		lastLineWasFailedToLoad = false
	}
}
