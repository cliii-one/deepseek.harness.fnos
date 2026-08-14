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
if command -v fnpack-1.2.3-windows-amd64 &> /dev/null; then
    fnpack-1.2.3-windows-amd64 build
elif command -v fnpack &> /dev/null; then
    fnpack build
else
    echo "[WARN] 未检测到 fnpack 打包 CLI 工具，跳过 fpk 生成"
fi
cd ..

echo "=================================================="
echo "[INFO] 核心构建与 .fpk 打包已完成！"
echo "=================================================="
