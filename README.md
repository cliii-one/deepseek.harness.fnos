# DeepSeek Harness for fnOS

专为飞牛 fnOS 打造的 DeepSeek Harness 一键部署与可视化管理应用。

---

## 主要功能

- **进程管理与自愈**：一键启动、停止、重启、拉取更新与强制重建；内置 Linux 进程组强杀清理、后台 3 秒探活巡检与异常自动纠偏。
- **插件生态管理**：
  - 支持 4 类安装方式：npm 官方包、scoped 组织包、GitHub 简写库、Monorepo 子路径（带引号及 `#branch&path:/subpath`）。
  - 支持插件压缩包上传、快速启停与构建白名单管理。
- **工作区会话管理**：自动扫描并以网格卡片展示活跃工作区、关联会话数与最后更新时间。
- **实时日志终端**：WebSocket 流式推送服务运行日志，支持自动滚动与日志一键导出下载。
- **网络与代理设置**：支持自定义服务端口、反向代理端口、访问密码及外网网络代理。

---

## 预览

<img width="1787" height="1113" alt="1" src="https://github.com/user-attachments/assets/e6b474d3-1fbe-4697-9048-027d4c40aa81" />
<img width="1783" height="1117" alt="4" src="https://github.com/user-attachments/assets/1c3ccb09-a39c-40ba-b5b3-358fe9bdc7d2" />

---

## 技术架构

- **后端**：Go 1.23 + Gin Web 框架 + Gorilla WebSocket + Linux 进程管理
- **前端**：Vue 3 + TypeScript + Naive UI + Pinia + Tailwind CSS + Vite
- **打包规范**：飞牛 fnOS 原生应用包规范（`fnpack`）

---

## 项目结构

```
deepseek.harness/
├── harness.go          # 服务生命周期管理与状态机
├── process.go          # 进程组控制、孤儿清理、端口等待与巡检自愈
├── build.go            # 源码拉取(Git/Zip)、GCC/Musl环境准备与构建
├── plugins.go          # 插件解析、安装、启停与安全校验
├── allowbuilds.go      # 插件构建白名单管理
├── workspace.go        # 工作区数据提取与文件监控
├── proxy.go            # 内置反向代理服务
├── api.go              # RESTful API 与 WebSocket 实时通道
├── config.go           # 应用配置持久化
├── logger.go           # 运行日志记录与订阅
├── main.go             # 程序入口与 Unix Socket 监听
├── fnpack/             # 飞牛 OS 应用包配置与生命周期脚本
│   ├── manifest        # 应用元数据清单
│   ├── config/         # 权限与资源配置
│   └── cmd/            # 安装、启动、停止与卸载脚本
└── frontend/           # Naive UI 前端项目
    ├── src/
    │   ├── api/        # 统一 API 接口服务
    │   ├── stores/     # Pinia 模块化状态管理
    │   ├── utils/      # HTTP 客户端与 WebSocket 管理器
    │   ├── types/      # TypeScript 契约与类型定义
    │   ├── views/      # 概览、工作区、插件、日志、设置视图
    │   └── theme.ts    # 主题定制
    └── package.json
```

---

## 构建与打包

### 1. 前端构建
```bash
cd frontend
npm install
npm run build
```

### 2. 后端编译（针对 Linux）
```bash
set GOOS=linux
set GOARCH=amd64
go build -ldflags "-s -w" -o fnpack/app/bin/deepseek.harness .
```

### 3. 生成飞牛 fnOS 安装包
在项目根目录下执行 `build.cmd` 或 `build.sh` 即可生成 `.fpk` 安装包。
