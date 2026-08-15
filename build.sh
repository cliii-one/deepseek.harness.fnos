#!/bin/bash
set -e

echo "=================================================="
echo "[INFO] 开始打包构建 DeepSeek Harness"
echo "=================================================="

echo "[INFO] 正在构建 Vue 3 前端静态资产..."
cd frontend
npm run build
cd ..

echo "[INFO] 正在交叉编译 Linux x86_64 二进制文件..."
mkdir -p fnpack/app/bin
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o fnpack/app/bin/deepseek.harness .

echo "[INFO] 正在调用 fnpack 构建 fpk 安装包..."
cd fnpack
# 使用 fnpack 目录下的本地打包 CLI
case "$(uname -s)" in
    Linux*) FP_BIN="fnpack-1.2.3-linux-amd64" ;;
    MINGW*|MSYS*|CYGWIN*) FP_BIN="fnpack-1.2.3-windows-amd64" ;;
    *) FP_BIN="" ;;
esac
if [ -n "$FP_BIN" ] && [ -x "./$FP_BIN" ]; then
    "./$FP_BIN" build
else
    echo "[WARN] fnpack 目录下未找到本地打包 CLI（$FP_BIN），跳过 fpk 生成"
fi
cd ..

echo "=================================================="
echo "[INFO] 核心构建与 .fpk 打包已完成！"
echo "=================================================="
