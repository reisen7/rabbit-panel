# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Rabbit Panel is an AI-powered, lightweight Docker container management panel built with Go (backend) and Vue 3 (frontend). It supports multi-node management (Master/Worker architecture), runs as a single static binary with no external database, and targets resource-constrained devices (4GB+ RAM, ARM64/armv7/x86_64).

## Build & Run Commands

### Development

**Frontend (hot-reload dev server):**
```bash
cd frontend && npm install && npm run dev
# Runs on http://localhost:3000, proxies API to backend at localhost:3958
```

**Backend (local):**
```bash
cd backend && go run main.go
# Listens on 0.0.0.0:3958 by default
```

### Production Build

```bash
./rabbit.sh build                  # Builds frontend + backend for current host architecture
./rabbit.sh build all              # Cross-compile all architectures (amd64, arm64, armv7)
./rabbit.sh build arm64            # Cross-compile for ARM64
./rabbit.sh build --skip-frontend  # Skip frontend build (use existing .dist/)
```

### Runtime Management

```bash
./rabbit.sh start    # Start the panel
./rabbit.sh stop     # Stop the panel
./rabbit.sh restart  # Restart
./rabbit.sh status   # Check running status
./rabbit.sh log      # Tail log file
```

### Frontend Tests

```bash
cd frontend && npm run test          # Run tests once
npm run test:watch                   # Watch mode
npm run test:coverage                # With coverage report
```

## Architecture

### Backend (Go, `/backend`)

Multi-package Go application using **Gin Web Framework**. Entry point is `main.go` which wires up the `App` struct (dependency injection container) and starts the Gin engine. Key packages:

| Package | Responsibility |
|---------|---------------|
| `config/` | App struct, env var loading, all service/repository injection |
| `router/` | All HTTP/WebSocket handlers (Gin), route registration |
| `service/` | Business logic layer (ContainerService, ImageService, etc.) |
| `repository/` | Data access layer — Docker API, SQLite, files |
| `middleware/` | JWT auth middleware, node auth middleware |
| `exec/` | Container terminal (WebSocket ↔ docker exec) |
| `agent/` | AI agent prompt templates |
| `tool/` | System stats utilities (CPU, memory, disk — Linux only) |
| `model/` | Data models |

**Architecture:**
- All services injected via `App` struct — no globals
- `IDockerRepository` interface wraps `*docker/client.Client` (testable)
- `ICacheRepository` for in-memory caches
- `ISQLiteRepository` for auth/sessions
- Frontend assets embedded via `//go:embed .dist`

**Data storage:** SQLite at `backend/data/auth.db` (modernc.org/sqlite, CGO-free) + JSON config at `backend/data/agent.json`. No external database required.

**Frontend assets** are embedded at compile time via `//go:embed .dist`.

### Frontend (Vue 3 + TypeScript, `/frontend/src`)

- **Views** (`views/`): Page-level components — Dashboard, Containers, Images, Networks, Volumes, Registry, DockerConfig, Compose, Nodes, Login, AgentChat, AgentSettings
- **API clients** (`api/`): One file per backend resource (auth.ts, containers.ts, images.ts, etc.)
- **Pinia stores** (`stores/`): State management per feature domain
- **Locales** (`locales/`): i18n translations (Chinese/English)
- **Types** (`types/`): Shared TypeScript interfaces

The frontend is built by Vite into `backend/.dist/` and embedded into the Go binary.

### Multi-Node Architecture

- **Master**: Runs the full web UI + API, orchestrates workers
- **Worker**: Registers with master via `MASTER_URL`, receives container exec/scheduler commands, exposes only internal API endpoints
- **Communication**: Workers poll master heartbeat; master dispatches container operations to workers via HTTP; auth uses HMAC-SHA256 via shared `NODE_SECRET`

Environment variables `MODE` (master/worker), `JWT_SECRET`, `NODE_SECRET`, `MASTER_URL`, `PORT`, `HOST` control runtime behavior.

## Key Frameworks & Dependencies

**Backend:** Go 1.25, gin-gonic/gin v1.12, docker/docker v25, gorilla/websocket, golang-jwt/jwt/v5, modernc.org/sqlite

**Frontend:** Vue 3.5, Vite 7, Element Plus 2, Pinia 3, Vue Router 4, Vue i18n 11, ECharts + vue-echarts, Xterm.js 6, Axios, Marked

## Important Notes

- Default credentials: `admin` / `admin` (password must be changed on first login)
- Default port: `3958`
- Docker socket must be mounted into the container/binary for container management
- Time sync between Master and Worker nodes must be within 1 hour (node auth uses JWT with 1-hour tolerance)
- The backend's WebSocket terminal handler lives in `router/router.go` — any changes to the exec/session protocol should be tested against the xterm.js frontend client
- Set `RABBIT_UPDATE_CHECK_DISABLED=true` to skip the update check (no outbound `MANIFEST_URL` fetch) — see `backend/service/update.go`

