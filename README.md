# Rabbit Panel ｜ 极致轻量容器运维面板

一个极致轻量的 Docker 运维面板，面向 4GB 内存设备，支持 ARM64 / armv7l / x86_64，多节点集中管理，开箱即用。

## 特性

- 🚀 **极致轻量**：运行时内存 ≤ 30MB，二进制 ≤ 10MB
- 🐳 **容器管理**：启动/停止/重启/删除，实时日志（SSE）
- 📦 **镜像管理**：查看与删除镜像
- 🧩 **Compose 管理**：在线创建/编辑 `docker-compose.yml`，一键 Up/Down/Restart/Pull/Logs
- 📊 **系统监控**：CPU/内存/磁盘实时监控（5 秒刷新）
- 🌐 **响应式设计**：PC/平板/手机良好体验
- 🔧 **零依赖**：单二进制，内置前端，无数据库，无额外安装
- 🎯 **多节点管理**：Master/Worker 统一管理多台服务器
- 🔒 **安全认证**：JWT 登录认证 + HMAC 节点认证

## 环境要求

- Docker 20.10+（已安装并运行，建议将用户加入 `docker` 组）
- Linux（Armbian / Ubuntu / Debian）
- 4GB+ 内存设备
- Go 1.22+（仅用于本地编译）

## 快速开始

### 1. 安装 Docker (如果未安装)

```bash
# Ubuntu/Debian
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# 将当前用户添加到 docker 组
sudo usermod -aG docker $USER
newgrp docker
```

### 2. 从源码编译

```bash
# 克隆代码
git clone https://github.com/reisen7/rabbit-panel.git
cd rabbit-panel

# 编译 (默认依据当前架构输出 dist/)
chmod +x rabbit.sh
./rabbit.sh build

# 需要交叉编译可指定目标: amd64 | arm64 | armv7 | all
./rabbit.sh build arm64

# 编译后的文件在 dist/ 目录

```

> 提示：`rabbit.sh build` 默认配置了 `GOPROXY=https://goproxy.cn,direct` 以加速国内构建。
> 如果需要跨架构且依赖 CGO，可通过 `CC_AMD64`、`CC_ARM64`、`CC_ARMV7` 指定对应的交叉编译器。


### 3. 运行面板（一键脚本）

推荐使用集成脚本 `rabbit.sh` 管理启动/停止/重启/状态/编译：

```bash
# 授权脚本
chmod +x rabbit.sh

# 编译并启动
./rabbit.sh build
./rabbit.sh start

# 查看状态与日志
./rabbit.sh status
./rabbit.sh log

# 停止或重启
./rabbit.sh stop
./rabbit.sh restart
```

也可直接运行二进制（默认端口 9999）：
> 下载地址：
> https://github.com/reisen7/rabbit-panel/releases
```bash
# 运行面板 (默认端口 9999)
./rabbit-panel-linux-amd64

# 或指定端口
PORT=9090 ./rabbit-panel-linux-amd64

# 或指定监听地址和端口
HOST=0.0.0.0 PORT=9999 ./rabbit-panel-linux-amd64
```

### 4. 访问面板

- **本地访问**: `http://localhost:9999`
- **外网访问**: `http://<服务器IP>:9999`
- **默认账户**: `admin` / `admin`

> 注意：首次登录必须修改密码。默认监听 `0.0.0.0:9999`，可外网访问。

## 功能说明

### 容器管理

- **列表展示**: 显示容器 ID、名称、镜像、状态、端口映射、内存占用、创建时间
- **操作功能**: 启动、停止、重启、删除、查看日志（实时流式输出）
- **自动刷新**: 容器列表每 5 秒自动刷新

### 镜像管理

- **列表展示**: 显示镜像 ID、名称、标签、大小、创建时间
- **操作功能**: 删除镜像（如果被容器使用会提示先删除容器）

### 系统监控

- **实时监控**: CPU 使用率、内存使用率、磁盘使用率
- **自动刷新**: 每 5 秒自动刷新
- **时间显示**: 显示服务器当前时间

## 多节点管理

Rabbit Panel 支持多节点容器管理，类似 Kubernetes 但更轻量化。

### 启动 Master 节点（控制节点）

```bash
# 方法 1: 使用启动脚本
chmod +x start-master.sh
./start-master.sh

# 方法 2: 直接运行
MODE=master PORT=9999 ./rabbit-panel-linux-arm64
```

### 启动 Worker 节点（工作节点）

```bash
# 方法 1: 使用启动脚本
MASTER_URL=http://master-ip:9999 NODE_NAME=worker-1 ./start-worker.sh

# 方法 2: 直接运行
MASTER_URL=http://master-ip:9999 \
NODE_NAME=worker-1 \
MODE=worker \
PORT=10001 \
./rabbit-panel-linux-arm64
```

### 多节点功能

- ✅ **统一管理**: 在 Master 节点查看和管理所有 Worker 节点的容器
- ✅ **智能调度**: 自动选择最佳节点部署容器
- ✅ **节点监控**: 实时监控所有节点的资源使用情况
- ✅ **跨节点操作**: 在 Master 节点操作任意 Worker 节点的容器

详细文档请参考：多节点管理将在后续版本完善。

## Compose 管理（在线）

- 在前端“Compose 管理”页新建项目（存储于 `compose_projects/<name>/docker-compose.yml`）
- 支持在线编辑（深色编辑器、Tab 插入空格），保存文件
- 支持执行：`up -d`、`down`、`restart`、`pull`、`logs`，输出结果在面板展示
- 切换到 Compose 标签页会自动刷新项目列表

> 需要本机已安装 `docker compose`（你已安装：`docker compose version` 返回成功）。



## 配置与安全

### 环境变量配置

- `MODE`：节点模式，`master` 或 `worker`，默认 `master`
- `PORT`：服务端口，默认 `9999`
- `HOST`：绑定地址，默认 `0.0.0.0`
- `JWT_SECRET`：用户认证密钥（生产环境必须设置）
- `NODE_SECRET`：节点通信密钥（生产环境必须设置）

示例：

```bash
MODE=master PORT=9999 HOST=0.0.0.0 \
JWT_SECRET=change-me NODE_SECRET=change-me \
./rabbit-panel
```

### 用户认证

所有 Web UI 访问的 API 都需要用户登录认证：

- **默认账户**: `admin` / `admin`
- **首次登录**: 必须修改密码
- **密码要求**: 至少 8 位，包含大小写字母、数字和特殊字符

### 节点间认证

Master 和 Worker 节点之间的通信使用 HMAC-SHA256 认证机制。

**生产环境必须设置节点密钥**:
```bash
NODE_SECRET=your-secret-key-here ./rabbit-panel-linux-amd64
```

### 生产环境建议

1. **使用 HTTPS**: 通过 Nginx/Caddy 配置 HTTPS 反向代理
2. **防火墙配置**: 限制访问端口
3. **定期更新密钥**: 定期更换 `NODE_SECRET` 和 `JWT_SECRET`

详细文档请参考: [SECURITY.md](SECURITY.md)

## 性能优化

### 资源占用

- **运行时内存**: ≤30MB
- **二进制文件大小**: ≤10MB
- **静态资源**: ≤100KB (Tailwind CSS 通过 CDN 加载)

### 优化措施

1. **编译优化**: 使用 `-ldflags="-s -w"` 去除符号表和调试信息
2. **无数据库**: 所有数据直接从 Docker API 获取，无持久化存储
3. **连接管理**: 容器日志流关闭后立即释放连接
4. **内存管理**: 定期清理无用协程，避免内存泄漏
5. **数据缓存**: 容器和镜像列表使用内存缓存，减少 Docker API 调用

## 项目结构

```
rabbit-panel/
├── main.go              # 后端主文件
├── auth.go              # 认证模块
├── node.go              # 节点管理模块
├── scheduler.go         # 容器调度模块
├── compose.go           # Compose 在线管理 API
├── static/index.html    # 前端页面
├── rabbit.sh            # 一键管理脚本
├── .air.toml            # 开发热重载配置
├── go.mod / go.sum      # Go 依赖
└── README.md            # 说明文档
```

## API 接口

### 单节点 API

- `GET /api/system/stats` - 获取系统监控数据
- `GET /api/containers` - 获取容器列表
- `POST /api/containers/action` - 容器操作 (start/stop/restart/remove)
- `GET /api/containers/logs?id=<container-id>` - 获取容器日志 (SSE 流式)
- `GET /api/images` - 获取镜像列表
- `POST /api/images/remove` - 删除镜像
- `GET /api/health` - 健康检查

### 多节点 API（Master）

- `GET /api/nodes` - 获取所有节点列表
- `POST /api/containers/schedule` - 跨节点调度容器
- `GET /api/containers/all` - 获取所有节点的容器

### 认证 API

- `POST /api/auth/login` - 用户登录
- `POST /api/auth/logout` - 用户登出
- `POST /api/auth/change-password` - 修改密码

## 扩展开发

如需添加新功能，请遵循以下原则:

1. **保持轻量化**: 新增功能不应显著增加内存占用
2. **无数据库**: 如需缓存，使用内存临时存储
3. **单文件后端**: 尽量将代码集中在主文件中
4. **原生前端**: 使用原生 HTML/CSS/JS，避免引入框架

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！

## 更新日志

### v1.1.0 (2025-12-16)

- 支持单文件部署（静态资源嵌入二进制）
- 添加用户登录认证
- 添加节点间通信认证 (HMAC-SHA256)
- 改进多节点管理功能
- 优化性能和内存占用

### v1.0.0 (2025-11-16)

- 初始版本发布
- 支持容器管理 (启动/停止/重启/删除/日志)
- 支持镜像管理 (列表/删除)
- 支持系统监控 (CPU/内存/磁盘)
- 支持多架构编译 (ARM64/armv7l/x86_64)
- 支持多节点管理 (Master/Worker 模式)
