#!/bin/bash

# Rabbit Panel 管理脚本
# 用法: ./rabbit.sh {start|stop|restart|status|log}

set -e

# 获取脚本所在目录
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# 配置
PID_FILE="rabbit-panel.pid"
LOG_FILE="rabbit-panel.log"

# 颜色
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 检查二进制文件
check_binary() {
    BINARY=""
    if [ -f "dist/rabbit-panel-linux-amd64-release/rabbit-panel-linux-amd64" ]; then
        BINARY="dist/rabbit-panel-linux-amd64-release/rabbit-panel-linux-amd64"
    elif [ -f "dist/rabbit-panel-linux-arm64-release/rabbit-panel-linux-arm64" ]; then
        BINARY="dist/rabbit-panel-linux-arm64-release/rabbit-panel-linux-arm64"
    elif [ -f "dist/rabbit-panel-linux-armv7-release/rabbit-panel-linux-armv7" ]; then
        BINARY="dist/rabbit-panel-linux-armv7-release/rabbit-panel-linux-armv7"
    elif [ -f "./rabbit-panel" ]; then
        BINARY="./rabbit-panel"
    else
        echo -e "${RED}错误: 找不到编译好的二进制文件${NC}"
        echo "请先运行: ./build.sh"
        exit 1
    fi
}

# 设置环境变量
set_env() {
    export MODE="${MODE:-master}"
    export PORT="${PORT:-9999}"
    export HOST="${HOST:-0.0.0.0}"

    if [ -z "$JWT_SECRET" ]; then
        # echo -e "${YELLOW}提示: JWT_SECRET 未设置，使用默认值${NC}"
        export JWT_SECRET="rabbit-panel-secret-key-change-in-production"
    fi

    if [ -z "$NODE_SECRET" ]; then
        # echo -e "${YELLOW}提示: NODE_SECRET 未设置，使用默认值${NC}"
        export NODE_SECRET="rabbit-panel-node-secret-change-in-production"
    fi
}

# 启动
start() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p "$PID" > /dev/null 2>&1; then
            echo -e "${YELLOW}Rabbit Panel 已经在运行中 (PID: $PID)${NC}"
            return
        else
            echo "清理过期的 PID 文件..."
            rm "$PID_FILE"
        fi
    fi

    check_binary
    set_env

    echo -e "${GREEN}正在启动 Rabbit Panel...${NC}"
    echo "模式: $MODE | 端口: $PORT | 架构: $(uname -m)"
    
    nohup "$BINARY" > "$LOG_FILE" 2>&1 &
    PID=$!
    echo "$PID" > "$PID_FILE"
    
    sleep 1
    if ps -p "$PID" > /dev/null 2>&1; then
        echo -e "${GREEN}启动成功! (PID: $PID)${NC}"
        echo "日志文件: $LOG_FILE"
    else
        echo -e "${RED}启动失败，请查看日志:${NC}"
        cat "$LOG_FILE"
    fi
}

# 停止
stop() {
    if [ ! -f "$PID_FILE" ]; then
        echo -e "${YELLOW}未找到 PID 文件，Rabbit Panel 可能未运行${NC}"
        return
    fi

    PID=$(cat "$PID_FILE")
    if ! ps -p "$PID" > /dev/null 2>&1; then
        echo "进程 $PID 不存在，清理 PID 文件..."
        rm "$PID_FILE"
        return
    fi

    echo "正在停止 Rabbit Panel (PID: $PID)..."
    kill "$PID"

    for i in {1..10}; do
        if ! ps -p "$PID" > /dev/null 2>&1; then
            echo -e "${GREEN}已停止${NC}"
            rm "$PID_FILE"
            return
        fi
        sleep 0.5
    done

    echo "强制停止..."
    kill -9 "$PID"
    rm "$PID_FILE"
    echo -e "${GREEN}已强制停止${NC}"
}

# 状态
status() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if ps -p "$PID" > /dev/null 2>&1; then
            echo -e "${GREEN}Rabbit Panel 正在运行 (PID: $PID)${NC}"
            return
        fi
    fi
    echo -e "${RED}Rabbit Panel 未运行${NC}"
}

# 日志
log() {
    if [ -f "$LOG_FILE" ]; then
        tail -f "$LOG_FILE"
    else
        echo "日志文件不存在"
    fi
}

# 编译
build() {
    echo -e "${GREEN}开始编译...${NC}"
    if [ -f "./build.sh" ]; then
        chmod +x ./build.sh
        ./build.sh
    else
        echo -e "${RED}错误: 找不到 build.sh${NC}"
        exit 1
    fi
}

# 主逻辑
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
        log
        ;;
    build)
        build
        ;;
    *)
        echo "用法: $0 {start|stop|restart|status|build|log}"
        exit 1
        ;;
esac
