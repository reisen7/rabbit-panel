package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

// 容器调度请求
type ScheduleRequest struct {
	Image       string            `json:"image"`
	Name        string            `json:"name"`
	Ports       map[string]string `json:"ports"`       // "8080:80" 格式
	Env         map[string]string `json:"env"`         // 环境变量
	Labels      map[string]string `json:"labels"`     // 标签
	NodeID      string            `json:"node_id"`     // 指定节点（可选）
	Constraints map[string]string `json:"constraints"` // 调度约束（可选）
}

// 跨节点创建容器（调度）
func handleContainerSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	if nodeManager == nil || nodeManager.mode != ModeMaster {
		http.Error(w, "当前节点不是 Master 模式", http.StatusBadRequest)
		return
	}

	var req ScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}

	// 选择目标节点
	var targetNode *NodeInfo
	var err error

	if req.NodeID != "" {
		// 指定了节点 ID
		var exists bool
		targetNode, exists = nodeManager.GetNode(req.NodeID)
		if !exists || targetNode == nil {
			http.Error(w, fmt.Sprintf("节点不存在: %s", req.NodeID), http.StatusBadRequest)
			return
		}
		if targetNode.Status != NodeStatusOnline {
			http.Error(w, fmt.Sprintf("节点不在线: %s", req.NodeID), http.StatusBadRequest)
			return
		}
	} else {
		// 自动选择最佳节点
		targetNode, err = nodeManager.SelectBestNode()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	log.Printf("调度容器到节点: %s (%s)", targetNode.Name, targetNode.Address)

	// 调用目标节点的 API 创建容器
	containerConfig := map[string]interface{}{
		"image": req.Image,
		"name":  req.Name,
		"ports": req.Ports,
		"env":   req.Env,
		"labels": req.Labels,
	}

	jsonData, _ := json.Marshal(containerConfig)
	workerURL := fmt.Sprintf("http://%s/api/containers/create", targetNode.Address)
	
	// 生成节点认证 Token（使用 Master 的节点 ID，这里使用 "master" 作为标识）
	masterNodeID := "master"
	nodeToken := generateNodeToken(masterNodeID)
	
	httpReq, err := http.NewRequest("POST", workerURL, bytes.NewBuffer(jsonData))
	if err != nil {
		http.Error(w, fmt.Sprintf("创建请求失败: %v", err), http.StatusInternalServerError)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Node-ID", masterNodeID)
	httpReq.Header.Set("X-Node-Token", nodeToken)
	
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("调用 Worker 节点失败: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		http.Error(w, fmt.Sprintf("Worker 节点错误: %s", string(body)), resp.StatusCode)
		return
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "success",
		"node_id":  targetNode.ID,
		"node":     targetNode.Name,
		"container": result,
	})
}

// 在 Worker 节点创建容器（供 Master 调用）
func handleContainerCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Image  string            `json:"image"`
		Name   string            `json:"name"`
		Ports  map[string]string `json:"ports"`
		Env    map[string]string `json:"env"`
		Labels map[string]string `json:"labels"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}

	// 构建环境变量
	env := make([]string, 0, len(req.Env))
	for k, v := range req.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// 构建端口映射
	portBindings := make(nat.PortMap)
	exposedPorts := make(nat.PortSet)
	
	for hostPort, containerPort := range req.Ports {
		// 解析 "8080:80" 格式
		parts := strings.Split(containerPort, ":")
		if len(parts) == 2 {
			port, err := nat.NewPort("tcp", parts[1])
			if err != nil {
				http.Error(w, fmt.Sprintf("无效的端口: %s", parts[1]), http.StatusBadRequest)
				return
			}
			exposedPorts[port] = struct{}{}
			portBindings[port] = []nat.PortBinding{
				{
					HostPort: hostPort,
				},
			}
		}
	}

	// 创建容器配置
	config := &container.Config{
		Image:        req.Image,
		Env:          env,
		ExposedPorts: exposedPorts,
		Labels:       req.Labels,
	}

	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
	}

	// 创建容器
	ctx := context.Background()
	createResp, err := dockerClient.ContainerCreate(ctx, config, hostConfig, nil, nil, req.Name)
	if err != nil {
		http.Error(w, fmt.Sprintf("创建容器失败: %v", err), http.StatusInternalServerError)
		return
	}

	// 启动容器
	if err := dockerClient.ContainerStart(ctx, createResp.ID, types.ContainerStartOptions{}); err != nil {
		http.Error(w, fmt.Sprintf("启动容器失败: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"id":     createResp.ID,
		"name":   req.Name,
	})
}

// 获取所有节点的容器列表
func handleAllContainers(w http.ResponseWriter, r *http.Request) {
	if nodeManager == nil || nodeManager.mode != ModeMaster {
		http.Error(w, "当前节点不是 Master 模式", http.StatusBadRequest)
		return
	}

	nodes := nodeManager.GetAllNodes()
	allContainers := make([]map[string]interface{}, 0)

	// 获取本地容器
	localContainers, _ := dockerClient.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	for _, c := range localContainers {
		allContainers = append(allContainers, map[string]interface{}{
			"node_id": "local",
			"node":    "本地节点",
			"id":      c.ID[:12],
			"name":    c.Names[0],
			"image":   c.Image,
			"status":  c.Status,
			"state":   c.State,
		})
	}

	// 获取所有 Worker 节点的容器
	for _, node := range nodes {
		if node.Status != NodeStatusOnline {
			continue
		}

		// 调用 Worker 节点的 API（需要用户认证，这里通过节点认证）
		workerURL := fmt.Sprintf("http://%s/api/containers", node.Address)
		
		// 生成节点认证 Token
		masterNodeID := "master"
		nodeToken := generateNodeToken(masterNodeID)
		
		httpReq, err := http.NewRequest("GET", workerURL, nil)
		if err != nil {
			log.Printf("创建请求失败: %v", err)
			continue
		}
		httpReq.Header.Set("X-Node-ID", masterNodeID)
		httpReq.Header.Set("X-Node-Token", nodeToken)
		
		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			log.Printf("获取节点 %s 容器列表失败: %v", node.Name, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var containers []ContainerInfo
			if err := json.NewDecoder(resp.Body).Decode(&containers); err == nil {
				for _, c := range containers {
					allContainers = append(allContainers, map[string]interface{}{
						"node_id": node.ID,
						"node":    node.Name,
						"id":      c.ID,
						"name":    c.Name,
						"image":   c.Image,
						"status":  c.Status,
						"state":   c.State,
					})
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(allContainers)
}

