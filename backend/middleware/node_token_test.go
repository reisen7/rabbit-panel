package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

func TestNodeTokenRoundTrip(t *testing.T) {
	token := GenerateNodeToken("worker-1", "test-secret")
	if !ValidateNodeToken("worker-1", token, "test-secret") {
		t.Fatal("fresh token must validate")
	}
}

func TestNodeTokenRejectsLegacyStaticFormat(t *testing.T) {
	// 旧格式：SHA256(nodeID+secret) 的裸 hex，无时间戳 —— 必须不再被接受
	h := sha256.New()
	h.Write([]byte("worker-1"))
	h.Write([]byte("test-secret"))
	legacy := hex.EncodeToString(h.Sum(nil))
	if ValidateNodeToken("worker-1", legacy, "test-secret") {
		t.Fatal("legacy static token must be rejected")
	}
}

func TestNodeTokenRejectsExpired(t *testing.T) {
	old := time.Now().Add(-2 * time.Hour)
	token := GenerateNodeTokenAt("worker-1", "test-secret", old)
	if ValidateNodeToken("worker-1", token, "test-secret") {
		t.Fatal("token older than tolerance must be rejected")
	}
}

func TestNodeTokenRejectsFarFuture(t *testing.T) {
	future := time.Now().Add(2 * time.Hour)
	token := GenerateNodeTokenAt("worker-1", "test-secret", future)
	if ValidateNodeToken("worker-1", token, "test-secret") {
		t.Fatal("token from far future must be rejected")
	}
}

func TestNodeTokenAcceptsWithinSkew(t *testing.T) {
	// 文档约定主/工作节点时钟偏差最多 1 小时
	for _, offset := range []time.Duration{-50 * time.Minute, 50 * time.Minute, 0} {
		token := GenerateNodeTokenAt("worker-1", "test-secret", time.Now().Add(offset))
		if !ValidateNodeToken("worker-1", token, "test-secret") {
			t.Fatalf("token within skew (%v) must validate", offset)
		}
	}
}

func TestNodeTokenRejectsWrongNodeID(t *testing.T) {
	token := GenerateNodeToken("worker-1", "test-secret")
	if ValidateNodeToken("worker-2", token, "test-secret") {
		t.Fatal("token must not validate for a different node ID")
	}
}

func TestNodeTokenRejectsWrongSecret(t *testing.T) {
	token := GenerateNodeToken("worker-1", "test-secret")
	if ValidateNodeToken("worker-1", token, "other-secret") {
		t.Fatal("token must not validate with a different secret")
	}
}

func TestNodeTokenRejectsTamperedMAC(t *testing.T) {
	token := GenerateNodeToken("worker-1", "test-secret")
	// 篡改签名最后一个字符
	tampered := token[:len(token)-1] + "0"
	if tampered == token {
		tampered = token[:len(token)-1] + "1"
	}
	if ValidateNodeToken("worker-1", tampered, "test-secret") {
		t.Fatal("tampered token must be rejected")
	}
}

func TestNodeTokenRejectsMalformed(t *testing.T) {
	for _, bad := range []string{"", "noTimestamp", "abc.", ".def", "notanumber.deadbeef", "12345.nothex!"} {
		if ValidateNodeToken("worker-1", bad, "test-secret") {
			t.Fatalf("malformed token %q must be rejected", bad)
		}
	}
	if ValidateNodeToken("", GenerateNodeToken("worker-1", "test-secret"), "test-secret") {
		t.Fatal("empty node ID must be rejected")
	}
}
