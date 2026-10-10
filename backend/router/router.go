package router

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/gin-gonic/gin"
	ws "github.com/gorilla/websocket"
	config2 "rabbit-panel/config"
	execlib "rabbit-panel/exec"
	"rabbit-panel/middleware"
	"rabbit-panel/model"
	"rabbit-panel/repository"
	"rabbit-panel/service"
	"rabbit-panel/tool"
)

// Router Gin 路由注册器
type Router struct {
	app *config2.App
}

// NewRouter 创建路由注册器
func NewRouter(app *config2.App) *Router {
	return &Router{app: app}
}

// Register 注册所有路由到 Gin engine
func (r *Router) Register() {
	app := r.app
	engine := app.Engine
	if app.Mode == "master" {
		engine.Use(r.containerNodeProxy())
	}

	// Middleware
	authMW := middleware.AuthMiddleware(app.JWTSecret, "/api/auth/login", "/api/health")
	nodeAuthMW := middleware.NodeAuthMiddleware(app.NodeSecret)
	authOrNodeMW := middleware.AuthOrNodeMiddleware(app.JWTSecret, app.NodeSecret, "/api/auth/login", "/api/health")

	// Public routes (no auth, with rate limiting on login)
	engine.GET("/api/auth/captcha", r.handleCaptcha)
	engine.POST("/api/auth/login", middleware.RateLimitMiddleware(5, time.Minute), r.handleLogin)
	engine.GET("/api/health", r.handleHealth)

	// Container routes (supports node auth)
	api := engine.Group("/api")
	api.Use(authOrNodeMW)
	{
		api.GET("/containers", r.handleContainersList)
		api.GET("/containers/logs", r.handleContainerLogs)
		api.GET("/containers/stats", r.handleContainerStats)
		api.GET("/containers/files", r.handleContainerFilesList)
		api.GET("/containers/files/download", r.handleContainerFileDownload)
		api.GET("/containers/files/read", r.handleContainerFileRead)

		// Image routes (supports node auth)
		api.GET("/images", r.handleImagesList)

		// System stats (supports node auth)
		api.GET("/system/stats", r.handleSystemStats)
	}
	apiAuth := api.Group("")
	apiAuth.Use(authMW)
	{
		// Container actions (user-only)
		apiAuth.POST("/containers/action", r.handleContainerAction)
		apiAuth.POST("/containers/run", r.handleContainerRun)
		apiAuth.POST("/containers/run/stream", r.handleContainerRunStream)
		apiAuth.POST("/containers/run/raw", r.handleContainerRunRaw)
		apiAuth.POST("/containers/exec", r.handleContainerExec)
		apiAuth.GET("/containers/inspect", r.handleContainerInspect)
		apiAuth.POST("/containers/update", r.handleContainerUpdate)
		apiAuth.POST("/containers/rename", r.handleContainerRename)
		apiAuth.POST("/containers/recreate", r.handleContainerRecreate)
		apiAuth.POST("/containers/files/mkdir", r.handleContainerFileMkdir)
		apiAuth.POST("/containers/files/delete", r.handleContainerFileDelete)
		apiAuth.POST("/containers/files/upload", r.handleContainerFileUpload)
		apiAuth.POST("/containers/files/write", r.handleContainerFileWrite)
		apiAuth.GET("/containers/terminal", r.handleContainerTerminalWS)

		// Image routes (user-only)
		apiAuth.POST("/images/build", r.handleImagesBuild)
		apiAuth.POST("/images/pull", r.handleImagesPull)
		apiAuth.POST("/images/remove", r.handleImagesRemove)

		// Network routes
		apiAuth.GET("/networks", r.handleNetworksList)
		apiAuth.POST("/networks/create", r.handleNetworksCreate)
		apiAuth.POST("/networks/remove", r.handleNetworksRemove)
		apiAuth.GET("/networks/inspect", r.handleNetworksInspect)
		apiAuth.POST("/networks/connect", r.handleNetworksConnect)
		apiAuth.POST("/networks/disconnect", r.handleNetworksDisconnect)

		// Volume routes
		apiAuth.GET("/volumes", r.handleVolumesList)
		apiAuth.POST("/volumes/create", r.handleVolumesCreate)
		apiAuth.POST("/volumes/remove", r.handleVolumesRemove)
		apiAuth.POST("/volumes/prune", r.handleVolumesPrune)

		// Compose routes
		apiAuth.GET("/compose/list", r.handleComposeList)
		apiAuth.POST("/compose/create", r.handleComposeCreate)
		apiAuth.GET("/compose/file", r.handleComposeFile)
		apiAuth.POST("/compose/file", r.handleComposeFile)
		apiAuth.POST("/compose/action", r.handleComposeAction)
		apiAuth.POST("/compose/delete", r.handleComposeDelete)
		apiAuth.GET("/compose/status", r.handleComposeStatus)
		apiAuth.POST("/compose/save", r.handleComposeSaveFile)

		// Registry routes
		apiAuth.GET("/registries", r.handleRegistriesList)
		apiAuth.POST("/registries/create", r.handleRegistriesCreate)
		apiAuth.POST("/registries/remove", r.handleRegistriesRemove)
		apiAuth.POST("/registries/test", r.handleRegistriesTest)

		// Docker config routes
		apiAuth.GET("/docker/info", r.handleDockerInfo)
		apiAuth.GET("/docker/config", r.handleDockerConfigGet)
		apiAuth.POST("/docker/config/update", r.handleDockerConfigUpdate)
		apiAuth.POST("/docker/restart", r.handleDockerRestart)

		// Auth routes
		apiAuth.POST("/auth/change-password", r.handleChangePassword)
		apiAuth.POST("/auth/logout", r.handleLogout)
		apiAuth.GET("/auth/me", r.handleGetCurrentUser)

		// Agent routes
		apiAuth.POST("/agent/chat", r.handleAgentChat)
		apiAuth.GET("/agent/history", r.handleAgentHistory)
		apiAuth.POST("/agent/history", r.handleAgentHistory)
		apiAuth.DELETE("/agent/history", r.handleAgentHistory)
		apiAuth.GET("/settings/agent", r.handleAgentConfig)
		apiAuth.POST("/settings/agent", r.handleAgentConfig)
		apiAuth.POST("/settings/agent/models", r.handleAgentModels)
		apiAuth.POST("/settings/agent/test", r.handleAgentTest)
		apiAuth.GET("/channels/wechat", r.handleWeChatStatus)
		apiAuth.POST("/channels/wechat/qrcode", r.handleWeChatQRCode)
		apiAuth.GET("/channels/wechat/qrcode/status", r.handleWeChatQRStatus)
		apiAuth.POST("/channels/wechat/verify", r.handleWeChatVerify)
		apiAuth.DELETE("/channels/wechat", r.handleWeChatUnbind)

		apiAuth.GET("/system/update/check", r.handleUpdateCheck)
		apiAuth.GET("/system/update/status", r.handleUpdateStatus)
		apiAuth.POST("/system/update/run", r.handleUpdateRun)
		apiAuth.POST("/system/update/apply", r.handleUpdateApply)
		apiAuth.POST("/system/update/ignore", r.handleUpdateIgnore)
		apiAuth.POST("/system/update/clear-ignore", r.handleUpdateClearIgnore)
		apiAuth.POST("/system/update/clear-state", r.handleUpdateClearState)
	}

	// Master-only routes
	if app.Mode == "master" {
		apiAuth.GET("/nodes", r.handleNodesList)

		// Node auth routes (separate group)
		nodeGroup := engine.Group("/api")
		nodeGroup.Use(nodeAuthMW)
		{
			nodeGroup.POST("/nodes/register", r.handleNodeRegister)
			nodeGroup.POST("/nodes/heartbeat", r.handleNodeHeartbeat)
		}

		apiAuth.POST("/containers/schedule", r.handleContainerSchedule)
		apiAuth.GET("/containers/all", r.handleAllContainers)
	}

	// SPA fallback - serve static files. Worker 只提供接口，不提供面板页面。
	engine.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		// Skip API routes
		if strings.HasPrefix(path, "/api/") {
			c.Status(http.StatusNotFound)
			return
		}
		if app.Mode != "master" {
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(workerOnlyPage()))
			return
		}
		// Handle static asset prefixes
		if strings.HasPrefix(path, "/static/") {
			path = strings.TrimPrefix(path, "/static")
		}
		if path == "/" {
			path = "/index.html"
		}
		serveEmbeddedFile(c, r.app.StaticFS, path)
	})
}

func workerOnlyPage() string {
	masterURL := strings.TrimSpace(os.Getenv("MASTER_URL"))
	hint := "请通过 Master 面板查看和操作。"
	if masterURL != "" {
		hint = "请打开 Master 面板：" + masterURL
	}
	return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Rabbit Panel Worker</title>
</head>
<body style="margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;background:#111;color:#eee;font-family:sans-serif">
  <div style="max-width:520px;padding:32px">
    <h1 style="margin:0 0 12px;font-size:28px">这是 Worker 节点</h1>
    <p style="margin:0;line-height:1.6;color:#bbb">此节点不提供管理界面。容器、日志和终端都在 Master 上查看和操作。</p>
    <p style="margin:16px 0 0;line-height:1.6">` + hint + `</p>
  </div>
</body>
</html>`
}

func serveEmbeddedFile(c *gin.Context, embeddedFS fs.FS, path string) {
	cleanPath := strings.TrimPrefix(path, "/")
	if cleanPath == "" {
		cleanPath = "index.html"
	}

	fileData, err := fs.ReadFile(embeddedFS, ".dist/"+cleanPath)
	if err != nil {
		fileData, err = fs.ReadFile(embeddedFS, ".dist/index.html")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", fileData)
		return
	}

	c.Data(http.StatusOK, detectContentType(cleanPath), fileData)
}

func detectContentType(path string) string {
	switch {
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(path, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".jpg"), strings.HasSuffix(path, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(path, ".webp"):
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// === Auth Handlers ===

func (r *Router) handleCaptcha(c *gin.Context) {
	captchaID, imgBase64, err := middleware.GetCaptchaManager().Generate()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成验证码失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"captcha_id": captchaID,
		"image":      "data:image/png;base64," + imgBase64,
	})
}

func (r *Router) handleLogin(c *gin.Context) {
	var reqBody struct {
		Username  string `json:"username"`
		Password  string `json:"password"`
		CaptchaID string `json:"captcha_id"`
		Captcha   string `json:"captcha"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	// 验证验证码
	if !middleware.GetCaptchaManager().Verify(reqBody.CaptchaID, reqBody.Captcha) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误或已过期"})
		return
	}

	user, err := r.app.SQLiteRepo.GetUser(reqBody.Username)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	err = r.app.SQLiteRepo.VerifyPassword(user.ID, reqBody.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	token, _ := middleware.GenerateToken(user.Username, user.NeedChangePassword, r.app.JWTSecret)

	c.SetCookie("token", token, 86400, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{
		"token":                token,
		"need_change_password": user.NeedChangePassword,
		"message":              "登录成功",
	})
}

func (r *Router) handleHealth(c *gin.Context) {
	_, err := r.app.DockerRepo.Ping(context.Background())
	if err != nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	c.String(http.StatusOK, "OK")
}

func (r *Router) handleChangePassword(c *gin.Context) {
	var reqBody struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	username, _ := c.Get("username")
	usernameStr, ok := username.(string)
	if !ok || usernameStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	err := r.app.SQLiteRepo.ChangePassword(usernameStr, reqBody.OldPassword, reqBody.NewPassword)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, _ := middleware.GenerateToken(usernameStr, false, r.app.JWTSecret)
	c.SetCookie("token", token, 86400, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功", "token": token})
}

func (r *Router) handleLogout(c *gin.Context) {
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"message": "登出成功"})
}

func (r *Router) handleGetCurrentUser(c *gin.Context) {
	username, _ := c.Get("username")
	c.JSON(http.StatusOK, gin.H{"username": username})
}

// === Container Handlers ===

func (r *Router) handleContainersList(c *gin.Context) {
	containers, err := r.app.ContainerService.ListContainers(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, containers)
}

func (r *Router) handleContainerAction(c *gin.Context) {
	var reqBody struct {
		ContainerID string `json:"container_id"`
		Action      string `json:"action"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	ctx := context.Background()
	var err error
	switch reqBody.Action {
	case "start":
		err = r.app.ContainerService.StartContainer(ctx, reqBody.ContainerID)
	case "stop":
		err = r.app.ContainerService.StopContainer(ctx, reqBody.ContainerID)
	case "restart":
		err = r.app.ContainerService.RestartContainer(ctx, reqBody.ContainerID)
	case "remove":
		err = r.app.ContainerService.RemoveContainer(ctx, reqBody.ContainerID, true)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown action"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (r *Router) handleContainerRun(c *gin.Context) {
	var reqBody struct {
		Image   string `json:"image"`
		Name    string `json:"name"`
		Restart string `json:"restart"`
		Network string `json:"network"`
		Ports   []struct {
			Host      string `json:"host"`
			Container string `json:"container"`
		} `json:"ports"`
		Env []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"env"`
		Volumes []struct {
			Host      string `json:"host"`
			Container string `json:"container"`
		} `json:"volumes"`
	}

	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	if reqBody.Image == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "镜像名称不能为空"})
		return
	}

	ctx := context.Background()

	// Pull image if needed
	_, _, err := r.app.DockerRepo.ImageInspectWithRaw(ctx, reqBody.Image)
	if err != nil {
		reader, err := r.app.DockerRepo.ImagePull(ctx, reqBody.Image, r.app.RegistryService.PullOptions(reqBody.Image))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("拉取镜像失败: %v", err)})
			return
		}
		io.Copy(io.Discard, reader)
	}

	// Build container config
	containerConfig := &container.Config{Image: reqBody.Image}
	for _, env := range reqBody.Env {
		if env.Key != "" {
			containerConfig.Env = append(containerConfig.Env, fmt.Sprintf("%s=%s", env.Key, env.Value))
		}
	}

	hostConfig := &container.HostConfig{}
	if len(reqBody.Ports) > 0 {
		portBindings := make(map[nat.Port][]nat.PortBinding)
		exposedPorts := make(map[nat.Port]struct{})
		for _, p := range reqBody.Ports {
			if p.Host != "" && p.Container != "" {
				cPort := nat.Port(p.Container + "/tcp")
				exposedPorts[cPort] = struct{}{}
				portBindings[cPort] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: p.Host}}
			}
		}
		containerConfig.ExposedPorts = exposedPorts
		hostConfig.PortBindings = portBindings
	}
	for _, v := range reqBody.Volumes {
		if v.Host != "" && v.Container != "" {
			hostConfig.Binds = append(hostConfig.Binds, fmt.Sprintf("%s:%s", v.Host, v.Container))
		}
	}
	if reqBody.Restart != "" {
		hostConfig.RestartPolicy = container.RestartPolicy{Name: container.RestartPolicyMode(reqBody.Restart)}
	}
	if reqBody.Network != "" {
		hostConfig.NetworkMode = container.NetworkMode(reqBody.Network)
	}

	resp, err := r.app.DockerRepo.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, reqBody.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建容器失败: %v", err)})
		return
	}

	if err := r.app.DockerRepo.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		r.app.DockerRepo.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("启动容器失败: %v", err)})
		return
	}

	r.app.CacheRepo.InvalidateContainers()
	c.JSON(http.StatusOK, gin.H{"status": "success", "id": resp.ID})
}

func (r *Router) handleContainerRunStream(c *gin.Context) {
	var reqBody struct {
		Image   string `json:"image"`
		Name    string `json:"name"`
		Restart string `json:"restart"`
		Network string `json:"network"`
		Ports   []struct {
			Host      string `json:"host"`
			Container string `json:"container"`
		} `json:"ports"`
		Env []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"env"`
		Volumes []struct {
			Host      string `json:"host"`
			Container string `json:"container"`
		} `json:"volumes"`
	}

	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	if reqBody.Image == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "镜像名称不能为空"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.Status(http.StatusInternalServerError)
		return
	}

	sendLog := func(msg string) {
		fmt.Fprintf(c.Writer, "data: {\"type\":\"log\",\"message\":\"%s\"}\n\n", strings.ReplaceAll(msg, "\"", "\\\""))
		flusher.Flush()
	}
	sendError := func(msg string) {
		fmt.Fprintf(c.Writer, "data: {\"type\":\"error\",\"message\":\"%s\"}\n\n", strings.ReplaceAll(msg, "\"", "\\\""))
		flusher.Flush()
	}
	sendSuccess := func(id string) {
		fmt.Fprintf(c.Writer, "data: {\"type\":\"success\",\"id\":\"%s\"}\n\n", id)
		flusher.Flush()
	}

	ctx := context.Background()

	// Check image
	sendLog("检查本地镜像...")
	_, _, err := r.app.DockerRepo.ImageInspectWithRaw(ctx, reqBody.Image)
	if err != nil {
		sendLog(fmt.Sprintf("镜像 %s 不存在，开始拉取...", reqBody.Image))
		reader, err := r.app.DockerRepo.ImagePull(ctx, reqBody.Image, r.app.RegistryService.PullOptions(reqBody.Image))
		if err != nil {
			sendError(fmt.Sprintf("拉取镜像失败: %v", err))
			return
		}
		decoder := json.NewDecoder(reader)
		for {
			var status struct {
				Status   string `json:"status"`
				Progress string `json:"progress"`
				ID       string `json:"id"`
			}
			if err := decoder.Decode(&status); err != nil {
				if err == io.EOF {
					break
				}
				continue
			}
			if status.Progress != "" {
				sendLog(fmt.Sprintf("%s: %s %s", status.ID, status.Status, status.Progress))
			} else if status.Status != "" {
				sendLog(status.Status)
			}
		}
		sendLog("镜像拉取完成")
	} else {
		sendLog("镜像已存在")
	}

	// Build config
	containerConfig := &container.Config{Image: reqBody.Image}
	for _, env := range reqBody.Env {
		if env.Key != "" {
			containerConfig.Env = append(containerConfig.Env, fmt.Sprintf("%s=%s", env.Key, env.Value))
		}
	}
	hostConfig := &container.HostConfig{}
	if len(reqBody.Ports) > 0 {
		portBindings := make(map[nat.Port][]nat.PortBinding)
		exposedPorts := make(map[nat.Port]struct{})
		for _, p := range reqBody.Ports {
			if p.Host != "" && p.Container != "" {
				cPort := nat.Port(p.Container + "/tcp")
				exposedPorts[cPort] = struct{}{}
				portBindings[cPort] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: p.Host}}
			}
		}
		containerConfig.ExposedPorts = exposedPorts
		hostConfig.PortBindings = portBindings
	}
	for _, v := range reqBody.Volumes {
		if v.Host != "" && v.Container != "" {
			hostConfig.Binds = append(hostConfig.Binds, fmt.Sprintf("%s:%s", v.Host, v.Container))
		}
	}
	if reqBody.Restart != "" {
		hostConfig.RestartPolicy = container.RestartPolicy{Name: container.RestartPolicyMode(reqBody.Restart)}
	}
	if reqBody.Network != "" {
		hostConfig.NetworkMode = container.NetworkMode(reqBody.Network)
	}

	sendLog("创建容器...")
	resp, err := r.app.DockerRepo.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, reqBody.Name)
	if err != nil {
		sendError(fmt.Sprintf("创建容器失败: %v", err))
		return
	}
	sendLog(fmt.Sprintf("容器已创建，ID: %s", resp.ID[:12]))

	sendLog("启动容器...")
	if err := r.app.DockerRepo.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		r.app.DockerRepo.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		sendError(fmt.Sprintf("启动容器失败: %v", err))
		return
	}

	sendLog("容器启动成功！")
	r.app.CacheRepo.InvalidateContainers()
	sendSuccess(resp.ID[:12])
}

func (r *Router) handleContainerRunRaw(c *gin.Context) {
	var reqBody struct {
		Command string `json:"command"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	cmd := strings.TrimSpace(reqBody.Command)
	if cmd == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "命令不能为空"})
		return
	}

	if !strings.HasPrefix(cmd, "docker run ") && !strings.HasPrefix(cmd, "docker run\t") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只支持 docker run 命令"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.Status(http.StatusInternalServerError)
		return
	}

	sendLog := func(msg string) {
		fmt.Fprintf(c.Writer, "data: {\"type\":\"log\",\"message\":\"%s\"}\n\n", strings.ReplaceAll(msg, "\"", "\\\""))
		flusher.Flush()
	}
	sendError := func(msg string) {
		fmt.Fprintf(c.Writer, "data: {\"type\":\"error\",\"message\":\"%s\"}\n\n", strings.ReplaceAll(msg, "\"", "\\\""))
		flusher.Flush()
	}
	sendSuccess := func(id string) {
		fmt.Fprintf(c.Writer, "data: {\"type\":\"success\",\"id\":\"%s\"}\n\n", id)
		flusher.Flush()
	}

	sendLog(fmt.Sprintf("执行命令: %s", cmd))

	var execCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		execCmd = exec.Command("cmd", "/C", cmd)
	} else {
		execCmd = exec.Command("sh", "-c", cmd)
	}

	stdout, err := execCmd.StdoutPipe()
	if err != nil {
		sendError(fmt.Sprintf("获取输出失败: %v", err))
		return
	}
	stderr, err := execCmd.StderrPipe()
	if err != nil {
		sendError(fmt.Sprintf("获取错误输出失败: %v", err))
		return
	}

	if err := execCmd.Start(); err != nil {
		sendError(fmt.Sprintf("启动命令失败: %v", err))
		return
	}

	var containerID string
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if len(line) == 64 || len(line) == 12 {
				containerID = line
				if len(containerID) > 12 {
					containerID = containerID[:12]
				}
			}
			sendLog(line)
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			sendLog(scanner.Text())
		}
	}()

	if err := execCmd.Wait(); err != nil {
		sendError(fmt.Sprintf("命令执行失败: %v", err))
		return
	}

	r.app.CacheRepo.InvalidateContainers()
	if containerID != "" {
		sendSuccess(containerID)
	} else {
		sendSuccess("completed")
	}
}

func (r *Router) handleContainerLogs(c *gin.Context) {
	id := c.Query("id")
	tail := c.DefaultQuery("tail", "100")
	follow := c.Query("follow") != "false"

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	reader, err := r.app.ContainerService.GetContainerLogs(ctx, id, tail, follow)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer reader.Close()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return
	}

	if follow {
		header := make([]byte, 8)
		for {
			_, err := io.ReadFull(reader, header)
			if err != nil {
				break
			}
			size := binary.BigEndian.Uint32(header[4:8])
			if size == 0 || size > 65536 {
				continue
			}
			data := make([]byte, size)
			_, err = io.ReadFull(reader, data)
			if err != nil {
				break
			}
			line := strings.TrimRight(string(data), "\r\n\t ")
			if line != "" {
				fmt.Fprintf(c.Writer, "data: %s\n\n", line)
				flusher.Flush()
			}
		}
	} else {
		var stdout, stderr bytes.Buffer
		_, _ = stdcopy.StdCopy(&stdout, &stderr, reader)
		content := stdout.String()
		if stderr.Len() > 0 {
			if content != "" {
				content += "\n"
			}
			content += stderr.String()
		}
		for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
			line = strings.TrimRight(line, "\r\n\t ")
			if line != "" {
				fmt.Fprintf(c.Writer, "data: %s\n\n", line)
			}
		}
		flusher.Flush()
	}
}

func (r *Router) handleContainerExec(c *gin.Context) {
	var reqBody struct {
		ContainerID string   `json:"container_id"`
		Command     []string `json:"command"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	output, exitCode, err := r.app.TerminalService.ExecCommand(c.Request.Context(), reqBody.ContainerID, reqBody.Command)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"output": output, "exit_code": exitCode})
}

func (r *Router) handleContainerInspect(c *gin.Context) {
	id := c.Query("id")
	inspect, err := r.app.TerminalService.GetContainerInspect(context.Background(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ports := []map[string]string{}
	for containerPort, bindings := range inspect.HostConfig.PortBindings {
		for _, binding := range bindings {
			ports = append(ports, map[string]string{
				"host":      binding.HostPort,
				"container": string(containerPort),
				"hostIP":    binding.HostIP,
			})
		}
	}

	volumes := []map[string]string{}
	for _, bind := range inspect.HostConfig.Binds {
		parts := strings.SplitN(bind, ":", 3)
		vol := map[string]string{"host": "", "container": "", "mode": "rw"}
		if len(parts) >= 1 {
			vol["host"] = parts[0]
		}
		if len(parts) >= 2 {
			vol["container"] = parts[1]
		}
		if len(parts) >= 3 {
			vol["mode"] = parts[2]
		}
		volumes = append(volumes, vol)
	}

	envs := []map[string]string{}
	for _, env := range inspect.Config.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envs = append(envs, map[string]string{
				"key":   parts[0],
				"value": parts[1],
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":              inspect.ID[:12],
		"fullId":          inspect.ID,
		"name":            strings.TrimPrefix(inspect.Name, "/"),
		"image":           inspect.Config.Image,
		"imageId":         inspect.Image,
		"created":         inspect.Created,
		"started":         inspect.State.StartedAt,
		"finished":        inspect.State.FinishedAt,
		"state":           inspect.State.Status,
		"running":         inspect.State.Running,
		"paused":          inspect.State.Paused,
		"pid":             inspect.State.Pid,
		"exitCode":        inspect.State.ExitCode,
		"platform":        inspect.Platform,
		"hostname":        inspect.Config.Hostname,
		"domainname":      inspect.Config.Domainname,
		"networkMode":     string(inspect.HostConfig.NetworkMode),
		"ports":           ports,
		"dns":             inspect.HostConfig.DNS,
		"dnsSearch":       inspect.HostConfig.DNSSearch,
		"extraHosts":      inspect.HostConfig.ExtraHosts,
		"macAddress":      inspect.NetworkSettings.MacAddress,
		"ipAddress":       inspect.NetworkSettings.IPAddress,
		"gateway":         inspect.NetworkSettings.Gateway,
		"volumes":         volumes,
		"workingDir":      inspect.Config.WorkingDir,
		"readOnly":        inspect.HostConfig.ReadonlyRootfs,
		"env":             envs,
		"cmd":             inspect.Config.Cmd,
		"entrypoint":      inspect.Config.Entrypoint,
		"user":            inspect.Config.User,
		"tty":             inspect.Config.Tty,
		"stdin":           inspect.Config.OpenStdin,
		"restart":         string(inspect.HostConfig.RestartPolicy.Name),
		"restartMaxRetry": inspect.HostConfig.RestartPolicy.MaximumRetryCount,
		"memory":          inspect.HostConfig.Memory / 1024 / 1024,
		"memorySwap":      inspect.HostConfig.MemorySwap / 1024 / 1024,
		"memoryRes":       inspect.HostConfig.MemoryReservation / 1024 / 1024,
		"cpus":            float64(inspect.HostConfig.NanoCPUs) / 1e9,
		"cpuShares":       inspect.HostConfig.CPUShares,
		"cpusetCpus":      inspect.HostConfig.CpusetCpus,
		"cpusetMems":      inspect.HostConfig.CpusetMems,
		"cpuPeriod":       inspect.HostConfig.CPUPeriod,
		"cpuQuota":        inspect.HostConfig.CPUQuota,
		"pidsLimit":       inspect.HostConfig.PidsLimit,
		"oomKillDisable":  inspect.HostConfig.OomKillDisable,
		"privileged":      inspect.HostConfig.Privileged,
		"capAdd":          inspect.HostConfig.CapAdd,
		"capDrop":         inspect.HostConfig.CapDrop,
		"securityOpt":     inspect.HostConfig.SecurityOpt,
		"labels":          inspect.Config.Labels,
		"logDriver":       inspect.HostConfig.LogConfig.Type,
		"logOptions":      inspect.HostConfig.LogConfig.Config,
	})
}

func (r *Router) handleContainerUpdate(c *gin.Context) {
	var reqBody struct {
		ContainerID string  `json:"container_id"`
		Memory      int64   `json:"memory"`
		CPUs        float64 `json:"cpus"`
		Restart     string  `json:"restart"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	err := r.app.TerminalService.UpdateContainer(context.Background(), reqBody.ContainerID, reqBody.Memory, reqBody.CPUs, reqBody.Restart)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (r *Router) handleContainerRename(c *gin.Context) {
	var reqBody struct {
		ContainerID string `json:"container_id"`
		NewName     string `json:"new_name"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	err := r.app.TerminalService.RenameContainer(context.Background(), reqBody.ContainerID, reqBody.NewName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (r *Router) handleContainerRecreate(c *gin.Context) {
	var reqBody model.RecreateContainerRequest
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	execReq := &execlib.RecreateRequest{
		ContainerID: reqBody.ContainerID,
		Name:        reqBody.Name,
		Image:       reqBody.Image,
		Restart:     reqBody.Restart,
		Network:     reqBody.Network,
		Memory:      reqBody.Memory,
		CPUs:        reqBody.CPUs,
		Privileged:  reqBody.Privileged,
		TTY:         reqBody.TTY,
	}
	for _, port := range reqBody.Ports {
		execReq.Ports = append(execReq.Ports, execlib.PortMapping{
			Host:      port.Host,
			Container: port.Container,
		})
	}
	for _, volume := range reqBody.Volumes {
		execReq.Volumes = append(execReq.Volumes, execlib.VolumeMapping{
			Host:      volume.Host,
			Container: volume.Container,
		})
	}
	for _, env := range reqBody.Env {
		execReq.Env = append(execReq.Env, execlib.EnvVar{
			Key:   env.Key,
			Value: env.Value,
		})
	}
	newContainerID, err := r.app.TerminalService.RecreateContainer(context.Background(), execReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "container_id": newContainerID})
}

func (r *Router) handleContainerStats(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "container id is required"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	containerInfo, err := r.app.DockerRepo.ContainerInspect(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	configuredMemoryLimit := containerInfo.HostConfig.Memory
	statsResp, err := r.app.DockerRepo.ContainerStats(ctx, id, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer statsResp.Body.Close()

	decoder := json.NewDecoder(statsResp.Body)
	var stats1 types.StatsJSON
	if err := decoder.Decode(&stats1); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var stats types.StatsJSON
	if err := decoder.Decode(&stats); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	cpuPercent := 0.0
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats1.CPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats1.CPUStats.SystemUsage)
	onlineCPUs := stats.CPUStats.OnlineCPUs
	if onlineCPUs == 0 {
		onlineCPUs = uint32(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if onlineCPUs == 0 {
		onlineCPUs = 1
	}
	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(onlineCPUs) * 100.0
	}

	memoryLimit := int64(stats.MemoryStats.Limit)
	hasMemoryLimit := configuredMemoryLimit > 0
	if hasMemoryLimit {
		memoryLimit = configuredMemoryLimit
	}

	memoryUsage := int64(stats.MemoryStats.Usage)
	if stats.MemoryStats.Stats != nil {
		if cache, ok := stats.MemoryStats.Stats["cache"]; ok {
			memoryUsage -= int64(cache)
		} else if totalInactiveFile, ok := stats.MemoryStats.Stats["total_inactive_file"]; ok {
			memoryUsage -= int64(totalInactiveFile)
		} else if inactiveFile, ok := stats.MemoryStats.Stats["inactive_file"]; ok {
			memoryUsage -= int64(inactiveFile)
		}
	}
	if memoryUsage < 0 {
		memoryUsage = int64(stats.MemoryStats.Usage)
	}

	memoryPercent := 0.0
	if memoryLimit > 0 {
		memoryPercent = float64(memoryUsage) / float64(memoryLimit) * 100.0
	}

	var networkRx, networkTx int64
	for _, netStats := range stats.Networks {
		networkRx += int64(netStats.RxBytes)
		networkTx += int64(netStats.TxBytes)
	}

	var blockRead, blockWrite int64
	for _, bioEntry := range stats.BlkioStats.IoServiceBytesRecursive {
		switch bioEntry.Op {
		case "read", "Read":
			blockRead += int64(bioEntry.Value)
		case "write", "Write":
			blockWrite += int64(bioEntry.Value)
		}
	}

	c.JSON(http.StatusOK, model.ContainerStats{
		CPUPercent:     cpuPercent,
		CPUCores:       int(onlineCPUs),
		MemoryUsage:    memoryUsage,
		MemoryLimit:    memoryLimit,
		MemoryPercent:  memoryPercent,
		HasMemoryLimit: hasMemoryLimit,
		NetworkRx:      networkRx,
		NetworkTx:      networkTx,
		BlockRead:      blockRead,
		BlockWrite:     blockWrite,
		PIDs:           stats.PidsStats.Current,
	})
}

func (r *Router) handleContainerFilesList(c *gin.Context) {
	id := c.Query("id")
	path := c.DefaultQuery("path", "/")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "container id is required"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"ls", "-la", path},
	}

	execID, err := r.app.DockerRepo.ContainerExecCreate(ctx, id, execConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp, err := r.app.DockerRepo.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer resp.Close()

	var stdout, stderr bytes.Buffer
	_, _ = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
	if stderr.Len() > 0 && (strings.Contains(stderr.String(), "No such file") || strings.Contains(stderr.String(), "not found")) {
		c.JSON(http.StatusNotFound, gin.H{"error": "directory not found"})
		return
	}

	c.JSON(http.StatusOK, parseLsOutput(stdout.String(), path))
}

func (r *Router) handleContainerFileMkdir(c *gin.Context) {
	var reqBody struct {
		ContainerID string `json:"container_id"`
		Path        string `json:"path"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	execID, err := r.app.DockerRepo.ContainerExecCreate(ctx, reqBody.ContainerID, types.ExecConfig{
		AttachStderr: true,
		Cmd:          []string{"mkdir", "-p", reqBody.Path},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp, err := r.app.DockerRepo.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer resp.Close()
	var stderr bytes.Buffer
	_, _ = stdcopy.StdCopy(io.Discard, &stderr, resp.Reader)
	if stderr.Len() > 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": strings.TrimSpace(stderr.String())})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (r *Router) handleContainerFileDelete(c *gin.Context) {
	var reqBody struct {
		ContainerID string `json:"container_id"`
		Path        string `json:"path"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	cleanPath := path.Clean(reqBody.Path)
	for _, protected := range []string{"/", "/bin", "/sbin", "/usr", "/lib", "/lib64", "/etc", "/var", "/root", "/home"} {
		if cleanPath == protected {
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot delete protected path"})
			return
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	execID, err := r.app.DockerRepo.ContainerExecCreate(ctx, reqBody.ContainerID, types.ExecConfig{
		AttachStderr: true,
		Cmd:          []string{"rm", "-rf", reqBody.Path},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp, err := r.app.DockerRepo.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer resp.Close()
	var stderr bytes.Buffer
	_, _ = stdcopy.StdCopy(io.Discard, &stderr, resp.Reader)
	if stderr.Len() > 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": strings.TrimSpace(stderr.String())})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (r *Router) handleContainerFileUpload(c *gin.Context) {
	var reqBody struct {
		ContainerID string `json:"container_id"`
		Path        string `json:"path"`
		Filename    string `json:"filename"`
		Content     string `json:"content"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	fileContent, err := base64.StdEncoding.DecodeString(reqBody.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid base64 content"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name: reqBody.Filename,
		Mode: 0644,
		Size: int64(len(fileContent)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if _, err := tw.Write(fileContent); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := tw.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := r.app.DockerRepo.CopyToContainer(ctx, reqBody.ContainerID, reqBody.Path, &buf, types.CopyToContainerOptions{}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (r *Router) handleContainerFileDownload(c *gin.Context) {
	id := c.Query("id")
	filePath := c.Query("path")
	if id == "" || filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id or path"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	reader, stat, err := r.app.DockerRepo.CopyFromContainer(ctx, id, filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer reader.Close()
	tr := tar.NewReader(reader)
	hdr, err := tr.Next()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	fileName := path.Base(filePath)
	if stat.Mode.IsDir() {
		fileName += ".tar"
		c.Header("Content-Type", "application/x-tar")
	} else {
		c.Header("Content-Type", "application/octet-stream")
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	c.Header("Content-Length", fmt.Sprintf("%d", hdr.Size))
	_, _ = io.Copy(c.Writer, tr)
}

func (r *Router) handleContainerFileRead(c *gin.Context) {
	id := c.Query("id")
	filePath := c.Query("path")
	if id == "" || filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id or path"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	execID, err := r.app.DockerRepo.ContainerExecCreate(ctx, id, types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"head", "-c", "1048576", filePath},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp, err := r.app.DockerRepo.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer resp.Close()
	var stdout, stderr bytes.Buffer
	_, _ = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
	if stderr.Len() > 0 && strings.Contains(stderr.String(), "No such file") {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"content": stdout.String()})
}

func (r *Router) handleContainerFileWrite(c *gin.Context) {
	var reqBody struct {
		ContainerID string `json:"container_id"`
		Path        string `json:"path"`
		Content     string `json:"content"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	fileName := path.Base(reqBody.Path)
	dirPath := path.Dir(reqBody.Path)
	hdr := &tar.Header{
		Name: fileName,
		Mode: 0644,
		Size: int64(len(reqBody.Content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if _, err := tw.Write([]byte(reqBody.Content)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := tw.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := r.app.DockerRepo.CopyToContainer(ctx, reqBody.ContainerID, dirPath, &buf, types.CopyToContainerOptions{}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func parseLsOutput(output string, basePath string) []model.FileInfo {
	files := make([]model.FileInfo, 0)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		mode := fields[0]
		var name string
		var modTime string
		var size int64

		for i := 3; i < len(fields) && i < 6; i++ {
			if n, err := fmt.Sscanf(fields[i], "%d", &size); n == 1 && err == nil {
				remaining := fields[i+1:]
				if len(remaining) >= 4 {
					modTime = strings.Join(remaining[:3], " ")
					name = strings.Join(remaining[3:], " ")
				} else if len(remaining) >= 3 {
					modTime = strings.Join(remaining[:2], " ")
					name = strings.Join(remaining[2:], " ")
				} else if len(remaining) >= 2 {
					modTime = remaining[0]
					name = strings.Join(remaining[1:], " ")
				} else if len(remaining) == 1 {
					name = remaining[0]
				}
				break
			}
		}

		if name == "" || name == "." || name == ".." {
			continue
		}

		if strings.Contains(name, " -> ") {
			parts := strings.SplitN(name, " -> ", 2)
			name = parts[0]
		}

		isDir := strings.HasPrefix(mode, "d") || strings.HasPrefix(mode, "l")
		filePath := path.Join(basePath, name)

		files = append(files, model.FileInfo{
			Name:    name,
			Path:    filePath,
			Size:    size,
			Mode:    mode,
			ModTime: modTime,
			IsDir:   isDir,
		})
	}

	return files
}

// === Image Handlers ===

func (r *Router) handleImagesList(c *gin.Context) {
	images, err := r.app.ImageService.ListImages(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, images)
}

func (r *Router) handleImagesBuild(c *gin.Context) {
	var reqBody struct {
		ImageName  string `json:"image_name"`
		Tag        string `json:"tag"`
		Dockerfile string `json:"dockerfile"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	if reqBody.ImageName == "" || reqBody.Dockerfile == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "镜像名称和 Dockerfile 不能为空"})
		return
	}
	if reqBody.Tag == "" {
		reqBody.Tag = "latest"
	}
	imageTag := reqBody.ImageName + ":" + reqBody.Tag

	cmd := exec.Command("docker", "build", "-t", imageTag, "-")
	cmd.Stdin = strings.NewReader(reqBody.Dockerfile)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.Status(http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(c.Writer, "data: {\"type\":\"start\",\"message\":\"开始构建镜像 %s\"}\n\n", imageTag)
	flusher.Flush()

	output, err := cmd.CombinedOutput()
	for _, line := range strings.Split(string(output), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			line = strings.ReplaceAll(line, "\\", "\\\\")
			line = strings.ReplaceAll(line, "\"", "\\\"")
			fmt.Fprintf(c.Writer, "data: {\"type\":\"log\",\"message\":\"%s\"}\n\n", line)
			flusher.Flush()
		}
	}

	r.app.CacheRepo.InvalidateImages()
	if err != nil {
		fmt.Fprintf(c.Writer, "data: {\"type\":\"error\",\"message\":\"构建失败: %v\"}\n\n", err)
	} else {
		fmt.Fprintf(c.Writer, "data: {\"type\":\"success\",\"message\":\"镜像 %s 构建成功！\"}\n\n", imageTag)
	}
	flusher.Flush()
}

func (r *Router) handleImagesPull(c *gin.Context) {
	var reqBody struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	err := r.app.ImageService.PullImage(c.Request.Context(), reqBody.Name, r.app.RegistryService.PullOptions(reqBody.Name))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (r *Router) handleImagesRemove(c *gin.Context) {
	var reqBody struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	err := r.app.ImageService.RemoveImage(context.Background(), reqBody.ID, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// === Network Handlers ===

func (r *Router) handleNetworksList(c *gin.Context) {
	networks, err := r.app.NetworkService.ListNetworks(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, networks)
}

func (r *Router) handleNetworksCreate(c *gin.Context) {
	var reqBody struct {
		Name     string `json:"name"`
		Driver   string `json:"driver"`
		Subnet   string `json:"subnet"`
		Gateway  string `json:"gateway"`
		Internal bool   `json:"internal"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	if reqBody.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "网络名称不能为空"})
		return
	}
	if reqBody.Driver == "" {
		reqBody.Driver = "bridge"
	}

	ipamConfig := []network.IPAMConfig{}
	if reqBody.Subnet != "" {
		cfg := network.IPAMConfig{Subnet: reqBody.Subnet}
		if reqBody.Gateway != "" {
			cfg.Gateway = reqBody.Gateway
		}
		ipamConfig = append(ipamConfig, cfg)
	}

	opts := types.NetworkCreate{Driver: reqBody.Driver, Internal: reqBody.Internal}
	if len(ipamConfig) > 0 {
		opts.IPAM = &network.IPAM{Config: ipamConfig}
	}

	_, err := r.app.DockerRepo.NetworkCreate(context.Background(), reqBody.Name, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (r *Router) handleNetworksRemove(c *gin.Context) {
	var reqBody struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	err := r.app.NetworkService.RemoveNetwork(context.Background(), reqBody.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (r *Router) handleNetworksInspect(c *gin.Context) {
	id := c.Query("id")
	net, err := r.app.NetworkService.InspectNetwork(context.Background(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, net)
}

func (r *Router) handleNetworksConnect(c *gin.Context) {
	var reqBody struct {
		NetworkID   string `json:"network_id"`
		ContainerID string `json:"container_id"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	err := r.app.NetworkService.ConnectNetwork(context.Background(), reqBody.NetworkID, reqBody.ContainerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (r *Router) handleNetworksDisconnect(c *gin.Context) {
	var reqBody struct {
		NetworkID   string `json:"network_id"`
		ContainerID string `json:"container_id"`
		Force       bool   `json:"force"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	err := r.app.NetworkService.DisconnectNetwork(context.Background(), reqBody.NetworkID, reqBody.ContainerID, reqBody.Force)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// === Volume Handlers ===

func (r *Router) handleVolumesList(c *gin.Context) {
	volumes, err := r.app.VolumeService.ListVolumes(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, volumes)
}

func (r *Router) handleVolumesCreate(c *gin.Context) {
	var reqBody struct {
		Name       string            `json:"name"`
		Driver     string            `json:"driver"`
		DriverOpts map[string]string `json:"driverOpts"`
		Labels     map[string]string `json:"labels"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	driver := reqBody.Driver
	if driver == "" {
		driver = "local"
	}
	err := r.app.VolumeService.CreateVolume(context.Background(), &model.CreateVolumeRequest{
		Name:       reqBody.Name,
		Driver:     driver,
		DriverOpts: reqBody.DriverOpts,
		Labels:     reqBody.Labels,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"name": reqBody.Name, "driver": driver, "status": "success"})
}

func (r *Router) handleVolumesRemove(c *gin.Context) {
	var reqBody struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	err := r.app.VolumeService.RemoveVolume(context.Background(), reqBody.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (r *Router) handleVolumesPrune(c *gin.Context) {
	result, err := r.app.VolumeService.PruneVolumes(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// === Compose Handlers ===

func (r *Router) handleComposeList(c *gin.Context) {
	projects, err := r.app.ComposeService.ListProjects()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, projects)
}

func (r *Router) handleComposeCreate(c *gin.Context) {
	var reqBody struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	err := r.app.ComposeService.CreateProject(reqBody.Name, reqBody.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (r *Router) handleComposeFile(c *gin.Context) {
	name := c.Query("project")
	if c.Request.Method == http.MethodGet {
		content, err := r.app.ComposeService.GetProjectFile(name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.String(http.StatusOK, content)
	} else if c.Request.Method == http.MethodPost {
		var reqBody struct {
			Project string `json:"project"`
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&reqBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
			return
		}
		targetProject := name
		if strings.TrimSpace(targetProject) == "" {
			targetProject = strings.TrimSpace(reqBody.Project)
		}
		err := r.app.ComposeService.SaveProjectFile(targetProject, reqBody.Content)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

func (r *Router) handleComposeSaveFile(c *gin.Context) {
	r.handleComposeFile(c)
}

func (r *Router) handleComposeAction(c *gin.Context) {
	var reqBody struct {
		Project string `json:"project"`
		Action  string `json:"action"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	var output bytes.Buffer
	err := r.app.ComposeService.ExecuteAction(reqBody.Project, reqBody.Action, &output)
	logs := strings.ReplaceAll(output.String(), "\r\n", "\n")
	for _, line := range strings.Split(logs, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fmt.Fprintf(c.Writer, "event: log\n")
		fmt.Fprintf(c.Writer, "data: %s\n\n", line)
		flusher.Flush()
	}
	if err != nil {
		fmt.Fprintf(c.Writer, "event: error\n")
		fmt.Fprintf(c.Writer, "data: %s\n\n", err.Error())
		fmt.Fprintf(c.Writer, "event: done\n")
		fmt.Fprintf(c.Writer, "data: failed\n\n")
		flusher.Flush()
		return
	}
	fmt.Fprintf(c.Writer, "event: done\n")
	fmt.Fprintf(c.Writer, "data: success\n\n")
	flusher.Flush()
}

func (r *Router) handleComposeDelete(c *gin.Context) {
	var reqBody struct {
		Project string `json:"project"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	err := r.app.ComposeService.DeleteProject(reqBody.Project)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (r *Router) handleComposeStatus(c *gin.Context) {
	name := c.Query("project")
	status, err := r.app.ComposeService.FetchProjectStatus(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

// === Registry Handlers ===

func (r *Router) handleRegistriesList(c *gin.Context) {
	registries, err := r.app.RegistryService.ListRegistries()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, registries)
}

func (r *Router) handleRegistriesCreate(c *gin.Context) {
	var reqBody struct {
		Name        string `json:"name"`
		URL         string `json:"url"`
		Username    string `json:"username"`
		Password    string `json:"password"`
		PreviousURL string `json:"previous_url"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	err := r.app.RegistryService.SaveRegistry(&repository.RegistryRecord{
		Name:     reqBody.Name,
		URL:      reqBody.URL,
		Username: reqBody.Username,
		Password: reqBody.Password,
	}, reqBody.PreviousURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (r *Router) handleRegistriesRemove(c *gin.Context) {
	var reqBody struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	err := r.app.RegistryService.DeleteRegistry(reqBody.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (r *Router) handleRegistriesTest(c *gin.Context) {
	var reqBody struct {
		URL      string `json:"url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	success, msg := r.app.RegistryService.TestRegistry(reqBody.URL, reqBody.Username, reqBody.Password)
	c.JSON(http.StatusOK, gin.H{"success": success, "message": msg})
}

// === Docker Config Handlers ===

func (r *Router) handleDockerInfo(c *gin.Context) {
	ctx := context.Background()

	info, err := r.app.DockerRepo.Info(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	version, err := r.app.DockerRepo.ServerVersion(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.DockerInfo{
		ServerVersion:      version.Version,
		APIVersion:         version.APIVersion,
		OS:                 info.OSType,
		Arch:               info.Architecture,
		KernelVersion:      info.KernelVersion,
		OperatingSystem:    info.OperatingSystem,
		Containers:         info.Containers,
		ContainersRunning:  info.ContainersRunning,
		ContainersPaused:   info.ContainersPaused,
		ContainersStopped:  info.ContainersStopped,
		Images:             info.Images,
		Driver:             info.Driver,
		MemoryLimit:        info.MemoryLimit,
		SwapLimit:          info.SwapLimit,
		CPUCfsPeriod:       info.CPUCfsPeriod,
		CPUCfsQuota:        info.CPUCfsQuota,
		IPv4Forwarding:     info.IPv4Forwarding,
		DockerRootDir:      info.DockerRootDir,
		IndexServerAddress: info.IndexServerAddress,
	})
}

func (r *Router) handleDockerConfigGet(c *gin.Context) {
	dc, err := service.LoadDaemonConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dc.ToPublicConfig())
}

func (r *Router) handleDockerConfigUpdate(c *gin.Context) {
	var cfg model.DockerConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	dc := service.FromPublicConfig(&cfg)
	err := service.SaveDaemonConfig(dc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "配置已保存，需要重启 Docker 服务生效"})
}

func (r *Router) handleDockerRestart(c *gin.Context) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		exec.Command("net", "stop", "com.docker.service").Run()
		cmd = exec.Command("net", "start", "com.docker.service")
	} else {
		cmd = exec.Command("sudo", "systemctl", "restart", "docker")
	}
	cmd.Run()
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// === Agent Handlers ===

func (r *Router) handleAgentChat(c *gin.Context) {
	var reqBody model.AgentChatRequest
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	// Save user message
	_ = r.app.AgentService.SaveChatMessage("user", reqBody.Message)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	var assistantReply strings.Builder
	sendChunk := func(chunk string) error {
		assistantReply.WriteString(chunk)
		lines := strings.Split(chunk, "\n")
		for _, line := range lines {
			if _, err := fmt.Fprintf(c.Writer, "data: %s\n", line); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(c.Writer, "\n"); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	if err := r.app.AgentService.StreamChat(c.Request.Context(), reqBody, sendChunk); err != nil {
		if assistantReply.Len() == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		_ = sendChunk(fmt.Sprintf("\n\n**错误**: %s", err.Error()))
		return
	}

	if strings.TrimSpace(assistantReply.String()) != "" {
		_ = r.app.AgentService.SaveChatMessage("assistant", assistantReply.String())
	}
}

func (r *Router) handleAgentHistory(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet:
		history, err := r.app.AgentService.GetChatHistory(100)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, history)
	case http.MethodPost:
		var reqBody struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		if err := c.ShouldBindJSON(&reqBody); err != nil || strings.TrimSpace(reqBody.Role) == "" || strings.TrimSpace(reqBody.Content) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
			return
		}
		if err := r.app.AgentService.SaveChatMessage(reqBody.Role, reqBody.Content); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	case http.MethodDelete:
		if err := r.app.AgentService.ClearChatHistory(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	}
}

func (r *Router) handleAgentConfig(c *gin.Context) {
	if c.Request.Method == http.MethodGet {
		cfg := r.app.AgentService.MaskedConfig()
		c.JSON(http.StatusOK, cfg)
	} else if c.Request.Method == http.MethodPost {
		var reqBody service.AgentConfig
		if err := c.ShouldBindJSON(&reqBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
			return
		}
		err := r.app.AgentService.SaveConfig(reqBody)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	} else {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	}
}

func (r *Router) handleAgentModels(c *gin.Context) {
	var req struct {
		APIURL    string `json:"api_url"`
		APIFormat string `json:"api_format"`
		APIKey    string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	models, err := r.app.AgentService.ListModels(c.Request.Context(), req.APIURL, req.APIFormat, req.APIKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"models": models})
}

func (r *Router) handleAgentTest(c *gin.Context) {
	var req service.AgentConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	reply, err := r.app.AgentService.TestConnection(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"reply": reply})
}

// === System Stats Handler ===

func (r *Router) handleSystemStats(c *gin.Context) {
	cpu, _ := tool.GetCPUUsage()
	mem, _ := tool.GetMemoryUsage()
	disk, _ := tool.GetDiskUsage()
	c.JSON(http.StatusOK, model.SystemStats{
		CPU:    cpu,
		Memory: mem.Usage,
		Disk:   disk.Usage,
		Time:   time.Now().Format("2006-01-02 15:04:05"),
	})
}

// === Node Handlers ===

func (r *Router) handleNodesList(c *gin.Context) {
	nodes := r.app.NodeService.GetAllNodes()
	c.JSON(http.StatusOK, nodes)
}

func (r *Router) handleNodeRegister(c *gin.Context) {
	var node model.NodeInfo
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	r.app.NodeService.RegisterNode(&node)
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (r *Router) handleNodeHeartbeat(c *gin.Context) {
	var reqBody struct {
		NodeID     string  `json:"node_id"`
		CPU        float64 `json:"cpu"`
		Memory     float64 `json:"memory"`
		Disk       float64 `json:"disk"`
		Containers int     `json:"containers"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	r.app.NodeService.UpdateHeartbeat(reqBody.NodeID, model.SystemStats{
		CPU:    reqBody.CPU,
		Memory: reqBody.Memory,
		Disk:   reqBody.Disk,
	}, reqBody.Containers)
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// === Scheduler Handlers ===

func (r *Router) handleContainerSchedule(c *gin.Context) {
	var reqBody model.ScheduleRequest
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	resp, err := r.app.Scheduler.ScheduleContainer(c.Request.Context(), &reqBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (r *Router) handleAllContainers(c *gin.Context) {
	containers, err := r.app.ContainerService.ListContainers(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, containers)
}

// === WebSocket Terminal Handler (Gin + gorilla/websocket) ===

var ginWsUpgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		host := r.Host
		// Allow same-origin and Vite dev proxy (localhost:3000 -> backend)
		if origin == "" {
			return true
		}
		// Production: exact host match
		if origin == "http://"+host || origin == "https://"+host {
			return true
		}
		// Dev proxy: Vite on :3000 proxies to backend
		if origin == "http://localhost:3000" && strings.HasPrefix(host, "localhost:") {
			return true
		}
		return false
	},
}

func (r *Router) handleContainerTerminalWS(c *gin.Context) {
	containerID := c.Query("id")
	if containerID == "" {
		c.String(http.StatusBadRequest, "容器ID不能为空")
		return
	}

	// Validate token
	token := c.Query("token")
	if token == "" {
		c.String(http.StatusUnauthorized, "未授权")
		return
	}
	if !middleware.ValidateToken(token, r.app.JWTSecret) {
		c.String(http.StatusUnauthorized, "Token 无效")
		return
	}

	conn, err := ginWsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx := context.Background()
	shell := r.detectShell(ctx, containerID)

	execID, err := r.app.DockerRepo.ContainerExecCreate(ctx, containerID, types.ExecConfig{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          []string{shell},
	})
	if err != nil {
		conn.WriteMessage(ws.TextMessage, []byte("\r\nError: "+err.Error()+"\r\n"))
		return
	}

	hijackedResp, err := r.app.DockerRepo.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{Tty: true})
	if err != nil {
		conn.WriteMessage(ws.TextMessage, []byte("\r\nError: "+err.Error()+"\r\n"))
		return
	}
	defer hijackedResp.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := hijackedResp.Reader.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("[Terminal] Read error: %v", err)
				}
				return
			}
			if n > 0 {
				if err := conn.WriteMessage(ws.BinaryMessage, buf[:n]); err != nil {
					return
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-done:
				return
			default:
			}
			mt, message, err := conn.ReadMessage()
			if err != nil {
				if ws.IsUnexpectedCloseError(err, ws.CloseGoingAway, ws.CloseAbnormalClosure, ws.CloseNoStatusReceived) {
					log.Printf("[Terminal] Read error: %v", err)
				}
				hijackedResp.Close()
				return
			}
			if mt == ws.TextMessage && len(message) > 0 && message[0] == '{' {
				var resize struct {
					Type string `json:"type"`
					Cols int    `json:"cols"`
					Rows int    `json:"rows"`
				}
				if json.Unmarshal(message, &resize) == nil && resize.Type == "resize" {
					r.app.DockerRepo.ContainerExecResize(ctx, execID.ID, container.ResizeOptions{
						Height: uint(resize.Rows),
						Width:  uint(resize.Cols),
					})
					continue
				}
			}
			if _, err := hijackedResp.Conn.Write(message); err != nil {
				return
			}
		}
	}()

	<-done
}

func (r *Router) detectShell(ctx context.Context, containerID string) string {
	shells := []string{"/bin/sh", "/bin/bash", "/bin/ash", "sh"}
	for _, shell := range shells {
		execID, err := r.app.DockerRepo.ContainerExecCreate(ctx, containerID, types.ExecConfig{
			AttachStdout: true,
			AttachStderr: true,
			Cmd:          []string{shell, "-c", "exit 0"},
		})
		if err != nil {
			continue
		}
		resp, err := r.app.DockerRepo.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
		if err != nil {
			continue
		}
		resp.Close()
		inspectResp, err := r.app.DockerRepo.ContainerExecInspect(ctx, execID.ID)
		if err == nil && inspectResp.ExitCode == 0 {
			return shell
		}
	}
	return "/bin/sh"
}
