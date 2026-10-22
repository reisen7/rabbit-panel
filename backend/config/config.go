package config

import (
	"embed"
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
	// 默认开启 HTTPS：面板承载容器终端与 Docker 管理接口，明文监听风险较高。
	// 需要 HTTP 时显式设置 TLS_ENABLED=false（例如前面已有反向代理做 TLS 终止）。
	tlsConf := TLSConfig{
		Enabled:    getBoolEnv("TLS_ENABLED", true),
		CertFile:   getEnv("TLS_CERT_FILE", defaultTLSCertFile),
		KeyFile:    getEnv("TLS_KEY_FILE", defaultTLSKeyFile),
		SelfSigned: getBoolEnv("TLS_SELF_SIGNED", true),
		Hosts:      splitEnvList(getEnv("TLS_HOSTS", "")),
	}
	if err := tlsConf.Prepare(); err != nil {
		return nil, err
	}

	app := &App{
		Mode:       getEnv("MODE", "master"),
		Port:       getEnv("PORT", "3958"),
		Host:       getEnv("HOST", "0.0.0.0"),
		HTTPPort:   getEnv("HTTP_PORT", ""),
		JWTSecret:  []byte(getEnv("JWT_SECRET", "change-me")),
		NodeSecret: getEnv("NODE_SECRET", "node-secret"),
		BuildInfo:  buildInfo,
		TLS:        tlsConf,
	}
	app.RedirectPort = getEnv("TLS_REDIRECT_PORT", app.Port)

	// Get hostname for node ID
	hostname, _ := os.Hostname()
	app.NodeID = hostname + ":" + app.Port

	// Initialize Docker client
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
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
