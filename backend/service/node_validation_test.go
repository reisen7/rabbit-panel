package service

import "testing"

func TestValidateNodeAddress(t *testing.T) {
	valid := []string{
		"192.168.1.20:3958",
		"http://192.168.1.20:3958",
		"https://worker.internal:3958",
		"rabbit-worker:3958",
		"[::1]:3958",
	}
	for _, addr := range valid {
		if err := ValidateNodeAddress(addr); err != nil {
			t.Errorf("valid address %q rejected: %v", addr, err)
		}
	}

	invalid := []string{
		"",
		"192.168.1.20",                  // 缺端口
		"http://user@192.168.1.20:3958", // userinfo 伪装
		"http://user:pass@host:3958",    // userinfo 带密码
		"192.168.1.20:3958/evil",        // 路径
		"192.168.1.20:3958?q=1",         // 查询串
		"ftp://192.168.1.20:3958",       // 非法协议
		"192.168.1.20:abc",              // 非数字端口
		"192.168.1.20:99999",            // 端口越界
		"192.168.1.20:0",                // 端口越界
		"host name:3958",                // 空白字符
	}
	for _, addr := range invalid {
		if err := ValidateNodeAddress(addr); err == nil {
			t.Errorf("invalid address %q accepted", addr)
		}
	}
}

func TestValidateNodeID(t *testing.T) {
	for _, id := range []string{"worker-1", "host:3958", "edge_node", "worker-1:10001", "node.local:3958", "节点甲"} {
		if err := ValidateNodeID(id); err != nil {
			t.Errorf("valid id %q rejected: %v", id, err)
		}
	}
	for _, id := range []string{
		"", "  ", "has space", "has\nnewline", "has\ttab",
		// 路径/URL 语义字符：ID 会进入 HMAC 消息与 HTTP 头，必须拒绝
		"../x", "a/b", `a\b`, "a%2e", "a?x", "a#x", "a@x", "a&x", "a=x",
	} {
		if err := ValidateNodeID(id); err == nil {
			t.Errorf("invalid id %q accepted", id)
		}
	}
}
