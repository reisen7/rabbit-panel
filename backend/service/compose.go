package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"

	"rabbit-panel/model"
	"rabbit-panel/repository"
)

// ComposeService Docker Compose 服务
type ComposeService struct {
	dockerRepo repository.IDockerRepository
	fileRepo   repository.IFileRepository
}

// NewComposeService 创建 Compose 服务
func NewComposeService(dr repository.IDockerRepository, fr repository.IFileRepository) *ComposeService {
	return &ComposeService{
		dockerRepo: dr,
		fileRepo:   fr,
	}
}

// ListProjects 列出 Compose 项目
func (s *ComposeService) ListProjects() ([]model.ComposeProject, error) {
	projects, err := s.fileRepo.ListComposeProjects()
	if err != nil {
		return nil, err
	}

	result := make([]model.ComposeProject, 0, len(projects))
	for _, name := range projects {
		status := s.getProjectStatus(name)
		result = append(result, model.ComposeProject{
			Name:   name,
			Status: status,
		})
	}
	return result, nil
}

// FetchProjectStatus 获取单个 Compose 项目状态
func (s *ComposeService) FetchProjectStatus(name string) (*model.ComposeProject, error) {
	return s.projectFromContainers(name), nil
}

// CreateProject 创建 Compose 项目
func (s *ComposeService) CreateProject(name, content string) error {
	return s.fileRepo.CreateComposeProject(name, content)
}

// GetProjectFile 获取项目 Compose 文件
func (s *ComposeService) GetProjectFile(name string) (string, error) {
	return s.fileRepo.GetComposeFile(name)
}

// SaveProjectFile 保存项目 Compose 文件
func (s *ComposeService) SaveProjectFile(name, content string) error {
	return s.fileRepo.SaveComposeFile(name, content)
}

// DeleteProject 删除项目
func (s *ComposeService) DeleteProject(name string) error {
	return s.fileRepo.DeleteComposeProject(name)
}

// ExecuteAction 执行 Compose 操作（up, down, restart, pull, logs）
func (s *ComposeService) ExecuteAction(name, action string, writer io.Writer) error {
	dir := s.fileRepo.GetComposeProjectDir(name)
	composeFile := filepath.Base(resolveComposeFile(dir))
	var cmd *exec.Cmd

	switch action {
	case "up":
		cmd = exec.Command("docker", "compose", "-f", composeFile, "up", "-d")
	case "down":
		cmd = exec.Command("docker", "compose", "-f", composeFile, "down")
	case "restart":
		cmd = exec.Command("docker", "compose", "-f", composeFile, "restart")
	case "pull":
		cmd = exec.Command("docker", "compose", "-f", composeFile, "pull")
	case "logs":
		cmd = exec.Command("docker", "compose", "-f", composeFile, "logs", "--tail=50")
	default:
		return fmt.Errorf("unknown action: %s", action)
	}

	cmd.Dir = dir
	cmd.Stdout = writer
	cmd.Stderr = writer
	return cmd.Run()
}

// getProjectStatus 获取项目状态。新建项目还没有容器时记为已停止。
func (s *ComposeService) getProjectStatus(name string) string {
	return s.projectFromContainers(name).Status
}

func (s *ComposeService) projectFromContainers(name string) *model.ComposeProject {
	project := &model.ComposeProject{
		Name:   name,
		Status: "stopped",
	}
	containers, err := s.dockerRepo.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		project.Status = "unknown"
		return project
	}

	running := 0
	for _, container := range containers {
		if container.Labels["com.docker.compose.project"] != name {
			continue
		}
		if container.State == "running" {
			running++
		}
		containerName := ""
		if len(container.Names) > 0 {
			containerName = strings.TrimPrefix(container.Names[0], "/")
		}
		ports := make([]string, 0, len(container.Ports))
		for _, port := range container.Ports {
			if port.PublicPort != 0 {
				ports = append(ports, fmt.Sprintf("%d:%d", port.PublicPort, port.PrivatePort))
			}
		}
		id := container.ID
		if len(id) > 12 {
			id = id[:12]
		}
		project.Containers = append(project.Containers, model.ComposeContainer{
			ID:      id,
			Name:    containerName,
			Service: container.Labels["com.docker.compose.service"],
			State:   container.State,
			Status:  container.Status,
			Ports:   strings.Join(ports, ", "),
		})
	}

	total := len(project.Containers)
	switch {
	case total == 0:
		project.Status = "stopped"
	case running == total:
		project.Status = "running"
	case running > 0:
		project.Status = "partial"
	default:
		project.Status = "stopped"
	}
	return project
}

func resolveComposeFile(dir string) string {
	yml := filepath.Join(dir, "docker-compose.yml")
	if _, err := os.Stat(yml); err == nil {
		return yml
	}
	yaml := filepath.Join(dir, "docker-compose.yaml")
	if _, err := os.Stat(yaml); err == nil {
		return yaml
	}
	return yml
}
