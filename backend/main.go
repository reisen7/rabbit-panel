/*
 * Rabbit Panel - 容器运维面板
 * Author: reisen7
 * Email: reisen7@foxmail.com
 */
package main

import (
	"embed"
	"log"
	"runtime"

	"rabbit-panel/config"
	"rabbit-panel/router"
	"rabbit-panel/service"
)

//go:embed .dist
var embeddedDist embed.FS

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	app, err := config.NewApp(service.BuildInfo{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
	})
	if err != nil {
		log.Fatalf("初始化应用失败: %v", err)
	}
	app.StaticFS = embeddedDist

	r := router.NewRouter(app)
	r.Register()

	logStartup(app)

	if err := app.Run(); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

func logStartup(app *config.App) {
	scheme := app.Scheme()

	log.Printf("Rabbit Panel started")
	if app.TLS.Enabled {
		log.Printf("Listen: %s:%s (HTTPS)", app.Host, app.Port)
	} else {
		log.Printf("Listen: %s:%s (HTTP)", app.Host, app.Port)
	}
	log.Printf("Local URL: %s://localhost:%s", scheme, app.Port)
	if app.TLS.Enabled && app.HTTPPort != "" && app.HTTPPort != app.Port {
		log.Printf("HTTP redirect: %s:%s -> https", app.Host, app.HTTPPort)
	}
	if app.Mode == "master" {
		log.Printf("Mode: master")
	} else {
		log.Printf("Mode: worker")
	}

	log.Printf("Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	log.Printf("Version: %s commit=%s buildTime=%s", app.BuildInfo.Version, app.BuildInfo.Commit, app.BuildInfo.BuildTime)
	log.Printf("Press Ctrl+C to stop")
}
