package middleware

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims JWT Claims
type JWTClaims struct {
	Username           string `json:"username"`
	NeedChangePassword bool   `json:"need_change_password"`
	jwt.RegisteredClaims
}

// AuthMiddleware creates a Gin JWT authentication middleware
func AuthMiddleware(jwtSecret []byte, publicPaths ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Check public paths
		for _, p := range publicPaths {
			if path == p {
				c.Next()
				return
			}
		}

		// Get token from Authorization header or Cookie
		token := c.GetHeader("Authorization")
		if token == "" {
			if cookie, err := c.Cookie("token"); err == nil {
				token = cookie
			}
		} else {
			token = strings.TrimPrefix(token, "Bearer ")
		}

		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未授权，请先登录"})
			return
		}

		// Validate token
		claims, err := validateToken(token, jwtSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token 无效或已过期"})
			return
		}

		// Check if password change required
		if claims.NeedChangePassword && path != "/api/auth/change-password" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":                "需要修改密码",
				"need_change_password": true,
			})
			return
		}

		// Set username in context for downstream handlers
		c.Set("username", claims.Username)
		c.Next()
	}
}

// validateToken validates a JWT token and returns its claims
func validateToken(tokenString string, jwtSecret []byte) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	if time.Now().After(claims.ExpiresAt.Time) {
		return nil, jwt.ErrTokenExpired
	}
	return claims, nil
}

// GenerateToken generates a JWT token for the given user
func GenerateToken(username string, needChangePassword bool, jwtSecret []byte) (string, error) {
	claims := &JWTClaims{
		Username:           username,
		NeedChangePassword: needChangePassword,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// NodeAuthMiddleware creates a Gin node authentication middleware
func NodeAuthMiddleware(nodeSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeID := c.GetHeader("X-Node-ID")
		nodeToken := c.GetHeader("X-Node-Token")

		if nodeID == "" || nodeToken == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "节点认证失败: 缺少节点ID或Token"})
			return
		}

		if !verifyNodeToken(nodeID, nodeToken, nodeSecret) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "节点认证失败: Token无效"})
			return
		}

		c.Next()
	}
}

// AuthOrNodeMiddleware creates a Gin middleware that accepts either user auth or node auth
func AuthOrNodeMiddleware(jwtSecret []byte, nodeSecret string, publicPaths ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Check public paths
		for _, p := range publicPaths {
			if path == p {
				c.Next()
				return
			}
		}

		// Try node auth first
		nodeID := c.GetHeader("X-Node-ID")
		nodeToken := c.GetHeader("X-Node-Token")
		if nodeID != "" && nodeToken != "" {
			if verifyNodeToken(nodeID, nodeToken, nodeSecret) {
				c.Set("node_id", nodeID)
				c.Next()
				return
			}
		}

		// Fall back to user auth
		token := c.GetHeader("Authorization")
		if token == "" {
			if cookie, err := c.Cookie("token"); err == nil {
				token = cookie
			}
		} else {
			token = strings.TrimPrefix(token, "Bearer ")
		}

		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "未授权，请先登录"})
			return
		}

		claims, err := validateToken(token, jwtSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token 无效或已过期"})
			return
		}

		if claims.NeedChangePassword && path != "/api/auth/change-password" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":                "需要修改密码",
				"need_change_password": true,
			})
			return
		}

		c.Set("username", claims.Username)
		c.Next()
	}
}

// verifyNodeToken verifies a node token using HMAC-SHA256
func verifyNodeToken(nodeID, token, secret string) bool {
	if nodeID == "" || token == "" {
		return false
	}
	// Token format: HMAC-SHA256(nodeID, secret) as hex
	h := sha256.New()
	h.Write([]byte(nodeID))
	h.Write([]byte(secret))
	expected := hex.EncodeToString(h.Sum(nil))
	return token == expected
}

// ValidateNodeToken validates a node token (exported for external use)
func ValidateNodeToken(nodeID, token, secret string) bool {
	return verifyNodeToken(nodeID, token, secret)
}

// ValidateToken validates a JWT token string and returns true if valid.
// Useful for WebSocket handlers that can't use Gin middleware.
func ValidateToken(tokenString string, jwtSecret []byte) bool {
	_, err := validateToken(tokenString, jwtSecret)
	return err == nil
}

// TokenFromRequest 从 Authorization、Cookie 或 token 查询参数中取出用户令牌。
func TokenFromRequest(r *http.Request) string {
	token := r.Header.Get("Authorization")
	if token != "" {
		return strings.TrimPrefix(token, "Bearer ")
	}
	if cookie, err := r.Cookie("token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	return r.URL.Query().Get("token")
}

// ParseUserToken 解析用户 JWT，供需要在转发前确认登录态的逻辑使用。
func ParseUserToken(tokenString string, jwtSecret []byte) (*JWTClaims, error) {
	return validateToken(tokenString, jwtSecret)
}

// RateLimitMiddleware creates a rate limiting middleware.
// maxRequests: max requests per window
// window: time window duration
func RateLimitMiddleware(maxRequests int, window time.Duration) gin.HandlerFunc {
	type record struct {
		count    int
		lastSeen time.Time
	}

	var (
		mu      sync.Mutex
		records = make(map[string]*record)
		ticker  = time.NewTicker(window)
	)

	// Cleanup old records periodically
	go func() {
		for range ticker.C {
			mu.Lock()
			now := time.Now()
			for ip, rec := range records {
				if now.Sub(rec.lastSeen) > window {
					delete(records, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		mu.Lock()
		rec, exists := records[ip]
		if !exists {
			records[ip] = &record{count: 1, lastSeen: time.Now()}
			mu.Unlock()
			c.Next()
			return
		}
		if time.Since(rec.lastSeen) > window {
			rec.count = 1
			rec.lastSeen = time.Now()
			mu.Unlock()
			c.Next()
			return
		}
		rec.lastSeen = time.Now()
		rec.count++
		count := rec.count
		mu.Unlock()

		if count > maxRequests {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "请求过于频繁，请稍后再试",
			})
			return
		}
		c.Next()
	}
}

// GenerateSecureToken generates a cryptographically random token
func GenerateSecureToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()
		latency := time.Since(start)
		gin.DefaultWriter.Write([]byte(
			fmt.Sprintf("[GIN] %s %s %d %v\n",
				c.Request.Method,
				path,
				c.Writer.Status(),
				latency,
			),
		))
	}
}
