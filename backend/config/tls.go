/*
 * Rabbit Panel - 容器运维面板
 * Author: reisen7
 * Email: reisen7@foxmail.com
 */
package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// 默认自签证书存放位置（位于 data 目录，Docker 部署时随卷持久化）
const (
	defaultTLSCertFile = "./data/certs/server.crt"
	defaultTLSKeyFile  = "./data/certs/server.key"

	selfSignedValidity = 10 * 365 * 24 * time.Hour

	// 自签证书标记文件（<cert>.hosts）中的字段前缀
	markerFingerprintPrefix = "fingerprint="
	markerHostsPrefix       = "hosts="
)

// TLSConfig 面板原生 HTTPS 配置。
type TLSConfig struct {
	Enabled    bool     // TLS_ENABLED
	CertFile   string   // TLS_CERT_FILE
	KeyFile    string   // TLS_KEY_FILE
	SelfSigned bool     // TLS_SELF_SIGNED：证书缺失时自动生成自签证书
	Hosts      []string // TLS_HOSTS：自签证书额外的域名 / IP

	tlsConfig *tls.Config
}

// Config 返回可用于 http.Server 的 TLS 配置。Prepare 成功后才非 nil。
func (tc *TLSConfig) Config() *tls.Config {
	return tc.tlsConfig
}

// Prepare 加载证书；证书缺失且允许自签时自动生成。未启用 HTTPS 时直接返回。
func (tc *TLSConfig) Prepare() error {
	if !tc.Enabled {
		return nil
	}
	if strings.TrimSpace(tc.CertFile) == "" || strings.TrimSpace(tc.KeyFile) == "" {
		return fmt.Errorf("TLS 已启用但未配置 TLS_CERT_FILE / TLS_KEY_FILE")
	}
	for _, path := range []string{tc.CertFile, tc.KeyFile} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("创建证书目录失败: %w", err)
		}
	}

	pair, err := tc.loadOrGenerate()
	if err != nil {
		return err
	}

	tc.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{pair},
		MinVersion:   tls.VersionTLS12,
		// 强制 HTTP/1.1：容器终端使用 gorilla/websocket，仅支持 HTTP/1.1 升级
		NextProtos: []string{"http/1.1"},
	}
	return nil
}

func (tc *TLSConfig) loadOrGenerate() (tls.Certificate, error) {
	missing := !fileExists(tc.CertFile) || !fileExists(tc.KeyFile)
	if missing {
		if !tc.SelfSigned {
			return tls.Certificate{}, fmt.Errorf(
				"未找到 TLS 证书 %s / %s：请挂载已有证书，或设置 TLS_SELF_SIGNED=true 由面板自动生成自签证书",
				tc.CertFile, tc.KeyFile,
			)
		}
		if err := tc.generate(); err != nil {
			return tls.Certificate{}, err
		}
	}

	pair, err := tls.LoadX509KeyPair(tc.CertFile, tc.KeyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("加载 TLS 证书失败: %w", err)
	}

	leaf := pair.Leaf
	if leaf == nil {
		if parsed, parseErr := x509.ParseCertificate(pair.Certificate[0]); parseErr == nil {
			leaf = parsed
			pair.Leaf = parsed
		}
	}
	if leaf == nil {
		return pair, nil
	}

	// 自签证书：过期、或 TLS_HOSTS 发生变化时重新签发。
	// 只处理面板自己生成的证书（靠 sidecar 标记文件 + 证书指纹识别），用户自备证书不动。
	if tc.SelfSigned && tc.isOwnSelfSigned(leaf) {
		if reason := tc.selfSignedRenewReason(leaf); reason != "" {
			log.Printf("TLS: %s，重新生成自签证书", reason)
			if err := tc.generate(); err != nil {
				return tls.Certificate{}, err
			}
			if pair, err = tls.LoadX509KeyPair(tc.CertFile, tc.KeyFile); err != nil {
				return tls.Certificate{}, fmt.Errorf("重新加载 TLS 证书失败: %w", err)
			}
			if parsed, parseErr := x509.ParseCertificate(pair.Certificate[0]); parseErr == nil {
				pair.Leaf = parsed
				leaf = parsed
			}
		} else {
			// 旧格式标记文件（只记主机列表、没记指纹）补上指纹，避免用户直接替换证书文件后被误判
			tc.upgradeLegacyMarker(leaf)
		}
	} else if time.Now().After(leaf.NotAfter) {
		return tls.Certificate{}, fmt.Errorf("TLS 证书已于 %s 过期，请更换证书", leaf.NotAfter.Format("2006-01-02"))
	}

	log.Printf("TLS: 已加载证书 %s", tc.CertFile)
	log.Printf("TLS: 主体 %s，有效期至 %s", leaf.Subject.CommonName, leaf.NotAfter.Format("2006-01-02 15:04:05"))
	if len(leaf.DNSNames) > 0 || len(leaf.IPAddresses) > 0 {
		log.Printf("TLS: 适用域名/IP %s", strings.Join(certificateHosts(leaf), ", "))
	}
	if time.Now().Add(14 * 24 * time.Hour).After(leaf.NotAfter) {
		log.Printf("TLS: 警告：证书将在 14 天内过期，请及时续期")
	}
	return pair, nil
}

// hostsMarkerPath 返回记录自签证书指纹与主机列表的标记文件路径。
func (tc *TLSConfig) hostsMarkerPath() string {
	return tc.CertFile + ".hosts"
}

// certFingerprint 返回证书 DER 的 SHA-256 十六进制指纹。
func certFingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}

// markerFingerprint 读取标记文件中记录的证书指纹；旧格式或读取失败时返回空串。
func (tc *TLSConfig) markerFingerprint() string {
	data, err := os.ReadFile(tc.hostsMarkerPath())
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), markerFingerprintPrefix); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// isOwnSelfSigned 判断当前证书是否由面板生成。
// 除了要求标记文件存在，还要校验指纹：标记文件存在但指纹对不上，说明用户直接替换了
// 证书文件，此时按用户自备证书处理，绝不覆盖。
// 旧格式标记文件（只记录主机列表、无指纹）视为面板生成，随后由 upgradeLegacyMarker 补齐。
func (tc *TLSConfig) isOwnSelfSigned(leaf *x509.Certificate) bool {
	if !fileExists(tc.hostsMarkerPath()) {
		return false
	}
	recorded := tc.markerFingerprint()
	if recorded == "" || leaf == nil {
		return true
	}
	return recorded == certFingerprint(leaf)
}

// selfSignedRenewReason 返回需要重新签发的原因；无需重签时返回空串。
func (tc *TLSConfig) selfSignedRenewReason(leaf *x509.Certificate) string {
	if time.Now().After(leaf.NotAfter) {
		return fmt.Sprintf("自签证书已过期（%s）", leaf.NotAfter.Format("2006-01-02 15:04:05"))
	}
	data, err := os.ReadFile(tc.hostsMarkerPath())
	if err != nil {
		return ""
	}
	previous := markerHosts(string(data))
	want := strings.Join(sortedHosts(tc.certificateHosts()), ",")
	if strings.TrimSpace(previous) != want {
		return "TLS_HOSTS 或本机地址已变化，原证书不覆盖当前域名/IP"
	}
	return ""
}

// upgradeLegacyMarker 为旧格式标记文件补上证书指纹。
func (tc *TLSConfig) upgradeLegacyMarker(leaf *x509.Certificate) {
	if leaf == nil || tc.markerFingerprint() != "" {
		return
	}
	data, err := os.ReadFile(tc.hostsMarkerPath())
	if err != nil {
		return
	}
	if err := tc.writeMarker(markerHosts(string(data)), leaf); err != nil {
		log.Printf("TLS: 升级自签证书标记文件失败（不影响本次启动）: %v", err)
	}
}

// writeMarker 写入标记文件：证书指纹 + 签发时使用的主机列表。
func (tc *TLSConfig) writeMarker(hosts string, leaf *x509.Certificate) error {
	content := markerFingerprintPrefix + certFingerprint(leaf) + "\n" + markerHostsPrefix + hosts + "\n"
	return writeFileAtomic(tc.hostsMarkerPath(), []byte(content), 0o644)
}

// markerHosts 从标记文件内容中取出主机列表，兼容只有一行主机列表的旧格式。
func markerHosts(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), markerHostsPrefix); ok {
			return strings.TrimSpace(value)
		}
	}
	// 旧格式：整个文件就是主机列表
	return strings.TrimSpace(content)
}

func (tc *TLSConfig) generate() error {
	hosts := tc.certificateHosts()
	log.Printf("TLS: 正在生成自签证书 %s（适用域名/IP: %s）", tc.CertFile, strings.Join(hosts, ", "))
	if err := GenerateSelfSignedCert(tc.CertFile, tc.KeyFile, hosts); err != nil {
		return err
	}
	// 记录证书指纹与签发时使用的主机列表，用于后续判断是否需要重签、以及证书是否被替换
	leaf, err := readLeaf(tc.CertFile)
	if err != nil {
		log.Printf("TLS: 读取新证书失败（跳过标记文件写入）: %v", err)
		return nil
	}
	if err := tc.writeMarker(strings.Join(sortedHosts(hosts), ","), leaf); err != nil {
		log.Printf("TLS: 写入自签证书标记文件失败（不影响本次启动）: %v", err)
	}
	return nil
}

// readLeaf 解析证书文件中的第一张证书。
func readLeaf(certFile string) (*x509.Certificate, error) {
	raw, err := os.ReadFile(certFile)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("%s 不是 PEM 格式", certFile)
	}
	return x509.ParseCertificate(block.Bytes)
}

// certificateHosts 汇总自签证书需要覆盖的域名与 IP。
func (tc *TLSConfig) certificateHosts() []string {
	hosts := append([]string{}, tc.Hosts...)
	hosts = append(hosts, "localhost", "127.0.0.1", "::1")
	if hostname, err := os.Hostname(); err == nil && strings.TrimSpace(hostname) != "" {
		hosts = append(hosts, hostname)
	}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
				hosts = append(hosts, ipNet.IP.String())
			}
		}
	}
	return dedupeHosts(hosts)
}

// GenerateSelfSignedCert 生成 ECDSA P-256 自签证书，写入 certFile / keyFile。
func GenerateSelfSignedCert(certFile, keyFile string, hosts []string) error {
	notBefore := time.Now().Add(-1 * time.Hour)
	notAfter := time.Now().Add(selfSignedValidity)

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("生成证书序列号失败: %w", err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("生成私钥失败: %w", err)
	}

	commonName := "rabbit-panel"
	if len(hosts) > 0 {
		commonName = hosts[0]
	}

	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName, Organization: []string{"Rabbit Panel"}},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
			continue
		}
		template.DNSNames = append(template.DNSNames, host)
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("签发证书失败: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("编码私钥失败: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	if err := writeFileAtomic(certFile, certPEM, 0o644); err != nil {
		return err
	}
	if err := writeFileAtomic(keyFile, keyPEM, 0o600); err != nil {
		return err
	}
	return nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return fmt.Errorf("写入 %s 失败: %w", path, err)
	}
	if err := os.Chmod(tmp, perm); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("设置 %s 权限失败: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("替换 %s 失败: %w", path, err)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func certificateHosts(cert *x509.Certificate) []string {
	hosts := append([]string{}, cert.DNSNames...)
	for _, ip := range cert.IPAddresses {
		hosts = append(hosts, ip.String())
	}
	if len(hosts) == 0 && cert.Subject.CommonName != "" {
		hosts = append(hosts, cert.Subject.CommonName)
	}
	return hosts
}

func dedupeHosts(hosts []string) []string {
	seen := make(map[string]struct{}, len(hosts))
	result := make([]string, 0, len(hosts))
	for _, host := range hosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		key := strings.ToLower(host)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, host)
	}
	return result
}

// sortedHosts 返回排序后的主机列表，用于稳定比较。
func sortedHosts(hosts []string) []string {
	result := append([]string{}, hosts...)
	sort.Strings(result)
	return result
}
