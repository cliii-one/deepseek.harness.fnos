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
	"strings"
	"sync"
	"time"

	"github.com/soheilhy/cmux"
)

const (
	authCookieName      = "harness_auth"
	authLoginPath       = "/_harness_auth"
	authMaxAttempts     = 3
	authLockoutDuration = 1 * time.Hour
)

type clientAuthStatus struct {
	failedCount int
	lockUntil   time.Time
}

var (
	authLockMu       sync.Mutex
	clientAuthRecord = make(map[string]*clientAuthStatus)
)

func getClientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func checkAuthLockout(clientIP string) (bool, time.Duration, int) {
	authLockMu.Lock()
	defer authLockMu.Unlock()

	status, exists := clientAuthRecord[clientIP]
	if !exists {
		return false, 0, authMaxAttempts
	}

	if status.failedCount >= authMaxAttempts {
		now := time.Now()
		if now.Before(status.lockUntil) {
			return true, status.lockUntil.Sub(now), 0
		}
		delete(clientAuthRecord, clientIP)
		return false, 0, authMaxAttempts
	}

	return false, 0, authMaxAttempts - status.failedCount
}

func recordAuthFailure(clientIP string) (bool, time.Duration, int) {
	authLockMu.Lock()
	defer authLockMu.Unlock()

	status, exists := clientAuthRecord[clientIP]
	if !exists {
		status = &clientAuthStatus{}
		clientAuthRecord[clientIP] = status
	}

	status.failedCount++
	if status.failedCount >= authMaxAttempts {
		status.lockUntil = time.Now().Add(authLockoutDuration)
		return true, authLockoutDuration, 0
	}

	return false, 0, authMaxAttempts - status.failedCount
}

func recordAuthSuccess(clientIP string) {
	authLockMu.Lock()
	delete(clientAuthRecord, clientIP)
	authLockMu.Unlock()
}

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

		clientIP := getClientIP(r)
		isLocked, lockRemaining, remainingAttempts := checkAuthLockout(clientIP)

		// 处理登录表单提交
		if r.URL.Path == authLoginPath && r.Method == http.MethodPost {
			if isLocked {
				serveLoginPage(w, isLocked, lockRemaining, 0)
				return
			}

			_ = r.ParseForm()
			if r.FormValue("password") == pwd {
				recordAuthSuccess(clientIP)
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
				lockedNow, remDuration, remAttempts := recordAuthFailure(clientIP)
				serveLoginPage(w, lockedNow, remDuration, remAttempts)
			}
			return
		}

		// 校验 cookie
		if c, err := r.Cookie(authCookieName); err != nil || c.Value != pwd {
			serveLoginPage(w, isLocked, lockRemaining, remainingAttempts)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func serveLoginPage(w http.ResponseWriter, isLocked bool, lockRemaining time.Duration, remainingAttempts int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	errorHTML := ""
	inputDisabledAttr := ""
	buttonDisabledAttr := ""
	buttonText := "进入"

	if isLocked {
		w.WriteHeader(http.StatusTooManyRequests)
		mins := int(lockRemaining.Minutes()) + 1
		errorHTML = fmt.Sprintf(`<div class="err">
      <svg width="15" height="15" viewBox="0 0 20 20" fill="currentColor">
        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"/>
      </svg>
      <span>密码错误达 3 次已锁定，请等待约 %d 分钟或重启服务</span>
    </div>`, mins)
		inputDisabledAttr = "disabled"
		buttonDisabledAttr = "disabled"
		buttonText = "已锁定冷却中"
	} else if remainingAttempts < authMaxAttempts {
		w.WriteHeader(http.StatusUnauthorized)
		errorHTML = fmt.Sprintf(`<div class="err">
      <svg width="15" height="15" viewBox="0 0 20 20" fill="currentColor">
        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"/>
      </svg>
      <span>密码错误，还可尝试 %d 次</span>
    </div>`, remainingAttempts)
	}

	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>DeepSeek Harness · 访问验证</title>
<style>
*{box-sizing:border-box;margin:0;padding:0;-webkit-tap-highlight-color:transparent}
body{
  min-height:100vh;display:flex;align-items:center;justify-content:center;
  background-color:#f4f6fb;
  background-image:radial-gradient(at 0%% 0%%, rgba(22,105,255,0.06) 0, transparent 50%%),
                   radial-gradient(at 100%% 100%%, rgba(22,105,255,0.04) 0, transparent 50%%);
  font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"PingFang SC","Hiragino Sans GB","Microsoft YaHei",sans-serif;
  color:#1e293b;padding:16px;
}
.card{
  background:#ffffff;border-radius:20px;padding:36px 32px;width:100%%;max-width:360px;
  border:1px solid rgba(226,232,240,0.8);
  box-shadow:0 10px 25px -5px rgba(0,0,0,0.04),0 8px 10px -6px rgba(0,0,0,0.02);
  transition:all 0.2s cubic-bezier(0.16,1,0.3,1);
}
.logo-wrap{
  width:52px;height:52px;background:#f0f5ff;border-radius:16px;border:1px solid #e0eaff;
  display:flex;align-items:center;justify-content:center;margin:0 auto 16px;
}
.logo-wrap svg{width:32px;height:32px}
h1{text-align:center;font-size:18px;font-weight:700;color:#0f172a;letter-spacing:-0.01em;margin-bottom:4px}
.sub{text-align:center;font-size:12px;color:#64748b;font-weight:500;margin-bottom:24px}
input{
  width:100%%;padding:11px 14px;border:1px solid #e2e8f0;border-radius:10px;
  font-size:14px;outline:none;background:#f8fafc;color:#0f172a;
  transition:all 0.18s ease;
}
input:focus{
  border-color:#1669ff;background:#ffffff;
  box-shadow:0 0 0 3px rgba(22,105,255,0.12);
}
input:disabled{
  background:#f1f5f9;color:#94a3b8;cursor:not-allowed;border-color:#e2e8f0;
}
button{
  width:100%%;margin-top:14px;padding:11px;background:#1669ff;color:#ffffff;border:none;border-radius:10px;
  font-size:14px;font-weight:600;cursor:pointer;
  transition:all 0.15s ease;box-shadow:0 2px 6px rgba(22,105,255,0.25);
}
button:hover:not(:disabled){background:#3b82f6;box-shadow:0 4px 12px rgba(22,105,255,0.35)}
button:active:not(:disabled){background:#0052e0;transform:scale(0.98)}
button:disabled{
  background:#94a3b8;cursor:not-allowed;box-shadow:none;transform:none;opacity:0.8;
}
.err{
  margin-top:14px;font-size:13px;color:#ef4444;
  background:#fef2f2;border:1px solid #fee2e2;padding:9px 12px;border-radius:10px;
  display:flex;align-items:center;justify-content:center;gap:6px;line-height:1.4;
}
</style>
</head>
<body>
<div class="card">
  <div class="logo-wrap">
    <svg viewBox="0 0 50 50" fill="none">
      <path d="M48.8354 10.0479C48.3232 9.79199 48.1025 10.2798 47.8032 10.5278C47.7007 10.6079 47.6143 10.7119 47.5273 10.8076C46.7793 11.624 45.9048 12.1597 44.7622 12.0957C43.0923 12 41.666 12.5356 40.4058 13.8398C40.1377 12.2319 39.2476 11.272 37.8926 10.6558C37.1836 10.3359 36.4668 10.0156 35.9702 9.31982C35.6235 8.82373 35.5293 8.27197 35.356 7.72754C35.2456 7.3999 35.1353 7.06396 34.7651 7.00781C34.3633 6.94385 34.2056 7.2876 34.0479 7.57568C33.418 8.75195 33.1733 10.0479 33.1973 11.3599C33.2524 14.312 34.4736 16.6641 36.8999 18.3359C37.1758 18.5278 37.2466 18.7197 37.1597 19C36.9946 19.5757 36.7974 20.1357 36.624 20.7119C36.5137 21.0801 36.3486 21.1597 35.9624 21C34.6309 20.4321 33.481 19.5918 32.4644 18.5757C30.7393 16.8721 29.1792 14.9917 27.2334 13.52C26.7764 13.1758 26.3193 12.856 25.8467 12.5518C23.8618 10.584 26.1069 8.96777 26.627 8.77588C27.1704 8.57568 26.8159 7.8877 25.0591 7.896C23.3022 7.90381 21.6953 8.50391 19.647 9.30371C19.3477 9.42383 19.0322 9.51172 18.7095 9.58398C16.8501 9.22363 14.9199 9.14355 12.9033 9.37598C9.10596 9.80762 6.07275 11.6396 3.84326 14.7681C1.16455 18.5278 0.53418 22.7998 1.30664 27.2559C2.11768 31.9521 4.46582 35.8398 8.07373 38.8799C11.8159 42.0322 16.1255 43.5762 21.041 43.2803C24.0269 43.104 27.3516 42.6963 31.1016 39.4561C32.0469 39.936 33.0396 40.1279 34.686 40.272C35.9546 40.3921 37.1758 40.208 38.1211 40.0078C39.6021 39.688 39.4995 38.2881 38.9639 38.0322C34.623 35.9678 35.5762 36.8081 34.71 36.1279C36.9155 33.4639 40.2402 30.6958 41.54 21.728C41.6426 21.0161 41.5557 20.5679 41.54 19.9917C41.5322 19.6396 41.6108 19.5039 42.0049 19.4639C43.0923 19.3359 44.1479 19.0317 45.1167 18.4878C47.9292 16.9199 49.064 14.3438 49.3315 11.2559C49.3711 10.7837 49.3237 10.2959 48.8354 10.0479ZM24.3262 37.8398C20.1196 34.4639 18.0791 33.3521 17.2358 33.3999C16.4482 33.4482 16.5898 34.3682 16.7632 34.9678C16.9443 35.5601 17.1812 35.9683 17.5117 36.4878C17.7402 36.832 17.8979 37.3442 17.2832 37.728C15.9282 38.584 13.5728 37.4399 13.4624 37.3838C10.7207 35.7358 8.42822 33.5601 6.81348 30.584C5.25342 27.7197 4.34766 24.6479 4.19775 21.3677C4.1582 20.5757 4.38672 20.2959 5.15869 20.1519C6.17529 19.96 7.22314 19.9199 8.23926 20.0718C12.5327 20.7119 16.1885 22.6719 19.2529 25.7759C21.002 27.5439 22.3252 29.6558 23.6885 31.7202C25.1377 33.9121 26.6978 36 28.6831 37.7119C29.3843 38.312 29.9434 38.7681 30.479 39.104C28.8643 39.2881 26.1699 39.3281 24.3262 37.8398ZM26.3433 24.6001C26.3433 24.248 26.6191 23.9678 26.9658 23.9678C27.0444 23.9678 27.1152 23.9839 27.1782 24.0078C27.2651 24.04 27.3438 24.0879 27.4067 24.1602C27.5171 24.272 27.5801 24.4321 27.5801 24.6001C27.5801 24.9521 27.3042 25.2319 26.9575 25.2319C26.6108 25.2319 26.3433 24.9521 26.3433 24.6001ZM32.6064 27.8799C32.2046 28.0479 31.8027 28.1919 31.4165 28.208C30.8179 28.2397 30.1641 27.9922 29.8096 27.688C29.2583 27.2158 28.8643 26.9521 28.6987 26.1279C28.6279 25.7759 28.6675 25.2319 28.7305 24.9199C28.8721 24.248 28.7144 23.8159 28.2495 23.4238C27.8716 23.104 27.3911 23.0161 26.8633 23.0161C26.666 23.0161 26.4849 22.9277 26.3511 22.856C26.1304 22.7441 25.9492 22.4639 26.1226 22.1201C26.1777 22.0078 26.4458 21.7358 26.5088 21.688C27.2256 21.272 28.0527 21.4077 28.8169 21.7197C29.5259 22.0161 30.0615 22.5601 30.834 23.3281C31.6216 24.2559 31.7632 24.5117 32.2124 25.208C32.5669 25.752 32.8901 26.312 33.1104 26.9521C33.2446 27.3521 33.0713 27.6802 32.6064 27.8799Z" fill="#1669ff"/>
    </svg>
  </div>
  <h1>访问验证</h1>
  <p class="sub">DeepSeek Harness</p>
  <form method="POST" action="%s">
    <input type="password" name="password" placeholder="请输入访问密码" autofocus autocomplete="current-password" %s>
    <button type="submit" %s>%s</button>
  </form>
  %s
</div>
</body>
</html>`, authLoginPath, inputDisabledAttr, buttonDisabledAttr, buttonText, errorHTML)
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
		port = 2298
	}
	proxyTarget, _ = url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))

	proxyPort := cfg.ProxyPort
	if proxyPort <= 0 {
		proxyPort = 2299
	}
	proxyAddr = fmt.Sprintf("0.0.0.0:%d", proxyPort)
}

func listAllDNSNames() []string {
	names := []string{"localhost", "deepseek-harness"}
	if hostname, err := os.Hostname(); err == nil && hostname != "" && hostname != "localhost" {
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
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
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

	LogInfo("TLS 自签名证书已就绪: %s", certPath)
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
		LogWarning("TLS 证书加载失败，反向代理未启动: %s", err)
		return err
	}
	proxyTLS = tlsCfg

	errHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		LogWarning("反向代理转发错误 [%s]: %s", proxyAddr, err)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "bad_gateway",
			"message": proxyErrMessage(),
			"detail":  err.Error(),
		})
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(proxyTarget)
			pr.SetXForwarded()
			// 改写为目标同源 Origin，保留标头供插件使用并防止上游 CSRF 校验失败
			if pr.Out.Header.Get("Origin") != "" {
				pr.Out.Header.Set("Origin", fmt.Sprintf("%s://%s", proxyTarget.Scheme, proxyTarget.Host))
			}
			// 改写为 same-origin，防止跨站/iframe 标记被上游拦截并保留标头
			if pr.Out.Header.Get("Sec-Fetch-Site") != "" {
				pr.Out.Header.Set("Sec-Fetch-Site", "same-origin")
			}
		},
		ErrorHandler: errHandler,
	}

	// 建立 TCP 监听器
	ln, err := net.Listen("tcp", proxyAddr)
	if err != nil {
		LogWarning("反向代理端口监听失败 [%s]: %s", proxyAddr, err)
		return err
	}

	// cmux 协议分发
	mx := cmux.New(ln)
	tlsL := mx.Match(cmux.TLS())
	httpL := mx.Match(cmux.Any())

	proxyCmux = mx
	proxyHTTPS = &http.Server{Handler: proxyWithAuth(proxy), TLSConfig: tlsCfg}
	proxyHTTP = &http.Server{Handler: proxyWithAuth(proxy)}

	LogInfo("Web 服务就绪探测通过，反向代理启动完成 [%s → %s]", proxyAddr, proxyTarget.String())

	go func() {
		if err := proxyHTTPS.ServeTLS(tlsL, "", ""); err != nil && !isExpectedCloseErr(err) {
			LogWarning("HTTPS 代理服务异常退出: %s", err)
		}
	}()
	go func() {
		if err := proxyHTTP.Serve(httpL); err != nil && !isExpectedCloseErr(err) {
			LogWarning("HTTP 代理服务异常退出: %s", err)
		}
	}()
	go func() {
		if err := mx.Serve(); err != nil && !isExpectedCloseErr(err) {
			LogWarning("cmux 协议多路复用器退出: %s", err)
		}
	}()

	return nil
}

func isExpectedCloseErr(err error) bool {
	if err == nil || err == http.ErrServerClosed || err == net.ErrClosed || err == cmux.ErrListenerClosed || err == cmux.ErrServerClosed {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "server closed") ||
		strings.Contains(msg, "closed network connection")
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
	LogInfo("反向代理服务已停止")
}

// proxyErrMessage 根据当前服务状态给出准确的代理错误提示
func proxyErrMessage() string {
	switch state.Status() {
	case StatusStarting:
		return "服务启动中，请稍候重试"
	case StatusRunning:
		return "服务响应异常，请检查后端运行状态"
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
	if state.Status() != StatusRunning {
		return
	}
	LogInfo("反向代理配置已变更，执行热重载")
	if err := startReverseProxyLocked(); err != nil {
		LogWarning("反向代理热重载失败: %s", err)
	}
}
