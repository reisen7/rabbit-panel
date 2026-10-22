/*
 * Rabbit Panel - 容器运维面板
 * Author: reisen7
 * Email: reisen7@foxmail.com
 */
package config

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	serverReadHeaderTimeout = 30 * time.Second
	serverIdleTimeout       = 180 * time.Second
	serverShutdownTimeout   = 10 * time.Second
)

// Run 启动 HTTP / HTTPS 服务，阻塞直到收到退出信号或服务异常。
func (app *App) Run() error {
	addr := net.JoinHostPort(app.Host, app.Port)
	main := app.newServer(addr, app.Engine)

	errCh := make(chan error, 2)
	servers := []*http.Server{main}

	// 可选的 HTTP → HTTPS 跳转端口（同时提供健康检查）
	if app.TLS.Enabled && strings.TrimSpace(app.HTTPPort) != "" && app.HTTPPort != app.Port {
		redirectAddr := net.JoinHostPort(app.Host, app.HTTPPort)
		redirect := app.newServer(redirectAddr, app.redirectHandler())
		servers = append(servers, redirect)
		go func() {
			log.Printf("HTTP 端口 %s 将重定向到 HTTPS:%s", redirectAddr, app.RedirectPort)
			if err := redirect.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}()
	}

	go func() {
		if app.TLS.Enabled {
			log.Printf("HTTPS 监听 %s（证书 %s）", addr, app.TLS.CertFile)
			if err := main.ListenAndServeTLS(app.TLS.CertFile, app.TLS.KeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
			return
		}
		log.Printf("HTTP 监听 %s", addr)
		if err := main.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-quit:
		log.Printf("收到信号 %s，正在关闭服务...", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
	defer cancel()
	for _, srv := range servers {
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("关闭服务失败: %v", err)
		}
	}
	return nil
}

func (app *App) newServer(addr string, handler http.Handler) *http.Server {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		IdleTimeout:       serverIdleTimeout,
		ErrorLog:          log.New(os.Stderr, "[http] ", log.LstdFlags),
	}
	if app.TLS.Enabled {
		srv.TLSConfig = app.TLS.Config()
		// 仅启用 HTTP/1.1：容器终端依赖 WebSocket 升级，而 gorilla/websocket 不支持 h2
		protocols := new(http.Protocols)
		protocols.SetHTTP1(true)
		srv.Protocols = protocols
	}
	return srv
}

// redirectHandler 把 HTTP 请求 301 到 HTTPS，并为负载均衡提供轻量健康检查。
func (app *App) redirectHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("OK"))
			return
		}
		// 前面已有反向代理做了 TLS 终止时，直接交给后端处理，避免跳转死循环
		if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			app.Engine.ServeHTTP(w, r)
			return
		}
		target := "https://" + redirectHost(r.Host, app.RedirectPort) + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
}

// redirectHost 生成跳转目标的主机部分，标准端口不附加端口号。
func redirectHost(rawHost, port string) string {
	host := strings.TrimSpace(rawHost)
	if host == "" {
		host = "localhost"
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if port == "" || port == "443" {
		if strings.Contains(host, ":") {
			return "[" + host + "]"
		}
		return host
	}
	return net.JoinHostPort(host, port)
}
