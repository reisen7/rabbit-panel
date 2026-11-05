package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// dockerHostFromContext 返回 Docker CLI 当前 context 的引擎地址。
// 已设置 DOCKER_HOST 时返回空串，交给 client.FromEnv。
// Windows 上 Docker Desktop 的当前 context 通常是 npipe:////./pipe/dockerDesktopLinuxEngine；
// Go 客户端的默认值却是需要管理员权限的 docker_engine 管道。
func dockerHostFromContext() string {
	if strings.TrimSpace(os.Getenv("DOCKER_HOST")) != "" {
		return ""
	}
	dir := dockerConfigDir()
	if dir == "" {
		return ""
	}
	name := currentDockerContext(filepath.Join(dir, "config.json"))
	if name == "" || name == "default" {
		return ""
	}
	return dockerContextHost(filepath.Join(dir, "contexts", "meta"), name)
}

func dockerConfigDir() string {
	if dir := strings.TrimSpace(os.Getenv("DOCKER_CONFIG")); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".docker")
}

func currentDockerContext(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var cfg struct {
		CurrentContext string `json:"currentContext"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("读取 Docker 配置失败 %s: %v", path, err)
		return ""
	}
	return strings.TrimSpace(cfg.CurrentContext)
}

func dockerContextHost(metaDir, name string) string {
	entries, err := os.ReadDir(metaDir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(metaDir, entry.Name(), "meta.json"))
		if err != nil {
			continue
		}
		var meta struct {
			Name      string `json:"Name"`
			Endpoints map[string]struct {
				Host string `json:"Host"`
			} `json:"Endpoints"`
		}
		if err := json.Unmarshal(data, &meta); err != nil || meta.Name != name {
			continue
		}
		if ep, ok := meta.Endpoints["docker"]; ok {
			return strings.TrimSpace(ep.Host)
		}
	}
	return ""
}
