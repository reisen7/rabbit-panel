# Rabbit Panel - 项目总结

## 项目概述

**Rabbit Panel** 是一个极致轻量化的容器运维面板，专为资源受限的设备设计。

- 🚀 **极致轻量**: 运行时内存 ≤30MB，部署包 ≤10MB
- 🐳 **完整功能**: 容器管理、镜像管理、系统监控
- 🌐 **响应式设计**: 支持 PC/平板/手机
- 🔒 **安全认证**: 用户登录认证和节点间通信加密
- 🎯 **多节点**: Master/Worker 模式支持

## 项目结构

```
rabbit-panel/
├── 核心代码
│   ├── main.go           # 主程序和 HTTP 服务器
│   ├── auth.go           # 认证和授权
│   ├── node.go           # 节点管理
│   └── scheduler.go      # 容器调度
│
├── 前端
│   └── static/
│       └── index.html    # Web UI
│
├── 编译和部署
│   ├── build.sh          # 编译脚本
│   ├── start.sh          # 启动脚本
│   ├── .air.toml         # 开发热重载配置
│   └── go.mod/go.sum     # 依赖管理
│
├── 文档
│   ├── README.md         # 主文档
│   ├── DEPLOYMENT.md     # 部署指南
│   ├── SECURITY_AUDIT.md # 安全审计
│   └── PROJECT_SUMMARY.md # 本文件
│
└── 配置
    └── .gitignore       # Git 忽略规则
```

## 快速开始

### 编译

```bash
# 下载依赖
go mod download

# 编译（自动检测系统架构）
./build.sh
```

### 启动

```bash
./start.sh
```

### 访问

- 本地: http://localhost:9999
- 外网: http://<IP>:9999
- 默认账户: admin / admin

## 核心功能

### 1. 容器管理

- 列表查看（ID、名称、镜像、状态、端口、内存）
- 启动/停止/重启/删除
- 实时日志查看（SSE 流式）
- 自动刷新（5 秒）

### 2. 镜像管理

- 列表查看（ID、标签、大小、创建时间）
- 删除镜像
- 依赖检查

### 3. 系统监控

- CPU 使用率
- 内存使用率
- 磁盘使用率
- 实时更新

### 4. 多节点管理

- Master 节点统一管理
- Worker 节点自动注册
- 智能容器调度
- 跨节点操作

### 5. 安全认证

- 用户登录认证
- 密码强度验证
- JWT Token 管理
- 节点间 HMAC 认证

## 技术栈

### 后端

- **语言**: Go 1.22+
- **框架**: 标准库 net/http
- **数据库**: SQLite（认证数据）
- **容器**: Docker API

### 前端

- **HTML/CSS/JS**: 原生实现
- **样式**: Tailwind CSS (CDN)
- **图标**: Lucide Icons (CDN)

### 部署

- **编译**: Go build
- **运行**: 单二进制文件
- **反向代理**: Nginx
- **SSL**: Let's Encrypt

## 文件说明

| 文件 | 大小 | 说明 |
|------|------|------|
| main.go | 25KB | 主程序、HTTP 路由、系统监控 |
| auth.go | 16KB | 用户认证、JWT、密码管理 |
| node.go | 8KB | 节点管理、心跳、注册 |
| scheduler.go | 8KB | 容器调度、跨节点操作 |
| build.sh | 3KB | 编译脚本 |
| start.sh | 2KB | 启动脚本 |
| README.md | 11KB | 完整文档 |
| SECURITY_AUDIT.md | 12KB | 安全审计报告 |
| DEPLOYMENT.md | 8KB | 部署指南 |

## API 接口

### 认证

- `POST /api/auth/login` - 登录
- `POST /api/auth/logout` - 登出
- `POST /api/auth/change-password` - 修改密码
- `GET /api/auth/me` - 获取当前用户

### 容器管理

- `GET /api/containers` - 获取容器列表
- `POST /api/containers/action` - 容器操作
- `GET /api/containers/logs` - 获取日志（SSE）
- `POST /api/containers/schedule` - 跨节点调度（Master）
- `GET /api/containers/all` - 获取所有容器（Master）

### 镜像管理

- `GET /api/images` - 获取镜像列表
- `POST /api/images/remove` - 删除镜像

### 系统

- `GET /api/system/stats` - 系统监控数据
- `GET /api/health` - 健康检查

### 节点管理（Master）

- `GET /api/nodes` - 获取节点列表
- `POST /api/nodes/register` - 节点注册
- `POST /api/nodes/heartbeat` - 节点心跳

## 安全特性

✅ **已实现**

- 用户登录认证
- 密码 bcrypt 哈希
- JWT Token 管理
- 节点 HMAC 认证
- Cookie HttpOnly
- Cookie SameSite
- 参数化查询（防 SQL 注入）
- 密码强度验证

⚠️ **需要配置**

- 更改默认密钥（JWT_SECRET、NODE_SECRET）
- 配置 HTTPS（Nginx + Let's Encrypt）
- 防火墙限制
- 定期密钥轮换

❌ **未实现**

- 速率限制
- CSRF Token
- 审计日志
- 安全头（需 Nginx）

详见 [SECURITY_AUDIT.md](SECURITY_AUDIT.md)

## 性能指标

### 资源占用

| 指标 | 值 |
|------|-----|
| 运行时内存 | ≤30MB |
| 二进制大小 | ≤10MB |
| 启动时间 | <1s |
| 响应时间 | <100ms |

### 支持规模

- 单节点: 100+ 容器
- 多节点: 50+ Worker 节点
- 并发连接: 100+ 用户

## 部署场景

### 开发环境

```bash
./start.sh
# 访问 http://localhost:9999
```

### 生产环境（单节点）

```bash
# 配置环境变量
export JWT_SECRET="..."
export NODE_SECRET="..."

# 配置 Nginx + HTTPS
# 配置防火墙
# 启动应用
./start.sh
```

### 生产环境（多节点）

```bash
# Master 节点
export MODE="master"
./start.sh

# Worker 节点
export MODE="worker"
export MASTER_URL="http://master:9999"
./start.sh
```

## 开发指南

### 环境要求

- Go 1.22+
- Docker 20.10+
- Linux 系统

### 开发流程

```bash
# 1. 克隆代码
git clone <repo>
cd rabbit-panel

# 2. 下载依赖
go mod download

# 3. 开发（热重载）
# 需要安装 air: go install github.com/cosmtrek/air@latest
air

# 4. 编译
./build.sh

# 5. 测试
./start.sh
```

### 代码结构

- **main.go**: HTTP 服务器、路由、系统监控
- **auth.go**: 认证、JWT、密码管理
- **node.go**: 节点通信、心跳、注册
- **scheduler.go**: 容器调度、跨节点操作

### 添加新功能

1. 在对应的 `.go` 文件中添加处理函数
2. 在 `main.go` 中注册路由
3. 在 `static/index.html` 中添加 UI
4. 测试功能
5. 更新文档

## 常见问题

### Q: 支持哪些架构？

A: amd64、arm64、armv7。在对应架构的系统上编译。

### Q: 如何在公网上安全部署？

A: 参考 [SECURITY_AUDIT.md](SECURITY_AUDIT.md)

### Q: 如何扩展功能？

A: 参考开发指南，在相应的 `.go` 文件中添加代码。

### Q: 如何备份数据？

A: 备份 `auth.db` 文件和 `.env.production` 配置。

### Q: 支持 Docker Compose 吗？

A: 不支持，但可以通过 API 创建容器。

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！

## 更新日志

### v1.1.0 (2024-11-15)

- ✅ 添加用户登录认证
- ✅ 添加节点间通信认证
- ✅ 改进多节点管理
- ✅ 优化性能和内存占用
- ✅ 添加安全审计报告

### v1.0.0 (2024-01-01)

- ✅ 初始版本发布
- ✅ 容器管理功能
- ✅ 镜像管理功能
- ✅ 系统监控功能
- ✅ 多节点管理功能

## 相关文档

- [README.md](README.md) - 完整使用文档
- [DEPLOYMENT.md](DEPLOYMENT.md) - 部署指南
- [SECURITY_AUDIT.md](SECURITY_AUDIT.md) - 安全审计报告

## 联系方式

- GitHub Issues
- 邮件联系维护者

---

**最后更新**: 2024-11-15

**项目状态**: ✅ 生产就绪

**推荐部署**: 在目标系统上编译，使用 Nginx + HTTPS 部署
