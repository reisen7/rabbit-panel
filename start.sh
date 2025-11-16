#!/bin/bash

# Rabbit Panel 启动脚本

set -e

# 获取脚本所在目录
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# 检查二进制文件
BINARY=""
if [ -f "dist/rabbit-panel-linux-amd64-release/rabbit-panel-linux-amd64" ]; then
    BINARY="dist/rabbit-panel-linux-amd64-release/rabbit-panel-linux-amd64"
elif [ -f "dist/rabbit-panel-linux-arm64-release/rabbit-panel-linux-arm64" ]; then
    BINARY="dist/rabbit-panel-linux-arm64-release/rabbit-panel-linux-arm64"
elif [ -f "dist/rabbit-panel-linux-armv7-release/rabbit-panel-linux-armv7" ]; then
    BINARY="dist/rabbit-panel-linux-armv7-release/rabbit-panel-linux-armv7"
else
    echo "错误: 找不到编译好的二进制文件"
    echo "请先运行: ./build.sh"
    exit 1
fi

# 设置默认环境变量（如果未设置）
export MODE="${MODE:-master}"
export PORT="${PORT:-9999}"
export HOST="${HOST:-0.0.0.0}"

# 生成默认密钥（如果未设置）
if [ -z "$JWT_SECRET" ]; then
    echo "警告: JWT_SECRET 未设置，使用默认值（仅用于开发）"
    export JWT_SECRET="rabbit-panel-secret-key-change-in-production"
fi

if [ -z "$NODE_SECRET" ]; then
    echo "警告: NODE_SECRET 未设置，使用默认值（仅用于开发）"
    export NODE_SECRET="rabbit-panel-node-secret-change-in-production"
fi

# 显示启动信息
echo "=========================================="
echo "启动 Rabbit Panel"
echo "=========================================="
echo "模式: $MODE"
echo "端口: $PORT"
echo "监听: $HOST"
echo "二进制: $BINARY"
echo ""

# 启动应用
exec "$BINARY"
