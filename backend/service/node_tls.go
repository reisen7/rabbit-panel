/*
 * Rabbit Panel - 容器运维面板
 * Author: reisen7
 * Email: reisen7@foxmail.com
 */
package service

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	nodeTLSOnce   sync.Once
	nodeTLSConfig *tls.Config
)

// NodeTLSConfig 返回访问其它节点（Master / Worker）时使用的 TLS 配置。
//
// 环境变量：
//   - NODE_TLS_INSECURE=true  跳过证书校验（自签证书集群常用）
//   - NODE_CA_FILE=/path/ca.crt  额外信任的根证书（用于私有 CA）
func NodeTLSConfig() *tls.Config {
	nodeTLSOnce.Do(func() {
		cfg := &tls.Config{MinVersion: tls.VersionTLS12}

		if caFile := strings.TrimSpace(os.Getenv("NODE_CA_FILE")); caFile != "" {
			pool, err := x509.SystemCertPool()
			if err != nil || pool == nil {
				pool = x509.NewCertPool()
			}
			if pemData, readErr := os.ReadFile(caFile); readErr == nil {
				if pool.AppendCertsFromPEM(pemData) {
					cfg.RootCAs = pool
				}
			}
		}

		switch strings.ToLower(strings.TrimSpace(os.Getenv("NODE_TLS_INSECURE"))) {
		case "1", "true", "yes", "on", "y":
			cfg.InsecureSkipVerify = true
		}
		nodeTLSConfig = cfg
	})
	return nodeTLSConfig
}

// NodeTransport 返回访问其它节点的 Transport（复用默认参数 + 节点 TLS 配置）。
func NodeTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = NodeTLSConfig()
	return transport
}

// NewNodeHTTPClient 创建访问其它节点的 HTTP 客户端。
func NewNodeHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: NodeTransport()}
}

// nodeEndpoint 拼出访问其它节点的完整 URL（带协议与可选查询参数）。
func nodeEndpoint(scheme, host, path string, query url.Values) string {
	if scheme != "https" {
		scheme = "http"
	}
	endpoint := scheme + "://" + host + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	return endpoint
}

// ParseNodeAddress 解析节点地址，支持省略协议（默认 http://）。
// 返回 scheme（http / https）与 host:port。
func ParseNodeAddress(raw string) (scheme, host string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "http", ""
	}
	if idx := strings.Index(raw, "://"); idx > 0 {
		scheme = strings.ToLower(raw[:idx])
		host = raw[idx+3:]
	} else {
		scheme = "http"
		host = raw
	}
	if cut := strings.IndexAny(host, "/?#"); cut >= 0 {
		host = host[:cut]
	}
	if scheme != "https" {
		scheme = "http"
	}
	return scheme, host
}

// WithNodeScheme 给节点地址补上协议前缀；已带协议或 scheme 为空时原样返回。
func WithNodeScheme(raw, scheme string) string {
	raw = strings.TrimSpace(raw)
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	if raw == "" || scheme == "" || strings.Contains(raw, "://") {
		return raw
	}
	return scheme + "://" + raw
}

// stripNodeScheme 去掉地址中的协议前缀。
func stripNodeScheme(raw string) string {
	raw = strings.TrimSpace(raw)
	if idx := strings.Index(raw, "://"); idx > 0 {
		raw = raw[idx+3:]
	}
	if cut := strings.IndexAny(raw, "/?#"); cut >= 0 {
		raw = raw[:cut]
	}
	return raw
}

// hostPortOnly 把地址标准化为 host:port 形式（用于展示与比较）。
func hostPortOnly(raw, defaultPort string) string {
	host := stripNodeScheme(raw)
	if host == "" {
		return ""
	}
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	if defaultPort == "" {
		return host
	}
	return net.JoinHostPort(host, defaultPort)
}
