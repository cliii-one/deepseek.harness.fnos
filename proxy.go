package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/soheilhy/cmux"
)

const (
	authCookieName = "harness_auth"
	authLoginPath  = "/_harness_auth"
)

// proxyWithAuth 密码中间件
func proxyWithAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		pwd := GetConfig().AccessPassword
		if pwd == "" {
			next.ServeHTTP(w, r)
			return
		}

		// 处理登录表单提交
		if r.URL.Path == authLoginPath && r.Method == http.MethodPost {
			_ = r.ParseForm()
			if r.FormValue("password") == pwd {
				http.SetCookie(w, &http.Cookie{
					Name:     authCookieName,
					Value:    pwd,
					Path:     "/",
					HttpOnly: true,
					Secure:   true,
					SameSite: http.SameSiteNoneMode,
				})
				http.Redirect(w, r, "/", http.StatusFound)
			} else {
				serveLoginPage(w, true)
			}
			return
		}

		// 校验 cookie
		if c, err := r.Cookie(authCookieName); err != nil || c.Value != pwd {
			serveLoginPage(w, false)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func serveLoginPage(w http.ResponseWriter, wrongPwd bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if wrongPwd {
		w.WriteHeader(http.StatusUnauthorized)
	}
	errorHTML := ""
	if wrongPwd {
		errorHTML = `<p class="err">密码错误，请重试</p>`
	}
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>DeepSeek Harness · 访问验证</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{min-height:100vh;display:flex;align-items:center;justify-content:center;
  background:linear-gradient(135deg,#0f172a 0%%,#1e293b 100%%);font-family:system-ui,sans-serif}
.card{background:#fff;border-radius:20px;padding:40px 36px;width:340px;
  box-shadow:0 25px 60px rgba(0,0,0,.4)}
.logo{width:44px;height:44px;background:#2563eb;border-radius:12px;
  display:flex;align-items:center;justify-content:center;margin:0 auto 20px}
.logo svg{color:#fff}
h1{text-align:center;font-size:18px;font-weight:700;color:#0f172a;margin-bottom:4px}
.sub{text-align:center;font-size:13px;color:#94a3b8;margin-bottom:24px}
input{width:100%%;padding:10px 14px;border:1.5px solid #e2e8f0;border-radius:10px;
  font-size:14px;outline:none;transition:border .15s;background:#f8fafc;color:#0f172a}
input:focus{border-color:#2563eb;background:#fff}
btn,button{width:100%%;margin-top:14px;padding:11px;background:#2563eb;
  color:#fff;border:none;border-radius:10px;font-size:14px;font-weight:600;
  cursor:pointer;transition:background .15s}
button:hover{background:#1d4ed8}
.err{margin-top:12px;text-align:center;font-size:13px;color:#ef4444;
  background:#fef2f2;padding:8px;border-radius:8px}
</style>
</head>
<body>
<div class="card">
  <div class="logo">
    <svg width="22" height="22" fill="none" stroke="currentColor" stroke-width="2"
         viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round"
         d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>
  </div>
  <h1>访问验证</h1>
  <p class="sub">DeepSeek Harness</p>
  <form method="POST" action="%s">
    <input type="password" name="password" placeholder="请输入访问密码" autofocus autocomplete="current-password">
    <button type="submit">进入</button>
  </form>
  %s
</div>
</body>
</html>`, authLoginPath, errorHTML)
}

var (
	proxyMu     sync.Mutex
	proxyHTTP   *http.Server
	proxyHTTPS  *http.Server
	proxyCmux   cmux.CMux
	proxyTarget *url.URL
	proxyAddr   string
	proxyTLS    *tls.Config
)

func updateReverseProxyTarget() {
	cfg := GetConfig()

	port := cfg.ServerPort
	if port <= 0 {
		port = 3080
	}
	proxyTarget, _ = url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))

	proxyPort := cfg.ProxyPort
	if proxyPort <= 0 {
		proxyPort = 2299
	}
	proxyAddr = fmt.Sprintf("0.0.0.0:%d", proxyPort)
}

func listAllIPs() []net.IP {
	var ips []net.IP
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil {
				continue
			}
			if ip.To4() != nil || ip.To16() != nil {
				ips = append(ips, ip)
			}
		}
	}
	ips = append(ips, net.ParseIP("127.0.0.1"), net.ParseIP("::1"))
	return ips
}

func listAllDNSNames() []string {
	names := []string{"localhost"}
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		names = append(names, hostname)
	}
	return names
}

func generateSelfSignedCert(certPath, keyPath string) error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("生成 ECDSA 密钥失败: %s", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("生成序列号失败: %s", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"DeepSeek Harness"},
			CommonName:   "deepseek-harness",
		},
		NotBefore:             time.Now().Add(-1 * time.Minute),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              listAllDNSNames(),
		IPAddresses:           listAllIPs(),
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("创建证书失败: %s", err)
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("写入证书文件失败: %s", err)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return fmt.Errorf("PEM 编码证书失败: %s", err)
	}

	keyOut, err := os.Create(keyPath)
	if err != nil {
		return fmt.Errorf("写入密钥文件失败: %s", err)
	}
	defer keyOut.Close()
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("序列化私钥失败: %s", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}); err != nil {
		return fmt.Errorf("PEM 编码私钥失败: %s", err)
	}

	LogInfo("自签名证书已生成: %s (共 %d 个 IP, %d 个域名)", certPath, len(template.IPAddresses), len(template.DNSNames))
	return nil
}

func loadOrCreateProxyTLS() (*tls.Config, error) {
	autoDir := globalPkgVar
	if autoDir == "" {
		autoDir = "."
	}
	certFile := filepath.Join(autoDir, "harness.crt")
	keyFile := filepath.Join(autoDir, "harness.key")

	needRegen := false
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		needRegen = true
	} else if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		needRegen = true
	}

	if needRegen {
		LogInfo("生成自签名证书...")
		if err := generateSelfSignedCert(certFile, keyFile); err != nil {
			return nil, fmt.Errorf("生成自签名证书失败: %s", err)
		}
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("加载 TLS 证书失败: %s", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func startReverseProxy() error {
	proxyMu.Lock()
	defer proxyMu.Unlock()
	return startReverseProxyLocked()
}

func startReverseProxyLocked() error {
	if proxyHTTP != nil || proxyHTTPS != nil {
		return nil
	}

	updateReverseProxyTarget()

	tlsCfg, err := loadOrCreateProxyTLS()
	if err != nil {
		LogWarning("TLS 初始化失败，反向代理未启动: %s", err)
		return err
	}
	proxyTLS = tlsCfg

	proxy := httputil.NewSingleHostReverseProxy(proxyTarget)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = proxyTarget.Scheme
		req.URL.Host = proxyTarget.Host
		req.Host = proxyTarget.Host
		req.Header.Set("X-Forwarded-For", req.RemoteAddr)
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Del("Origin")
		req.Header.Del("Sec-Fetch-Site")
		req.Header.Del("Sec-Fetch-Mode")
		req.Header.Del("Sec-Fetch-Dest")
	}
	errHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		LogWarning("反向代理 %s 错误: %s", proxyAddr, err)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "bad_gateway",
			"message": proxyErrMessage(),
			"detail":  err.Error(),
		})
	}
	proxy.ErrorHandler = errHandler

	// 建立 TCP 监听器
	ln, err := net.Listen("tcp", proxyAddr)
	if err != nil {
		LogWarning("监听 %s 失败: %s", proxyAddr, err)
		return err
	}

	// cmux 协议分发
	mx := cmux.New(ln)
	tlsL := mx.Match(cmux.TLS())
	httpL := mx.Match(cmux.Any())

	proxyCmux = mx
	proxyHTTPS = &http.Server{Handler: proxyWithAuth(proxy), TLSConfig: tlsCfg}
	proxyHTTP = &http.Server{Handler: proxyWithAuth(proxy)}

	LogInfo("反向代理启动(HTTP+HTTPS): %s → %s", proxyAddr, proxyTarget.String())

	go func() {
		if err := proxyHTTPS.ServeTLS(tlsL, "", ""); err != nil && err != http.ErrServerClosed {
			LogWarning("HTTPS 反向代理退出: %s", err)
		}
	}()
	go func() {
		if err := proxyHTTP.Serve(httpL); err != nil && err != http.ErrServerClosed {
			LogWarning("HTTP 反向代理退出: %s", err)
		}
	}()
	go func() {
		if err := mx.Serve(); err != nil && err != net.ErrClosed {
			LogWarning("cmux 退出: %s", err)
		}
	}()

	return nil
}

func stopReverseProxy() {
	proxyMu.Lock()
	defer proxyMu.Unlock()
	stopReverseProxyLocked()
}

func stopReverseProxyLocked() {
	if proxyHTTP == nil && proxyHTTPS == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if proxyHTTPS != nil {
		_ = proxyHTTPS.Shutdown(ctx)
		proxyHTTPS = nil
	}
	if proxyHTTP != nil {
		_ = proxyHTTP.Shutdown(ctx)
		proxyHTTP = nil
	}
	if proxyCmux != nil {
		proxyCmux.Close()
		proxyCmux = nil
	}
	LogInfo("反向代理已停止")
}

// proxyErrMessage 根据当前服务状态给出准确的代理错误提示
func proxyErrMessage() string {
	switch state.Status() {
	case StatusRunning:
		return "服务启动中，请稍候重试"
	case StatusBuilding:
		return "服务构建中，请稍候重试"
	case StatusStopped:
		return "服务未启动，请在概览页启动"
	default:
		return "无法连接到后端服务"
	}
}

// restartReverseProxy 按最新配置重启反向代理
func restartReverseProxy() {
	proxyMu.Lock()
	defer proxyMu.Unlock()
	stopReverseProxyLocked()
	LogInfo("反向代理配置变更，按新配置重启")
	if err := startReverseProxyLocked(); err != nil {
		LogWarning("反向代理重启失败: %s", err)
	}
}