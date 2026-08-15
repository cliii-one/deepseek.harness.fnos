package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const repoURL = "https://github.com/deepseek-ai/deepseek-harness"

const (
	StatusStopped  = "stopped"
	StatusRunning  = "running"
	StatusBuilding = "building"
)

type HarnessState struct {
	mu          sync.RWMutex
	status      string
	startTime   time.Time
	lastMessage string
	stateSubs   map[chan struct{}]struct{}
}

var state = &HarnessState{status: StatusStopped, stateSubs: make(map[chan struct{}]struct{})}

func (s *HarnessState) SetStatus(status, msg string) {
	s.mu.Lock()
	becameRunning := status == StatusRunning && s.status != status
	changed := s.status != status || s.lastMessage != msg
	s.status = status
	s.lastMessage = msg
	if becameRunning {
		s.startTime = time.Now()
	}
	s.mu.Unlock()
	if changed {
		s.notify()
	}
}

func (s *HarnessState) Status() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *HarnessState) Snapshot() (status, uptime, lastMsg, commit, version, buildTime string, startedAt int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status = s.status
	lastMsg = s.lastMessage
	commit = GetCommit()
	version = GetVersion()
	if status == StatusRunning && !s.startTime.IsZero() {
		startedAt = s.startTime.Unix()
		uptime = formatDuration(time.Since(s.startTime))
	} else {
		uptime = "-"
	}
	buildTime = GetBuildTime()
	if buildTime == "" {
		buildTime = "-"
	}
	return
}

func (s *HarnessState) notify() {
	s.mu.RLock()
	subs := make([]chan struct{}, 0, len(s.stateSubs))
	for ch := range s.stateSubs {
		subs = append(subs, ch)
	}
	s.mu.RUnlock()
	for _, ch := range subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (s *HarnessState) SubscribeState(buf int) (<-chan struct{}, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan struct{}, buf)
	s.stateSubs[ch] = struct{}{}
	return ch, func() {
		s.mu.Lock()
		delete(s.stateSubs, ch)
		s.mu.Unlock()
	}
}

func (s *HarnessState) Poke() {
	s.notify()
}

type managedProcess struct {
	cmd           *exec.Cmd
	stopRequested bool
	done          chan struct{}
}

var (
	procMu    sync.Mutex
	process   *managedProcess
	srcDir    string
	appDest   string
	pkgVarDir string
)

const nodeBinDir = "/var/apps/nodejs_v24/target/bin"
const gitBin = "/usr/bin/git"

func npmBin() string  { return filepath.Join(nodeBinDir, "npm") }
func pnpmBin() string { return filepath.Join(pkgVarDir, "pnpm-env", "node_modules", ".bin", "pnpm") }

func InitHarness(pkgVar, appdest string) {
	pkgVarDir = pkgVar
	srcDir = filepath.Join(pkgVar, "src", "deepseek-harness")
	appDest = appdest

	KillHarness()

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		state.SetStatus(StatusBuilding, "正在准备初始化...")
		go func() {
			zipPath := filepath.Join(appDest, "deepseek-harness.zip")
			if _, err := os.Stat(zipPath); err == nil {
				state.SetStatus(StatusBuilding, "正在解压源码包...")
				LogInfo("解压 %s", zipPath)
				if err := extractZip(zipPath, filepath.Dir(srcDir)); err != nil {
					LogWarning("解压失败: %s", err)
					state.SetStatus(StatusStopped, "解压失败，请点击【更新构建】重试")
					return
				}
				_ = os.Remove(zipPath)
			} else {
				state.SetStatus(StatusBuilding, "正在克隆源码...")
				LogInfo("git clone %s", repoURL)
				if err := gitClone(); err != nil {
					LogWarning("git clone 失败: %s", err)
					state.SetStatus(StatusStopped, "克隆失败，请检查网络后点击【更新构建】")
					return
				}
			}

			if err := buildSource(); err != nil {
				LogWarning("构建失败: %s", err)
				state.SetStatus(StatusStopped, "构建失败，请点击【更新构建】重试")
				return
			}
			refreshCommit()
			state.SetStatus(StatusStopped, "")
			LogInfo("安装完成，启动服务...")
			if err := Start(); err != nil {
				LogWarning("启动失败: %s", err)
			}
		}()
		return
	}

	refreshCommit()

	zipPath := filepath.Join(appDest, "deepseek-harness.zip")
	if _, err := os.Stat(zipPath); err == nil {
		_ = os.Remove(zipPath)
		LogInfo("srcDir 已存在，清理 deepseek-harness.zip: %s", zipPath)
	}
}

func Start() error {
	procMu.Lock()
	defer procMu.Unlock()

	if state.Status() == StatusBuilding {
		return fmt.Errorf("正在构建中，请稍候再试")
	}
	if state.Status() == StatusRunning {
		return fmt.Errorf("服务已在运行中")
	}
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("源码不存在，请先点击【拉取更新】进行初始化")
	}
	if _, err := os.Stat(filepath.Join(srcDir, "node_modules")); os.IsNotExist(err) {
		return fmt.Errorf("依赖未安装，请先点击【强制重建】进行构建")
	}

	return startLocked()
}

func startLocked() error {
	killHarnessLocked()

	cfg := GetConfig()
	port := cfg.ServerPort
	if port <= 0 {
		port = 3080
	}

	cmd := exec.Command(pnpmBin(), "dsh", "web", "--port", fmt.Sprintf("%d", port))
	cmd.Dir = srcDir
	cmd.Env = buildEnv()
	cmd.Stdout = &logWriter{}
	cmd.Stderr = &logWriter{}
	setProcessGroup(cmd)

	LogInfo("启动 deepseek-harness 端口=%d", port)

	if err := cmd.Start(); err != nil {
		state.SetStatus(StatusStopped, "启动失败: "+err.Error())
		return err
	}

	mp := &managedProcess{cmd: cmd, done: make(chan struct{})}
	process = mp

	_ = os.WriteFile(pidFilePath(), []byte(strconv.Itoa(cmd.Process.Pid)), 0644)

	state.SetStatus(StatusRunning, "")
	LogInfo("启动成功，PID=%d", cmd.Process.Pid)

	startReverseProxy()

	go func(mp *managedProcess) {
		err := mp.cmd.Wait()

		procMu.Lock()
		current := process
		if current == mp {
			process = nil
			removePidFileIfMatches(mp.cmd.Process.Pid)
			stopReverseProxy()

			if mp.stopRequested {
				state.SetStatus(StatusStopped, "")
			} else if err != nil {
				LogWarning("进程意外退出: %s", err)
				state.SetStatus(StatusStopped, "进程意外退出: "+err.Error())
			} else {
				state.SetStatus(StatusStopped, "")
			}
		} else {
			removePidFileIfMatches(mp.cmd.Process.Pid)
		}
		procMu.Unlock()

		close(mp.done)
	}(mp)

	return nil
}

func stopAndWait() {
	procMu.Lock()
	mp := process
	if mp != nil {
		mp.stopRequested = true
		LogInfo("停止进程 PID=%d", mp.cmd.Process.Pid)
		killProcessGroup(mp.cmd.Process.Pid)
		removePidFileIfMatches(mp.cmd.Process.Pid)
	}
	procMu.Unlock()

	if mp != nil {
		<-mp.done
	}
}

func Stop() error {
	stopAndWait()
	return nil
}

func Restart() error {
	stopAndWait()
	return Start()
}

func Upgrade() {
	state.SetStatus(StatusBuilding, "正在准备更新...")
	go update(false)
}

func Rebuild() {
	state.SetStatus(StatusBuilding, "正在准备强制重建...")
	go update(true)
}

func update(forceRebuild bool) {
	stopAndWait()

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		if forceRebuild {
			state.SetStatus(StatusStopped, "源码不存在，请先拉取更新")
			return
		}
		state.SetStatus(StatusBuilding, "正在检查远程更新...")
		if err := gitClone(); err != nil {
			LogWarning("git clone 失败: %s", err)
			state.SetStatus(StatusStopped, "克隆失败: "+err.Error())
			return
		}
		if err := buildSource(); err != nil {
			LogWarning("构建失败: %s", err)
			state.SetStatus(StatusStopped, "构建失败: "+err.Error())
			return
		}
		refreshCommit()
		restartService()
		return
	}

	if forceRebuild {
		state.SetStatus(StatusBuilding, "正在强制重建...")
	} else {
		commitBefore := gitHead()
		state.SetStatus(StatusBuilding, "正在拉取远程更新...")
		if err := gitPull(); err != nil {
			LogWarning("git pull 失败: %s", err)
			state.SetStatus(StatusStopped, "git pull 失败: "+err.Error())
			return
		}
		commitAfter := gitHead()
		if commitBefore != "" && commitBefore == commitAfter {
			LogInfo("已是最新版本（%s），跳过构建", commitAfter)
			refreshCommit()
			restartService()
			return
		}
		LogInfo("更新: %s → %s，开始构建...", commitBefore, commitAfter)
	}

	if err := buildSource(); err != nil {
		LogWarning("构建失败: %s", err)
		state.SetStatus(StatusStopped, "构建失败: "+err.Error())
		return
	}

	refreshCommit()
	restartService()
}

func restartService() {
	LogInfo("构建完成，重启服务...")
	stopAndWait()
	state.SetStatus(StatusStopped, "")
	if err := Start(); err != nil {
		LogWarning("启动失败: %s", err)
		state.SetStatus(StatusStopped, "启动失败: "+err.Error())
	}
}

func killHarnessLocked() {
	// 1. 优先清理内存中的当前进程
	if process != nil && process.cmd != nil && process.cmd.Process != nil {
		pid := process.cmd.Process.Pid
		LogInfo("停止进程 PID=%d", pid)
		_ = killProcessGroup(pid)
		removePidFileIfMatches(pid)
		process = nil
	}

	// 2. 清理 PID 文件中的进程
	if data, err := os.ReadFile(pidFilePath()); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
			if syscall.Kill(pid, 0) == nil {
				LogInfo("通过 PID 文件发现进程 PID=%d", pid)
				killProcessTree(pid)
				removePidFileIfMatches(pid)
			} else {
				_ = os.Remove(pidFilePath())
			}
		}
	}

	// 3. 端口占用兜底
	cfg := GetConfig()
	port := cfg.ServerPort
	if port <= 0 {
		port = 3080
	}
	for _, pid := range findPidsOnPort(port) {
		LogInfo("端口 %d 被 PID=%d 占用，强制清理", port, pid)
		killProcessTree(pid)
	}

	time.Sleep(300 * time.Millisecond)

	_ = os.Remove(pidFilePath())
}

func KillHarness() {
	procMu.Lock()
	defer procMu.Unlock()
	killHarnessLocked()
}

func killProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	out, err := exec.Command("ps", "-o", "pgid=", "-p", strconv.Itoa(pid)).Output()
	if err == nil {
		pgidStr := strings.TrimSpace(string(out))
		if pgid, err := strconv.Atoi(pgidStr); err == nil && pgid > 0 {
			if killProcessGroup(pgid) {
				return
			}
		}
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

func findPidsOnPort(port int) []int {
	if port <= 0 {
		return nil
	}
	out, err := exec.Command("fuser", fmt.Sprintf("%d/tcp", port)).Output()
	if err != nil {
		return nil
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	var pids []int
	for _, p := range parts {
		if pid, err := strconv.Atoi(p); err == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids
}

func gitClone() error {
	_ = os.MkdirAll(filepath.Dir(srcDir), 0755)
	args := append(gitProxyArgs(), "clone", "--depth=1", repoURL, srcDir)
	cmd := exec.Command(gitBin, args...)
	cmd.Env = buildEnv()
	cmd.Stdout = &logWriter{}
	cmd.Stderr = &logWriter{}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	return nil
}

func gitPull() error {
	args := append(gitProxyArgs(), "-C", srcDir, "pull", "--ff-only")
	cmd := exec.Command(gitBin, args...)
	cmd.Env = buildEnv()
	cmd.Stdout = &logWriter{}
	cmd.Stderr = &logWriter{}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git pull: %w", err)
	}
	return nil
}

func gitHead() string {
	out, err := exec.Command(gitBin, "-C", srcDir, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func extractZip(zipPath, dst string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	_ = os.MkdirAll(dst, 0755)
	for _, f := range r.File {
		if f.Name == "" {
			continue
		}
		target := filepath.Join(dst, f.Name)
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0755)
			continue
		}
		_ = os.MkdirAll(filepath.Dir(target), 0755)
		if err := extractZipFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func extractZipFile(f *zip.File, dst string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

func installPnpm() error {
	if _, err := os.Stat(pnpmBin()); err == nil {
		return nil
	}
	pnpmDir := filepath.Join(pkgVarDir, "pnpm-env")
	_ = os.MkdirAll(pnpmDir, 0755)
	return runCmd(pnpmDir, npmBin(), "install", "pnpm")
}

func ensureGCC() error {
	if _, err := exec.LookPath("gcc"); err == nil {
		return nil
	}
	state.SetStatus(StatusBuilding, "缺少构建工具，正在安装 gcc...")
	LogInfo("未检测到 gcc，尝试自动安装")
	if err := installGCC(); err != nil {
		return fmt.Errorf("自动安装 gcc 失败: %w\n请手动安装 gcc/g++ 后重试", err)
	}
	if _, err := exec.LookPath("gcc"); err != nil {
		return fmt.Errorf("gcc 安装后仍未检测到，请手动安装")
	}
	LogInfo("gcc 安装完成")
	return nil
}

func installGCC() error {
	args := []string{"install", "-y", "build-essential"}
	LogInfo("apt-get install build-essential")
	cmd := exec.Command("apt-get", args...)
	cmd.Env = buildEnv()
	cmd.Stdout = &logWriter{}
	cmd.Stderr = &logWriter{}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apt-get install build-essential 失败: %w", err)
	}
	return nil
}

func landlockNativeDir() string {
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	}
	return filepath.Join(srcDir, "native", "landlock-run", "packages", "linux-"+arch)
}

func ensureMusl() error {
	if _, err := exec.LookPath("musl-gcc"); err == nil {
		return nil
	}
	state.SetStatus(StatusBuilding, "缺少 musl 工具链，正在安装 musl-tools...")
	LogInfo("未检测到 musl-gcc，尝试自动安装")
	cmd := exec.Command("apt-get", "install", "-y", "musl-tools")
	cmd.Env = buildEnv()
	cmd.Stdout = &logWriter{}
	cmd.Stderr = &logWriter{}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apt-get install musl-tools 失败: %w", err)
	}
	if _, err := exec.LookPath("musl-gcc"); err != nil {
		return fmt.Errorf("musl-tools 安装后仍未检测到 musl-gcc，请手动安装")
	}
	LogInfo("musl-tools 安装完成")
	return nil
}

func buildLandlock() error {
	bin := filepath.Join(landlockNativeDir(), "bin", "landlock-run")
	if _, err := os.Stat(bin); err == nil {
		LogInfo("landlock-run 已存在，跳过构建: %s", bin)
		return nil
	}
	state.SetStatus(StatusBuilding, "正在构建 landlock 沙箱组件...")
	LogInfo("未检测到 landlock-run，开始构建原生组件")
	if err := ensureMusl(); err != nil {
		return err
	}
	if err := runCmd(srcDir, pnpmBin(), "--filter", "@deepseek-ai/node-addon-landlock-run-workspace", "run", "build:native"); err != nil {
		return fmt.Errorf("landlock build:native: %w", err)
	}
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("landlock 构建完成但产物缺失: %s", bin)
	}
	LogInfo("landlock-run 构建完成: %s", bin)
	return nil
}

func buildSource() error {
	state.SetStatus(StatusBuilding, "正在安装 pnpm...")
	if err := installPnpm(); err != nil {
		return fmt.Errorf("install pnpm: %w", err)
	}
	if err := ensureGCC(); err != nil {
		return err
	}
	state.SetStatus(StatusBuilding, "正在安装依赖...")
	if err := runCmd(srcDir, pnpmBin(), "install", "--frozen-lockfile", "--registry", "https://registry.npmmirror.com"); err != nil {
		return fmt.Errorf("pnpm install: %w", err)
	}
	state.SetStatus(StatusBuilding, "正在编译构建...")
	if err := runCmd(srcDir, pnpmBin(), "run", "build"); err != nil {
		return fmt.Errorf("pnpm run build: %w", err)
	}
	if err := buildLandlock(); err != nil {
		return err
	}
	SetBuildTime(time.Now())
	return nil
}

func runCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = buildEnv()
	cmd.Stdout = &logWriter{}
	cmd.Stderr = &logWriter{}
	return cmd.Run()
}

func buildEnv() []string {
	env := os.Environ()
	path := nodeBinDir
	env = appendOrReplace(env, "PATH", path+":/bin:/usr/bin:"+os.Getenv("PATH"))
	env = appendOrReplace(env, "HOME", filepath.Join(pkgVarDir, "home"))
	env = appendOrReplace(env, "npm_config_cache", filepath.Join(pkgVarDir, "npm-cache"))
	env = appendOrReplace(env, "npm_config_registry", "https://registry.npmmirror.com")
	env = appendOrReplace(env, "npm_config_nodedir", "/var/apps/nodejs_v24/target")
	env = appendOrReplace(env, "PNPM_HOME", filepath.Join(pkgVarDir, "pnpm-home"))
	env = appendOrReplace(env, "DSH_HOME", filepath.Join(pkgVarDir, "dsh-data"))
	env = appendOrReplace(env, "DSH_AGENTS_HOME", filepath.Join(pkgVarDir, "dsh-data", "agents"))

	cfg := GetConfig()
	if cfg.NetworkProxy != "" {
		noProxy := "localhost,127.0.0.1,::1,registry.npmmirror.com,npmmirror.com"
		env = appendOrReplace(env, "npm_config_proxy", cfg.NetworkProxy)
		env = appendOrReplace(env, "npm_config_https_proxy", cfg.NetworkProxy)
		env = appendOrReplace(env, "npm_config_noproxy", noProxy)
		env = appendOrReplace(env, "HTTP_PROXY", cfg.NetworkProxy)
		env = appendOrReplace(env, "HTTPS_PROXY", cfg.NetworkProxy)
		env = appendOrReplace(env, "ALL_PROXY", cfg.NetworkProxy)
		env = appendOrReplace(env, "NO_PROXY", noProxy)
		env = appendOrReplace(env, "no_proxy", noProxy)
	}
	return env
}

func gitProxyArgs() []string {
	cfg := GetConfig()
	if cfg.NetworkProxy == "" {
		return nil
	}
	return []string{
		"-c", "http.proxy=" + cfg.NetworkProxy,
		"-c", "https.proxy=" + cfg.NetworkProxy,
	}
}

func appendOrReplace(env []string, key, val string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + val
			return env
		}
	}
	return append(env, prefix+val)
}

func refreshCommit() {
	out, err := exec.Command(gitBin, "-C", srcDir, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		SetCommit("-")
	} else {
		SetCommit(strings.TrimSpace(string(out)))
	}
	SetVersion(readVersion())
}

func readVersion() string {
	data, err := os.ReadFile(filepath.Join(srcDir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return ""
	}
	return pkg.Version
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d小时%d分%d秒", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%d分%d秒", m, s)
	}
	return fmt.Sprintf("%d秒", s)
}

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcessGroup(pgid int) bool {
	if pgid <= 0 {
		return true
	}
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err == syscall.ESRCH {
		return true
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(-pgid, 0); err == syscall.ESRCH {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
	time.Sleep(200 * time.Millisecond)
	return syscall.Kill(-pgid, 0) == syscall.ESRCH
}

func pidFilePath() string {
	return filepath.Join(pkgVarDir, "harness.pid")
}

func removePidFileIfMatches(pid int) {
	if data, err := os.ReadFile(pidFilePath()); err == nil {
		if filePid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && filePid == pid {
			_ = os.Remove(pidFilePath())
		}
	}
}

type logWriter struct{}

func (w *logWriter) Write(p []byte) (int, error) {
	AppendToLog(p)
	return len(p), nil
}