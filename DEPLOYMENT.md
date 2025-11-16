# 部署指南

## 快速开始

### 1. 编译

在目标系统上编译（确保 Go 1.22+ 已安装）：

```bash
# 下载依赖
go mod download
go mod tidy

# 编译
chmod +x build.sh
./build.sh
```

编译完成后，二进制文件位于 `dist/rabbit-panel-linux-<arch>-release/`

### 2. 启动

```bash
# 方式 1：使用启动脚本
./start.sh

# 方式 2：直接运行
cd dist/rabbit-panel-linux-arm64-release/
./rabbit-panel-linux-arm64
```

### 3. 访问

- **本地**: http://localhost:9999
- **外网**: http://<服务器IP>:9999
- **默认账户**: admin / admin

## 生产环境部署

### 前置要求

- Docker 20.10+ 已安装并运行
- Linux 系统 (Ubuntu/Debian/Armbian)
- 4GB+ 内存
- Go 1.22+ (仅编译时需要)

### 部署步骤

#### 第 1 步：编译

```bash
git clone <repo>
cd rabbit-panel
go mod download
./build.sh
```

#### 第 2 步：配置环境变量

```bash
# 生成强密钥
JWT_SECRET=$(openssl rand -base64 32)
NODE_SECRET=$(openssl rand -base64 32)

# 创建环境变量文件
cat > .env.production << EOF
export JWT_SECRET="$JWT_SECRET"
export NODE_SECRET="$NODE_SECRET"
export MODE="master"
export PORT="9999"
export HOST="127.0.0.1"
EOF

chmod 600 .env.production
```

#### 第 3 步：配置 HTTPS（Nginx）

```bash
# 安装 Nginx
sudo apt-get install nginx

# 获取 SSL 证书
sudo apt-get install certbot python3-certbot-nginx
sudo certbot certonly --standalone -d your-domain.com

# 配置 Nginx（参考 SECURITY_AUDIT.md）
sudo nano /etc/nginx/sites-available/rabbit-panel

# 启用并测试
sudo ln -s /etc/nginx/sites-available/rabbit-panel /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl restart nginx
```

#### 第 4 步：配置防火墙

```bash
# 允许 HTTPS
sudo ufw allow 443/tcp

# 允许 HTTP（重定向用）
sudo ufw allow 80/tcp

# 禁止直接访问应用端口
sudo ufw deny 9999/tcp

# 启用防火墙
sudo ufw enable
```

#### 第 5 步：启动应用

```bash
# 加载环境变量
source .env.production

# 启动应用（后台运行）
nohup ./dist/rabbit-panel-linux-arm64-release/rabbit-panel-linux-arm64 > logs/app.log 2>&1 &

# 或使用 systemd
sudo nano /etc/systemd/system/rabbit-panel.service
```

#### 第 6 步：首次登录

- 访问 https://your-domain.com
- 用户名: admin
- 密码: admin
- **立即修改密码**

### Systemd 服务配置

创建 `/etc/systemd/system/rabbit-panel.service`：

```ini
[Unit]
Description=Rabbit Panel Container Management
After=network.target docker.service
Wants=docker.service

[Service]
Type=simple
User=root
WorkingDirectory=/root/rabbit-panel
EnvironmentFile=/root/rabbit-panel/.env.production
ExecStart=/root/rabbit-panel/dist/rabbit-panel-linux-arm64-release/rabbit-panel-linux-arm64
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

启用服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable rabbit-panel
sudo systemctl start rabbit-panel
sudo systemctl status rabbit-panel
```

## 多节点部署

### Master 节点

```bash
export MODE="master"
export PORT="9999"
export HOST="0.0.0.0"
./start.sh
```

### Worker 节点

```bash
export MODE="worker"
export MASTER_URL="http://master-ip:9999"
export NODE_NAME="worker-1"
export PORT="10001"
export HOST="0.0.0.0"
./start.sh
```

## 监控和维护

### 查看日志

```bash
# 查看实时日志
tail -f logs/app.log

# 或使用 systemd
sudo journalctl -u rabbit-panel -f
```

### 定期备份

```bash
# 备份认证数据库
cp auth.db auth.db.backup.$(date +%Y%m%d)

# 备份配置
cp .env.production .env.production.backup
```

### 更新应用

```bash
# 停止应用
sudo systemctl stop rabbit-panel

# 重新编译
./build.sh

# 启动应用
sudo systemctl start rabbit-panel
```

## 故障排查

### 应用无法启动

```bash
# 检查 Docker 连接
docker ps

# 检查端口占用
sudo lsof -i :9999

# 查看详细错误
./start.sh
```

### 认证失败

```bash
# 检查环境变量
echo $JWT_SECRET
echo $NODE_SECRET

# 重置管理员密码（删除数据库）
rm auth.db
# 重启应用，使用默认账户 admin/admin
```

### 内存占用过高

```bash
# 检查容器和镜像
docker ps -a
docker images

# 清理无用容器和镜像
docker container prune
docker image prune
```

## 安全建议

1. **更改默认密钥**
   - 设置 JWT_SECRET 和 NODE_SECRET
   - 定期轮换密钥

2. **使用 HTTPS**
   - 配置 SSL 证书
   - 启用 HSTS

3. **防火墙配置**
   - 限制访问端口
   - 只允许必要的流量

4. **定期备份**
   - 备份认证数据库
   - 备份配置文件

5. **监控日志**
   - 检查异常登录
   - 检查 API 异常请求

详细信息请参考 [SECURITY_AUDIT.md](SECURITY_AUDIT.md)

## 性能优化

### 资源占用

- **运行时内存**: ≤30MB
- **二进制大小**: ≤10MB
- **静态资源**: ≤100KB

### 优化措施

1. 编译优化：`-ldflags="-s -w"`
2. 无数据库：直接从 Docker API 获取
3. 内存缓存：减少 API 调用
4. 连接管理：及时释放连接

## 常见问题

### Q: 如何在多个服务器上部署？

A: 在每个服务器上：
1. 编译应用
2. 配置环境变量
3. 启动应用
4. 配置 Nginx 反向代理

### Q: 如何升级应用？

A: 
```bash
git pull
./build.sh
sudo systemctl restart rabbit-panel
```

### Q: 如何备份数据？

A:
```bash
# 备份认证数据
cp auth.db auth.db.backup

# 备份配置
cp .env.production .env.production.backup
```

### Q: 如何卸载应用？

A:
```bash
# 停止服务
sudo systemctl stop rabbit-panel
sudo systemctl disable rabbit-panel

# 删除服务文件
sudo rm /etc/systemd/system/rabbit-panel.service

# 删除应用目录
rm -rf /root/rabbit-panel
```

## 支持

- 查看 [README.md](README.md) 了解功能
- 查看 [SECURITY_AUDIT.md](SECURITY_AUDIT.md) 了解安全问题
- 提交 Issue 或 Pull Request
