package service

import (
	"net/url"
	"testing"
)

func TestParseNodeAddress(t *testing.T) {
	cases := []struct {
		in         string
		wantScheme string
		wantHost   string
	}{
		{"192.168.1.20:10001", "http", "192.168.1.20:10001"},
		{"https://192.168.1.20:10001", "https", "192.168.1.20:10001"},
		{"https://worker-1:10001/", "https", "worker-1:10001"},
		{"HTTP://worker-1:10001", "http", "worker-1:10001"},
		{"worker-1:10001", "http", "worker-1:10001"},
		{"  ", "http", ""},
	}
	for _, c := range cases {
		scheme, host := ParseNodeAddress(c.in)
		if scheme != c.wantScheme || host != c.wantHost {
			t.Errorf("ParseNodeAddress(%q) = (%q, %q), want (%q, %q)", c.in, scheme, host, c.wantScheme, c.wantHost)
		}
	}
}

func TestWithNodeScheme(t *testing.T) {
	cases := []struct{ in, scheme, want string }{
		{"1.2.3.4:10001", "https", "https://1.2.3.4:10001"},
		{"1.2.3.4:10001", "", "1.2.3.4:10001"},
		{"https://1.2.3.4:10001", "https", "https://1.2.3.4:10001"},
		{"", "https", ""},
	}
	for _, c := range cases {
		if got := WithNodeScheme(c.in, c.scheme); got != c.want {
			t.Errorf("WithNodeScheme(%q, %q) = %q, want %q", c.in, c.scheme, got, c.want)
		}
	}
}

func TestHostPortOnly(t *testing.T) {
	cases := []struct{ in, port, want string }{
		{"https://192.168.1.20:10001", "10001", "192.168.1.20:10001"},
		{"192.168.1.20", "10001", "192.168.1.20:10001"},
		{"", "10001", ""},
	}
	for _, c := range cases {
		if got := hostPortOnly(c.in, c.port); got != c.want {
			t.Errorf("hostPortOnly(%q, %q) = %q, want %q", c.in, c.port, got, c.want)
		}
	}
}

// 智能体跨节点调用必须跟随节点协议，不能固定 http。
func TestNodeEndpointFollowsScheme(t *testing.T) {
	cases := []struct {
		scheme string
		host   string
		path   string
		query  url.Values
		want   string
	}{
		{"https", "192.168.1.20:10001", "/api/containers", nil, "https://192.168.1.20:10001/api/containers"},
		{"http", "192.168.1.20:10001", "/api/containers", nil, "http://192.168.1.20:10001/api/containers"},
		{"", "worker-1:10001", "/api/images", nil, "http://worker-1:10001/api/images"},
		{"https", "worker-1:10001", "/api/containers/logs", url.Values{"id": {"abc"}}, "https://worker-1:10001/api/containers/logs?id=abc"},
	}
	for _, c := range cases {
		if got := nodeEndpoint(c.scheme, c.host, c.path, c.query); got != c.want {
			t.Errorf("nodeEndpoint(%q, %q, %q) = %q, want %q", c.scheme, c.host, c.path, got, c.want)
		}
	}
}
