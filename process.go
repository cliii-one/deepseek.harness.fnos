package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type managedProcess struct {
	cmd           *exec.Cmd
	stopRequested bool
	done          chan struct{}
}

var (
	procMu  sync.Mutex
	process *managedProcess
)

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

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

func killProcess(pid int) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

func killProcessGroup(pgid int) bool {
	if pgid <= 0 {
		return true
	}
	// 向负数 PGID 发送信号，作用于整个进程组中所有遗留进程
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err == syscall.ESRCH {
		return true
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(-pgid, 0); err == syscall.ESRCH {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
	time.Sleep(100 * time.Millisecond)
	return syscall.Kill(-pgid, 0) == syscall.ESRCH
}

func killProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	// 查询该进程所属的 PGID 尝试整组清理
	out, err := exec.Command("ps", "-o", "pgid=", "-p", strconv.Itoa(pid)).Output()
	if err == nil {
		pgidStr := strings.TrimSpace(string(out))
		if pgid, err := strconv.Atoi(pgidStr); err == nil && pgid > 0 {
			if killProcessGroup(pgid) {
				return
			}
		}
	}
	killProcess(pid)
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

// waitForPortFree 循环检测端口是否彻底释放（最多等待 500ms）
func waitForPortFree(port int) {
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(findPidsOnPort(port)) == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// killHarnessLocked 彻底无死角强杀：无论主 PID 是否存活，对记录的进程组、端口占用进程进行深度连根清理
func killHarnessLocked() {
	// 1. 优先清理内存记录的进程句柄
	if process != nil && process.cmd != nil && process.cmd.Process != nil {
		pid := process.cmd.Process.Pid
		LogInfo("清理运行进程组 (PGID=%d)", pid)
		_ = killProcessGroup(pid)
		removePidFileIfMatches(pid)
		process = nil
	}

	// 2. 清理 PID 文件记录的进程（即使主 PID 已不存在，也向 -pid 强发信号清理孤儿进程群）
	if data, err := os.ReadFile(pidFilePath()); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
			LogInfo("清理残留进程组 (PGID=%d)", pid)
			_ = killProcessGroup(pid)
			killProcessTree(pid)
			removePidFileIfMatches(pid)
		}
	}

	// 3. 端口占用深度排查兜底（清理占用该端口的所有残留进程）
	cfg := GetConfig()
	port := cfg.ServerPort
	if port <= 0 {
		port = 2298
	}
	for _, pid := range findPidsOnPort(port) {
		LogInfo("端口 %d 被残留进程 (PID=%d) 占用，执行强制终止", port, pid)
		killProcessTree(pid)
		_ = killProcessGroup(pid)
	}

	// 4. 等待确认端口真正释放
	waitForPortFree(port)

	_ = os.Remove(pidFilePath())
}

func KillHarness() {
	procMu.Lock()
	defer procMu.Unlock()
	killHarnessLocked()
}

// StartWatchdog 启动后台健康巡检与主动自愈协程
func StartWatchdog() {
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			inspectAndHeal()
		}
	}()
}

// inspectAndHeal 主动巡检与自愈维护
func inspectAndHeal() {
	curStatus := state.Status()

	if curStatus == StatusRunning {
		procMu.Lock()
		mp := process
		var pid int
		if mp != nil && mp.cmd != nil && mp.cmd.Process != nil {
			pid = mp.cmd.Process.Pid
		}
		procMu.Unlock()

		// 运行状态下探活：如果内存无句柄或主 PID 已死，检查端口
		if pid <= 0 || !isProcessAlive(pid) {
			cfg := GetConfig()
			port := cfg.ServerPort
			if port <= 0 {
				port = 2298
			}
			pids := findPidsOnPort(port)
			if len(pids) == 0 {
				LogWarning("巡检发现服务进程异常终止，执行状态自愈纠偏")
				procMu.Lock()
				if process == mp {
					process = nil
					if pid > 0 {
						removePidFileIfMatches(pid)
					}
					stopReverseProxy()
					state.SetStatus(StatusStopped, "巡检发现进程已异常终止")
				}
				procMu.Unlock()
			}
		}
		return
	}

	if curStatus == StatusStopped {
		// 停止状态下巡检：清理失效的 PID 残留文件
		if data, err := os.ReadFile(pidFilePath()); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 {
				if !isProcessAlive(pid) {
					_ = os.Remove(pidFilePath())
				}
			}
		}
	}
}
