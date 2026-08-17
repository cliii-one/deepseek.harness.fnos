import type { Plugin } from 'vite'
import { WebSocketServer, WebSocket } from 'ws'
import type { IncomingMessage, ServerResponse } from 'http'

export function viteDevMock(): Plugin {
  return {
    name: 'vite-plugin-dev-mock',
    configureServer(server) {
      // 仿真状态机数据源
      let status = 'running'
      let pid: number | null = 1043416
      let startedAt = Math.floor(Date.now() / 1000) - 3600
      let lastMsg = ''

      const config = {
        server_port: 2298,
        proxy_port: 2299,
        network_proxy: '',
        reverse_proxy_url: '',
        access_password: '',
        data_library_path: '/vol1/@appdata/deepseek.harness',
        version: '0.12.2',
        commit: '7b8f9a2',
        build_time: '2026-08-17 11:30:00'
      }

      const workspaces = [
        {
          workspaceId: 'ws-main-dev',
          title: 'DeepSeek 核心研发项目',
          path: '/vol1/1000/Projects/deepseek-core',
          sessionIds: ['sess-001', 'sess-002', 'sess-003'],
          updatedAt: new Date(Date.now() - 1000 * 60 * 25).toISOString()
        },
        {
          workspaceId: 'ws-docs-site',
          title: 'Harness 官方文档站点',
          path: '/vol1/1000/Docs/harness-site',
          sessionIds: ['sess-101'],
          updatedAt: new Date(Date.now() - 1000 * 60 * 180).toISOString()
        }
      ]

      let plugins = [
        {
          name: 'dsh-better-sidebar',
          version: '0.12.2',
          spec: '^0.12.2',
          layer: true
        },
        {
          name: '@dsh-external/dsh-client-ui-skin-maid-atelier',
          version: '1.0.4',
          spec: 'github:Small-tailqwq/dsh-deep-whale#path:/maid-atelier',
          layer: false
        }
      ]

      let logs = [
        '2026-08-17 11:30:00 [INFO ] 启动 DeepSeek Harness 守护服务 v0.12.2 (Commit=7b8f9a2)',
        '2026-08-17 11:30:01 [INFO ] 服务主进程已拉起 (PID=1043416)，正在等待 Web 服务就绪...',
        '2026-08-17 11:30:03 [INFO ] Web 服务就绪探测通过，反向代理启动完成 [0.0.0.0:2299 → http://127.0.0.1:2298]',
        '2026-08-17 11:30:03 [INFO ] [状态变更] stopped → running',
        '2026-08-17 11:30:04 [INFO ] dsh web: http://127.0.0.1:2298'
      ]

      function getStatusPayload() {
        return {
          name: 'DeepSeek Harness',
          version: config.version,
          commit: config.commit,
          status,
          uptime: status === 'running' ? '1小时0分0秒' : null,
          started_at: status === 'running' ? startedAt : 0,
          server_port: config.server_port,
          server_time: Math.floor(Date.now() / 1000),
          build_time: config.build_time,
          app_url: `:${config.proxy_port}/`,
          pid,
          last_message: lastMsg
        }
      }

      // WebSocket 仿真服务
      const wss = new WebSocketServer({ noServer: true })
      const clients = new Set<WebSocket>()

      function broadcast(type: string, data: unknown) {
        const payload = JSON.stringify({
          type,
          data,
          timestamp: Date.now()
        })
        clients.forEach((client) => {
          if (client.readyState === WebSocket.OPEN) {
            client.send(payload)
          }
        })
      }

      wss.on('connection', (ws) => {
        clients.add(ws)
        ws.send(JSON.stringify({ type: 'status', data: getStatusPayload(), timestamp: Date.now() }))
        ws.send(JSON.stringify({ type: 'workspace', data: { workspaces, dataLibraryPath: config.data_library_path }, timestamp: Date.now() }))
        ws.send(JSON.stringify({ type: 'plugin', data: { running: false }, timestamp: Date.now() }))

        ws.on('message', (msg) => {
          try {
            const parsed = JSON.parse(msg.toString())
            if (parsed.type === 'ping') {
              ws.send(JSON.stringify({ type: 'pong', data: { server_time: Math.floor(Date.now() / 1000) }, timestamp: Date.now() }))
            }
          } catch {
            // 忽略
          }
        })

        ws.on('close', () => {
          clients.delete(ws)
        })
      })

      if (server.httpServer) {
        server.httpServer.on('upgrade', (req, socket, head) => {
          const url = req.url || ''
          if (url.includes('/api/ws')) {
            wss.handleUpgrade(req, socket, head, (ws) => {
              wss.emit('connection', ws, req)
            })
          }
        })
      }

      function sendJson(res: ServerResponse, code: number, message: string, data: unknown, httpStatus = 200) {
        res.statusCode = httpStatus
        res.setHeader('Content-Type', 'application/json; charset=utf-8')
        res.end(JSON.stringify({
          code,
          message,
          data,
          timestamp: Date.now()
        }))
      }

      function readJsonBody(req: IncomingMessage): Promise<any> {
        return new Promise((resolve) => {
          let body = ''
          req.on('data', (chunk) => (body += chunk))
          req.on('end', () => {
            try {
              resolve(body ? JSON.parse(body) : {})
            } catch {
              resolve({})
            }
          })
        })
      }

      // HTTP 中间件拦截
      server.middlewares.use(async (req, res, next) => {
        const url = req.url || ''
        if (!url.includes('/api/')) {
          return next()
        }

        const path = url.split('?')[0]

        // 1. 状态快照
        if (path.endsWith('/api/status') && req.method === 'GET') {
          return sendJson(res, 0, 'success', getStatusPayload())
        }

        // 2. 控制动作
        if (path.endsWith('/api/action') && req.method === 'POST') {
          const body = await readJsonBody(req)
          const action = body.action

          if (action === 'start') {
            status = 'starting'
            lastMsg = '服务主进程已拉起，正在等待 Web 服务就绪…'
            broadcast('status', getStatusPayload())
            setTimeout(() => {
              status = 'running'
              pid = 1043500 + Math.floor(Math.random() * 1000)
              startedAt = Math.floor(Date.now() / 1000)
              lastMsg = ''
              const logLine = `${new Date().toISOString().replace('T', ' ').slice(0, 19)} [INFO ] 服务主进程已拉起 (PID=${pid})，Web 就绪`
              logs.push(logLine)
              broadcast('log', logLine + '\n')
              broadcast('status', getStatusPayload())
            }, 1800)
            return sendJson(res, 0, '启动指令已发送，正在等待服务就绪…', getStatusPayload())
          }

          if (action === 'stop') {
            status = 'stopped'
            pid = null
            lastMsg = ''
            broadcast('status', getStatusPayload())
            return sendJson(res, 0, '服务已停止', getStatusPayload())
          }

          if (action === 'restart') {
            status = 'starting'
            pid = null
            lastMsg = '服务正在热重启中，正在等待端口就绪…'
            broadcast('status', getStatusPayload())
            setTimeout(() => {
              status = 'running'
              pid = 1044000 + Math.floor(Math.random() * 1000)
              startedAt = Math.floor(Date.now() / 1000)
              lastMsg = ''
              const logLine = `${new Date().toISOString().replace('T', ' ').slice(0, 19)} [INFO ] 服务已热重启完成 (PID=${pid})`
              logs.push(logLine)
              broadcast('log', logLine + '\n')
              broadcast('status', getStatusPayload())
            }, 2500)
            return sendJson(res, 0, '重启指令已发送，正在等待服务就绪…', getStatusPayload())
          }

          if (action === 'upgrade' || action === 'rebuild') {
            status = 'building'
            lastMsg = action === 'upgrade' ? '正在检查远程更新并编译…' : '正在重新拉取依赖并完整编译…'
            broadcast('status', getStatusPayload())
            setTimeout(() => {
              status = 'running'
              lastMsg = ''
              config.build_time = new Date().toISOString().replace('T', ' ').slice(0, 19)
              broadcast('status', getStatusPayload())
            }, 3500)
            return sendJson(res, 0, action === 'upgrade' ? '开始拉取远程更新并构建…' : '开始强制重建源码…', getStatusPayload())
          }
        }

        // 3. 工作区
        if (path.endsWith('/api/workspaces') && req.method === 'GET') {
          return sendJson(res, 0, 'success', {
            workspaces,
            dataLibraryPath: config.data_library_path
          })
        }

        // 4. 插件管理
        if (path.endsWith('/api/plugins') && req.method === 'GET') {
          return sendJson(res, 0, '插件列表已更新', {
            profile: 'web',
            plugins,
            builtin: ['@deepseek-ai/dsh-base', '@deepseek-ai/dsh-web-app'],
            bundles: plugins.filter((p) => p.layer).map((p) => p.name)
          })
        }

        if (path.endsWith('/api/plugins/preview') && req.method === 'POST') {
          const body = await readJsonBody(req)
          const cmd = body.command || ''
          if (cmd.includes('add')) {
            return sendJson(res, 0, 'success', {
              valid: true,
              ok: true,
              verb: 'add',
              command: cmd,
              specs: [cmd.split('add')[1]?.trim() || 'plugin']
            })
          }
          return sendJson(res, 0, 'success', {
            valid: false,
            ok: false,
            reason: '输入框仅支持安装命令（add）'
          })
        }

        if (path.endsWith('/api/plugins/toggle') && req.method === 'POST') {
          const body = await readJsonBody(req)
          const p = plugins.find((item) => item.name === body.name)
          if (p) p.layer = body.enabled
          const msg = body.enabled ? `已启用插件「${body.name}」` : `已禁用插件「${body.name}」`
          return sendJson(res, 0, msg, { name: body.name, enabled: body.enabled })
        }

        if (path.endsWith('/api/plugins/run') && req.method === 'POST') {
          const body = await readJsonBody(req)
          const name = body.command?.split('add')[1]?.trim() || 'new-plugin'
          if (!plugins.find((p) => p.name === name)) {
            plugins.push({ name, version: '1.0.0', spec: 'latest', layer: true })
          }
          broadcast('plugin', { running: true })
          setTimeout(() => {
            broadcast('plugin', { running: false, ok: true, message: '安装完成，重启服务后生效' })
          }, 1500)
          return sendJson(res, 0, '已开始执行插件安装', { command: body.command })
        }

        // 5. 日志
        if (path.endsWith('/api/logs') && req.method === 'GET') {
          return sendJson(res, 0, 'success', {
            lines: logs.map((l) => l + '\n'),
            content: logs.join('\n')
          })
        }

        if (path.endsWith('/api/logs') && req.method === 'DELETE') {
          logs = []
          return sendJson(res, 0, '运行日志已清空', true)
        }

        // 6. 配置
        if (path.endsWith('/api/config') && req.method === 'GET') {
          return sendJson(res, 0, 'success', config)
        }

        if (path.endsWith('/api/config') && req.method === 'POST') {
          const body = await readJsonBody(req)
          Object.assign(config, body)
          return sendJson(res, 0, '应用设置保存成功', config)
        }

        next()
      })
    }
  }
}
