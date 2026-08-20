package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	repoURL    = "https://github.com/deepseek-ai/deepseek-harness"
	nodeBinDir = "/var/apps/nodejs_v24/target/bin"
	gitBin     = "/usr/bin/git"
)

// semverTagRe 语义化版本 tag 形态：主版本.次版本[.修订][-预发布][+构建元数据]
var semverTagRe = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*([-+][0-9A-Za-z.-]+)?$`)

func nodeBin() string { return filepath.Join(nodeBinDir, "node") }
func npmBin() string  { return filepath.Join(nodeBinDir, "npm") }
func pnpmBin() string { return filepath.Join(pkgVarDir, "pnpm-env", "node_modules", ".bin", "pnpm") }

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
	// origCommit 记录升级前的 git 版本，构建失败时用于回滚（forceRebuild 时为空）
	origCommit := ""

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		if forceRebuild {
			state.SetStatus(StatusStopped, "源码不存在，请先拉取更新")
			return
		}
		state.SetStatus(StatusBuilding, "正在检查远程更新...")
		if err := gitClone(); err != nil {
			LogWarning("Git 克隆失败: %s", err)
			state.SetStatus(StatusStopped, "克隆失败: "+err.Error())
			return
		}
		if err := buildSource(false); err != nil {
			LogWarning("源码构建失败: %s", err)
			state.SetStatus(StatusStopped, "构建失败: "+err.Error())
			return
		}
		refreshCommit()
		restartService()
		return
	}

	if forceRebuild {
		state.SetStatus(StatusBuilding, "正在强制重建（重新安装依赖并全量编译）...")
	} else {
		commitBefore := gitHead()
		origCommit = commitBefore
		state.SetStatus(StatusBuilding, "正在拉取远程更新...")
		if err := gitPull(); err != nil {
			LogWarning("Git 拉取失败: %s", err)
			state.SetStatus(StatusStopped, "git pull 失败: "+err.Error())
			return
		}
		commitAfter := gitHead()
		if commitBefore != "" && commitBefore == commitAfter {
			LogInfo("当前已是最新版本 (%s)，跳过构建", commitAfter)
			state.SetStatus(StatusBuilding, fmt.Sprintf("当前已是最新版本 (%s)，跳过构建", commitAfter))
			refreshCommit()
			restartService()
			return
		}
		shortTarget := commitAfter
		if len(shortTarget) > 7 {
			shortTarget = shortTarget[:7]
		}
		state.SetTargetCommit(shortTarget)
		LogInfo("检测到版本变更 (%s → %s)，开始构建", commitBefore, commitAfter)
		state.SetStatus(StatusBuilding, fmt.Sprintf("检测到版本变更 (%s → %s)，正在同步依赖与构建...", commitBefore, commitAfter))
	}

	// 构建前备份当前可运行产物；构建失败时回滚 git 并恢复产物，避免"新源码+残缺产物"导致服务无法启动
	snapshotDir, snapErr := snapshotArtifacts()
	if snapErr != nil {
		LogWarning("构建前产物快照失败（继续构建）: %s", snapErr)
	}

	if err := buildSource(false); err != nil {
		msg := fmt.Sprintf("构建失败: %s", err)
		rollbackOk := false
		if origCommit == "" {
			msg += "；无升级前版本可回滚"
		} else if rerr := gitResetTo(origCommit); rerr != nil {
			msg += fmt.Sprintf("；git 回滚失败: %s", rerr)
		} else {
			rollbackOk = true
			short := origCommit
			if len(short) > 7 {
				short = short[:7]
			}
			msg += fmt.Sprintf("；已自动回滚源码到原版本 %s", short)
		}
		if snapshotDir != "" {
			if rerr := restoreArtifacts(snapshotDir); rerr != nil {
				LogWarning("产物恢复失败: %s", rerr)
				if rollbackOk {
					msg += "；产物恢复失败，建议重新构建"
				}
			} else {
				msg += "；构建失败前的可运行产物已恢复"
			}
		} else {
			LogWarning("无产物快照可恢复")
		}
		LogWarning(msg)
		state.SetStatus(StatusStopped, msg)
		return
	}
	if snapshotDir != "" {
		discardSnapshot(snapshotDir)
	}

	refreshCommit()
	restartService()
}

// gitResetTo 将 srcDir 的 git 状态硬重置到指定 commit
func gitResetTo(commit string) error {
	cmd := gitCmd("-C", srcDir, "reset", "--hard", commit)
	cmd.Stdout = NewLogWriterInfo()
	cmd.Stderr = NewLogWriterWarn()
	return cmd.Run()
}

// snapshotArtifacts 备份当前可运行的基础产物（web 前端、核心 lib、landlock 原生组件），
// 返回快照目录；无任何可备份产物时返回错误（调用方忽略并继续）。
func snapshotArtifacts() (string, error) {
	dir, err := os.MkdirTemp(pkgVarDir, "build-snap-")
	if err != nil {
		return "", fmt.Errorf("创建快照目录失败: %w", err)
	}
	snap := filepath.Join(dir, "artifacts.tar.gz")
	var exist []string
	for _, p := range []string{"apps/web/dist", "packages/api/remotes/lib", "native/landlock-run/packages"} {
		if _, err := os.Stat(filepath.Join(srcDir, p)); err == nil {
			exist = append(exist, p)
		}
	}
	if len(exist) == 0 {
		os.RemoveAll(dir)
		return "", fmt.Errorf("无已有产物可备份")
	}
	args := append([]string{"-czf", snap, "-C", srcDir}, exist...)
	cmd := exec.Command("tar", args...)
	cmd.Stdout = NewLogWriterInfo()
	cmd.Stderr = NewLogWriterWarn()
	if err := cmd.Run(); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("产物备份失败: %w", err)
	}
	LogInfo("已备份构建前基础产物快照: %s", snap)
	return dir, nil
}

// restoreArtifacts 从快照恢复产物（覆盖被半成品污染的目录）
func restoreArtifacts(snapshotDir string) error {
	snap := filepath.Join(snapshotDir, "artifacts.tar.gz")
	if _, err := os.Stat(snap); err != nil {
		return fmt.Errorf("快照文件不存在: %s", snap)
	}
	cmd := exec.Command("tar", "-xzf", snap, "-C", srcDir)
	cmd.Stdout = NewLogWriterInfo()
	cmd.Stderr = NewLogWriterWarn()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("产物恢复失败: %w", err)
	}
	LogInfo("已恢复构建前的可运行产物")
	return nil
}

// discardSnapshot 删除快照目录
func discardSnapshot(snapshotDir string) {
	_ = os.RemoveAll(snapshotDir)
	LogInfo("构建成功，已清理构建前产物快照")
}

func gitClone() error {
	_ = os.MkdirAll(filepath.Dir(srcDir), 0755)
	cmd := gitCmd("clone", "--depth=1", repoURL, srcDir)
	cmd.Stdout = NewLogWriterInfo()
	cmd.Stderr = NewLogWriterWarn()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	return nil
}

func gitPull() error {
	// 显式 fetch 远程默认分支：避免 fetch 所有分支导致 FETCH_HEAD 多行时 reset 目标不确定
	branch := gitRemoteDefaultBranch()
	fetchCmd := gitCmd("-C", srcDir, "fetch", "--depth=1", "origin", branch)
	fetchCmd.Stdout = NewLogWriterInfo()
	fetchCmd.Stderr = NewLogWriterWarn()
	if err := fetchCmd.Run(); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}

	resetCmd := gitCmd("-C", srcDir, "reset", "--hard", "FETCH_HEAD")
	resetCmd.Stdout = NewLogWriterInfo()
	resetCmd.Stderr = NewLogWriterWarn()
	if err := resetCmd.Run(); err != nil {
		return fmt.Errorf("git reset: %w", err)
	}
	return nil
}

func gitHead() string {
	cmd := gitCmd("-C", srcDir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitRemoteHead 查询远程默认分支（HEAD 符号引用）最新提交 hash（只读，不修改本地仓库、不拉取内容）。
// 使用 HEAD 而非固定分支名：上游将默认分支从 main 改名为 master 后旧实现查不到 refs/heads/main 而误报无更新，
// HEAD 符号引用始终跟随默认分支，分支改名后依然正确。
func gitRemoteHead() string {
	cmd := gitCmd("ls-remote", repoURL, "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// gitRemoteDefaultBranch 解析远程默认分支名（如 master），用于更新时显式 fetch 该分支。
// 解析失败时回退 "master"。
func gitRemoteDefaultBranch() string {
	cmd := gitCmd("ls-remote", "--symref", repoURL, "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "master"
	}
	// 输出形如:
	//   ref: refs/heads/master	HEAD
	//   <hash>	HEAD
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "ref:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			name := strings.TrimPrefix(fields[1], "refs/heads/")
			if name != "" && name != "HEAD" {
				return name
			}
		}
	}
	return "master"
}

// gitLatestRemoteVersion 查询远程版本 tag（dsh-v* / v*）中的最高语义化版本。
// 当上游只发布 tag、默认分支尚未合并时，仍能发现新版本。
func gitLatestRemoteVersion() string {
	cmd := gitCmd("ls-remote", "--tags", repoURL)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	var versions []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ref := fields[1]
		// 跳过 peel 到对象 ID 的附注 tag 行（形如 refs/tags/xxx^{}），避免重复
		if strings.HasSuffix(ref, "^{}") {
			continue
		}
		name := strings.TrimPrefix(ref, "refs/tags/")
		var ver string
		switch {
		case strings.HasPrefix(name, "dsh-v"):
			ver = strings.TrimPrefix(name, "dsh-v")
		case strings.HasPrefix(name, "v"):
			ver = strings.TrimPrefix(name, "v")
		default:
			continue
		}
		// 只保留语义化版本形态（如 0.1.0-rc.8），过滤其他 tag 命名
		if ver == "" || !semverTagRe.MatchString(ver) {
			continue
		}
		versions = append(versions, ver)
	}
	if len(versions) == 0 {
		return ""
	}
	sort.Slice(versions, func(i, j int) bool {
		return compareSemver(versions[i], versions[j]) > 0
	})
	return versions[0]
}

func extractTarGz(tarPath, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	cmd := exec.Command("tar", "--no-same-owner", "-xzf", tarPath, "-C", dst)
	cmd.Stdout = NewLogWriterInfo()
	cmd.Stderr = NewLogWriterWarn()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("解压 tar.gz 失败: %w", err)
	}
	return nil
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
	if err := installGCC(); err != nil {
		return fmt.Errorf("自动安装 gcc 失败: %w\n请手动安装 gcc/g++ 后重试", err)
	}
	if _, err := exec.LookPath("gcc"); err != nil {
		return fmt.Errorf("gcc 安装后仍未检测到，请手动安装")
	}
	LogInfo("gcc 工具链安装完成")
	return nil
}

func installGCC() error {
	args := []string{"install", "-y", "build-essential"}
	LogInfo("缺失 gcc 工具链，正在执行 apt-get install build-essential")
	cmd := exec.Command("apt-get", args...)
	cmd.Stdout = NewLogWriterInfo()
	cmd.Stderr = NewLogWriterWarn()
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
	LogInfo("缺失 musl 工具链，正在执行 apt-get install musl-tools")
	cmd := exec.Command("apt-get", "install", "-y", "musl-tools")
	cmd.Stdout = NewLogWriterInfo()
	cmd.Stderr = NewLogWriterWarn()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apt-get install musl-tools 失败: %w", err)
	}
	if _, err := exec.LookPath("musl-gcc"); err != nil {
		return fmt.Errorf("musl-tools 安装后仍未检测到 musl-gcc，请手动安装")
	}
	LogInfo("musl-tools 工具链安装完成")
	return nil
}

func landlockBinPath() string {
	return filepath.Join(landlockNativeDir(), "bin", "landlock-run")
}

func buildLandlock() error {
	state.SetStatus(StatusBuilding, "正在构建 landlock 沙箱组件...")
	LogInfo("开始编译 landlock 原生沙箱组件")
	if err := ensureMusl(); err != nil {
		return err
	}
	if err := runCmd(srcDir, pnpmBin(), "--filter", "@deepseek-ai/node-addon-landlock-run-workspace", "run", "build:native"); err != nil {
		return fmt.Errorf("landlock build:native: %w", err)
	}
	bin := landlockBinPath()
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("landlock 构建完成但产物缺失: %s", bin)
	}
	LogInfo("landlock 原生沙箱组件构建完成: %s", bin)
	return nil
}

func hasPrebuiltArtifacts() bool {
	webIndex := filepath.Join(srcDir, "apps", "web", "dist", "index.html")
	if _, err := os.Stat(webIndex); err != nil {
		return false
	}
	coreLib := filepath.Join(srcDir, "packages", "api", "remotes", "lib")
	if _, err := os.Stat(coreLib); err != nil {
		return false
	}
	if _, err := os.Stat(landlockBinPath()); err != nil {
		return false
	}
	return true
}

func hasNodeModules() bool {
	info, err := os.Stat(filepath.Join(srcDir, "node_modules"))
	return err == nil && info.IsDir()
}

func buildSource(allowFastStart bool) error {
	state.SetStatus(StatusBuilding, "正在安装 pnpm...")
	if err := installPnpm(); err != nil {
		return fmt.Errorf("install pnpm: %w", err)
	}

	prebuilt := hasPrebuiltArtifacts()
	hasModules := hasNodeModules()
	isPrebuilt := isPrebuiltPkg()

	// 预构建包极速启动：产物完备直接跳过编译
	if allowFastStart && isPrebuilt && hasModules && prebuilt {
		state.SetStatus(StatusBuilding, "检测到预构建产物，正在极速启动...")
		LogInfo("检测到内置离线预构建源码包（产物与依赖完备），跳过依赖拉取与项目编译，极速启动")
	} else {
		if allowFastStart && (!isPrebuilt || !prebuilt || !hasModules) {
			LogInfo("检测到内置离线源码包，开始自动配置编译环境并安装依赖")
		}
		// 内存预检：避免低内存 NAS 上构建导致 OOM 系统失联
		availMB := buildMemoryMB()
		if availMB >= 0 && availMB < minBuildMemoryMB {
			return fmt.Errorf("当前可用内存不足（可用 ≈ %d MB，需 ≥ %d MB），已取消构建以防系统失联。\n请先停止其他占用内存的服务或增大 Swap 后重试", availMB, minBuildMemoryMB)
		}
		LogInfo("构建前内存检查: 可用内存+Swap ≈ %d MB (阈值 %d MB)", availMB, minBuildMemoryMB)
		// 按可用内存给 v8 堆上限：上限过低会让 tsc/tsdown 在低堆提前 OOM 自爆（实测 1536MB 不够 monorepo 构建）
		nodeOpts := fmt.Sprintf("NODE_OPTIONS=--max-old-space-size=%d", buildNodeHeapMB(availMB))
		if err := ensureGCC(); err != nil {
			return err
		}
		state.SetStatus(StatusBuilding, "正在安装依赖...")
		// 限制 pnpm 子进程并发与网络并发，降低低内存设备的内存峰值
		if err := runCmdEnv(srcDir, pnpmBin(), []string{nodeOpts},
			"install", "--prefer-offline", "--config.confirm-modules-purge=false", "--registry", "https://registry.npmmirror.com",
			"--child-concurrency=2", "--network-concurrency=4"); err != nil {
			return fmt.Errorf("pnpm install: %w", err)
		}
		state.SetStatus(StatusBuilding, "正在编译构建...")
		// 注入官方品牌环境变量：rc.8 起仅 official profile 下 ui-brand-official 启用品牌 UI
		buildEnv := append([]string{nodeOpts}, dshBrandEnv()...)
		if err := runCmdEnv(srcDir, pnpmBin(), buildEnv, "run", "build"); err != nil {
			return fmt.Errorf("pnpm run build: %w", err)
		}
		LogInfo("项目源码编译完成")

		if err := buildLandlock(); err != nil {
			return err
		}
	}

	SetBuildTime(time.Now())
	return nil
}

// minBuildMemoryMB 触发构建所需的最低可用内存（含 Swap），低于该值拒绝构建防止 OOM 失联。
// 3.6GB 内存的 NAS 上完整 monorepo 构建峰值可达 2GB+，阈值取 2GB 保守拦截。
const minBuildMemoryMB = 2048

// buildMemoryMB 读取 /proc/meminfo，返回 MemAvailable+SwapFree 可用内存（MB）；读取失败时返回 -1 表示放行
func buildMemoryMB() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return -1
	}
	parse := func(key string) int64 {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, key) {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if v, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
						return v // kB
					}
				}
			}
		}
		return 0
	}
	availKB := parse("MemAvailable:") + parse("SwapFree:")
	if availKB <= 0 {
		return -1
	}
	return availKB / 1024
}

// buildNodeHeapMB 按当前可用内存给 v8 堆上限：内存越充足给得越高。
// 上限过低会导致 tsc/tsdown 在低堆时提前 OOM 自爆（实测 1536MB 不够 monorepo 构建），
// 上限过高又可能在物理内存不足时拖垮系统，因此按可用内存动态取值。
func buildNodeHeapMB(availMB int64) int64 {
	switch {
	case availMB >= 3584:
		return 2560
	case availMB >= 3072:
		return 2352
	case availMB >= 2560:
		return 2048
	case availMB >= 2304:
		return 1792
	default:
		return 1536
	}
}

func runCmd(dir, name string, args ...string) error {
	return runCmdEnv(dir, name, nil, args...)
}

func runCmdEnv(dir, name string, env []string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = NewLogWriterInfo()
	cmd.Stderr = NewLogWriterWarn()
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	return cmd.Run()
}

func gitBaseArgs() []string {
	args := []string{"-c", "safe.directory=*"}
	cfg := GetConfig()
	if cfg.NetworkProxy != "" {
		args = append(args,
			"-c", "http.proxy="+cfg.NetworkProxy,
			"-c", "https.proxy="+cfg.NetworkProxy,
		)
	}
	return args
}

func gitCmd(extraArgs ...string) *exec.Cmd {
	args := append(gitBaseArgs(), extraArgs...)
	return exec.Command(gitBin, args...)
}

func refreshCommit() {
	cmd := gitCmd("-C", srcDir, "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	commit := ""
	if err == nil {
		commit = strings.TrimSpace(string(out))
	}
	if commit == "" {
		if data, err := os.ReadFile(filepath.Join(srcDir, ".commit")); err == nil {
			commit = strings.TrimSpace(string(data))
		} else if data, err := os.ReadFile(filepath.Join(appDest, ".commit")); err == nil {
			commit = strings.TrimSpace(string(data))
		}
	}
	if commit == "" {
		commit = "-"
	}
	SetCommit(commit)
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

func readAppDestVersion() string {
	data, err := os.ReadFile(filepath.Join(appDest, ".version"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func isPrebuiltPkg() bool {
	data, err := os.ReadFile(filepath.Join(appDest, ".pkg_type"))
	if err == nil && strings.TrimSpace(string(data)) == "prebuilt" {
		return true
	}
	return false
}

func pkgTypeName() string {
	if isPrebuiltPkg() {
		return "内置离线预构建源码包"
	}
	return "内置离线源码包"
}

// compareSemver 比较语义化版本，返回 1 (v1>v2), -1 (v1<v2), 0 (相等)
func compareSemver(v1, v2 string) int {
	v1 = strings.TrimPrefix(strings.TrimSpace(v1), "v")
	v2 = strings.TrimPrefix(strings.TrimSpace(v2), "v")
	if v1 == v2 {
		return 0
	}
	if v1 == "" {
		return -1
	}
	if v2 == "" {
		return 1
	}

	splitVer := func(v string) (core string, pre string) {
		if idx := strings.Index(v, "-"); idx != -1 {
			return v[:idx], v[idx+1:]
		}
		return v, ""
	}

	core1, pre1 := splitVer(v1)
	core2, pre2 := splitVer(v2)

	parseNums := func(s string) []int {
		parts := strings.Split(s, ".")
		nums := make([]int, 0, len(parts))
		for _, p := range parts {
			var n int
			for _, r := range p {
				if r >= '0' && r <= '9' {
					n = n*10 + int(r-'0')
				} else {
					break
				}
			}
			nums = append(nums, n)
		}
		return nums
	}

	nums1, nums2 := parseNums(core1), parseNums(core2)
	maxLen := len(nums1)
	if len(nums2) > maxLen {
		maxLen = len(nums2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(nums1) {
			n1 = nums1[i]
		}
		if i < len(nums2) {
			n2 = nums2[i]
		}
		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}

	if pre1 == "" && pre2 != "" {
		return 1
	}
	if pre1 != "" && pre2 == "" {
		return -1
	}
	if pre1 != "" && pre2 != "" {
		parts1 := strings.Split(pre1, ".")
		parts2 := strings.Split(pre2, ".")
		maxParts := len(parts1)
		if len(parts2) > maxParts {
			maxParts = len(parts2)
		}

		for i := 0; i < maxParts; i++ {
			if i >= len(parts1) {
				return -1
			}
			if i >= len(parts2) {
				return 1
			}

			p1, p2 := parts1[i], parts2[i]
			if p1 == p2 {
				continue
			}

			n1, err1 := strconv.Atoi(p1)
			n2, err2 := strconv.Atoi(p2)

			if err1 == nil && err2 == nil {
				if n1 > n2 {
					return 1
				}
				if n1 < n2 {
					return -1
				}
			} else if err1 == nil && err2 != nil {
				return -1
			} else if err1 != nil && err2 == nil {
				return 1
			} else {
				if p1 > p2 {
					return 1
				}
				if p1 < p2 {
					return -1
				}
			}
		}
	}
	return 0
}

