package router

import (
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"rabbit-panel/middleware"
)

// containerNodeProxy 在 Master 上把带目标节点的容器、网络、存储卷和 Compose 请求转发到对应 Worker。
func (r *Router) containerNodeProxy() gin.HandlerFunc {
	return func(c *gin.Context) {
		if r.app.Mode != "master" || !isNodeProxyPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		nodeID := strings.TrimSpace(c.GetHeader("X-Target-Node"))
		if nodeID == "" {
			nodeID = strings.TrimSpace(c.Query("target_node"))
		}
		if nodeID == "" {
			c.Next()
			return
		}

		claims, err := middleware.ParseUserToken(middleware.TokenFromRequest(c.Request), r.app.JWTSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未授权，请先登录"})
			return
		}
		if claims.NeedChangePassword {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":                "需要修改密码",
				"need_change_password": true,
			})
			return
		}

		address, err := r.app.NodeService.ProxyTarget(nodeID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if address == "" {
			c.Next()
			return
		}

		target := &url.URL{Scheme: "http", Host: address}
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.FlushInterval = -1
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			req.Host = target.Host
			req.Header.Del("X-Target-Node")
			query := req.URL.Query()
			query.Del("target_node")
			req.URL.RawQuery = query.Encode()
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, proxyErr error) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(gin.H{"error": "节点不可达: " + proxyErr.Error()})
		}
		proxy.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}

func isNodeProxyPath(path string) bool {
	if isContainerProxyPath(path) {
		return true
	}
	for _, prefix := range []string{"/api/networks", "/api/volumes", "/api/compose", "/api/registries"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func isContainerProxyPath(path string) bool {
	if path != "/api/containers" && !strings.HasPrefix(path, "/api/containers/") {
		return false
	}
	if path == "/api/containers/schedule" || strings.HasPrefix(path, "/api/containers/schedule/") {
		return false
	}
	if path == "/api/containers/all" || strings.HasPrefix(path, "/api/containers/all/") {
		return false
	}
	return true
}
