package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// allowBuilds 自动管理：安装/更新被 pnpm 拦截构建脚本时自动写入
// pnpm-workspace.yaml 的 allowBuilds 并重试，卸载时清理对应条目。
// 放行构建脚本 = 允许该包安装时执行任意代码（root），只处理 pnpm 明确报告的包。
var (
	// pnpm 错误行: "[ERR_PNPM_IGNORED_BUILDS] Ignored build scripts: a@1.0.0, b@2.0.0"
	blockedBuildsRe = regexp.MustCompile(`(?i)Ignored build scripts:\s*(.+)`)
	// 提取 "cloudflared@0.7.3" 或 "@scope/pkg@1.0.0" 中的包名
	pkgNameRe = regexp.MustCompile(`^(@?[a-zA-Z0-9][\w.-]*(?:/[@a-zA-Z0-9][\w.-]*)?)@[0-9]`)
	// npm 风格 spec 去掉版本号: "foo@^1.2.0" → "foo"，"@s/p@latest" → "@s/p"
	npmNameStripRe = regexp.MustCompile(`^((?:@[a-z0-9-~][\w.-]*/)?[a-z0-9-~][\w.-]*)@.+$`)
)

// parseBlockedPackages 从 pnpm 错误输出中提取被拦截构建脚本的包名
func parseBlockedPackages(tail string) []string {
	m := blockedBuildsRe.FindStringSubmatch(tail)
	if len(m) < 2 {
		return nil
	}
	var pkgs []string
	for _, part := range strings.Split(m[1], ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if name := pkgNameRe.FindStringSubmatch(part); len(name) >= 2 {
			pkgs = append(pkgs, name[1])
		} else {
			// 无版本号（理论上 pnpm 都会带），原样保留
			pkgs = append(pkgs, part)
		}
	}
	return pkgs
}

// normalizePluginKey 插件归属键：npm 包名去掉版本号，其他 spec 原样
func normalizePluginKey(spec string) string {
	if m := npmNameStripRe.FindStringSubmatch(spec); len(m) >= 2 {
		return m[1]
	}
	return spec
}

func profileDirFor(name string) string {
	return filepath.Join(pkgVarDir, "dsh-data", "profiles", name)
}

func profileWorkspaceYamlPathFor(name string) string {
	return filepath.Join(profileDirFor(name), "pnpm-workspace.yaml")
}

func allowBuildsSidecarPath() string {
	return filepath.Join(pkgVarDir, "plugins", "allowbuilds.json")
}

// readAllowBuildsSidecar package → 请求放行的插件键列表（仅记录面板自动管理的条目）
func readAllowBuildsSidecar() map[string][]string {
	m := map[string][]string{}
	data, err := os.ReadFile(allowBuildsSidecarPath())
	if err != nil {
		return m
	}
	_ = json.Unmarshal(data, &m)
	if m == nil {
		m = map[string][]string{}
	}
	return m
}

func writeAllowBuildsSidecar(m map[string][]string) error {
	if err := os.MkdirAll(filepath.Dir(allowBuildsSidecarPath()), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(allowBuildsSidecarPath(), data, 0644)
}

// yamlEntryName 从 "  name: true" 这类缩进行提取键名；注释/列表行返回空
func yamlEntryName(trimmed string) string {
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "-") {
		return ""
	}
	name := strings.SplitN(trimmed, ":", 2)[0]
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return name
}

// mergeAllowBuildsEntries 文本级合并 allowBuilds 块（保留文件其余内容与注释）。
// 无块则追加；已有条目仅布尔值保留，非法值（如占位串）改写为 true，缺失补插。
func mergeAllowBuildsEntries(yamlPath string, pkgs []string) error {
	content := ""
	if data, err := os.ReadFile(yamlPath); err == nil {
		content = string(data)
	} else if !os.IsNotExist(err) {
		return err
	}
	lines := strings.Split(content, "\n")

	idx := -1
	entryLine := map[string]int{}
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if idx < 0 {
			if trimmed == "allowBuilds:" || strings.HasPrefix(trimmed, "allowBuilds: ") {
				idx = i
			}
			continue
		}
		// 块内：缩进行收集条目，空白/注释继续，回到根级则块结束
		if trimmed == "" || strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t") {
			if name := yamlEntryName(trimmed); name != "" {
				entryLine[name] = i
			}
			continue
		}
		break
	}

	var missing []string
	var fix []int
	for _, p := range pkgs {
		i, ok := entryLine[p]
		if !ok {
			missing = append(missing, p)
			continue
		}
		parts := strings.SplitN(strings.TrimSpace(lines[i]), ":", 2)
		val := ""
		if len(parts) >= 2 {
			val = strings.TrimSpace(parts[1])
		}
		if val != "true" && val != "false" {
			fix = append(fix, i)
		}
	}
	if len(missing) == 0 && len(fix) == 0 {
		return nil
	}
	sort.Strings(missing)

	if idx < 0 {
		content = strings.TrimRight(content, "\n") + "\n\nallowBuilds:\n"
		for _, p := range missing {
			content += "  " + p + ": true\n"
		}
	} else {
		for _, i := range fix {
			name := yamlEntryName(strings.TrimSpace(lines[i]))
			lines[i] = "  " + name + ": true"
		}
		var out []string
		out = append(out, lines[:idx+1]...)
		for _, p := range missing {
			out = append(out, "  "+p+": true")
		}
		out = append(out, lines[idx+1:]...)
		content = strings.Join(out, "\n")
	}
	if err := os.MkdirAll(filepath.Dir(yamlPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(yamlPath, []byte(content), 0644)
}

// removeAllowBuildsEntries 删除指定包的 allowBuilds 条目（自动 true 及占位等非法条目；用户手写的 false 保留）
func removeAllowBuildsEntries(yamlPath string, pkgs []string) error {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	drop := map[string]bool{}
	for _, p := range pkgs {
		drop[p] = true
	}
	lines := strings.Split(string(data), "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t")) &&
			trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "-") {
			parts := strings.SplitN(trimmed, ":", 2)
			name := strings.TrimSpace(parts[0])
			if drop[name] && (len(parts) < 2 || strings.TrimSpace(parts[1]) != "false") {
				continue
			}
		}
		out = append(out, l)
	}
	return os.WriteFile(yamlPath, []byte(strings.Join(out, "\n")), 0644)
}

// ensureAllowBuildsFor 安装/更新被拦截时：写入 allowBuilds 并记录归属
func ensureAllowBuildsFor(profile, pluginKey string, pkgs []string) error {
	if err := mergeAllowBuildsEntries(profileWorkspaceYamlPathFor(profile), pkgs); err != nil {
		return err
	}
	sidecar := readAllowBuildsSidecar()
	for _, p := range pkgs {
		found := false
		for _, k := range sidecar[p] {
			if k == pluginKey {
				found = true
				break
			}
		}
		if !found {
			sidecar[p] = append(sidecar[p], pluginKey)
		}
	}
	return writeAllowBuildsSidecar(sidecar)
}

// cleanupAllowBuildsFor 卸载后：移除该插件的归属记录，孤儿条目从 allowBuilds 删除
func cleanupAllowBuildsFor(profile, pluginKey string) error {
	sidecar := readAllowBuildsSidecar()
	var orphan []string
	for pkg, keys := range sidecar {
		var keep []string
		for _, k := range keys {
			if k != pluginKey {
				keep = append(keep, k)
			}
		}
		if len(keep) == 0 {
			delete(sidecar, pkg)
			orphan = append(orphan, pkg)
		} else {
			sidecar[pkg] = keep
		}
	}
	if len(orphan) > 0 {
		if err := removeAllowBuildsEntries(profileWorkspaceYamlPathFor(profile), orphan); err != nil {
			return fmt.Errorf("清理 allowBuilds 失败: %s", err)
		}
	}
	return writeAllowBuildsSidecar(sidecar)
}
