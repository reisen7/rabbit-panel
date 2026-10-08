package config

import (
	"embed"
	"log"
	"os"
	"os/signal"
	"syscall"

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
	Mode       string
	Port       string
	Host       string
	JWTSecret  []byte
	NodeSecret string
	NodeID     string
	ServerIP   string
	BuildInfo  service.BuildInfo
	StaticFS   embed.FS

	// Repositories
	DockerRepo repository.IDockerRepository
	SQLiteRepo *repository.SQLiteRepository
	FileRepo   repository.IFileRepository
	CacheRepo  repository.ICacheRepository

	// Services
	ContainerService     *service.ContainerService
	ImageService        *service.ImageService
	NetworkService      *service.NetworkService
	VolumeService       *service.VolumeService
	ComposeService      *service.ComposeService
	RegistryService     *service.RegistryService
	DockerConfigService *service.DockerConfigService
	AgentService       *service.AgentService
	NodeService        *service.NodeService
	Scheduler          *service.Scheduler
	UpdateService      *service.UpdateService

	// Terminal service
	TerminalService *exec.TerminalService

	// Gin router
	Engine *gin.Engine
}

// NewApp 创建并初始化 App 实例
func NewApp(buildInfo service.BuildInfo) (*App, error) {
	app := &App{
		Mode:       getEnv("MODE", "master"),
		Port:       getEnv("PORT", "3958"),
		Host:       getEnv("HOST", "0.0.0.0"),
		JWTSecret:  []byte(getEnv("JWT_SECRET", "change-me")),
		NodeSecret: getEnv("NODE_SECRET", "node-secret"),
		BuildInfo:  buildInfo,
	}

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
		)
	}

	// Terminal service
	app.TerminalService = exec.NewTerminalService(app.DockerRepo)
}

// Run 启动应用
func (app *App) Run() error {
	// Graceful shutdown
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("收到关闭信号，正在关闭...")
		os.Exit(0)
	}()

	addr := app.Host + ":" + app.Port
	log.Printf("服务器启动: %s", addr)
	return app.Engine.Run(addr)
}

// getEnv 获取环境变量，带默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
