package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	_ "github.com/mattn/go-sqlite3"
)

// 获取节点密钥（从环境变量或使用默认值）
func getNodeSecret() string {
	secret := os.Getenv("NODE_SECRET")
	if secret == "" {
		// 默认密钥（生产环境必须设置环境变量）
		secret = "rabbit-panel-node-secret-change-in-production"
		log.Println("警告: 使用默认节点密钥，生产环境请设置 NODE_SECRET 环境变量")
	}
	return secret
}

// 生成节点认证 Token
func generateNodeToken(nodeID string) string {
	h := hmac.New(sha256.New, []byte(nodeSecret))
	h.Write([]byte(nodeID + ":" + time.Now().Format("2006-01-02 15:04"))) // 每小时更新一次
	return hex.EncodeToString(h.Sum(nil))
}

// 验证节点 Token
func verifyNodeToken(nodeID, token string) bool {
	// 验证当前小时和上一小时的 token（允许1小时的时间差）
	now := time.Now()
	for i := -1; i <= 1; i++ {
		t := now.Add(time.Duration(i) * time.Hour)
		expectedToken := generateNodeTokenForTime(nodeID, t)
		if hmac.Equal([]byte(token), []byte(expectedToken)) {
			return true
		}
	}
	return false
}

// 为指定时间生成 token
func generateNodeTokenForTime(nodeID string, t time.Time) string {
	h := hmac.New(sha256.New, []byte(nodeSecret))
	h.Write([]byte(nodeID + ":" + t.Format("2006-01-02 15:04")))
	return hex.EncodeToString(h.Sum(nil))
}

// 节点间通信密钥（用于 Master 和 Worker 之间的认证）
var nodeSecret = getNodeSecret()

// 节点认证中间件
func nodeAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 获取节点 ID 和 Token
		nodeID := r.Header.Get("X-Node-ID")
		nodeToken := r.Header.Get("X-Node-Token")
		
		if nodeID == "" || nodeToken == "" {
			http.Error(w, `{"error": "节点认证失败: 缺少节点ID或Token"}`, http.StatusUnauthorized)
			return
		}
		
		// 验证 Token
		if !verifyNodeToken(nodeID, nodeToken) {
			http.Error(w, `{"error": "节点认证失败: Token无效"}`, http.StatusUnauthorized)
			return
		}
		
		next(w, r)
	}
}

// 用户认证或节点认证中间件（支持两种认证方式）
func authOrNodeAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 先尝试节点认证
		nodeID := r.Header.Get("X-Node-ID")
		nodeToken := r.Header.Get("X-Node-Token")
		
		if nodeID != "" && nodeToken != "" {
			// 验证节点 Token
			if verifyNodeToken(nodeID, nodeToken) {
				next(w, r)
				return
			}
		}
		
		// 如果节点认证失败，尝试用户认证
		authMiddleware(next)(w, r)
	}
}

// JWT 密钥（生产环境应该使用环境变量或配置文件）
var jwtSecret = []byte("rabbit-panel-secret-key-change-in-production")

// 会话管理
var (
	sessions = make(map[string]*Session)
	sessionMutex sync.RWMutex
)

// Session 会话信息
type Session struct {
	Username   string
	ExpiresAt  time.Time
	NeedChangePassword bool
}

// 用户信息
type User struct {
	ID                int       `json:"id"`
	Username          string    `json:"username"`
	PasswordHash      string    `json:"-"`
	NeedChangePassword bool     `json:"need_change_password"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// 登录请求
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// 登录响应
type LoginResponse struct {
	Token              string `json:"token"`
	NeedChangePassword bool   `json:"need_change_password"`
	Message            string `json:"message"`
}

// 全局数据库连接
var authDB *sql.DB

// 初始化认证数据库
func initAuthDB() error {
	var err error
	authDB, err = sql.Open("sqlite3", "./auth.db")
	if err != nil {
		return fmt.Errorf("打开数据库失败: %v", err)
	}

	// 创建用户表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		need_change_password INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	_, err = authDB.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("创建表失败: %v", err)
	}

	// 检查是否有用户，如果没有则创建默认管理员
	var count int
	err = authDB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return fmt.Errorf("查询用户数失败: %v", err)
	}

	if count == 0 {
		// 创建默认管理员账户 admin/admin
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("生成密码哈希失败: %v", err)
		}

		_, err = authDB.Exec(
			"INSERT INTO users (username, password_hash, need_change_password) VALUES (?, ?, ?)",
			"admin", string(hashedPassword), 1,
		)
		if err != nil {
			return fmt.Errorf("创建默认用户失败: %v", err)
		}
		log.Println("已创建默认管理员账户: admin/admin")
	}

	return nil
}

// 验证密码强度
func validatePasswordStrength(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("密码长度至少8位")
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	for _, char := range password {
		switch {
		case 'A' <= char && char <= 'Z':
			hasUpper = true
		case 'a' <= char && char <= 'z':
			hasLower = true
		case '0' <= char && char <= '9':
			hasDigit = true
		case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;:,.<>?", char):
			hasSpecial = true
		}
	}

	var missing []string
	if !hasUpper {
		missing = append(missing, "大写字母")
	}
	if !hasLower {
		missing = append(missing, "小写字母")
	}
	if !hasDigit {
		missing = append(missing, "数字")
	}
	if !hasSpecial {
		missing = append(missing, "特殊字符")
	}

	if len(missing) > 0 {
		return fmt.Errorf("密码必须包含: %s", strings.Join(missing, "、"))
	}

	return nil
}

// 验证用户登录
func verifyUser(username, password string) (*User, error) {
	var user User
	var needChangePassword int

	err := authDB.QueryRow(
		"SELECT id, username, password_hash, need_change_password FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &needChangePassword)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("用户名或密码错误")
	}
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %v", err)
	}

	user.NeedChangePassword = needChangePassword == 1

	// 验证密码
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, fmt.Errorf("用户名或密码错误")
	}

	return &user, nil
}

// 生成 JWT Token
func generateToken(username string, needChangePassword bool) (string, error) {
	claims := jwt.MapClaims{
		"username": username,
		"need_change_password": needChangePassword,
		"exp": time.Now().Add(24 * time.Hour).Unix(), // 24小时过期
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// 验证 JWT Token
func verifyToken(tokenString string) (*Session, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("意外的签名方法: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("token 无效")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("无法解析 claims")
	}

	username, ok := claims["username"].(string)
	if !ok {
		return nil, fmt.Errorf("无法获取用户名")
	}

	needChangePassword, _ := claims["need_change_password"].(bool)

	// 检查会话是否存在
	sessionMutex.RLock()
	session, exists := sessions[tokenString]
	sessionMutex.RUnlock()

	if !exists {
		// 创建新会话
		exp, _ := claims["exp"].(float64)
		session = &Session{
			Username: username,
			ExpiresAt: time.Unix(int64(exp), 0),
			NeedChangePassword: needChangePassword,
		}
		sessionMutex.Lock()
		sessions[tokenString] = session
		sessionMutex.Unlock()
	}

	// 检查是否过期
	if time.Now().After(session.ExpiresAt) {
		sessionMutex.Lock()
		delete(sessions, tokenString)
		sessionMutex.Unlock()
		return nil, fmt.Errorf("token 已过期")
	}

	return session, nil
}

// 认证中间件
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 允许登录和健康检查接口
		if r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/health" {
			next(w, r)
			return
		}

		// 获取 token
		token := r.Header.Get("Authorization")
		if token == "" {
			// 尝试从 Cookie 获取
			cookie, err := r.Cookie("token")
			if err != nil || cookie.Value == "" {
				http.Error(w, `{"error": "未授权，请先登录"}`, http.StatusUnauthorized)
				return
			}
			token = cookie.Value
		} else {
			// 移除 "Bearer " 前缀
			token = strings.TrimPrefix(token, "Bearer ")
		}

		// 验证 token
		session, err := verifyToken(token)
		if err != nil {
			http.Error(w, `{"error": "token 无效或已过期"}`, http.StatusUnauthorized)
			return
		}

		// 检查是否需要修改密码（除了修改密码接口）
		if session.NeedChangePassword && r.URL.Path != "/api/auth/change-password" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "需要修改密码",
				"need_change_password": true,
			})
			return
		}

		// 将用户名添加到请求上下文
		r.Header.Set("X-Username", session.Username)

		next(w, r)
	}
}

// 登录处理
func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}

	// 验证用户
	user, err := verifyUser(req.Username, req.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// 生成 token
	token, err := generateToken(user.Username, user.NeedChangePassword)
	if err != nil {
		http.Error(w, "生成 token 失败", http.StatusInternalServerError)
		return
	}

	// 保存会话
	sessionMutex.Lock()
	sessions[token] = &Session{
		Username: user.Username,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		NeedChangePassword: user.NeedChangePassword,
	}
	sessionMutex.Unlock()

	// 设置 Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		MaxAge:   86400, // 24小时
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	// 返回响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Token:              token,
		NeedChangePassword: user.NeedChangePassword,
		Message:            "登录成功",
	})
}

// 修改密码处理
func handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "方法不允许", http.StatusMethodNotAllowed)
		return
	}

	// 获取当前用户
	token := r.Header.Get("Authorization")
	if token == "" {
		cookie, err := r.Cookie("token")
		if err != nil || cookie.Value == "" {
			http.Error(w, "未授权", http.StatusUnauthorized)
			return
		}
		token = cookie.Value
	} else {
		token = strings.TrimPrefix(token, "Bearer ")
	}

	session, err := verifyToken(token)
	if err != nil {
		http.Error(w, "token 无效", http.StatusUnauthorized)
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误", http.StatusBadRequest)
		return
	}

	// 验证新密码强度
	if err := validatePasswordStrength(req.NewPassword); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// 获取用户信息
	var passwordHash string
	err = authDB.QueryRow(
		"SELECT password_hash FROM users WHERE username = ?",
		session.Username,
	).Scan(&passwordHash)

	if err != nil {
		http.Error(w, "查询用户失败", http.StatusInternalServerError)
		return
	}

	// 验证旧密码（如果不是首次修改密码）
	if !session.NeedChangePassword {
		err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.OldPassword))
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "旧密码错误",
			})
			return
		}
	}

	// 生成新密码哈希
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "生成密码哈希失败", http.StatusInternalServerError)
		return
	}

	// 更新密码
	_, err = authDB.Exec(
		"UPDATE users SET password_hash = ?, need_change_password = 0, updated_at = CURRENT_TIMESTAMP WHERE username = ?",
		string(newHash), session.Username,
	)
	if err != nil {
		http.Error(w, "更新密码失败", http.StatusInternalServerError)
		return
	}

	// 更新会话
	sessionMutex.Lock()
	if s, exists := sessions[token]; exists {
		s.NeedChangePassword = false
	}
	sessionMutex.Unlock()

	// 生成新 token（因为 need_change_password 状态改变了）
	newToken, _ := generateToken(session.Username, false)
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    newToken,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "密码修改成功",
		"token":   newToken,
	})
}

// 登出处理
func handleLogout(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token == "" {
		cookie, err := r.Cookie("token")
		if err == nil {
			token = cookie.Value
		}
	} else {
		token = strings.TrimPrefix(token, "Bearer ")
	}

	if token != "" {
		sessionMutex.Lock()
		delete(sessions, token)
		sessionMutex.Unlock()
	}

	// 清除 Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "登出成功"})
}

// 获取当前用户信息
func handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token == "" {
		cookie, err := r.Cookie("token")
		if err != nil || cookie.Value == "" {
			http.Error(w, "未授权", http.StatusUnauthorized)
			return
		}
		token = cookie.Value
	} else {
		token = strings.TrimPrefix(token, "Bearer ")
	}

	session, err := verifyToken(token)
	if err != nil {
		http.Error(w, "token 无效", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"username":            session.Username,
		"need_change_password": session.NeedChangePassword,
	})
}

