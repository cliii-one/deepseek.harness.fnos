# DeepSeek Harness for fnOS

专为飞牛 fnOS 打造的 DeepSeek Harness 一键部署与可视化管理应用，当前版本 **v0.3.5**。

> 本仓库基于 **[yuexps/deepseek.harness.fnos](https://github.com/yuexps/deepseek.harness.fnos)** 二次开发与持续维护。
> 感谢原作者 **yuexps** 的开源基础与成果，本项目的服务生命周期、插件管理、网关代理、源码构建等核心机制均建立在其工作之上；
> 本仓库在保留核心能力的前提下，针对个人实际使用场景做了界面精简、交互优化与体验打磨（详见下方「相对原版的改动」）。

---

## 主要功能

- **服务控制**：支持启动、停止、重启、源码构建与升级，内置进程组清理与看门狗巡检自愈（进程异常自动拉起、孤儿进程清理）。
- **检查更新**：一键检查 DSH 服务是否有新版本 —— 先只读比对本地与远程 commit，发现新版本自动走「拉取 → 构建 → 重启」更新链路；无更新则直接提示已是最新，不浪费网络请求。
- **强制重建**：重新拉取依赖并完整编译，用于修复异常损坏的构建环境。
- **沉浸式 WebUI**：管理面板内直接打开 Harness WebUI（DSH 聊天界面），全屏内嵌；右下角悬浮工具按钮（返回管理 / 新标签页打开 / 重新加载），不遮挡界面内容。
- **插件管理**：支持命令安装（npm 包 / GitHub 仓库）、压缩包上传、构建脚本自动放行（allowBuilds）、一键启停与卸载，含故障自愈。
- **工作区查看**：实时同步工作区列表、关联会话数及更新时间，支持在文件管理中一键定位目录。
- **运行日志**：WebSocket 实时推送日志，支持语法高亮、自动滚动、日志轮转、清空与下载。
- **安全与网关**：访问固定走飞牛统一网关（fngateway），管理面板及内嵌 WebUI 均经网关反代；内置访问鉴权页面。

---

## 相对原版的改动

- **界面统一**：合并桌面端两个入口为一个，统一经飞牛统一网关访问。
- **沉浸式体验**：进入 Harness WebUI 后自动收起管理侧栏，按钮悬浮于右下角，不遮挡网页内容与 logo。
- **交互精简**：移除「应用设置」页面及相关设置流程（服务端口 / 反代端口 / 打开方式 / 访问密码等均在部署时以配置为准），去掉冗余状态提示。
- **更新流程**：将「拉取更新」改造为「检查更新」—— 先只读比对远程 commit，确认有新版本再自动执行更新构建。
- **死代码清理**：对原版中已不再使用的功能（如自定义反代地址、端口占用检测、设置项持久化等）前后端做了彻底的清理，保持代码库精简。

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
├── build.go            # 源码拉取(Git)、检查更新、构建与版本比对
├── plugins.go          # 插件解析、安装、启停与安全校验
├── allowbuilds.go      # 插件构建白名单管理
├── workspace.go        # 工作区数据提取与文件监控
├── proxy.go            # 内置反向代理服务与访问认证
├── fngateway.go        # 飞牛网关直连反代与状态感知
├── api.go              # RESTful API 与 WebSocket 实时通道
├── config.go           # 应用配置持久化
├── env.go              # 全局运行环境变量与代理注入
├── logger.go           # 运行日志记录、轮转与增量订阅
├── main.go             # 程序入口与 Unix Socket 监听
├── templates/          # 独立内嵌页面模板 (//go:embed)
│   ├── auth_login.html     # 访问鉴权页面
│   └── gateway_status.html # 网关状态与错误引导页面
├── fnpack/             # 飞牛 OS 应用包配置与生命周期脚本
│   ├── manifest        # 应用元数据清单
│   ├── config/         # 权限与资源配置
│   └── cmd/            # 安装、启动、停止与卸载脚本
└── frontend/           # Naive UI 前端项目
    ├── src/
    │   ├── api/        # 统一 API 接口服务
    │   ├── mock/       # 本地离线开发仿真插件 (Vite)
    │   ├── stores/     # Pinia 模块化状态管理
    │   ├── utils/      # HTTP 客户端与 WebSocket 管理器
    │   ├── types/      # TypeScript 契约与类型定义
    │   ├── views/      # 概览、工作区、插件、日志、WebUI 视图
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
cd ..
```

### 2. 后端编译（目标 Linux arm64）
```bash
GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o fnpack/app/bin/deepseek.harness .
```

### 3. 生成飞牛 fnOS 安装包
在项目根目录下执行 `build.sh`（或 Windows 下 `build.cmd`）即可生成 `.fpk` 安装包。

---

## 致谢

再次感谢 [yuexps/deepseek.harness.fnos](https://github.com/yuexps/deepseek.harness.fnos) 原作者的开源成果。
本仓库的维护与改进均基于其原始工作，特此说明并致谢。