package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"

	"rabbit-panel/model"
	"rabbit-panel/tool"
)

const workerReportInterval = 10 * time.Second

// StartWorker 在 Worker 模式下向 Master 注册并持续发送心跳。
// MASTER_URL 为空时无法加入集群。NODE_ADDRESS 是 Master 能访问到的地址（宿主机 IP:端口）。
func (s *NodeService) StartWorker(masterURL, nodeName, advertiseAddr string) {
	if s.mode != model.ModeWorker {
		return
	}
	masterURL = strings.TrimRight(strings.TrimSpace(masterURL), "/")
	if masterURL == "" {
		log.Println("Worker 模式未设置 MASTER_URL，不会向 Master 注册")
		return
	}
	if !strings.Contains(masterURL, "://") {
		masterURL = "http://" + masterURL
	}
	go s.workerLoop(masterURL, strings.TrimSpace(nodeName), strings.TrimSpace(advertiseAddr))
}

func (s *NodeService) workerLoop(masterURL, nodeName, advertiseAddr string) {
	log.Printf("Worker 开始连接 Master: %s", masterURL)
	client := &http.Client{Timeout: 8 * time.Second}
	registered := false

	report := func() {
		addr, err := s.resolveAdvertiseAddr(masterURL, advertiseAddr)
		if err != nil {
			log.Printf("Worker 无法确定对外地址: %v", err)
			registered = false
			return
		}
		if err := s.registerWithMaster(client, masterURL, nodeName, addr); err != nil {
			log.Printf("Worker 注册失败: %v", err)
			registered = false
			return
		}
		if err := s.heartbeatToMaster(client, masterURL); err != nil {
			log.Printf("Worker 心跳失败: %v", err)
			registered = false
			return
		}
		if !registered {
			log.Printf("Worker 已注册到 Master，节点 %s，地址 %s", s.displayName(nodeName), addr)
			registered = true
		}
	}

	report()
	ticker := time.NewTicker(workerReportInterval)
	defer ticker.Stop()
	for range ticker.C {
		report()
	}
}

func (s *NodeService) displayName(nodeName string) string {
	if nodeName != "" {
		return nodeName
	}
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return s.nodeID
	}
	return hostname
}

func (s *NodeService) resolveAdvertiseAddr(masterURL, advertiseAddr string) (string, error) {
	if advertiseAddr != "" {
		if !strings.Contains(advertiseAddr, ":") && s.port != "" {
			return advertiseAddr + ":" + s.port, nil
		}
		return advertiseAddr, nil
	}

	ip := detectOutboundIP(masterURL)
	if ip == "" {
		return "", fmt.Errorf("未设置 NODE_ADDRESS，且无法探测本机 IP。Docker 部署请设置为宿主机 IP:%s", s.port)
	}
	if s.port == "" {
		return ip, nil
	}
	return net.JoinHostPort(ip, s.port), nil
}

func detectOutboundIP(masterURL string) string {
	target, err := masterDialAddr(masterURL)
	if err != nil {
		return ""
	}
	conn, err := net.DialTimeout("udp", target, 2*time.Second)
	if err != nil {
		return ""
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP == nil || addr.IP.IsLoopback() {
		return ""
	}
	return addr.IP.String()
}

func masterDialAddr(masterURL string) (string, error) {
	parsed, err := url.Parse(masterURL)
	if err != nil {
		return "", err
	}
	host := parsed.Host
	if host == "" {
		return "", fmt.Errorf("invalid master url")
	}
	if _, _, err := net.SplitHostPort(host); err != nil {
		port := "80"
		if parsed.Scheme == "https" {
			port = "443"
		}
		host = net.JoinHostPort(host, port)
	}
	return host, nil
}

func (s *NodeService) registerWithMaster(client *http.Client, masterURL, nodeName, addr string) error {
	cpu, mem, disk, containers := s.localStats()
	node := model.NodeInfo{
		ID:         s.nodeID,
		Name:       s.displayName(nodeName),
		Address:    addr,
		Mode:       model.ModeWorker,
		CPU:        cpu,
		Memory:     mem,
		Disk:       disk,
		Containers: containers,
		Labels:     map[string]string{},
	}
	return s.postMaster(client, masterURL, "/api/nodes/register", node)
}

func (s *NodeService) heartbeatToMaster(client *http.Client, masterURL string) error {
	cpu, mem, disk, containers := s.localStats()
	body := map[string]interface{}{
		"node_id":    s.nodeID,
		"cpu":        cpu,
		"memory":     mem,
		"disk":       disk,
		"containers": containers,
	}
	return s.postMaster(client, masterURL, "/api/nodes/heartbeat", body)
}

func (s *NodeService) localStats() (cpu, memory, disk float64, containers int) {
	cpu, _ = tool.GetCPUUsage()
	mem, _ := tool.GetMemoryUsage()
	diskInfo, _ := tool.GetDiskUsage()
	list, err := s.dockerRepo.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err == nil {
		containers = len(list)
	}
	return cpu, mem.Usage, diskInfo.Usage, containers
}

func (s *NodeService) postMaster(client *http.Client, masterURL, path string, body interface{}) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, masterURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-ID", s.nodeID)
	req.Header.Set("X-Node-Token", nodeAuthToken(s.nodeID, s.nodeSecret))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		text := strings.TrimSpace(string(msg))
		if text == "" {
			text = resp.Status
		}
		return fmt.Errorf("%s %s", path, text)
	}
	return nil
}

// nodeAuthToken 与 middleware.verifyNodeToken 使用同一算法：SHA256(nodeID + secret)。
func nodeAuthToken(nodeID, secret string) string {
	h := sha256.New()
	h.Write([]byte(nodeID))
	h.Write([]byte(secret))
	return hex.EncodeToString(h.Sum(nil))
}
