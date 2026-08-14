@echo off
chcp 65001 >nul
echo ==================================================
echo [INFO] 开始打包构建 DeepSeek Harness
echo ==================================================

echo [INFO] 正在构建 Vue 3 前端静态资产...
cd frontend
call npm run build
if %ERRORLEVEL% neq 0 (
    echo [ERROR] 前端构建失败！
    pause
    exit /b %ERRORLEVEL%
)
cd ..

echo [INFO] 正在交叉编译 Linux x86_64 二进制文件...
if not exist fnpack\app\bin (
    mkdir fnpack\app\bin
)
set GOOS=linux
set GOARCH=amd64
go build -trimpath -ldflags="-s -w" -o fnpack\app\bin\deepseek.harness .
if %ERRORLEVEL% neq 0 (
    echo [ERROR] 后端编译失败！
    pause
    exit /b %ERRORLEVEL%
)

echo [INFO] 正在调用 fnpack 构建 fpk 安装包...
cd fnpack
call fnpack-1.2.3-windows-amd64 build
if %ERRORLEVEL% neq 0 (
    echo [ERROR] fpk 包构建失败！
    cd ..
    pause
    exit /b %ERRORLEVEL%
)
cd ..

echo ==================================================
echo [INFO] 核心构建与 .fpk 打包已完成！
echo ==================================================
pause
