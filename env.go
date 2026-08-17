package main

import (
	"os"
	"path/filepath"
)

// InitAppEnv 初始化全局环境变量，供所有子进程天然继承
func InitAppEnv(pkgVar string) {
	pnpmBinDir := filepath.Join(pkgVar, "pnpm-env", "node_modules", ".bin")
	_ = os.Setenv("PATH", pnpmBinDir+":"+nodeBinDir+":/bin:/usr/bin:"+os.Getenv("PATH"))
	_ = os.Setenv("HOME", filepath.Join(pkgVar, "home"))
	_ = os.Setenv("CI", "true")

	pnpmHome := filepath.Join(pkgVar, "pnpm-home")
	storeDir := filepath.Join(pnpmHome, "store")
	_ = os.Setenv("PNPM_HOME", pnpmHome)
	_ = os.Setenv("pnpm_config_store_dir", storeDir)
	_ = os.Setenv("npm_config_store_dir", storeDir)
	_ = os.Setenv("npm_config_cache", filepath.Join(pkgVar, "npm-cache"))
	_ = os.Setenv("npm_config_registry", "https://registry.npmmirror.com")

	dshHome := filepath.Join(pkgVar, "dsh-data")
	_ = os.Setenv("DSH_HOME", dshHome)
	_ = os.Setenv("DSH_AGENTS_HOME", filepath.Join(dshHome, "agents"))

	ApplyProxyEnv()
}

// ApplyProxyEnv 根据当前配置应用或清理网络代理环境变量
func ApplyProxyEnv() {
	cfg := GetConfig()
	if cfg.NetworkProxy != "" {
		noProxy := "localhost,127.0.0.1,::1,registry.npmmirror.com,npmmirror.com"
		_ = os.Setenv("npm_config_proxy", cfg.NetworkProxy)
		_ = os.Setenv("npm_config_https_proxy", cfg.NetworkProxy)
		_ = os.Setenv("npm_config_noproxy", noProxy)
		_ = os.Setenv("HTTP_PROXY", cfg.NetworkProxy)
		_ = os.Setenv("HTTPS_PROXY", cfg.NetworkProxy)
		_ = os.Setenv("ALL_PROXY", cfg.NetworkProxy)
		_ = os.Setenv("NO_PROXY", noProxy)
		_ = os.Setenv("no_proxy", noProxy)
	} else {
		_ = os.Unsetenv("npm_config_proxy")
		_ = os.Unsetenv("npm_config_https_proxy")
		_ = os.Unsetenv("npm_config_noproxy")
		_ = os.Unsetenv("HTTP_PROXY")
		_ = os.Unsetenv("HTTPS_PROXY")
		_ = os.Unsetenv("ALL_PROXY")
		_ = os.Unsetenv("NO_PROXY")
		_ = os.Unsetenv("no_proxy")
	}
}
