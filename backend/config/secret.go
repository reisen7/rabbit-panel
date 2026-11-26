package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// resolveSecret 解析密钥，优先级：环境变量 > 持久化文件 > 首次生成。
//
// 环境变量未设置时（Master/单机），不再每次启动换新随机值——
// 而是首次启动生成后写入 dataDir/secrets.env（0600）持久保存，
// 之后每次启动从文件读回。开箱即用的同时重启不掉登录态、
// worker 密钥保持固定。密钥不打印日志，查看用 cat ./data/secrets.env。
func resolveSecret(name, envValue string, allowGenerate bool) (string, error) {
	return resolveSecretWithDir(name, envValue, allowGenerate, "./data")
}

func resolveSecretWithDir(name, envValue string, allowGenerate bool, dataDir string) (string, error) {
	if strings.TrimSpace(envValue) != "" {
		return requireUserSecret(name, envValue)
	}

	if !allowGenerate {
		_, err := requireUserSecret(name, envValue)
		return "", fmt.Errorf("%w（Worker 节点必须与 Master 使用相同的密钥）", err)
	}

	// 持久化文件里已有就直接用，保证重启后密钥稳定
	if secret := readPersistedSecret(dataDir, name); secret != "" {
		return secret, nil
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("生成 %s 失败: %w", name, err)
	}
	secret := hex.EncodeToString(raw)
	if err := persistSecret(dataDir, name, secret); err != nil {
		// 写不进去不至于起不来，但要明确告知重启即失效
		log.Printf("警告: %s 无法持久化到 %s（%v），本次使用随机密钥，重启后失效", name, dataDir, err)
		return secret, nil
	}
	log.Printf("首次启动：已自动生成 %s 并保存到 %s（cat 该文件可查看，worker 配置需要它）",
		name, filepath.Join(dataDir, secretsFileName))
	return secret, nil
}

const secretsFileName = "secrets.env"

// readPersistedSecret 从 dataDir/secrets.env 读取指定密钥；文件不存在/无此项返回空串。
func readPersistedSecret(dataDir, name string) string {
	raw, err := os.ReadFile(filepath.Join(dataDir, secretsFileName))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && k == name {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// persistSecret 把密钥追加写入 dataDir/secrets.env（0600，属主可读写的最小权限）。
// 与 JWT/NODE 两个密钥共用同一文件，已存在的键不重复写。
func persistSecret(dataDir, name, secret string) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dataDir, secretsFileName)
	// 已存在则跳过（并发/重复调用安全）
	if readPersistedSecret(dataDir, name) != "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	// 已存在文件权限可能过宽时收紧
	_ = os.Chmod(path, 0600)
	_, err = fmt.Fprintf(f, "%s=%s\n", name, secret)
	return err
}
