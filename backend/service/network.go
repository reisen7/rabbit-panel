package service

import (
	"context"
	"sort"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"

	"rabbit-panel/model"
	"rabbit-panel/repository"
)

// NetworkService 网络服务
type NetworkService struct {
	dockerRepo repository.IDockerRepository
}

// NewNetworkService 创建网络服务
func NewNetworkService(dr repository.IDockerRepository) *NetworkService {
	return &NetworkService{dockerRepo: dr}
}

// ListNetworks 列出网络
func (s *NetworkService) ListNetworks(ctx context.Context) ([]model.NetworkInfo, error) {
	networks, err := s.dockerRepo.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return nil, err
	}

	// /networks 列表不返回挂载的容器，从容器列表的 NetworkSettings 反查使用关系
	usage := make(map[string]map[string]struct{})
	if containers, err := s.dockerRepo.ContainerList(ctx, types.ContainerListOptions{All: true}); err == nil {
		for _, c := range containers {
			if c.NetworkSettings == nil {
				continue
			}
			for netName := range c.NetworkSettings.Networks {
				if usage[netName] == nil {
					usage[netName] = make(map[string]struct{})
				}
				usage[netName][c.ID] = struct{}{}
			}
		}
	}

	result := make([]model.NetworkInfo, 0, len(networks))
	for _, n := range networks {
		info := model.NetworkInfo{
			ID:         n.ID[:12],
			Name:       n.Name,
			Driver:     n.Driver,
			Scope:      n.Scope,
			Internal:   n.Internal,
			Attachable: n.Attachable,
			Created:    n.Created.Format("2006-01-02 15:04:05"),
		}

		// Get subnet and gateway
		if n.IPAM.Config != nil && len(n.IPAM.Config) > 0 {
			info.Subnet = n.IPAM.Config[0].Subnet
			info.Gateway = n.IPAM.Config[0].Gateway
		}

		// Get connected containers（按网络名反查，NetworkSettings.Networks 以网络名为键）
		for id := range usage[n.Name] {
			shortID := id
			if len(shortID) > 12 {
				shortID = shortID[:12]
			}
			info.Containers = append(info.Containers, shortID)
		}
		sort.Strings(info.Containers)
		info.ContainerCount = len(info.Containers)
		info.InUse = info.ContainerCount > 0

		result = append(result, info)
	}
	return result, nil
}

// CreateNetwork 创建网络
func (s *NetworkService) CreateNetwork(ctx context.Context, req *model.CreateNetworkRequest) error {
	opts := types.NetworkCreate{
		Driver: req.Driver,
	}
	if req.Subnet != "" {
		ipamConfig := network.IPAMConfig{Subnet: req.Subnet, Gateway: req.Gateway}
		opts.IPAM = &network.IPAM{
			Config: []network.IPAMConfig{ipamConfig},
		}
	}
	opts.Internal = req.Internal
	opts.Attachable = req.Attachable
	opts.Labels = req.Labels

	_, err := s.dockerRepo.NetworkCreate(ctx, req.Name, opts)
	return err
}

// RemoveNetwork 删除网络
func (s *NetworkService) RemoveNetwork(ctx context.Context, id string) error {
	return s.dockerRepo.NetworkRemove(ctx, id)
}

// InspectNetwork 获取网络详情
func (s *NetworkService) InspectNetwork(ctx context.Context, id string) (*model.NetworkDetail, error) {
	n, err := s.dockerRepo.NetworkInspect(ctx, id, types.NetworkInspectOptions{Verbose: true})
	if err != nil {
		return nil, err
	}

	detail := &model.NetworkDetail{
		NetworkInfo: model.NetworkInfo{
			ID:         n.ID,
			Name:       n.Name,
			Driver:     n.Driver,
			Scope:      n.Scope,
			Internal:   n.Internal,
			Attachable: n.Attachable,
			Created:    n.Created.Format("2006-01-02 15:04:05"),
		},
		IPAM:    n.IPAM,
		Options: n.Options,
		Labels:  n.Labels,
	}

	if n.IPAM.Config != nil && len(n.IPAM.Config) > 0 {
		detail.Subnet = n.IPAM.Config[0].Subnet
		detail.Gateway = n.IPAM.Config[0].Gateway
	}

	if n.Containers != nil {
		detail.Containers = make([]string, 0, len(n.Containers))
		detail.ConnectedContainers = make([]model.ConnectedContainer, 0, len(n.Containers))
		for containerID, endpoint := range n.Containers {
			shortID := containerID
			if len(shortID) > 12 {
				shortID = shortID[:12]
			}
			name := strings.TrimPrefix(endpoint.Name, "/")
			if name == "" {
				name = shortID
			}
			detail.Containers = append(detail.Containers, shortID)
			detail.ConnectedContainers = append(detail.ConnectedContainers, model.ConnectedContainer{
				ID:         shortID,
				Name:       name,
				IPAddress:  endpoint.IPv4Address,
				MacAddress: endpoint.MacAddress,
			})
		}
		sort.Strings(detail.Containers)
		sort.Slice(detail.ConnectedContainers, func(i, j int) bool {
			return detail.ConnectedContainers[i].Name < detail.ConnectedContainers[j].Name
		})
	}
	detail.ContainerCount = len(detail.Containers)
	detail.InUse = detail.ContainerCount > 0

	return detail, nil
}

// ConnectNetwork 连接容器到网络
func (s *NetworkService) ConnectNetwork(ctx context.Context, networkID, containerID string) error {
	return s.dockerRepo.NetworkConnect(ctx, networkID, containerID, nil)
}

// DisconnectNetwork 断开容器与网络的连接
func (s *NetworkService) DisconnectNetwork(ctx context.Context, networkID, containerID string, force bool) error {
	return s.dockerRepo.NetworkDisconnect(ctx, networkID, containerID, force)
}
