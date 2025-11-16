# 安全审计报告 - Rabbit Panel

## 执行摘要

本报告针对 Rabbit Panel 在**公网部署**场景下的安全性进行了全面审计。总体评估：**中等风险**，存在多个需要立即修复的问题。

---

## 🔴 严重问题（必须修复）

### 1. 默认密钥未更改

**位置**: `auth.go:107`, `auth.go:27`

**问题**:
```go
var jwtSecret = []byte("rabbit-panel-secret-key-change-in-production")
secret = "rabbit-panel-node-secret-change-in-production"
```

**风险**: 
- 使用硬编码的默认密钥
- 任何人都可以伪造 JWT Token
- 节点间通信可被冒充

**修复方案**:
```bash
# 启动时必须设置环境变量
export JWT_SECRET="your-strong-random-secret-key-here"
export NODE_SECRET="your-strong-random-node-secret-here"
./rabbit-panel-linux-amd64
```

**建议**: 
- 生成强随机密钥（至少 32 字符）
- 使用密钥管理服务（如 HashiCorp Vault）
- 定期轮换密钥

---

### 2. 默认账户和弱密码

**位置**: `auth.go:186-199`

**问题**:
```go
// 创建默认管理员账户 admin/admin
hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
```

**风险**:
- 默认账户 `admin/admin` 众所周知
- 首次部署时任何人都能登录
- 密码修改不是强制的（仅提示）

**修复方案**:

在 `auth.go` 中修改初始化逻辑：

```go
// 从环境变量读取初始密码，如果未设置则生成随机密码
func getInitialPassword() string {
    pwd := os.Getenv("INITIAL_ADMIN_PASSWORD")
    if pwd != "" {
        return pwd
    }
    // 生成随机密码
    return generateRandomPassword(16)
}

// 启动时必须设置
export INITIAL_ADMIN_PASSWORD="YourStrongPassword123!@#"
```

---

### 3. HTTP 通信未加密

**位置**: `main.go:832-840`

**问题**:
- 所有通信都是 HTTP（明文）
- 在公网上传输敏感数据（密码、Token、容器信息）
- 容易被中间人攻击

**风险**:
- Token 可被截获
- 密码可被窃听
- 节点间通信可被篡改

**修复方案**:

**方案 A: 使用 Nginx 反向代理（推荐）**

```nginx
server {
    listen 443 ssl http2;
    server_name your-domain.com;
    
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    
    # 安全头
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-XSS-Protection "1; mode=block" always;
    
    location / {
        proxy_pass http://127.0.0.1:9999;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket 支持
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}

# HTTP 重定向到 HTTPS
server {
    listen 80;
    server_name your-domain.com;
    return 301 https://$server_name$request_uri;
}
```

**方案 B: 使用 Let's Encrypt 免费证书**

```bash
# 安装 Certbot
sudo apt-get install certbot python3-certbot-nginx

# 获取证书
sudo certbot certonly --standalone -d your-domain.com

# 证书位置
# /etc/letsencrypt/live/your-domain.com/fullchain.pem
# /etc/letsencrypt/live/your-domain.com/privkey.pem

# 自动续期
sudo systemctl enable certbot.timer
```

---

### 4. 缺少速率限制

**位置**: 所有 API 端点

**问题**:
- 没有登录尝试限制
- 没有 API 请求限制
- 容易被暴力破解或 DDoS

**风险**:
- 密码暴力破解
- API 滥用
- 服务不可用

**修复方案**:

添加速率限制中间件到 `auth.go`:

```go
import "golang.org/x/time/rate"

// 速率限制器
var (
    loginLimiters = make(map[string]*rate.Limiter)
    limiterMutex sync.RWMutex
)

// 获取登录限制器
func getLoginLimiter(username string) *rate.Limiter {
    limiterMutex.RLock()
    limiter, exists := loginLimiters[username]
    limiterMutex.RUnlock()
    
    if !exists {
        limiter = rate.NewLimiter(rate.Every(time.Minute/5), 5) // 每分钟 5 次
        limiterMutex.Lock()
        loginLimiters[username] = limiter
        limiterMutex.Unlock()
    }
    return limiter
}

// 在 handleLogin 中添加
func handleLogin(w http.ResponseWriter, r *http.Request) {
    var req LoginRequest
    json.NewDecoder(r.Body).Decode(&req)
    
    // 检查速率限制
    limiter := getLoginLimiter(req.Username)
    if !limiter.Allow() {
        http.Error(w, "登录尝试过于频繁，请稍后再试", http.StatusTooManyRequests)
        return
    }
    
    // ... 继续登录逻辑
}
```

---

## 🟡 中等问题（应该修复）

### 5. Cookie 安全配置不完整

**位置**: `auth.go:439-446`

**问题**:
```go
http.SetCookie(w, &http.Cookie{
    Name:     "token",
    Value:    token,
    Path:     "/",
    MaxAge:   86400,
    HttpOnly: true,
    SameSite: http.SameSiteStrictMode,
    // 缺少 Secure 标志
})
```

**修复**:
```go
http.SetCookie(w, &http.Cookie{
    Name:     "token",
    Value:    token,
    Path:     "/",
    MaxAge:   86400,
    HttpOnly: true,
    Secure:   true,  // 仅在 HTTPS 上传输
    SameSite: http.SameSiteStrictMode,
})
```

---

### 6. 缺少 CSRF 保护

**位置**: 所有 POST/PUT/DELETE 请求

**问题**:
- 没有 CSRF Token 验证
- 跨站请求伪造攻击

**修复方案**:

```go
// 生成 CSRF Token
func generateCSRFToken() string {
    b := make([]byte, 32)
    rand.Read(b)
    return base64.StdEncoding.EncodeToString(b)
}

// CSRF 中间件
func csrfMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet && r.Method != http.MethodHead {
            token := r.Header.Get("X-CSRF-Token")
            if token == "" {
                http.Error(w, "缺少 CSRF Token", http.StatusForbidden)
                return
            }
            // 验证 Token（需要存储在 session 中）
        }
        next(w, r)
    }
}
```

---

### 7. 日志中可能泄露敏感信息

**位置**: `main.go:897-911`

**问题**:
- 服务器 IP 被记录到日志
- 可能的密码或 Token 泄露

**修复**:
```go
// 不要记录敏感信息
log.Printf("容器运维面板启动成功！")
log.Printf("监听地址: %s", server.Addr)
// 不要记录外网 IP 到日志文件
// 如果需要，使用 stderr 而不是日志文件
```

---

### 8. 缺少安全头

**位置**: 所有 HTTP 响应

**问题**:
- 没有 `X-Content-Type-Options`
- 没有 `X-Frame-Options`
- 没有 `Content-Security-Policy`

**修复**:

```go
// 添加安全头中间件
func securityHeadersMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")
        next(w, r)
    }
}
```

---

### 9. 节点间通信无加密

**位置**: `node.go` 中的 HTTP 请求

**问题**:
- Worker 向 Master 的心跳是明文 HTTP
- 容器调度信息未加密

**修复**:
- 使用 HTTPS 进行节点间通信
- 或使用 VPN/内网隔离

---

### 10. 缺少审计日志

**位置**: 所有敏感操作

**问题**:
- 没有记录谁做了什么
- 无法追踪安全事件

**建议**:
```go
// 添加审计日志
func auditLog(username, action, resource string, success bool) {
    status := "success"
    if !success {
        status = "failed"
    }
    log.Printf("[AUDIT] User: %s | Action: %s | Resource: %s | Status: %s | Time: %s",
        username, action, resource, status, time.Now().Format(time.RFC3339))
}
```

---

## 🟢 已实现的安全措施

✅ **密码哈希**: 使用 bcrypt（安全）
✅ **密码强度验证**: 要求大小写、数字、特殊字符
✅ **JWT Token**: 使用 HS256 签名
✅ **会话管理**: Token 24 小时过期
✅ **节点认证**: HMAC-SHA256 Token
✅ **参数化查询**: 使用 SQL 参数化（防止 SQL 注入）
✅ **Cookie HttpOnly**: 防止 XSS 访问
✅ **Cookie SameSite**: 防止 CSRF

---

## 📋 公网部署检查清单

### 部署前必须完成

- [ ] **设置环境变量**
  ```bash
  export JWT_SECRET="$(openssl rand -base64 32)"
  export NODE_SECRET="$(openssl rand -base64 32)"
  export INITIAL_ADMIN_PASSWORD="YourStrongPassword123!@#"
  ```

- [ ] **配置 HTTPS**
  - 使用 Nginx/Caddy 反向代理
  - 配置 SSL 证书（Let's Encrypt）
  - 启用 HSTS

- [ ] **防火墙配置**
  ```bash
  # 只允许 HTTPS
  ufw allow 443/tcp
  ufw allow 80/tcp  # 重定向用
  ufw deny 9999/tcp # 不允许直接访问
  ```

- [ ] **修改默认账户**
  - 首次登录立即修改密码
  - 创建强密码（至少 12 字符）

- [ ] **启用审计日志**
  - 记录所有登录尝试
  - 记录敏感操作

- [ ] **定期备份**
  - 备份 `auth.db` 数据库
  - 备份配置文件

- [ ] **监控和告警**
  - 监控异常登录
  - 监控 API 异常请求

### 定期维护

- [ ] 每月轮换密钥
- [ ] 定期检查日志
- [ ] 及时更新依赖
- [ ] 定期安全审计

---

## 🚀 快速修复步骤

### 第 1 步：生成强密钥
```bash
JWT_SECRET=$(openssl rand -base64 32)
NODE_SECRET=$(openssl rand -base64 32)
echo "JWT_SECRET=$JWT_SECRET"
echo "NODE_SECRET=$NODE_SECRET"
```

### 第 2 步：设置环境变量
```bash
# 编辑 /etc/environment 或 .env 文件
export JWT_SECRET="your-generated-key"
export NODE_SECRET="your-generated-key"
export INITIAL_ADMIN_PASSWORD="YourStrongPassword123!@#"
```

### 第 3 步：配置 HTTPS（使用 Nginx）
```bash
# 安装 Nginx
sudo apt-get install nginx

# 获取 SSL 证书
sudo apt-get install certbot python3-certbot-nginx
sudo certbot certonly --standalone -d your-domain.com

# 配置 Nginx（参考上面的配置）
sudo nano /etc/nginx/sites-available/rabbit-panel

# 启用配置
sudo ln -s /etc/nginx/sites-available/rabbit-panel /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl restart nginx
```

### 第 4 步：启动应用
```bash
# 使用环境变量启动
source /path/to/.env
./rabbit-panel-linux-amd64
```

### 第 5 步：首次登录
- 访问 `https://your-domain.com`
- 使用初始密码登录
- 立即修改密码

---

## 📞 安全事件响应

如果发现安全问题：

1. **立即停止服务**
   ```bash
   pkill rabbit-panel
   ```

2. **检查日志**
   ```bash
   tail -f /var/log/rabbit-panel.log
   ```

3. **备份数据**
   ```bash
   cp auth.db auth.db.backup
   ```

4. **更新密钥**
   ```bash
   export JWT_SECRET="$(openssl rand -base64 32)"
   export NODE_SECRET="$(openssl rand -base64 32)"
   ```

5. **重启服务**
   ```bash
   ./rabbit-panel-linux-amd64
   ```

---

## 总结

**当前状态**: 🟡 中等风险

**建议**:
1. **立即**: 修复默认密钥和账户问题
2. **本周**: 配置 HTTPS 和防火墙
3. **本月**: 实现速率限制和审计日志
4. **持续**: 定期安全审计和更新

**预计修复时间**: 2-4 小时

**修复后风险等级**: 🟢 低风险

---

## 参考资源

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [Go 安全最佳实践](https://golang.org/doc/effective_go)
- [Let's Encrypt](https://letsencrypt.org/)
- [Nginx 安全配置](https://nginx.org/en/docs/)
