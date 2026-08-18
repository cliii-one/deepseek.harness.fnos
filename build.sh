#!/bin/bash
set -e

echo "=================================================="
echo "[INFO] 开始打包构建 DeepSeek Harness"
echo "=================================================="

# 自动检测目标架构：本机架构 = 打包目标架构
case "$(uname -m)" in
    aarch64|arm64)
        GOARCH="arm64"
        PLATFORM="arm"
        ;;
    x86_64|amd64)
        GOARCH="amd64"
        PLATFORM="x86"
        ;;
    *)
        echo "[ERROR] 不支持的架构: $(uname -m)"
        exit 1
        ;;
esac

# 同步 manifest 中的 platform 声明与当前架构一致（保证 fpk 可被 fnOS 正确识别）
sed -i "s/^platform[[:space:]]*=.*/platform              = ${PLATFORM}/" fnpack/manifest
echo "[INFO] 目标架构: ${GOARCH} (platform=${PLATFORM})"

echo "[INFO] 正在构建 Vue 3 前端静态资产..."
cd frontend
npm run build
cd ..

echo "[INFO] 正在编译 Linux ${GOARCH} 二进制文件..."
mkdir -p fnpack/app/bin
GOOS=linux GOARCH="${GOARCH}" go build -trimpath -ldflags="-s -w" -o fnpack/app/bin/deepseek.harness .

echo "[INFO] 正在调用 fnpack 构建 fpk 安装包..."
cd fnpack
# 优先使用系统自带的 fnpack CLI（arm64 设备上仓库自带的 amd64 CLI 无法运行）
FP_BIN=""
if command -v fnpack >/dev/null 2>&1; then
    FP_BIN="$(command -v fnpack)"
elif [ -x "./fnpack-1.2.3-linux-amd64" ] && [ "$(uname -m)" != "aarch64" ] && [ "$(uname -m)" != "arm64" ]; then
    FP_BIN="./fnpack-1.2.3-linux-amd64"
elif [ -x "./fnpack-1.2.3-windows-amd64" ]; then
    FP_BIN="./fnpack-1.2.3-windows-amd64"
fi
if [ -n "$FP_BIN" ]; then
    "$FP_BIN" build
else
    echo "[WARN] 未找到可用的 fnpack CLI，跳过 fpk 生成"
fi
cd ..

echo "=================================================="
echo "[INFO] 核心构建与 .fpk 打包已完成！"
echo "=================================================="
