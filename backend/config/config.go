package config

import (
	"embed"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"

	"rabbit-panel/exec"
	"rabbit-panel/middleware"
	"rabbit-panel/repository"
	"rabbit-panel/service"
)

// App 应用全局结构体，所有服务通过依赖注入
type App struct {
	// Infrastructure
	Docker *client.Client

	// Configuration
	Mode     string
	Port     string
	Host     string
	HTTPPort string // TLS 启用时可选：HTTP → HTTPS 跳转端口
	// RedirectPort 是跳转目标使用的对外 HTTPS 端口（容器映射端口与内部不一致时设置 TLS_REDIRECT_PORT）
	RedirectPort string
	JWTSecret    []byte
	NodeSecret   string
	NodeID       string
	ServerIP     string
	BuildInfo    service.BuildInfo
	StaticFS     embed.FS
	TLS          TLSConfig

	// Repositories
	DockerRepo repository.IDockerRepository
	SQLiteRepo *repository.SQLiteRepository
	FileRepo   repository.IFileRepository
	CacheRepo  repository.ICacheRepository

	// Services
	ContainerService    *service.ContainerService
	ImageService        *service.ImageService
	NetworkService      *service.NetworkService
	VolumeService       *service.VolumeService
	ComposeService      *service.ComposeService
	RegistryService     *service.RegistryService
	DockerConfigService *service.DockerConfigService
	AgentService        *service.AgentService
	WeChatService       *service.WeChatService
	NotifyService       *service.NotifyService
	NodeService         *service.NodeService
	Scheduler           *service.Scheduler
	UpdateService       *service.UpdateService

	// Terminal service
	TerminalService *exec.TerminalService

	// Gin router
	Engine *gin.Engine
}

// NewApp 创建并初始化 App 实例
func NewApp(buildInfo service.BuildInfo) (*App, error) {
	// 默认 HTTP。需要 HTTPS 时设置 TLS_ENABLED=true；未提供证书且 TLS_SELF_SIGNED=true 时自动生成自签证书。
	tlsConf := TLSConfig{
		Enabled:    getBoolEnv("TLS_ENABLED", false),
		CertFile:   getEnv("TLS_CERT_FILE", defaultTLSCertFile),
		KeyFile:    getEnv("TLS_KEY_FILE", defaultTLSKeyFile),
		SelfSigned: getBoolEnv("TLS_SELF_SIGNED", true),
		Hosts:      splitEnvList(getEnv("TLS_HOSTS", "")),
	}
	if err := tlsConf.Prepare(); err != nil {
		return nil, err
	}

	jwtSecret, err := requireUserSecret("JWT_SECRET", os.Getenv("JWT_SECRET"))
	if err != nil {
		return nil, err
	}
	nodeSecret, err := requireUserSecret("NODE_SECRET", os.Getenv("NODE_SECRET"))
	if err != nil {
		return nil, err
	}

	app := &App{
		Mode:       getEnv("MODE", "master"),
		Port:       getEnv("PORT", "3958"),
		Host:       getEnv("HOST", "0.0.0.0"),
		HTTPPort:   getEnv("HTTP_PORT", ""),
		JWTSecret:  []byte(jwtSecret),
		NodeSecret: nodeSecret,
		BuildInfo:  buildInfo,
		TLS:        tlsConf,
	}
	app.RedirectPort = getEnv("TLS_REDIRECT_PORT", app.Port)

	// Get hostname for node ID
	hostname, _ := os.Hostname()
	app.NodeID = hostname + ":" + app.Port

	// Initialize Docker client.
	// DOCKER_HOST 优先。未设置时沿用 Docker CLI 当前 context，
	// 这样在 Windows 上会连到 dockerDesktopLinuxEngine，而不是需要管理员权限的 docker_engine。
	dockerOpts := []client.Opt{client.WithAPIVersionNegotiation()}
	if host := dockerHostFromContext(); host != "" {
		log.Printf("Docker host: %s (docker context)", host)
		dockerOpts = append(dockerOpts, client.WithHost(host))
	} else {
		dockerOpts = append(dockerOpts, client.FromEnv)
	}
	cli, err := client.NewClientWithOpts(dockerOpts...)
	if err != nil {
		return nil, err
	}
	app.Docker = cli
	app.DockerRepo = repository.NewDockerRepository(cli)

	// Initialize SQLite
	sqliteRepo, err := repository.NewSQLiteRepository("./data/auth.db")
	if err != nil {
		return nil, err
	}
	app.SQLiteRepo = sqliteRepo

	// Initialize other repositories
	app.FileRepo = repository.NewFileRepository()
	app.CacheRepo = repository.NewCacheRepository()

	// Initialize services
	app.initServices()

	// Create Gin engine
	gin.SetMode(gin.ReleaseMode)
	app.Engine = gin.New()
	app.Engine.Use(gin.Recovery())
	app.Engine.Use(middleware.LoggingMiddleware())

	return app, nil
}

// initServices 初始化所有服务
func (app *App) initServices() {
	// Ensure compose directory exists
	app.FileRepo.EnsureComposeDir()

	// Node service (must be initialized first for Master mode)
	app.NodeService = service.NewNodeService(app.DockerRepo, app.CacheRepo, app.Mode, app.NodeSecret, app.NodeID, app.Host, app.Port)

	// Core services
	app.ContainerService = service.NewContainerService(app.DockerRepo, app.CacheRepo)
	app.ImageService = service.NewImageService(app.DockerRepo, app.CacheRepo)
	app.NetworkService = service.NewNetworkService(app.DockerRepo)
	app.VolumeService = service.NewVolumeService(app.DockerRepo)
	app.ComposeService = service.NewComposeService(app.DockerRepo, app.FileRepo)
	app.RegistryService = service.NewRegistryService(app.FileRepo)
	app.DockerConfigService = service.NewDockerConfigService()
	app.NotifyService = service.NewNotifyService()
	app.AgentService = service.NewAgentService(app.SQLiteRepo, app.DockerRepo)
	app.AgentService.UseRegistry(app.RegistryService)
	app.AgentService.UseNodes(app.NodeService, app.JWTSecret)
	if app.Mode != "worker" {
		app.WeChatService = service.NewWeChatService(app.AgentService)
	}
	app.UpdateService = service.NewUpdateService(app.FileRepo, app.BuildInfo)

	// Master-only services
	if app.Mode == "master" {
		app.Scheduler = service.NewScheduler(app.NodeService, app.DockerRepo, app.NodeSecret, app.RegistryService)
	}
	if app.Mode == "worker" {
		app.NodeService.StartWorker(
			getEnv("MASTER_URL", ""),
			getEnv("NODE_NAME", ""),
			getEnv("NODE_ADDRESS", ""),
			app.NodeScheme(),
		)
	}

	// Terminal service
	app.TerminalService = exec.NewTerminalService(app.DockerRepo)
}

// NodeScheme 返回本节点对外提供服务的协议；HTTP 模式下返回空串以保持地址简洁。
func (app *App) NodeScheme() string {
	if app.TLS.Enabled {
		return "https"
	}
	return ""
}

// Scheme 返回本节点对外提供服务的协议，用于日志与健康检查。
func (app *App) Scheme() string {
	if app.TLS.Enabled {
		return "https"
	}
	return "http"
}

// placeholderSecrets 是仓库和文档里出现过的示例值。它们能签出令牌，不能当正式密钥。
var placeholderSecrets = map[string]bool{
	"":                        true,
	"change-me":               true,
	"node-secret":             true,
	"change-me-in-production": true,
	"rabbit-panel-secret-key-change-in-production":  true,
	"rabbit-panel-node-secret-change-in-production": true,
}

// requireUserSecret 要求用户自己填入足够长的密钥。空值、示例值和短于 32 字符都拒绝启动。
func requireUserSecret(name, value string) (string, error) {
	value = strings.TrimSpace(value)
	if placeholderSecrets[value] || len(value) < 32 {
		return "", fmt.Errorf("%s 未设置或仍是示例值。请自行生成后填入环境变量，例如：openssl rand -hex 32", name)
	}
	return value, nil
}

// getEnv 获取环境变量，带默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getBoolEnv 解析布尔型环境变量。
func getBoolEnv(key string, defaultValue bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return defaultValue
	}
	switch value {
	case "1", "true", "yes", "on", "y":
		return true
	default:
		return false
	}
}

// splitEnvList 按逗号/空格拆分列表型环境变量。
func splitEnvList(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	})
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
