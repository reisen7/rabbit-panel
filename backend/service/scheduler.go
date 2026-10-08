package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"

	"rabbit-panel/model"
	"rabbit-panel/repository"
)

// Scheduler 容器调度器
type Scheduler struct {
	nodeService *NodeService
	dockerRepo  repository.IDockerRepository
	registry    *RegistryService
	nodeSecret  string
}

// NewScheduler 创建调度器
func NewScheduler(ns *NodeService, dr repository.IDockerRepository, nodeSecret string, registry *RegistryService) *Scheduler {
	return &Scheduler{
		nodeService: ns,
		dockerRepo:  dr,
		registry:    registry,
		nodeSecret:  nodeSecret,
	}
}

// ScheduleContainer 调度容器到最佳节点
func (s *Scheduler) ScheduleContainer(ctx context.Context, req *model.ScheduleRequest) (*model.ScheduleResponse, error) {
	var targetNode *model.NodeInfo
	var err error

	if strings.TrimSpace(req.NodeID) != "" {
		var ok bool
		targetNode, ok = s.nodeService.GetNode(req.NodeID)
		if !ok {
			return nil, fmt.Errorf("node %s not found", req.NodeID)
		}
	} else {
		targetNode, err = s.nodeService.SelectBestNode()
		if err != nil {
			return nil, err
		}
	}

	// If local (Master) or no nodes, create locally
	if targetNode.Mode == model.ModeMaster {
		return s.createLocalContainer(ctx, req, targetNode)
	}

	// Forward to worker node
	return s.forwardToWorker(ctx, targetNode.Address, req)
}

// createLocalContainer 在本地创建容器
func (s *Scheduler) createLocalContainer(ctx context.Context, req *model.ScheduleRequest, node *model.NodeInfo) (*model.ScheduleResponse, error) {
	_, _, err := s.dockerRepo.ImageInspectWithRaw(ctx, req.Image)
	if err != nil {
		opts := types.ImagePullOptions{}
		if s.registry != nil {
			opts = s.registry.PullOptions(req.Image)
		}
		reader, pullErr := s.dockerRepo.ImagePull(ctx, req.Image, opts)
		if pullErr != nil {
			return nil, pullErr
		}
		defer reader.Close()
		_, _ = bytes.NewBuffer(nil).ReadFrom(reader)
	}

	containerConfig := &container.Config{
		Image: req.Image,
	}

	for key, value := range req.Env {
		containerConfig.Env = append(containerConfig.Env, fmt.Sprintf("%s=%s", key, value))
	}

	hostConfig := &container.HostConfig{}
	if len(req.Ports) > 0 {
		portBindings := nat.PortMap{}
		exposedPorts := nat.PortSet{}
		for _, mapping := range req.Ports {
			parts := strings.Split(mapping, ":")
			if len(parts) != 2 {
				continue
			}
			hostPort := strings.TrimSpace(parts[0])
			containerPort := strings.TrimSpace(parts[1])
			if hostPort == "" || containerPort == "" {
				continue
			}
			port := nat.Port(containerPort + "/tcp")
			exposedPorts[port] = struct{}{}
			portBindings[port] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: hostPort}}
		}
		containerConfig.ExposedPorts = exposedPorts
		hostConfig.PortBindings = portBindings
	}

	resp, err := s.dockerRepo.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, req.Name)
	if err != nil {
		return nil, err
	}

	if err := s.dockerRepo.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = s.dockerRepo.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, err
	}

	return &model.ScheduleResponse{
		Status:   "success",
		NodeID:   node.ID,
		NodeName: node.Name,
		Container: map[string]interface{}{
			"status": "running",
			"id":     resp.ID[:12],
			"name":   req.Name,
		},
	}, nil
}

// forwardToWorker 转发请求到 Worker 节点
func (s *Scheduler) forwardToWorker(ctx context.Context, workerAddr string, req *model.ScheduleRequest) (*model.ScheduleResponse, error) {
	containerConfig := map[string]interface{}{
		"image": req.Image,
		"name":  req.Name,
		"ports": req.Ports,
		"env":   req.Env,
	}

	jsonData, _ := json.Marshal(containerConfig)
	workerURL := fmt.Sprintf("http://%s/api/containers/create", workerAddr)

	httpReq, _ := http.NewRequest("POST", workerURL, bytes.NewBuffer(jsonData))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Node-ID", "master")
	httpReq.Header.Set("X-Node-Token", s.generateNodeToken("master"))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		if message, ok := errResp["error"].(string); ok && message != "" {
			return nil, fmt.Errorf("worker request failed: %s", message)
		}
		return nil, fmt.Errorf("worker request failed with status %s", resp.Status)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &model.ScheduleResponse{
		Status:   "success",
		NodeID:   "worker",
		NodeName: "Worker",
		Container: result,
	}, nil
}

// generateNodeToken generates a node authentication token using HMAC-SHA256
func (s *Scheduler) generateNodeToken(nodeID string) string {
	h := sha256.New()
	h.Write([]byte(nodeID))
	h.Write([]byte(s.nodeSecret))
	return hex.EncodeToString(h.Sum(nil))
}
