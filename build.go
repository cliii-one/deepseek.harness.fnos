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
	"strings"
	"time"
)

const (
	repoURL    = "https://github.com/deepseek-ai/deepseek-harness"
	nodeBinDir = "/var/apps/nodejs_v24/target/bin"
	gitBin     = "/usr/bin/git"
)

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
		if err := buildSource(); err != nil {
			LogWarning("源码构建失败: %s", err)
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
			LogWarning("Git 拉取失败: %s", err)
			state.SetStatus(StatusStopped, "git pull 失败: "+err.Error())
			return
		}
		commitAfter := gitHead()
		if commitBefore != "" && commitBefore == commitAfter {
			LogInfo("当前已是最新版本 (%s)，跳过构建", commitAfter)
			refreshCommit()
			restartService()
			return
		}
		LogInfo("检测到版本变更 (%s → %s)，开始构建", commitBefore, commitAfter)
	}

	if err := buildSource(); err != nil {
		LogWarning("源码构建失败: %s", err)
		state.SetStatus(StatusStopped, "构建失败: "+err.Error())
		return
	}

	refreshCommit()
	restartService()
}

func gitClone() error {
	_ = os.MkdirAll(filepath.Dir(srcDir), 0755)
	args := append(gitProxyArgs(), "clone", "--depth=1", repoURL, srcDir)
	cmd := exec.Command(gitBin, args...)
	cmd.Env = buildEnv()
	cmd.Stdout = NewLogWriterInfo()
	cmd.Stderr = NewLogWriterWarn()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	return nil
}

func gitPull() error {
	args := append(gitProxyArgs(), "-C", srcDir, "pull", "--ff-only")
	cmd := exec.Command(gitBin, args...)
	cmd.Env = buildEnv()
	cmd.Stdout = NewLogWriterInfo()
	cmd.Stderr = NewLogWriterWarn()
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
	cmd.Env = buildEnv()
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
	cmd.Env = buildEnv()
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

func buildLandlock() error {
	bin := filepath.Join(landlockNativeDir(), "bin", "landlock-run")
	if _, err := os.Stat(bin); err == nil {
		LogInfo("landlock 原生沙箱组件已就绪: %s", bin)
		return nil
	}
	state.SetStatus(StatusBuilding, "正在构建 landlock 沙箱组件...")
	LogInfo("未检测到 landlock-run，开始编译原生沙箱组件")
	if err := ensureMusl(); err != nil {
		return err
	}
	if err := runCmd(srcDir, pnpmBin(), "--filter", "@deepseek-ai/node-addon-landlock-run-workspace", "run", "build:native"); err != nil {
		return fmt.Errorf("landlock build:native: %w", err)
	}
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("landlock 构建完成但产物缺失: %s", bin)
	}
	LogInfo("landlock 原生沙箱组件构建完成: %s", bin)
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
	cmd.Stdout = NewLogWriterInfo()
	cmd.Stderr = NewLogWriterWarn()
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
