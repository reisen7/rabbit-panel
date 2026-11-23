package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
)

// resolveSecret 解析密钥：环境变量非空时走 requireUserSecret 校验；
// 未设置且允许生成时，生成随机密钥打印到日志（仅本次运行有效）。
func resolveSecret(name, envValue string, allowGenerate bool) (string, error) {
	if strings.TrimSpace(envValue) != "" {
		return requireUserSecret(name, envValue)
	}

	if !allowGenerate {
		_, err := requireUserSecret(name, envValue)
		return "", fmt.Errorf("%w（Worker 节点必须与 Master 使用相同的密钥）", err)
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("生成 %s 失败: %w", name, err)
	}
	secret := hex.EncodeToString(raw)
	log.Printf("已自动生成 %s（仅本次运行有效，重启会变化；如需固定请通过环境变量设置）: %s", name, secret)
	return secret, nil
}
