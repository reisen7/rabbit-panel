#!/bin/bash

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

PID_FILE="rabbit-panel.pid"
LOG_FILE="rabbit-panel.log"
FRONTEND_DIR="frontend"
BACKEND_DIR="backend"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SUPPORTED_TARGETS=("amd64" "arm64" "armv7")
BUILD_LDFLAGS="${BUILD_LDFLAGS:--s -w}"
VERSION=""
COMMIT_HASH="${COMMIT_HASH:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
BUILD_TIME_VALUE="${BUILD_TIME_VALUE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo unknown)}"

ensure_go() {
    if ! command -v go > /dev/null 2>&1; then
        echo -e "${RED}Error: go not found. Please install Go 1.22+${NC}"
        exit 1
    fi
}

ensure_node() {
    if ! command -v node > /dev/null 2>&1; then
        echo -e "${RED}Error: node not found. Please install Node.js 18+${NC}"
        exit 1
    fi
    if ! command -v npm > /dev/null 2>&1; then
        echo -e "${RED}Error: npm not found${NC}"
        exit 1
    fi
}

detect_host_target() {
    case "$(uname -m)" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        armv7l|armv7) echo "armv7" ;;
        *) echo "" ;;
    esac
}

is_supported_target() {
    local needle="$1"
    for target in "${SUPPORTED_TARGETS[@]}"; do
        if [ "$target" = "$needle" ]; then
            return 0
        fi
    done
    return 1
}

build_frontend() {
    echo -e "${BLUE}Building frontend...${NC}"
    ensure_node
    cd "$FRONTEND_DIR"

    if [ ! -d "node_modules" ]; then
        echo -e "${YELLOW}Installing frontend dependencies...${NC}"
        npm install
    fi

    npm run build
    cd "$SCRIPT_DIR"
    echo -e "${GREEN}Frontend build completed${NC}"
}

build_target() {
    local target="$1"
    local goarch=""
    local goarm=""
    local suffix=""

    case "$target" in
        amd64)
            goarch="amd64"
            suffix="linux-amd64"
            ;;
        arm64)
            goarch="arm64"
            suffix="linux-arm64"
            ;;
        armv7)
            goarch="arm"
            goarm="7"
            suffix="linux-armv7"
            ;;
        *)
            echo -e "${RED}Unsupported build target: $target${NC}"
            exit 1
            ;;
    esac

    local output_name="rabbit-panel-${suffix}"
    if [ -n "$VERSION" ]; then
        output_name="rabbit-panel-${suffix}-${VERSION}"
    fi

    local output_dir=".dist/${output_name}-release"
    local output_file="${output_dir}/${output_name}"

    mkdir -p "$output_dir"
    cd "$BACKEND_DIR"

    local -a envs=("GOOS=linux" "GOARCH=$goarch" "CGO_ENABLED=0" "GOPROXY=https://goproxy.cn,direct")
    if [ -n "$goarm" ]; then
        envs+=("GOARM=$goarm")
    fi

    local version_value="${VERSION:-dev}"
    local ldflags="${BUILD_LDFLAGS} -X 'main.Version=${version_value}' -X 'main.Commit=${COMMIT_HASH}' -X 'main.BuildTime=${BUILD_TIME_VALUE}'"

    echo -e "${GREEN}Building ${output_name}...${NC}"
    env "${envs[@]}" go build -trimpath -ldflags "${ldflags}" -o "../$output_file" .
    chmod +x "../$output_file"

    cd "$SCRIPT_DIR"

    if [ "$(detect_host_target)" = "$target" ]; then
        cp "$output_file" ./rabbit-panel
        chmod +x ./rabbit-panel
    fi

    echo -e "${GREEN}Output: $output_file${NC}"
}

check_binary() {
    BINARY=""
    if [ -f ".dist/rabbit-panel-linux-amd64-release/rabbit-panel-linux-amd64" ]; then
        BINARY=".dist/rabbit-panel-linux-amd64-release/rabbit-panel-linux-amd64"
    elif [ -f ".dist/rabbit-panel-linux-arm64-release/rabbit-panel-linux-arm64" ]; then
        BINARY=".dist/rabbit-panel-linux-arm64-release/rabbit-panel-linux-arm64"
    elif [ -f ".dist/rabbit-panel-linux-armv7-release/rabbit-panel-linux-armv7" ]; then
        BINARY=".dist/rabbit-panel-linux-armv7-release/rabbit-panel-linux-armv7"
    elif [ -f "./rabbit-panel" ]; then
        BINARY="./rabbit-panel"
    else
        echo -e "${RED}Error: compiled binary not found${NC}"
        echo "Please run: ./rabbit.sh build"
        exit 1
    fi
}

set_env() {
    export MODE="${MODE:-master}"
    export PORT="${PORT:-3958}"
    export HOST="${HOST:-0.0.0.0}"
    export JWT_SECRET="${JWT_SECRET:-rabbit-panel-secret-key-change-in-production}"
    export NODE_SECRET="${NODE_SECRET:-rabbit-panel-node-secret-change-in-production}"
    # 原生 HTTPS 默认开启，证书缺失时自动生成自签证书（保存在 ./data/certs）
    # 想用 HTTP 启动：TLS_ENABLED=false ./rabbit.sh start
    export TLS_ENABLED="${TLS_ENABLED:-true}"
    export TLS_SELF_SIGNED="${TLS_SELF_SIGNED:-true}"
    export TLS_CERT_FILE="${TLS_CERT_FILE:-./data/certs/server.crt}"
    export TLS_KEY_FILE="${TLS_KEY_FILE:-./data/certs/server.key}"
    export TLS_HOSTS="${TLS_HOSTS:-}"
    export HTTP_PORT="${HTTP_PORT:-}"
    export TLS_REDIRECT_PORT="${TLS_REDIRECT_PORT:-$PORT}"
}

scheme() {
    if [ "${TLS_ENABLED:-true}" = "false" ] || [ "${TLS_ENABLED}" = "0" ]; then
        echo "http"
    else
        echo "https"
    fi
}

start() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p "$PID" > /dev/null 2>&1; then
            echo -e "${YELLOW}Rabbit Panel is already running (PID: $PID)${NC}"
            return
        fi
        rm -f "$PID_FILE"
    fi

    check_binary
    set_env

    echo -e "${GREEN}Starting Rabbit Panel...${NC}"
    echo "Mode: $MODE | Port: $PORT | Arch: $(uname -m)"
    if [ "$(scheme)" = "https" ]; then
        echo "Access URL: https://<ip>:$PORT  (自签证书，浏览器首次会提示不受信任)"
    else
        echo "Access URL: http://<ip>:$PORT"
    fi

    nohup "$BINARY" > "$LOG_FILE" 2>&1 &
    PID=$!
    echo "$PID" > "$PID_FILE"

    sleep 1
    if ps -p "$PID" > /dev/null 2>&1; then
        echo -e "${GREEN}Started successfully (PID: $PID)${NC}"
        echo "Log file: $LOG_FILE"
    else
        echo -e "${RED}Start failed. Check logs:${NC}"
        cat "$LOG_FILE"
    fi
}

stop() {
    if [ ! -f "$PID_FILE" ]; then
        echo -e "${YELLOW}PID file not found. Rabbit Panel may not be running.${NC}"
        return
    fi

    PID=$(cat "$PID_FILE")
    if ! ps -p "$PID" > /dev/null 2>&1; then
        rm -f "$PID_FILE"
        return
    fi

    echo "Stopping Rabbit Panel (PID: $PID)..."
    kill "$PID"

    for _ in {1..10}; do
        if ! ps -p "$PID" > /dev/null 2>&1; then
            echo -e "${GREEN}Stopped${NC}"
            rm -f "$PID_FILE"
            return
        fi
        sleep 0.5
    done

    echo "Force stopping..."
    kill -9 "$PID"
    rm -f "$PID_FILE"
    echo -e "${GREEN}Stopped${NC}"
}

status() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p "$PID" > /dev/null 2>&1; then
            echo -e "${GREEN}Rabbit Panel is running (PID: $PID)${NC}"
            return
        fi
    fi
    echo -e "${RED}Rabbit Panel is not running${NC}"
}

log_cmd() {
    if [ -f "$LOG_FILE" ]; then
        tail -f "$LOG_FILE"
    else
        echo "Log file not found"
    fi
}

build() {
    ensure_go

    local target="auto"
    local skip_frontend=""

    while [ $# -gt 0 ]; do
        case "$1" in
            --skip-frontend)
                skip_frontend="1"
                shift
                ;;
            -v|--version)
                VERSION="$2"
                shift 2
                ;;
            auto|all|amd64|arm64|armv7)
                target="$1"
                shift
                ;;
            *)
                echo -e "${RED}Unknown argument: $1${NC}"
                exit 1
                ;;
        esac
    done

    if [ "$skip_frontend" != "1" ]; then
        build_frontend
    fi

    if [ "$target" = "auto" ]; then
        target="$(detect_host_target)"
        if [ -z "$target" ]; then
            echo -e "${RED}Cannot detect host architecture: $(uname -m)${NC}"
            exit 1
        fi
    fi

    if [ "$target" = "all" ]; then
        for t in "${SUPPORTED_TARGETS[@]}"; do
            build_target "$t"
        done
        return
    fi

    if ! is_supported_target "$target"; then
        echo -e "${RED}Unknown build target: $target${NC}"
        echo "Available targets: auto | all | ${SUPPORTED_TARGETS[*]}"
        exit 1
    fi

    build_target "$target"
}

show_help() {
    echo "Rabbit Panel management script"
    echo ""
    echo "Usage: $0 <command> [args]"
    echo ""
    echo "Commands:"
    echo "  start"
    echo "  stop"
    echo "  restart"
    echo "  status"
    echo "  log"
    echo "  build [target]"
    echo ""
    echo "Build targets:"
    echo "  auto | all | amd64 | arm64 | armv7"
    echo "  --skip-frontend"
    echo "  -v, --version <ver>"
}

case "$1" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        stop
        sleep 1
        start
        ;;
    status)
        status
        ;;
    log)
        log_cmd
        ;;
    build)
        shift
        build "$@"
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status|build|log|help}"
        exit 1
        ;;
esac

