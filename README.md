# Rabbit Panel ｜ 极致轻量 AI 智能容器运维面板

一个面向 Linux 服务器的轻量 Docker 运维面板，支持 ARM64 / armv7 / x86_64、多节点管理、AI 助手与在线更新。默认端口 `3958`。

## 特性

- 轻量：单二进制、内置前端、SQLite 本地存储
- 容器 / 镜像 / 网络 / 存储卷 / Compose 管理
- Docker 配置、仓库配置、系统监控
- 多节点 Master / Worker 管理
- 在线更新、版本检测、进度反馈
- 中文 / English 界面

![首页](.doc/images/image.png)
![容器管理](.doc/images/image-1.png)
![镜像管理](.doc/images/image-2.png)
![Compose 管理](.doc/images/image-3.png)
![网络管理](.doc/images/image-4.png)
![配置管理](.doc/images/image-5.png)
![LLM](.doc/images/image-6.png)
![LLM](.doc/images/image-7.png)
## 环境要求

- Linux
- Docker 20.10+
- 4GB+ RAM

## 快速开始

### Docker

```bash
docker run -d \
  --name rabbit-panel \
  --restart unless-stopped \
  -p 3958:3958 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /root/rabbit-panel/compose_projects:/app/compose_projects \
  -v /root/rabbit-panel/data:/app/data \
  -e TZ=Asia/Shanghai \
  reisen7/rabbit-panel:latest
```

或：

```bash
git clone https://github.com/reisen7/rabbit-panel.git
cd rabbit-panel
docker compose -f docker-compose.deploy.yml up -d
```

### 一键安装二进制（systemd）

```bash
curl -fsSL https://raw.githubusercontent.com/reisen7/rabbit-panel/main/install.sh | bash
```

> 如果你希望使用面板内“立即更新”，二进制部署必须通过 `systemd` 管理。

## 多节点

Master：

```bash
docker compose -f docker-compose.master.yml up -d
```

Worker：

```bash
docker compose -f docker-compose.worker.yml up -d
```

> `JWT_SECRET` 和 `NODE_SECRET` 必须一致。  
> Master / Worker 系统时间差不能超过 1 小时。

## 配置

关键环境变量：

- `MODE=master|worker`
- `HOST=0.0.0.0`
- `PORT=3958`
- `JWT_SECRET=...`
- `NODE_SECRET=...`
- `RABBIT_UPDATE_CHECK_DISABLED=true` 跳过更新检查（不再请求 `MANIFEST_URL`，界面不显示更新提示）

默认账户：

- `admin / admin`

首次登录后必须修改密码。

## 更新日志

查看完整更新日志：[.doc/CHANGELOG.md](.doc/CHANGELOG.md)

## 许可证

MIT License
