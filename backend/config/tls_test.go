package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestTLSDisabledSkipPrepare(t *testing.T) {
	tc := TLSConfig{Enabled: false}
	if err := tc.Prepare(); err != nil {
		t.Fatalf("Prepare with TLS disabled should not fail: %v", err)
	}
	if tc.Config() != nil {
		t.Fatal("TLS config should stay nil when disabled")
	}
}

func TestPrepareFailsWithoutCertificate(t *testing.T) {
	dir := t.TempDir()
	tc := TLSConfig{
		Enabled:  true,
		CertFile: filepath.Join(dir, "server.crt"),
		KeyFile:  filepath.Join(dir, "server.key"),
	}
	err := tc.Prepare()
	if err == nil {
		t.Fatal("expected error when certificate is missing and self-signed disabled")
	}
	if !strings.Contains(err.Error(), "TLS_SELF_SIGNED") {
		t.Fatalf("error should hint TLS_SELF_SIGNED, got: %v", err)
	}
}

func TestPrepareGeneratesSelfSignedCertificate(t *testing.T) {
	dir := t.TempDir()
	tc := TLSConfig{
		Enabled:    true,
		SelfSigned: true,
		CertFile:   filepath.Join(dir, "certs", "server.crt"),
		KeyFile:    filepath.Join(dir, "certs", "server.key"),
		Hosts:      []string{"panel.example.com", "10.0.0.5"},
	}
	if err := tc.Prepare(); err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	if tc.Config() == nil {
		t.Fatal("TLS config should be ready")
	}
	if tc.Config().MinVersion != tls.VersionTLS12 {
		t.Fatalf("unexpected min TLS version: %x", tc.Config().MinVersion)
	}
	// 必须禁用 h2，否则容器终端的 WebSocket 升级会失败
	if len(tc.Config().NextProtos) != 1 || tc.Config().NextProtos[0] != "http/1.1" {
		t.Fatalf("NextProtos should be http/1.1 only, got %v", tc.Config().NextProtos)
	}

	raw, err := os.ReadFile(tc.CertFile)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		t.Fatal("cert file is not PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	if cert.Subject.CommonName != "panel.example.com" {
		t.Fatalf("unexpected CN: %s", cert.Subject.CommonName)
	}
	if len(cert.IPAddresses) == 0 {
		t.Fatal("IP SAN missing")
	}
	ipFound := false
	for _, ip := range cert.IPAddresses {
		if ip.String() == "10.0.0.5" {
			ipFound = true
		}
	}
	if !ipFound {
		t.Fatalf("IP SAN 10.0.0.5 missing: %v", cert.IPAddresses)
	}
	found := false
	for _, name := range cert.DNSNames {
		if name == "panel.example.com" {
			found = true
		}
	}
	if !found {
		t.Fatalf("DNS SAN missing: %v", cert.DNSNames)
	}

	info, err := os.Stat(tc.KeyFile)
	if err != nil {
		t.Fatalf("stat key: %v", err)
	}
	// Windows 不支持 Unix 权限位，跳过该断言
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("private key should be 0600, got %v", info.Mode().Perm())
	}

	// 重复 Prepare 应复用已有证书而不是重新生成
	first, _ := os.ReadFile(tc.KeyFile)
	tc2 := tc
	if err := tc2.Prepare(); err != nil {
		t.Fatalf("second Prepare failed: %v", err)
	}
	second, _ := os.ReadFile(tc.KeyFile)
	if string(first) != string(second) {
		t.Fatal("existing certificate should be reused")
	}
}

// TLS_HOSTS 变化后应重新签发自签证书，否则新增域名/IP 不在 SAN 里。
func TestSelfSignedCertReissuedWhenHostsChange(t *testing.T) {
	dir := t.TempDir()
	tc := TLSConfig{
		Enabled:    true,
		SelfSigned: true,
		CertFile:   filepath.Join(dir, "server.crt"),
		KeyFile:    filepath.Join(dir, "server.key"),
		Hosts:      []string{"panel.example.com"},
	}
	if err := tc.Prepare(); err != nil {
		t.Fatalf("first Prepare failed: %v", err)
	}
	first, err := os.ReadFile(tc.KeyFile)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}

	tc2 := tc
	tc2.Hosts = []string{"panel.example.com", "new.example.com"}
	if err := tc2.Prepare(); err != nil {
		t.Fatalf("second Prepare failed: %v", err)
	}
	second, err := os.ReadFile(tc2.KeyFile)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	if string(first) == string(second) {
		t.Fatal("certificate should be reissued after TLS_HOSTS changed")
	}

	raw, err := os.ReadFile(tc2.CertFile)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		t.Fatal("cert file is not PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	found := false
	for _, name := range cert.DNSNames {
		if name == "new.example.com" {
			found = true
		}
	}
	if !found {
		t.Fatalf("new host missing from SAN: %v", cert.DNSNames)
	}

	// 主机列表未变化时不应重复签发
	third, err := os.ReadFile(tc2.KeyFile)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	tc3 := tc2
	if err := tc3.Prepare(); err != nil {
		t.Fatalf("third Prepare failed: %v", err)
	}
	fourth, err := os.ReadFile(tc3.KeyFile)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	if string(third) != string(fourth) {
		t.Fatal("certificate should be reused when hosts did not change")
	}
}

// 标记文件存在但证书已被用户替换（指纹对不上）时，不得覆盖用户证书。
func TestReplacedCertificateNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "server.crt")
	keyFile := filepath.Join(dir, "server.key")

	// 面板先生成自签证书，留下带指纹的标记文件
	tc := TLSConfig{
		Enabled:    true,
		SelfSigned: true,
		CertFile:   certFile,
		KeyFile:    keyFile,
		Hosts:      []string{"original.example.com"},
	}
	if err := tc.Prepare(); err != nil {
		t.Fatalf("first Prepare failed: %v", err)
	}
	if !fileExists(tc.hostsMarkerPath()) {
		t.Fatal("marker file should exist after self-signed generation")
	}

	// 用户直接把自备证书覆盖到同一路径（标记文件残留）
	if err := GenerateSelfSignedCert(certFile, keyFile, []string{"user-provided.example.com"}); err != nil {
		t.Fatalf("generate user cert: %v", err)
	}
	before, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}

	// 即便 TLS_HOSTS 变化、且开启了自签，也不能覆盖用户证书
	tc2 := tc
	tc2.Hosts = []string{"another.example.com"}
	if err := tc2.Prepare(); err != nil {
		t.Fatalf("second Prepare failed: %v", err)
	}
	after, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("replaced certificate must not be overwritten even when marker file exists")
	}
}

// 旧格式标记文件（只有主机列表）应被识别为面板生成，并补上证书指纹。
func TestLegacyMarkerUpgradedWithFingerprint(t *testing.T) {
	dir := t.TempDir()
	tc := TLSConfig{
		Enabled:    true,
		SelfSigned: true,
		CertFile:   filepath.Join(dir, "server.crt"),
		KeyFile:    filepath.Join(dir, "server.key"),
		Hosts:      []string{"legacy.example.com"},
	}
	if err := tc.Prepare(); err != nil {
		t.Fatalf("first Prepare failed: %v", err)
	}

	// 退回旧格式：只保留主机列表那一行
	legacy := strings.Join(sortedHosts(tc.certificateHosts()), ",")
	if err := os.WriteFile(tc.hostsMarkerPath(), []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy marker: %v", err)
	}

	if err := tc.Prepare(); err != nil {
		t.Fatalf("second Prepare failed: %v", err)
	}
	if got := tc.markerFingerprint(); got == "" {
		t.Fatal("legacy marker should be upgraded with a certificate fingerprint")
	}

	leaf, err := readLeaf(tc.CertFile)
	if err != nil {
		t.Fatalf("read leaf: %v", err)
	}
	if got := tc.markerFingerprint(); got != certFingerprint(leaf) {
		t.Fatalf("fingerprint = %q, want %q", got, certFingerprint(leaf))
	}
	// 升级后主机列表仍然保留，不应触发重签
	if got := markerHosts(readMarkerFile(t, tc)); got != legacy {
		t.Fatalf("hosts in marker = %q, want %q", got, legacy)
	}
}

func readMarkerFile(t *testing.T, tc TLSConfig) string {
	t.Helper()
	data, err := os.ReadFile(tc.hostsMarkerPath())
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	return string(data)
}

// 用户自备证书不应被面板覆盖，即使开启了 TLS_SELF_SIGNED。
func TestExternalCertificateNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "server.crt")
	keyFile := filepath.Join(dir, "server.key")
	if err := GenerateSelfSignedCert(certFile, keyFile, []string{"external.example.com"}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	before, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}

	// 没有 .hosts 标记文件 = 不是面板生成的证书
	tc := TLSConfig{
		Enabled:    true,
		SelfSigned: true,
		CertFile:   certFile,
		KeyFile:    keyFile,
		Hosts:      []string{"totally-different.example.com"},
	}
	if err := tc.Prepare(); err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	after, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("external certificate must not be overwritten")
	}
}

func TestRedirectHost(t *testing.T) {
	cases := []struct{ in, port, want string }{
		{"panel.example.com", "3958", "panel.example.com:3958"},
		{"panel.example.com:8080", "3958", "panel.example.com:3958"},
		{"panel.example.com", "443", "panel.example.com"},
		{"panel.example.com:80", "443", "panel.example.com"},
		{"[::1]:8080", "3958", "[::1]:3958"},
		{"", "3958", "localhost:3958"},
	}
	for _, c := range cases {
		if got := redirectHost(c.in, c.port); got != c.want {
			t.Errorf("redirectHost(%q, %q) = %q, want %q", c.in, c.port, got, c.want)
		}
	}
}

// WebSocket 终端依赖 HTTP/1.1 升级，验证 HTTPS 下仍可正常升级。
func TestHTTPSAllowsWebSocketUpgrade(t *testing.T) {
	dir := t.TempDir()
	tc := TLSConfig{
		Enabled:    true,
		SelfSigned: true,
		CertFile:   filepath.Join(dir, "server.crt"),
		KeyFile:    filepath.Join(dir, "server.key"),
	}
	if err := tc.Prepare(); err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	upgrader := websocket.Upgrader{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte("pong"))
	})

	app := &App{TLS: tc}
	srv := app.newServer("127.0.0.1:0", handler)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() { _ = srv.ServeTLS(ln, tc.CertFile, tc.KeyFile) }()
	defer srv.Close()

	dialer := websocket.Dialer{
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true},
		HandshakeTimeout: 5 * time.Second,
	}
	conn, resp, err := dialer.Dial("wss://"+ln.Addr().String()+"/terminal", nil)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("websocket dial over TLS failed (status %d): %v", status, err)
	}
	defer conn.Close()

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	if string(msg) != "pong" {
		t.Fatalf("unexpected message: %q", msg)
	}
}

// 校验 HTTPS 服务端确实只协商 HTTP/1.1。
func TestHTTPSNegotiatesHTTP11Only(t *testing.T) {
	dir := t.TempDir()
	tc := TLSConfig{
		Enabled:    true,
		SelfSigned: true,
		CertFile:   filepath.Join(dir, "server.crt"),
		KeyFile:    filepath.Join(dir, "server.key"),
	}
	if err := tc.Prepare(); err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	app := &App{TLS: tc}
	srv := app.newServer("127.0.0.1:0", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() { _ = srv.ServeTLS(ln, tc.CertFile, tc.KeyFile) }()
	defer srv.Close()

	conn, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"h2", "http/1.1"},
	})
	if err != nil {
		t.Fatalf("tls dial: %v", err)
	}
	defer conn.Close()
	if proto := conn.ConnectionState().NegotiatedProtocol; proto != "http/1.1" {
		t.Fatalf("expected http/1.1 ALPN, got %q", proto)
	}
}

func TestRedirectHandler(t *testing.T) {
	app := &App{Port: "3958", RedirectPort: "8443"}
	app.TLS.Enabled = true
	handler := app.redirectHandler()

	req := httptest.NewRequest(http.MethodGet, "/dashboard?tab=1", nil)
	req.Host = "panel.example.com"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://panel.example.com:8443/dashboard?tab=1" {
		t.Fatalf("unexpected redirect target: %s", loc)
	}

	health := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	health.Host = "panel.example.com"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, health)
	if rec.Code != http.StatusOK || rec.Body.String() != "OK" {
		t.Fatalf("health on redirect port should return OK, got %d %q", rec.Code, rec.Body.String())
	}
}
