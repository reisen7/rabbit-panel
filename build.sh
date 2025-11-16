#!/bin/bash

# 容器运维面板编译脚本
# 支持 ARM64/armv7l/x86_64 架构

set -e

VERSION="1.0.0"
APP_NAME="rabbit-panel"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}开始编译容器运维面板...${NC}"

# 检查 Go 环境
if ! command -v go &> /dev/null; then
    echo -e "${YELLOW}错误: 未安装 Go 环境，请先安装 Go 1.22+${NC}"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo -e "Go 版本: ${GO_VERSION}"

# 设置 Go 代理（解决网络问题）
if [ -z "$GOPROXY" ]; then
    export GOPROXY=https://goproxy.cn,direct
    echo -e "${GREEN}已设置 Go 代理: ${GOPROXY}${NC}"
fi

# 下载依赖
echo -e "${GREEN}正在下载依赖...${NC}"
go mod download
go mod tidy

# 创建输出目录
mkdir -p dist

# 编译函数
build_target() {
    local arch=$1
    local os_name=$2
    local output_name=$3
    
    echo -e "${GREEN}编译 ${os_name}/${arch}...${NC}"
    
    export GOOS=$os_name
    export GOARCH=$arch
    export CGO_ENABLED=1
    
    output_path="dist/${output_name}"
    
    # 编译（使用 . 编译整个包，包含所有 .go 文件）
    go build -ldflags="-s -w" -o "$output_path" .
    if [ $? -ne 0 ]; then
        echo -e "${YELLOW}✗ ${output_name} 编译失败${NC}"
        return 1
    fi
    
    # 计算文件大小
    size=$(du -h "$output_path" | cut -f1)
    echo -e "${GREEN}✓ ${output_name} 编译完成 (${size})${NC}"
    
    # 创建发布目录
    release_dir="dist/${output_name}-release"
    mkdir -p "$release_dir"
    cp "$output_path" "$release_dir/"
    
    # 检查 static 目录是否存在
    if [ ! -d "static" ]; then
        echo -e "${YELLOW}警告: static 目录不存在，跳过复制${NC}"
    else
        cp -r static "$release_dir/"
    fi
    
    echo -e "${GREEN}  发布包已创建: ${release_dir}/${NC}"
}

# 编译当前系统架构
echo -e "\n${GREEN}=== 开始编译 ===${NC}\n"

# 获取当前系统架构
CURRENT_ARCH=$(go env GOARCH)
CURRENT_OS=$(go env GOOS)

echo -e "${GREEN}当前系统: ${CURRENT_OS}/${CURRENT_ARCH}${NC}"
echo ""

# 编译当前架构
if [ "$CURRENT_ARCH" = "amd64" ]; then
    build_target "amd64" "linux" "${APP_NAME}-linux-amd64"
elif [ "$CURRENT_ARCH" = "arm64" ]; then
    build_target "arm64" "linux" "${APP_NAME}-linux-arm64"
elif [ "$CURRENT_ARCH" = "arm" ]; then
    export GOARM=7
    build_target "arm" "linux" "${APP_NAME}-linux-armv7"
    unset GOARM
else
    echo -e "${YELLOW}警告: 不支持的架构 ${CURRENT_ARCH}${NC}"
    build_target "$CURRENT_ARCH" "linux" "${APP_NAME}-linux-${CURRENT_ARCH}"
fi

echo -e "\n${GREEN}=== 编译完成 ===${NC}\n"
echo -e "输出目录: dist/"
echo -e "\n使用说明:"
echo -e "1. 将 dist/*-release/ 目录复制到目标服务器"
echo -e "2. 进入目录，运行: ./${APP_NAME}-linux-amd64 (或对应的架构版本)"
echo -e "3. 访问: http://<服务器IP>:9999"
echo -e "\n注意: 确保目标服务器已安装 Docker 并运行，当前用户有 Docker 访问权限"

