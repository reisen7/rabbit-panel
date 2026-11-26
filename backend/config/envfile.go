package config

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// envFilePath 环境变量文件的唯一约定位置：数据目录下的 secrets.env。
// 选这里而不是 /etc 的原因：
//   - Docker 部署时 data 目录本来就必须卷挂载（auth.db 在内），宿主机编辑容器即生效；
//     /etc 在容器内是易失层，重建即丢
//   - Windows 无 /etc 概念，./data 全平台行为一致
//   - 该文件同时是程序自动生成密钥的持久化位置（追加写，不覆盖用户配置），
//     一个文件覆盖「程序写」与「人写」两类配置
//
// 注意路径跟随进程工作目录（与 ./data 整体行为一致），应在固定目录启动。
var envFilePath = filepath.Join(".", "data", secretsFileName)

// loadEnvFile 从 data/secrets.env 读取 KEY=VALUE 配置并注入进程环境。
// 仅设置当前环境中不存在的键——显式传入的环境变量（含 docker -e）永远优先。
// 必须在读取任何配置之前调用。
func loadEnvFile() {
	data, err := os.ReadFile(envFilePath)
	if err != nil {
		return
	}
	if loaded := applyEnvLines(string(data)); loaded > 0 {
		log.Printf("已加载环境变量文件 %s（%d 项）", envFilePath, loaded)
	}
}

// applyEnvLines 解析 KEY=VALUE 行并注入缺失的环境变量，返回设置的条目数。
// 支持空行、# 注释行、export 前缀与成对的单双引号值；非法行静默跳过。
func applyEnvLines(content string) int {
	loaded := 0
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !validEnvKey(key) {
			continue
		}
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if os.Setenv(key, value) == nil {
			loaded++
		}
	}
	return loaded
}

func validEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, c := range key {
		if c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			continue
		}
		if c >= '0' && c <= '9' && i > 0 {
			continue
		}
		return false
	}
	return true
}
