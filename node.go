package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
)

// 节点模式
const (
	ModeMaster = "master"
	ModeWorker = "worker"
)

// 节点状态
const (
	NodeStatusOnline  = "online"
	NodeStatusOffline = "offline"
	NodeStatusError   = "error"
)

// 节点信息
type NodeInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Address     string    `json:"address"`     // 节点地址 (IP:Port)
	Mode        string    `json:"mode"`         // master 或 worker
	Status      string    `json:"status"`      // online, offline, error
	CPU         float64   `json:"cpu"`         // CPU 使用率
	Memory      float64   `json:"memory"`       // 内存使用率
	Disk        float64   `json:"disk"`        // 磁盘使用率
	Containers  int       `json:"containers"`  // 容器数量
	LastSeen    time.Time `json:"last_seen"`   // 最后心跳时间
	Labels      map[string]string `json:"labels"` // 节点标签
}

// 节点管理器（Master 节点使用）
type NodeManager struct {
	sync.RWMutex
	nodes map[string]*NodeInfo // nodeID -> NodeInfo
	mode  string               // master 或 worker
}

var nodeManager *NodeManager

// 初始化节点管理器
func initNodeManager(mode string) {
	nodeManager = &NodeManager{
		nodes: make(map[string]*NodeInfo),
		mode:  mode,
	}
	
	if mode == ModeMaster {
		// Master 节点：启动节点管理服务
		go nodeManager.startHealthCheck()
		log.Printf("节点管理器已启动 (Master 模式)")
	} else {
		log.Printf("节点管理器已启动 (Worker 模式)")
	}
}

// 注册节点（Worker 向 Master 注册）
func (nm *NodeManager) RegisterNode(node *NodeInfo) error {
	nm.Lock()
	defer nm.Unlock()
	
	node.LastSeen = time.Now()
	node.Status = NodeStatusOnline
	nm.nodes[node.ID] = node
	
	log.Printf("节点已注册: %s (%s) - %s", node.Name, node.ID, node.Address)
	return nil
}

// 更新节点状态
func (nm *NodeManager) UpdateNodeStatus(nodeID string, status string) {
	nm.Lock()
	defer nm.Unlock()
	
	if node, exists := nm.nodes[nodeID]; exists {
		node.Status = status
		node.LastSeen = time.Now()
	}
}

// 更新节点资源信息
func (nm *NodeManager) UpdateNodeResources(nodeID string, cpu, memory, disk float64, containers int) {
	nm.Lock()
	defer nm.Unlock()
	
	if node, exists := nm.nodes[nodeID]; exists {
		node.CPU = cpu
		node.Memory = memory
		node.Disk = disk
		node.Containers = containers
		node.LastSeen = time.Now()
	}
}

// 获取所有节点
func (nm *NodeManager) GetAllNodes() []*NodeInfo {
	nm.RLock()
	defer nm.RUnlock()
	
	nodes := make([]*NodeInfo, 0, len(nm.nodes))
	for _, node := range nm.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// 根据 ID 获取节点
func (nm *NodeManager) GetNode(nodeID string) (*NodeInfo, bool) {
	nm.RLock()
	defer nm.RUnlock()
	
	node, exists := nm.nodes[nodeID]
	return node, exists
}

// 选择最佳节点（调度算法）
func (nm *NodeManager) SelectBestNode() (*NodeInfo, error) {
	nm.RLock()
	defer nm.RUnlock()
	
	var bestNode *NodeInfo
	minLoad := 100.0
	
	for _, node := range nm.nodes {
		if node.Status != NodeStatusOnline {
			continue
		}
		
		// 简单的负载计算：CPU + Memory
		load := (node.CPU + node.Memory) / 2
		if load < minLoad {
			minLoad = load
			bestNode = node
		}
	}
	
	if bestNode == nil {
		return nil, fmt.Errorf("没有可用的在线节点")
	}
	
	return bestNode, nil
}

// 健康检查（定期检查节点状态）
func (nm *NodeManager) startHealthCheck() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		nm.checkNodeHealth()
	}
}

// 检查节点健康状态
func (nm *NodeManager) checkNodeHealth() {
	nm.Lock()
	defer nm.Unlock()
	
	now := time.Now()
	for _, node := range nm.nodes {
		// 如果超过 30 秒没有心跳，标记为离线
		if now.Sub(node.LastSeen) > 30*time.Second {
			if node.Status == NodeStatusOnline {
				node.Status = NodeStatusOffline
				log.Printf("节点离线: %s (%s)", node.Name, node.ID)
			}
		}
	}
}

// ========== HTTP API Handlers ==========

// 获取所有节点列表（Master）
func handleNodesList(w http.ResponseWriter, r *http.Request) {
	if nodeManager == nil || nodeManager.mode != ModeMaster {
		http.Error(w, "当前节点不是 Master 模式", http.StatusBadRequest)
		return
	}
	
	nodes := nodeManager.GetAllNodes()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
}

// 节点注册 API（Worker 向 Master 注册）
func handleNodeRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}
	
	if nodeManager == nil || nodeManager.mode != ModeMaster {
		http.Error(w, "当前节点不是 Master 模式", http.StatusBadRequest)
		return
	}
	
	var node NodeInfo
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}
	
	if err := nodeManager.RegisterNode(&node); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// 节点心跳 API（Worker 向 Master 发送心跳）
func handleNodeHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}
	
	if nodeManager == nil || nodeManager.mode != ModeMaster {
		http.Error(w, "当前节点不是 Master 模式", http.StatusBadRequest)
		return
	}
	
	var req struct {
		NodeID     string  `json:"node_id"`
		CPU        float64 `json:"cpu"`
		Memory     float64 `json:"memory"`
		Disk       float64 `json:"disk"`
		Containers int     `json:"containers"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}
	
	nodeManager.UpdateNodeResources(req.NodeID, req.CPU, req.Memory, req.Disk, req.Containers)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// Worker 节点：向 Master 发送心跳
func sendHeartbeatToMaster(masterURL string, nodeID string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		// 获取当前节点资源信息
		cpu, _ := getCPUUsage()
		memory, _ := getMemoryUsage()
		disk, _ := getDiskUsage()
		
		// 获取容器数量
		containers, err := dockerClient.ContainerList(context.Background(), types.ContainerListOptions{All: true})
		containerCount := 0
		if err == nil {
			containerCount = len(containers)
		}
		
		// 生成节点认证 Token
		nodeToken := generateNodeToken(nodeID)
		
		// 发送心跳
		req := map[string]interface{}{
			"node_id":    nodeID,
			"cpu":        cpu,
			"memory":     memory,
			"disk":       disk,
			"containers": containerCount,
		}
		
		jsonData, _ := json.Marshal(req)
		httpReq, _ := http.NewRequest("POST", masterURL+"/api/nodes/heartbeat", strings.NewReader(string(jsonData)))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("X-Node-ID", nodeID)
		httpReq.Header.Set("X-Node-Token", nodeToken)
		
		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			log.Printf("发送心跳失败: %v", err)
			continue
		}
		resp.Body.Close()
	}
}

// Worker 节点：向 Master 注册
func registerToMaster(masterURL string, nodeID, nodeName, nodeAddress string) error {
	node := NodeInfo{
		ID:      nodeID,
		Name:    nodeName,
		Address: nodeAddress,
		Mode:    ModeWorker,
		Status:  NodeStatusOnline,
		Labels:  make(map[string]string),
	}
	
	// 生成节点认证 Token
	nodeToken := generateNodeToken(nodeID)
	
	jsonData, _ := json.Marshal(node)
	httpReq, err := http.NewRequest("POST", masterURL+"/api/nodes/register", strings.NewReader(string(jsonData)))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Node-ID", nodeID)
	httpReq.Header.Set("X-Node-Token", nodeToken)
	
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("注册失败: %d", resp.StatusCode)
	}
	
	log.Printf("已向 Master 注册: %s", masterURL)
	return nil
}

