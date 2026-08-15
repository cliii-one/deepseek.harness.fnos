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
	ReverseProxyURL string `json:"reverse_proxy_url,omitempty"`
	AccessPassword  string `json:"access_password,omitempty"`
	DataLibraryPath string `json:"data_library_path,omitempty"`
	Version         string `json:"version,omitempty"`
	Commit          string `json:"commit,omitempty"`
	BuildTime       string `json:"build_time,omitempty"`
}

var (
	globalConfig   Config
	configMu       sync.RWMutex
	configFilePath string
)

func InitConfig(pkgVar string) {
	configFilePath = filepath.Join(pkgVar, "config.json")
	globalConfig = Config{ServerPort: 3080, ProxyPort: 2299, DataLibraryPath: pkgVar}
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

// persistConfig 将内存配置序列化写入 config.json
func persistConfig() {
	data, err := json.MarshalIndent(globalConfig, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(configFilePath, data, 0644)
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

func SaveConfig(cfg Config) error {
	configMu.Lock()
	globalConfig = cfg
	configMu.Unlock()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFilePath, data, 0644)
}