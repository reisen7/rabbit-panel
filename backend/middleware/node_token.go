package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// nodeTokenTolerance 是节点令牌的时间容忍窗。
// 与文档约定的主/工作节点时钟偏差上限（1 小时）一致：
// 令牌只在签发前后 1 小时内有效，过期或来自更远未来的令牌一律拒绝。
const nodeTokenTolerance = time.Hour

// GenerateNodeToken 生成带时间戳的 HMAC-SHA256 节点令牌。
// 格式：<unix秒>.<hex(HMAC-SHA256(key=secret, msg=nodeID+"."+ts))>
//
// 取代旧的静态 SHA256(nodeID+secret)：旧令牌永不过期，泄露一次即永久有效；
// 新令牌超过容忍窗即失效，且使用真正的 HMAC 与恒定时间比较。
func GenerateNodeToken(nodeID, secret string) string {
	return GenerateNodeTokenAt(nodeID, secret, time.Now())
}

// GenerateNodeTokenAt 与 GenerateNodeToken 相同，但使用指定时间（供测试与调用方注入时钟）。
func GenerateNodeTokenAt(nodeID, secret string, at time.Time) string {
	ts := strconv.FormatInt(at.Unix(), 10)
	return ts + "." + nodeTokenMAC(nodeID, ts, secret)
}

func nodeTokenMAC(nodeID, ts, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(nodeID))
	mac.Write([]byte{'.'})
	mac.Write([]byte(ts))
	return hex.EncodeToString(mac.Sum(nil))
}

// ValidateNodeToken 校验节点令牌的时间戳与 HMAC。
// 拒绝：格式错误、时间超出容忍窗、签名不符。比较使用恒定时间算法。
func ValidateNodeToken(nodeID, token, secret string) bool {
	if nodeID == "" || token == "" || secret == "" {
		return false
	}
	tsPart, macPart, ok := strings.Cut(token, ".")
	if !ok || tsPart == "" || macPart == "" {
		return false
	}
	ts, err := strconv.ParseInt(tsPart, 10, 64)
	if err != nil {
		return false
	}
	issuedAt := time.Unix(ts, 0)
	now := time.Now()
	if issuedAt.Before(now.Add(-nodeTokenTolerance)) || issuedAt.After(now.Add(nodeTokenTolerance)) {
		return false
	}
	expected := nodeTokenMAC(nodeID, tsPart, secret)
	if len(expected) != len(macPart) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(macPart)) == 1
}
