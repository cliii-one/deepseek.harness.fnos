package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Config struct {
	ServerPort      int    `json:"server_port"`
	ProxyPort       int    `json:"proxy_port"`
	NetworkProxy    string `json:"network_proxy"`
	AccessPassword  string `json:"access_password,omitempty"`
	DataLibraryPath string `json:"data_library_path,omitempty"`
	Version         string `json:"version,omitempty"`
	Commit          string `json:"commit,omitempty"`
	BuildTime       string `json:"build_time,omitempty"`
	LastRunState    string `json:"last_run_state,omitempty"`
}

var (
	globalConfig   Config
	configMu       sync.RWMutex
	configFilePath string
)

func InitConfig(pkgVar string) {
	configFilePath = filepath.Join(pkgVar, "config.json")
	// 访问方式固定为飞牛统一网关（fngateway），前端与代理层无需读取 access_mode
	globalConfig = Config{ServerPort: 2298, ProxyPort: 2299, DataLibraryPath: pkgVar}
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &globalConfig)
	globalConfig.DataLibraryPath = pkgVar
}

func GetConfig() Config {
	configMu.RLock()
	defer configMu.RUnlock()
	return globalConfig
}

func GetBuildTime() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return globalConfig.BuildTime
}

func GetVersion() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return globalConfig.Version
}

func GetCommit() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return globalConfig.Commit
}

func GetLastRunState() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return globalConfig.LastRunState
}

// persistConfig 将内存配置序列化写入 config.json（临时文件原子写入）
func persistConfig() {
	data, err := json.MarshalIndent(globalConfig, "", "  ")
	if err != nil {
		return
	}
	tmpFile := configFilePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return
	}
	_ = os.Rename(tmpFile, configFilePath)
}

// SetBuildTime 记录构建完成时刻并持久化
func SetBuildTime(t time.Time) {
	configMu.Lock()
	globalConfig.BuildTime = t.Format("2006-01-02 15:04")
	configMu.Unlock()
	persistConfig()
}

// SetVersion 持久化版本号
func SetVersion(v string) {
	configMu.Lock()
	globalConfig.Version = v
	configMu.Unlock()
	persistConfig()
}

// SetCommit 持久化 commit
func SetCommit(c string) {
	configMu.Lock()
	globalConfig.Commit = c
	configMu.Unlock()
	persistConfig()
}

// SetLastRunState 持久化最近一次运行状态
func SetLastRunState(st string) {
	configMu.Lock()
	globalConfig.LastRunState = st
	configMu.Unlock()
	persistConfig()
}