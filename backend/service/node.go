package service

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"

	"rabbit-panel/model"
	"rabbit-panel/repository"
	"rabbit-panel/tool"
)

// NodeService 节点服务
type NodeService struct {
	dockerRepo repository.IDockerRepository
	cacheRepo  repository.ICacheRepository
	mode       string
	nodeSecret string
	nodeID     string
	host       string
	port       string

	mu    sync.RWMutex
	nodes map[string]*model.NodeInfo
}

// NewNodeService 创建节点服务
func NewNodeService(dr repository.IDockerRepository, cr repository.ICacheRepository, mode, nodeSecret, nodeID, host, port string) *NodeService {
	service := &NodeService{
		dockerRepo: dr,
		cacheRepo:  cr,
		mode:       mode,
		nodeSecret: nodeSecret,
		nodeID:     nodeID,
		host:       host,
		port:       port,
		nodes:      make(map[string]*model.NodeInfo),
	}
	service.ensureLocalNode()
	return service
}

// RegisterNode 注册节点
func (s *NodeService) RegisterNode(node *model.NodeInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node.LastSeen = time.Now()
	node.Status = model.NodeStatusOnline
	s.nodes[node.ID] = node
}

// UpdateHeartbeat 更新节点心跳。节点尚未注册时忽略，等待下一次注册。
func (s *NodeService) UpdateHeartbeat(nodeID string, resources model.SystemStats, containers int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.nodes[nodeID]
	if !ok {
		return
	}
	node.CPU = resources.CPU
	node.Memory = resources.Memory
	node.Disk = resources.Disk
	node.Containers = containers
	node.LastSeen = time.Now()
	node.Status = model.NodeStatusOnline
}

// GetAllNodes 获取所有节点
func (s *NodeService) GetAllNodes() []*model.NodeInfo {
	s.ensureLocalNode()
	s.refreshNodeStatus()
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*model.NodeInfo, 0, len(s.nodes))
	for _, n := range s.nodes {
		n.LastSeenStr = n.LastSeen.Format("2006-01-02 15:04:05")
		result = append(result, n)
	}
	return result
}

// ProxyTarget 返回需要转发到的 Worker 地址。本机节点返回空地址。
func (s *NodeService) ProxyTarget(nodeID string) (string, error) {
	if nodeID == "" || nodeID == s.nodeID {
		return "", nil
	}
	s.refreshNodeStatus()
	s.mu.RLock()
	defer s.mu.RUnlock()
	node, ok := s.nodes[nodeID]
	if !ok {
		return "", fmt.Errorf("节点不存在")
	}
	if node.ID == s.nodeID || node.Mode == model.ModeMaster {
		return "", nil
	}
	if node.Status != model.NodeStatusOnline {
		return "", fmt.Errorf("节点离线")
	}
	if strings.TrimSpace(node.Address) == "" {
		return "", fmt.Errorf("节点地址为空")
	}
	return node.Address, nil
}

// FindNode 按 ID、名称或地址查找节点。
func (s *NodeService) FindNode(hint string) (*model.NodeInfo, error) {
	hint = strings.TrimSpace(hint)
	if hint == "" {
		return nil, fmt.Errorf("节点名称为空")
	}
	s.ensureLocalNode()
	s.refreshNodeStatus()
	s.mu.RLock()
	defer s.mu.RUnlock()

	if n, ok := s.nodes[hint]; ok {
		cp := *n
		return &cp, nil
	}
	var matches []*model.NodeInfo
	for _, n := range s.nodes {
		if strings.EqualFold(n.Name, hint) || strings.EqualFold(n.ID, hint) || strings.EqualFold(n.Address, hint) {
			cp := *n
			matches = append(matches, &cp)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("节点名称不唯一: %s", hint)
	}
	names := make([]string, 0, len(s.nodes))
	for _, n := range s.nodes {
		if strings.TrimSpace(n.Name) != "" {
			names = append(names, n.Name)
			continue
		}
		names = append(names, n.ID)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("节点不存在: %s", hint)
	}
	return nil, fmt.Errorf("节点不存在: %s。可用节点: %s", hint, strings.Join(names, "、"))
}

// GetNode 获取单个节点
func (s *NodeService) GetNode(nodeID string) (*model.NodeInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	node, ok := s.nodes[nodeID]
	if ok {
		node.LastSeenStr = node.LastSeen.Format("2006-01-02 15:04:05")
	}
	return node, ok
}

// RemoveNode 移除节点
func (s *NodeService) RemoveNode(nodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.nodes, nodeID)
}

// SelectBestNode 选择最佳节点（CPU 和内存负载最低）
func (s *NodeService) SelectBestNode() (*model.NodeInfo, error) {
	s.ensureLocalNode()
	s.refreshNodeStatus()
	s.mu.RLock()
	defer s.mu.RUnlock()
	var best *model.NodeInfo
	minLoad := 100.0
	for _, n := range s.nodes {
		if n.Status != model.NodeStatusOnline {
			continue
		}
		load := (n.CPU + n.Memory) / 2
		if load < minLoad {
			minLoad = load
			best = n
		}
	}
	if best == nil {
		return nil, ErrNoAvailableNodes
	}
	return best, nil
}

// GetMode 获取运行模式
func (s *NodeService) GetMode() string {
	return s.mode
}

// localMasterName 是主节点在界面上的名称。容器里主机名通常是容器 ID，不适合展示。
func localMasterName() string {
	name := strings.TrimSpace(os.Getenv("NODE_NAME"))
	if name == "" {
		return "master"
	}
	return name
}

func (s *NodeService) ensureLocalNode() {
	if s.mode != "master" {
		return
	}

	address := s.host
	if address == "" || address == "0.0.0.0" {
		address = "127.0.0.1"
	}
	if s.port != "" {
		address = address + ":" + s.port
	}

	cpu, _ := tool.GetCPUUsage()
	mem, _ := tool.GetMemoryUsage()
	disk, _ := tool.GetDiskUsage()
	containers, _ := s.dockerRepo.ContainerList(context.Background(), types.ContainerListOptions{All: true})

	s.mu.Lock()
	defer s.mu.Unlock()

	node, ok := s.nodes[s.nodeID]
	if !ok {
		node = &model.NodeInfo{
			ID:      s.nodeID,
			Address: address,
			Mode:    model.ModeMaster,
			Labels:  map[string]string{},
		}
		s.nodes[s.nodeID] = node
	}
	node.Name = localMasterName()

	node.CPU = cpu
	node.Memory = mem.Usage
	node.Disk = disk.Usage
	node.Containers = len(containers)
	node.LastSeen = time.Now()
	node.Status = model.NodeStatusOnline
}

const nodeOfflineAfter = 30 * time.Second

// refreshNodeStatus 将长时间没有心跳的远程节点标为离线。
func (s *NodeService) refreshNodeStatus() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for _, n := range s.nodes {
		if n.ID == s.nodeID {
			continue
		}
		if now.Sub(n.LastSeen) > nodeOfflineAfter {
			n.Status = model.NodeStatusOffline
		}
	}
}

var ErrNoAvailableNodes = &NodeError{"no available nodes"}

type NodeError struct {
	msg string
}

func (e *NodeError) Error() string {
	return e.msg
}
