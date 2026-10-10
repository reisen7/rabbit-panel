package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"

	"rabbit-panel/model"
	"rabbit-panel/repository"
)

// ImageService 镜像服务
type ImageService struct {
	dockerRepo repository.IDockerRepository
	cacheRepo  repository.ICacheRepository
}

// NewImageService 创建镜像服务
func NewImageService(dr repository.IDockerRepository, cr repository.ICacheRepository) *ImageService {
	return &ImageService{
		dockerRepo: dr,
		cacheRepo:  cr,
	}
}

// ListImages 列出镜像
func (s *ImageService) ListImages(ctx context.Context) ([]model.ImageInfo, error) {
	if cached, _, ok := s.cacheRepo.GetImages(); ok {
		return cached, nil
	}

	images, err := s.dockerRepo.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return nil, err
	}

	containers, err := s.dockerRepo.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	usageMap := buildImageUsage(containers)
	result := s.convertImages(images, usageMap)
	s.cacheRepo.SetImages(result)
	return result, nil
}

// PullImage 拉取镜像并等待完成。
func (s *ImageService) PullImage(ctx context.Context, name string, opts types.ImagePullOptions) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("镜像名称不能为空")
	}
	reader, err := s.dockerRepo.ImagePull(ctx, name, opts)
	if err != nil {
		return err
	}
	defer reader.Close()
	_, err = io.Copy(io.Discard, reader)
	s.cacheRepo.InvalidateImages()
	return err
}

// RemoveImage 删除镜像
func (s *ImageService) RemoveImage(ctx context.Context, id string, force bool) error {
	_, err := s.dockerRepo.ImageRemove(ctx, id, types.ImageRemoveOptions{Force: force})
	s.cacheRepo.InvalidateImages()
	return err
}

// PruneImages 清理未使用的镜像 (no-op, ImagePrune not in current interface)
// func (s *ImageService) PruneImages(ctx context.Context) error {
//     _, err := s.dockerRepo.ImagePrune(ctx, filters.NewArgs())
//     return err
// }

// convertImages 转换 Docker 镜像类型到模型
func (s *ImageService) convertImages(images []image.Summary, usageMap map[string][]string) []model.ImageInfo {
	result := make([]model.ImageInfo, 0, len(images))
	for _, img := range images {
		// 提取标签
		var name, tag string
		if len(img.RepoTags) > 0 {
			repoTag := img.RepoTags[0]
			parts := splitLast(repoTag, ":")
			name = parts[0]
			tag = parts[1]
			if tag == "" {
				tag = "latest"
			}
		} else if len(img.RepoDigests) > 0 {
			name = "<none>"
			tag = "<none>"
		}

		// 格式化大小
		size := formatSize(img.Size)

		shortID := img.ID
		if strings.HasPrefix(shortID, "sha256:") {
			shortID = shortID[len("sha256:"):]
		}
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}

		usedBy := append([]string(nil), usageMap[img.ID]...)
		if len(usedBy) == 0 {
			usedBy = append([]string(nil), usageMap[shortID]...)
		}

		result = append(result, model.ImageInfo{
			ID:          shortID,
			Name:        name,
			Tag:         tag,
			Size:        size,
			Created:     time.Unix(img.Created, 0).Format("2006-01-02 15:04:05"),
			InUse:       len(usedBy) > 0,
			UsedBy:      usedBy,
			UsedByCount: len(usedBy),
		})
	}
	return result
}

func buildImageUsage(containers []types.Container) map[string][]string {
	usageMap := make(map[string][]string)
	for _, c := range containers {
		name := c.ID
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		if c.ImageID != "" {
			usageMap[c.ImageID] = append(usageMap[c.ImageID], name)
			shortImageID := strings.TrimPrefix(c.ImageID, "sha256:")
			if len(shortImageID) > 12 {
				shortImageID = shortImageID[:12]
			}
			usageMap[shortImageID] = append(usageMap[shortImageID], name)
		}
		if c.Image != "" {
			usageMap[c.Image] = append(usageMap[c.Image], name)
		}
	}
	return usageMap
}

func splitLast(s, sep string) []string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i:i+len(sep)] == sep && (i == 0 || s[i-1:i] != sep) {
			return []string{s[:i], s[i+len(sep):]}
		}
	}
	return []string{s, ""}
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
