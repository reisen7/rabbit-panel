package exec

import (
	"fmt"
	"time"
	"strings"
	"bytes"
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/docker/docker/pkg/stdcopy"

	"rabbit-panel/repository"
)

// TerminalService 终端服务
type TerminalService struct {
	dockerRepo repository.IDockerRepository
}

// NewTerminalService 创建终端服务
func NewTerminalService(dr repository.IDockerRepository) *TerminalService {
	return &TerminalService{dockerRepo: dr}
}

// ExecCommand 在容器中执行命令
func (s *TerminalService) ExecCommand(ctx context.Context, containerID string, cmd []string) (string, int, error) {
	// Create exec
	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	}

	execID, err := s.dockerRepo.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", -1, err
	}

	resp, err := s.dockerRepo.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		return "", -1, err
	}
	defer resp.Close()

	var stdout, stderr bytes.Buffer
	_, copyErr := stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
	if copyErr != nil && copyErr != io.EOF {
		return "", -1, copyErr
	}

	// Wait for completion
	inspect, err := s.dockerRepo.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return "", -1, err
	}

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	return output, inspect.ExitCode, nil
}

// GetContainerFile 获取容器文件内容
func (s *TerminalService) GetContainerFile(ctx context.Context, containerID, path string) (io.ReadCloser, types.ContainerPathStat, error) {
	return s.dockerRepo.CopyFromContainer(ctx, containerID, path)
}

// PutContainerFile 上传文件到容器
func (s *TerminalService) PutContainerFile(ctx context.Context, containerID, path string, content io.Reader) error {
	return s.dockerRepo.CopyToContainer(ctx, containerID, path, content, types.CopyToContainerOptions{})
}

// GetContainerInspect 获取容器详情
func (s *TerminalService) GetContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	return s.dockerRepo.ContainerInspect(ctx, containerID)
}

// UpdateContainer 更新容器配置
func (s *TerminalService) UpdateContainer(ctx context.Context, containerID string, mem int64, cpus float64, restart string) error {
	resources := &container.Resources{}
	if mem > 0 {
		resources.Memory = mem * 1024 * 1024
		resources.MemorySwap = resources.Memory * 2
	}
	if cpus > 0 {
		resources.CPUPeriod = 100000
		resources.CPUQuota = int64(cpus * 100000)
	}

	updateConfig := container.UpdateConfig{
		Resources: *resources,
	}
	if restart != "" {
		updateConfig.RestartPolicy = container.RestartPolicy{
			Name: container.RestartPolicyMode(restart),
		}
	}

	_, err := s.dockerRepo.ContainerUpdate(ctx, containerID, updateConfig)
	return err
}

// RenameContainer 重命名容器
func (s *TerminalService) RenameContainer(ctx context.Context, containerID, newName string) error {
	return s.dockerRepo.ContainerRename(ctx, containerID, newName)
}

// RecreateContainer 重建容器
func (s *TerminalService) RecreateContainer(ctx context.Context, req *RecreateRequest) (string, error) {
	// 以原容器配置为基底，只覆盖表单schema内的字段。
	// 原实现白手起家重建配置，静默丢失 Cmd/Entrypoint/Labels/WorkingDir/
	// User/Healthcheck/日志驱动/DNS 等，重建后容器行为改变甚至无法启动。
	inspect, err := s.dockerRepo.ContainerInspect(ctx, req.ContainerID)
	if err != nil {
		return "", fmt.Errorf("读取原容器配置失败: %v", err)
	}

	containerConfig := inspect.Config
	if containerConfig == nil {
		containerConfig = &container.Config{}
	}
	if req.Image != "" {
		containerConfig.Image = req.Image
	}
	containerConfig.Tty = req.TTY

	if len(req.Env) > 0 {
		containerConfig.Env = containerConfig.Env[:0]
		for _, env := range req.Env {
			if env.Key != "" {
				containerConfig.Env = append(containerConfig.Env, env.Key+"="+env.Value)
			}
		}
	}

	hostConfig := inspect.HostConfig
	if hostConfig == nil {
		hostConfig = &container.HostConfig{}
	}
	hostConfig.Privileged = req.Privileged

	if req.Restart != "" {
		hostConfig.RestartPolicy = container.RestartPolicy{
			Name: container.RestartPolicyMode(req.Restart),
		}
	}

	if req.Memory > 0 {
		hostConfig.Resources.Memory = req.Memory
	}
	if req.CPUs > 0 {
		hostConfig.Resources.CPUPeriod = 100000
		hostConfig.Resources.CPUQuota = int64(req.CPUs * 100000)
	}

	if req.Network != "" {
		hostConfig.NetworkMode = container.NetworkMode(req.Network)
	}

	if len(req.Ports) > 0 {
		// 协议按原容器同端口的已有映射保留（UDP 不再静默变 TCP），缺省 tcp
		protoOf := func(containerPort string) string {
			if inspect.HostConfig != nil {
				for _, proto := range []string{"udp", "sctp"} {
					p := nat.Port(containerPort + "/" + proto)
					if _, ok := inspect.HostConfig.PortBindings[p]; ok {
						return proto
					}
				}
			}
			return "tcp"
		}
		portBindings := nat.PortMap{}
		exposedPorts := nat.PortSet{}
		for _, p := range req.Ports {
			if p.Host == "" || p.Container == "" {
				continue
			}
			port := nat.Port(p.Container + "/" + protoOf(p.Container))
			exposedPorts[port] = struct{}{}
			portBindings[port] = []nat.PortBinding{{
				HostIP:   "0.0.0.0",
				HostPort: p.Host,
			}}
		}
		containerConfig.ExposedPorts = exposedPorts
		hostConfig.PortBindings = portBindings
	}

	if len(req.Volumes) > 0 {
		hostConfig.Binds = hostConfig.Binds[:0]
		for _, v := range req.Volumes {
			if v.Host == "" || v.Container == "" {
				continue
			}
			hostConfig.Binds = append(hostConfig.Binds, v.Host+":"+v.Container)
		}
	}

	// 失败回滚：先把原容器改名保留，新容器创建/启动失败时还原，
	// 不再「先删后建」——创建失败（如镜像名错误）容器不至于直接消失。
	if err := s.dockerRepo.ContainerStop(ctx, req.ContainerID, nil); err != nil {
		return "", fmt.Errorf("停止原容器失败: %v", err)
	}

	origName := strings.TrimPrefix(inspect.Name, "/")
	backupName := origName + "-backup-" + time.Now().Format("20060102150405")
	if err := s.dockerRepo.ContainerRename(ctx, req.ContainerID, backupName); err != nil {
		return "", fmt.Errorf("备份原容器失败: %v", err)
	}

	newName := req.Name
	if newName == "" {
		newName = origName
	}

	resp, createErr := s.dockerRepo.ContainerCreate(ctx, containerConfig, hostConfig, &network.NetworkingConfig{}, nil, newName)
	if createErr == nil {
		createErr = s.dockerRepo.ContainerStart(ctx, resp.ID, container.StartOptions{})
	}
	if createErr != nil {
		// 回滚：清理半成品新容器，恢复原容器名字
		if resp.ID != "" {
			_ = s.dockerRepo.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		}
		_ = s.dockerRepo.ContainerRename(ctx, req.ContainerID, origName)
		return "", fmt.Errorf("重建失败（原容器已保留并恢复）: %v", createErr)
	}

	// 新容器运行成功，删除备份
	_ = s.dockerRepo.ContainerRemove(ctx, req.ContainerID, container.RemoveOptions{Force: true})
	return resp.ID, nil
}

// RecreateRequest 重建容器请求
type RecreateRequest struct {
	ContainerID string
	Name        string
	Image       string
	Ports       []PortMapping
	Volumes     []VolumeMapping
	Env         []EnvVar
	Restart     string
	Network     string
	Memory      int64
	CPUs        float64
	Privileged  bool
	TTY         bool
}

// PortMapping 端口映射
type PortMapping struct {
	Host      string
	Container string
}

// VolumeMapping 卷映射
type VolumeMapping struct {
	Host      string
	Container string
}

// EnvVar 环境变量
type EnvVar struct {
	Key   string
	Value string
}

// CopyWithProgress 复制数据并显示进度
func CopyWithProgress(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}

// stdcopy wrapper for tar stream handling
var _ = stdcopy.StdCopy // Ensure stdcopy is used
