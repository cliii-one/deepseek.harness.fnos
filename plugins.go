package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type pluginVerb string

const (
	pluginAdd     pluginVerb = "add"
	pluginRemove  pluginVerb = "remove"
	pluginUpdate  pluginVerb = "update"
	pluginList    pluginVerb = "list"
	pluginWhy     pluginVerb = "why"
	pluginInstall pluginVerb = "install"
)

var pluginVerbAliases = map[string]pluginVerb{
	"add": pluginAdd,
	// pnpm 的 install/i 是"按清单安装"，与 add 语义不同
	"install": pluginInstall, "i": pluginInstall,
	"remove": pluginRemove, "rm": pluginRemove, "uninstall": pluginRemove, "un": pluginRemove,
	"update": pluginUpdate, "up": pluginUpdate, "upgrade": pluginUpdate,
	"list": pluginList, "ls": pluginList,
	"why": pluginWhy,
}

var pluginNeedSpecs = map[pluginVerb]bool{
	pluginAdd: true, pluginRemove: true, pluginUpdate: true, pluginWhy: true,
}

var (
	npmSpecRe       = regexp.MustCompile(`^(@[a-z0-9-~][\w.-]*\/)?[a-z0-9-~][\w.-]*(@[0-9A-Za-z.*+~^<>=,\- ]+)?$`)
	gitURLRe        = regexp.MustCompile(`^(git\+)?(https?:\/\/|ssh:\/\/)[^\s;|&` + "`" + `$()]+(\.git)?(#[\w.\/-]+)?$`)
	gitShorthandRe  = regexp.MustCompile(`^github:[a-zA-Z0-9_.-]+\/[a-zA-Z0-9_.-]+(#[\w.\/-]+)?$`)
	localSpecRe     = regexp.MustCompile(`^(file:|\/).+$`)
	profileNameRe   = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	specForbiddenRe = regexp.MustCompile(`[;|&` + "`" + `$()\r\n]`)
)

func validatePluginSpec(spec string) error {
	if strings.TrimSpace(spec) == "" {
		return fmt.Errorf("包名为空")
	}
	if specForbiddenRe.MatchString(spec) {
		return fmt.Errorf("包含不允许的字符")
	}
	if npmSpecRe.MatchString(spec) || gitURLRe.MatchString(spec) || gitShorthandRe.MatchString(spec) || localSpecRe.MatchString(spec) {
		return nil
	}
	if strings.HasPrefix(spec, ".") {
		return fmt.Errorf("相对路径请使用 file: 绝对路径，或改用上传方式")
	}
	return fmt.Errorf("无法识别的包名/地址格式")
}

type pluginCommand struct {
	Verb    pluginVerb
	Profile string
	Specs   []string
	// AllowKey allowBuilds 归属键（上传用解析出的包名，命令用 spec）
	AllowKey string
}

// parsePluginCommand 仅接受 dsh plugin 形式，解析为受控 argv；非法输入一律拒绝
func parsePluginCommand(input string) (*pluginCommand, error) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return nil, fmt.Errorf("请输入插件命令")
	}
	if len(fields) < 2 || fields[0] != "dsh" || fields[1] != "plugin" {
		return nil, fmt.Errorf("请输入 dsh 命令，例如: dsh plugin add 包名")
	}

	profile := "web"
	rest := make([]string, 0, len(fields)-2)
	for i := 2; i < len(fields); i++ {
		tok := fields[i]
		if tok == "--profile" {
			if i+1 >= len(fields) {
				return nil, fmt.Errorf("--profile 缺少参数")
			}
			name := fields[i+1]
			if !profileNameRe.MatchString(name) {
				return nil, fmt.Errorf("非法的 profile 名称: %s", name)
			}
			profile = name
			i++
			continue
		}
		if strings.HasPrefix(tok, "--") {
			return nil, fmt.Errorf("不支持的参数: %s", tok)
		}
		rest = append(rest, tok)
	}
	if len(rest) == 0 {
		return nil, fmt.Errorf("缺少操作动词（支持 add / remove / update / list / why / install）")
	}

	verb, ok := pluginVerbAliases[rest[0]]
	if !ok {
		return nil, fmt.Errorf("未知操作 %q（支持 add / remove / update / list / why / install）", rest[0])
	}
	cmd := &pluginCommand{Verb: verb, Profile: profile, Specs: rest[1:]}

	if pluginNeedSpecs[cmd.Verb] && len(cmd.Specs) == 0 {
		return nil, fmt.Errorf("%s 操作需要一个或多个包名", cmd.Verb)
	}
	if cmd.Verb == pluginInstall && len(cmd.Specs) > 0 {
		return nil, fmt.Errorf("install 操作不接受包名参数")
	}
	if len(cmd.Specs) == 0 {
		cmd.Specs = nil
	}
	for _, s := range cmd.Specs {
		if err := validatePluginSpec(s); err != nil {
			return nil, fmt.Errorf("参数 %q: %s", s, err)
		}
	}
	return cmd, nil
}

func (c *pluginCommand) argv() []string {
	argv := []string{"dsh", "plugin", "--profile", c.Profile, string(c.Verb)}
	argv = append(argv, c.Specs...)
	return argv
}

func (c *pluginCommand) display() string {
	return "dsh plugin --profile " + c.Profile + " " + string(c.Verb) + " " + strings.Join(c.Specs, " ")
}

func pluginProfileDir() string {
	return filepath.Join(pkgVarDir, "dsh-data", "profiles", "web")
}

type pluginItem struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Spec    string `json:"spec"`
	Layer   bool   `json:"layer"`
}

type pluginListPayload struct {
	Profile string       `json:"profile"`
	Plugins []pluginItem `json:"plugins"`
	Builtin []string     `json:"builtin"`
	Bundles []string     `json:"bundles"`
}

func readProfileManifest() (deps map[string]string, bundles []string, err error) {
	data, err := os.ReadFile(filepath.Join(pluginProfileDir(), "package.json"))
	if err != nil {
		return nil, nil, err
	}
	var m struct {
		Dependencies map[string]string `json:"dependencies"`
		Dsh          *struct {
			Profile *struct {
				Bundles []string `json:"bundles"`
			} `json:"profile"`
		} `json:"dsh"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, nil, err
	}
	if m.Dependencies == nil {
		m.Dependencies = map[string]string{}
	}
	if m.Dsh != nil && m.Dsh.Profile != nil {
		bundles = m.Dsh.Profile.Bundles
	}
	return m.Dependencies, bundles, nil
}

// profileManifest profile package.json 完整结构（读改写）
type profileManifest struct {
	Name         string            `json:"name"`
	Private      bool              `json:"private"`
	Version      string            `json:"version,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Dsh          *struct {
		Profile *struct {
			Bundles []string `json:"bundles"`
		} `json:"profile"`
	} `json:"dsh,omitempty"`
}

func profileManifestPath() string {
	return filepath.Join(pluginProfileDir(), "package.json")
}

func readProfileManifestFull() (profileManifest, error) {
	var m profileManifest
	data, err := os.ReadFile(profileManifestPath())
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, err
	}
	if m.Dependencies == nil {
		m.Dependencies = map[string]string{}
	}
	return m, nil
}

func writeProfileManifestFull(m profileManifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(profileManifestPath(), data, 0644)
}

// applyPluginToggle 启用=加入层列表，禁用=移除；依赖不动，重启后生效
func applyPluginToggle(bundles []string, deps map[string]string, name string, enabled bool) ([]string, error) {
	idx := -1
	for i, b := range bundles {
		if b == name {
			idx = i
			break
		}
	}
	inBundles := idx >= 0
	if enabled {
		if _, ok := deps[name]; !ok {
			return nil, fmt.Errorf("插件 %s 未安装（不在依赖中），无法启用", name)
		}
		if !inBundles {
			return append(bundles, name), nil
		}
		return bundles, nil
	}
	if inBundles {
		out := make([]string, 0, len(bundles)-1)
		out = append(out, bundles[:idx]...)
		out = append(out, bundles[idx+1:]...)
		return out, nil
	}
	return bundles, nil
}

func togglePlugin(name string, enabled bool) (string, error) {
	m, err := readProfileManifestFull()
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("profile 尚未初始化，无法切换插件状态")
		}
		return "", fmt.Errorf("读取 profile manifest 失败: %s", err)
	}
	if m.Dsh == nil || m.Dsh.Profile == nil {
		return "", fmt.Errorf("profile manifest 缺少 dsh.profile 配置")
	}
	bundles, err := applyPluginToggle(m.Dsh.Profile.Bundles, m.Dependencies, name, enabled)
	if err != nil {
		return "", err
	}
	m.Dsh.Profile.Bundles = bundles
	if err := writeProfileManifestFull(m); err != nil {
		return "", fmt.Errorf("写入 profile manifest 失败: %s", err)
	}
	if enabled {
		return "插件已启用，重启服务后生效", nil
	}
	return "插件已禁用，重启服务后生效", nil
}

func installedPluginVersion(name string) string {
	candidates := []string{
		filepath.Join(pluginProfileDir(), "node_modules", name, "package.json"),
		filepath.Join(pkgVarDir, "dsh-data", "profiles", "node_modules", name, "package.json"),
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var m struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(data, &m) == nil && m.Version != "" {
			return m.Version
		}
	}
	return ""
}

func handleListPlugins(c *gin.Context) {
	deps, bundles, err := readProfileManifest()
	if err != nil {
		if os.IsNotExist(err) {
			// profile 尚未初始化
			OK(c, pluginListPayload{Profile: "web", Plugins: []pluginItem{}, Builtin: []string{}, Bundles: []string{}})
			return
		}
		Fail(c, http.StatusInternalServerError, "读取插件列表失败: "+err.Error())
		return
	}

	bundleSet := make(map[string]bool, len(bundles))
	for _, b := range bundles {
		bundleSet[b] = true
	}

	// 内置层：在 bundles 中但不在 dependencies（随发行版交付，不可管理）
	var builtin []string
	for _, b := range bundles {
		if _, isDep := deps[b]; !isDep {
			builtin = append(builtin, b)
		}
	}

	// 用户插件：dependencies 中的包（与 bundles 求交得到是否激活为层）
	names := make([]string, 0, len(deps))
	for name := range deps {
		names = append(names, name)
	}
	sort.Strings(names)
	plugins := make([]pluginItem, 0, len(names))
	for _, name := range names {
		plugins = append(plugins, pluginItem{
			Name:    name,
			Spec:    deps[name],
			Version: installedPluginVersion(name),
			Layer:   bundleSet[name],
		})
	}

	OK(c, pluginListPayload{Profile: "web", Plugins: plugins, Builtin: builtin, Bundles: bundles})
}

func handlePluginStatus(c *gin.Context) {
	OK(c, pluginStatusPayload())
}

type pluginOpState struct {
	Running bool   `json:"running"`
	OK      *bool  `json:"ok,omitempty"`
	Message string `json:"message,omitempty"`
}

var (
	pluginStateMu sync.Mutex
	pluginOp      pluginOpState
	pluginSubs    = make(map[chan struct{}]struct{})
	pluginSubsMu  sync.Mutex
)

func setPluginRunning() error {
	pluginStateMu.Lock()
	defer pluginStateMu.Unlock()
	if pluginOp.Running {
		return fmt.Errorf("插件操作正在进行中，请稍候")
	}
	if state.Status() == StatusBuilding {
		return fmt.Errorf("正在构建中，请稍候再试")
	}
	if _, err := os.Stat(filepath.Join(srcDir, "node_modules")); err != nil {
		return fmt.Errorf("依赖未安装，请先点击【强制重建】")
	}
	pluginOp = pluginOpState{Running: true}
	notifyPlugin()
	return nil
}

func setPluginDone(ok bool, msg string) {
	pluginStateMu.Lock()
	pluginOp = pluginOpState{Running: false, OK: &ok, Message: msg}
	pluginStateMu.Unlock()
	notifyPlugin()
}

func pluginStatusPayload() pluginOpState {
	pluginStateMu.Lock()
	defer pluginStateMu.Unlock()
	return pluginOp
}

func SubscribePlugin(buf int) (<-chan struct{}, func()) {
	pluginSubsMu.Lock()
	defer pluginSubsMu.Unlock()
	ch := make(chan struct{}, buf)
	pluginSubs[ch] = struct{}{}
	return ch, func() {
		pluginSubsMu.Lock()
		delete(pluginSubs, ch)
		pluginSubsMu.Unlock()
	}
}

func notifyPlugin() {
	pluginSubsMu.Lock()
	subs := make([]chan struct{}, 0, len(pluginSubs))
	for ch := range pluginSubs {
		subs = append(subs, ch)
	}
	pluginSubsMu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// pluginEnv 把 pnpm 可执行目录加进 PATH（dsh plugin 内部以 spawnSync("pnpm") 转发）
func pluginEnv() []string {
	env := buildEnv()
	pnpmDir := filepath.Join(pkgVarDir, "pnpm-env", "node_modules", ".bin")
	cur := ""
	prefix := "PATH="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			cur = strings.TrimPrefix(e, prefix)
			break
		}
	}
	return appendOrReplace(env, "PATH", pnpmDir+":"+cur)
}

// tailWriter 保留最近 max 字节输出，供失败时回显原因（并发写，需加锁）
type tailWriter struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func newTailWriter(max int) *tailWriter {
	return &tailWriter{max: max}
}

func (t *tailWriter) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(p) >= t.max {
		t.buf = append([]byte(nil), p[len(p)-t.max:]...)
		return len(p), nil
	}
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	return len(p), nil
}

func (t *tailWriter) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}

// pluginFailMessage 把进程错误与 stderr 尾部合并成可读的失败信息。
// 构建脚本被 pnpm 拦截的常见场景由 runPluginOpWithAutoAllow 自动放行并重试，
// 此处作为兜底：无法解析被拦包名或重试仍失败时附加指引。
func pluginFailMessage(err error, tail string) string {
	msg := err.Error()
	if tail = strings.TrimSpace(tail); tail != "" {
		msg += "\n" + tail
	}
	if strings.Contains(tail, "ERR_PNPM_IGNORED_BUILDS") ||
		strings.Contains(tail, "approve-builds") ||
		strings.Contains(tail, "allowBuilds") {
		msg += "\n提示：构建脚本被 pnpm 拦截。管理器已自动放行并重试；若仍失败，请检查 " +
			filepath.Join(pluginProfileDir(), "pnpm-workspace.yaml") + " 的 allowBuilds 配置或查看上方 pnpm 输出。"
	}
	return msg
}

// shortPluginFailReason 日志用简短失败原因，避免重复打进 pnpm 输出尾部
func shortPluginFailReason(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "ERR_PNPM_IGNORED_BUILDS") ||
		strings.Contains(msg, "approve-builds") ||
		strings.Contains(msg, "allowBuilds") {
		return "构建脚本被 pnpm 拦截（详见上方 pnpm 输出，按提示配置 allowBuilds 后重试）"
	}
	return msg
}

func runPluginSubprocess(argv []string) error {
	tail := newTailWriter(800)
	cmd := exec.Command(pnpmBin(), argv...)
	cmd.Dir = srcDir
	cmd.Env = pluginEnv()
	cmd.Stdout = io.MultiWriter(&logWriter{}, tail)
	cmd.Stderr = io.MultiWriter(&logWriter{}, tail)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%s", pluginFailMessage(err, tail.String()))
	}
	return nil
}

func runPluginSync(argv []string) (string, error) {
	cmd := exec.Command(pnpmBin(), argv...)
	cmd.Dir = srcDir
	cmd.Env = pluginEnv()
	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(&logWriter{}, &buf)
	cmd.Stderr = io.MultiWriter(&logWriter{}, &buf)
	err := cmd.Run()
	if err != nil {
		return buf.String(), fmt.Errorf("%s", pluginFailMessage(err, buf.String()))
	}
	return buf.String(), nil
}

func pluginAllowKey(cmd *pluginCommand) string {
	if cmd.AllowKey != "" {
		return cmd.AllowKey
	}
	if len(cmd.Specs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(cmd.Specs))
	for _, s := range cmd.Specs {
		keys = append(keys, normalizePluginKey(s))
	}
	return strings.Join(keys, " ")
}

// runPluginOpWithAutoAllow add/update/install 遇到 pnpm 拦截构建脚本时自动放行并重试一次
func runPluginOpWithAutoAllow(cmd *pluginCommand, doneMsg string) (string, error) {
	var out string
	var runErr error
	if cmd.Verb == pluginList || cmd.Verb == pluginWhy {
		out, runErr = runPluginSync(cmd.argv())
	} else {
		runErr = runPluginSubprocess(cmd.argv())
	}
	if runErr == nil {
		if out = strings.TrimSpace(out); out != "" {
			return out, nil
		}
		return doneMsg, nil
	}

	// 仅修改型安装类操作且是构建脚本拦截时自动放行
	if cmd.Verb != pluginAdd && cmd.Verb != pluginUpdate && cmd.Verb != pluginInstall {
		return "", runErr
	}
	pkgs := parseBlockedPackages(runErr.Error())
	if len(pkgs) == 0 {
		return "", runErr
	}

	LogWarning("检测到构建脚本被 pnpm 拦截（%s），由管理器自动写入 allowBuilds 并重试", strings.Join(pkgs, ", "))
	if err := ensureAllowBuildsFor(cmd.Profile, pluginAllowKey(cmd), pkgs); err != nil {
		return "", fmt.Errorf("%s\n（自动配置 allowBuilds 失败: %s）", runErr.Error(), err)
	}
	LogInfo("已自动放行构建脚本: %s，重新执行 %s", strings.Join(pkgs, ", "), cmd.display())
	if runErr = runPluginSubprocess(cmd.argv()); runErr != nil {
		return "", runErr
	}
	return doneMsg + "（已自动放行构建脚本: " + strings.Join(pkgs, ", ") + "）", nil
}

// launchPluginOp 在 goroutine 中执行插件操作（调用方需先 setPluginRunning 成功）
func launchPluginOp(cmd *pluginCommand, doneMsg string) {
	go func() {
		msg, runErr := runPluginOpWithAutoAllow(cmd, doneMsg)
		if runErr != nil {
			LogWarning("插件操作失败: %s", shortPluginFailReason(runErr))
			setPluginDone(false, runErr.Error())
			return
		}
		// 卸载成功后清理该插件相关的 allowBuilds 条目（best-effort）
		if cmd.Verb == pluginRemove && pluginAllowKey(cmd) != "" {
			if err := cleanupAllowBuildsFor(cmd.Profile, pluginAllowKey(cmd)); err != nil {
				LogWarning("清理 allowBuilds 失败: %s", err)
			}
		}
		LogInfo("插件操作完成: %s", msg)
		setPluginDone(true, msg)
	}()
}

const maxPluginUploadSize = 64 << 20 // 64MB

var pluginDirNameRe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func sanitizePluginDirName(filename string) string {
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	base = pluginDirNameRe.ReplaceAllString(base, "-")
	base = strings.Trim(base, ".-")
	if base == "" {
		base = "plugin"
	}
	return base
}

func detectArchiveType(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	head := make([]byte, 4)
	if _, err := io.ReadFull(f, head); err != nil {
		return "", fmt.Errorf("文件为空或读取失败")
	}
	switch {
	case head[0] == 0x1f && head[1] == 0x8b:
		return "tgz", nil
	case head[0] == 'P' && head[1] == 'K' && head[2] == 3 && head[3] == 4:
		return "zip", nil
	default:
		return "", fmt.Errorf("不支持的文件格式（仅支持 .tgz / .zip）")
	}
}

// safeJoin 防止 zip-slip：目标必须落在 base 内
func safeJoin(base, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || clean == ".." || filepath.IsAbs(clean) ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("非法路径: %s", name)
	}
	return filepath.Join(base, clean), nil
}

func extractTgzFile(path, dst string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeJoin(dst, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		default:
			// 忽略符号链接等非普通文件
		}
	}
}

func extractZipArchive(path, dst string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		target, err := safeJoin(dst, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
	}
	return nil
}

// findPackageRoot 定位包根：优先根目录，其次唯一的单层子目录
func findPackageRoot(dir string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return dir, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var found string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(dir, e.Name())
		if _, err := os.Stat(filepath.Join(sub, "package.json")); err == nil {
			if found != "" {
				return "", fmt.Errorf("发现多个插件包，无法确定安装目标")
			}
			found = sub
		}
	}
	if found == "" {
		return "", fmt.Errorf("未找到有效的插件包（缺少 package.json）")
	}
	return found, nil
}

// validatePluginPackage 校验 package.json 与 dsh.bundle.patch 声明
func validatePluginPackage(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return "", fmt.Errorf("读取 package.json 失败: %s", err)
	}
	var m struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Dsh     *struct {
			Bundle *struct {
				Patch string `json:"patch"`
			} `json:"bundle"`
		} `json:"dsh"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return "", fmt.Errorf("package.json 解析失败: %s", err)
	}
	if m.Name == "" {
		return "", fmt.Errorf("package.json 缺少 name 字段")
	}
	if m.Dsh == nil || m.Dsh.Bundle == nil || m.Dsh.Bundle.Patch == "" {
		return "", fmt.Errorf("不是 dsh 插件包：缺少 dsh.bundle.patch 声明")
	}
	patchPath := filepath.Join(dir, filepath.FromSlash(m.Dsh.Bundle.Patch))
	if !strings.HasPrefix(filepath.Clean(patchPath), filepath.Clean(dir)+string(filepath.Separator)) {
		return "", fmt.Errorf("dsh.bundle.patch 路径非法")
	}
	if _, err := os.Stat(patchPath); err != nil {
		return "", fmt.Errorf("dsh.bundle.patch 指向的文件不存在: %s", m.Dsh.Bundle.Patch)
	}
	return m.Name, nil
}

// pluginPreviewError 输入框仅支持安装（add），更新/卸载由列表按钮承担
func pluginPreviewError(cmd *pluginCommand) string {
	if cmd.Verb != pluginAdd {
		return "输入框仅支持安装（add）。更新/卸载请在下方已安装插件列表中操作"
	}
	return ""
}

func handlePluginPreview(c *gin.Context) {
	var req struct {
		Command string `json:"command"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	cmd, err := parsePluginCommand(req.Command)
	if err != nil {
		OK(c, gin.H{"ok": false, "reason": err.Error()})
		return
	}
	if reason := pluginPreviewError(cmd); reason != "" {
		OK(c, gin.H{"ok": false, "reason": reason})
		return
	}
	OK(c, gin.H{
		"ok":      true,
		"verb":    cmd.Verb,
		"profile": cmd.Profile,
		"specs":   cmd.Specs,
		"command": cmd.display(),
	})
}

func handlePluginRun(c *gin.Context) {
	var req struct {
		Command string `json:"command"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	cmd, err := parsePluginCommand(req.Command)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := setPluginRunning(); err != nil {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	LogInfo("插件操作开始: %s", cmd.display())

	doneMsg := "操作完成"
	switch cmd.Verb {
	case pluginAdd:
		doneMsg = "安装完成，重启服务后生效"
	case pluginRemove:
		doneMsg = "卸载完成，重启服务后生效"
	case pluginUpdate:
		doneMsg = "更新完成，重启服务后生效"
	case pluginInstall:
		doneMsg = "安装完成，重启服务后生效"
	}
	launchPluginOp(cmd, doneMsg)
	OK(c, gin.H{"command": cmd.display()})
}

func handlePluginToggle(c *gin.Context) {
	var req struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Name == "" {
		Fail(c, http.StatusBadRequest, "缺少插件名")
		return
	}
	// 与 pnpm 操作共用互斥，避免竞态改写 manifest
	if err := setPluginRunning(); err != nil {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	go func() {
		action := "禁用"
		if req.Enabled {
			action = "启用"
		}
		LogInfo("插件操作开始: %s插件 %s", action, req.Name)
		msg, err := togglePlugin(req.Name, req.Enabled)
		if err != nil {
			LogWarning("插件操作失败: %s", shortPluginFailReason(err))
			setPluginDone(false, err.Error())
			return
		}
		LogInfo("插件操作完成: %s", msg)
		setPluginDone(true, msg)
	}()
	OK(c, gin.H{"name": req.Name, "enabled": req.Enabled})
}

func handlePluginUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		Fail(c, http.StatusBadRequest, "未收到上传文件")
		return
	}
	if file.Size > maxPluginUploadSize {
		Fail(c, http.StatusBadRequest, "文件过大（上限 64MB）")
		return
	}

	// 先落盘到临时文件，便于按类型统一解包
	tmpPath := filepath.Join(pkgVarDir, "plugins", ".upload-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	if err := os.MkdirAll(filepath.Dir(tmpPath), 0755); err != nil {
		Fail(c, http.StatusInternalServerError, "创建临时目录失败: "+err.Error())
		return
	}
	defer os.Remove(tmpPath)

	f, err := file.Open()
	if err != nil {
		Fail(c, http.StatusInternalServerError, "读取上传文件失败: "+err.Error())
		return
	}
	out, err := os.Create(tmpPath)
	if err != nil {
		f.Close()
		Fail(c, http.StatusInternalServerError, "写入临时文件失败: "+err.Error())
		return
	}
	if _, err := io.Copy(out, f); err != nil {
		out.Close()
		f.Close()
		Fail(c, http.StatusInternalServerError, "写入临时文件失败: "+err.Error())
		return
	}
	out.Close()
	f.Close()

	kind, err := detectArchiveType(tmpPath)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	extractDir := filepath.Join(pkgVarDir, "plugins", sanitizePluginDirName(file.Filename))
	_ = os.RemoveAll(extractDir)
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		Fail(c, http.StatusInternalServerError, "创建解压目录失败: "+err.Error())
		return
	}

	switch kind {
	case "tgz":
		err = extractTgzFile(tmpPath, extractDir)
	case "zip":
		err = extractZipArchive(tmpPath, extractDir)
	}
	if err != nil {
		Fail(c, http.StatusBadRequest, "解压失败: "+err.Error())
		return
	}

	pkgDir, err := findPackageRoot(extractDir)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	name, err := validatePluginPackage(pkgDir)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	LogInfo("插件包校验通过: %s (%s)", name, pkgDir)

	if err := setPluginRunning(); err != nil {
		Fail(c, http.StatusConflict, err.Error())
		return
	}

	cmd := &pluginCommand{Verb: pluginAdd, Profile: "web", Specs: []string{"file:" + pkgDir}, AllowKey: name}
	LogInfo("插件操作开始: 安装上传插件 %s", name)
	launchPluginOp(cmd, fmt.Sprintf("「%s」安装完成，重启服务后生效", name))
	OK(c, gin.H{"command": cmd.display(), "name": name, "dir": pkgDir})
}
