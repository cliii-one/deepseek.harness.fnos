import { ref } from 'vue'

export interface StatusData {
  name: string
  version: string
  commit: string
  status: string
  uptime: string
  /** 启动时间戳（epoch 秒），前端据此本地计算运行时长 */
  started_at: number
  build_time: string
  app_url: string
  last_message: string
}

export const statusData = ref<StatusData>({
  name: 'DeepSeek Harness',
  version: '-',
  commit: '-',
  status: 'stopped',
  uptime: '-',
  started_at: 0,
  build_time: '-',
  app_url: '/app/deepseek-harness/',
  last_message: ''
})

const logListeners = new Set<(chunk: string) => void>()

export function onLogChunk(fn: (chunk: string) => void): () => void {
  logListeners.add(fn)
  return () => { logListeners.delete(fn) }
}

const reconnectListeners = new Set<() => void>()

/** WebSocket 断线重连成功时触发（首次连接不触发），用于补拉断线期间遗漏的数据 */
export function onReconnect(fn: () => void): () => void {
  reconnectListeners.add(fn)
  return () => { reconnectListeners.delete(fn) }
}

let ws: WebSocket | null = null
let opened = false
let reconnectTimer: ReturnType<typeof setTimeout> | null = null

/** WS 连接状态：断线时 UI 提示（初始 true 避免首屏闪现） */
export const wsConnected = ref(true)

/** 断线重连间隔（毫秒） */
const RECONNECT_DELAY = 3000

interface WSMessage {
  type: string
  data?: unknown
}

export function connectWS() {
  if (ws) return
  try {
    // 固定生产路径（应用部署于 /app/deepseek-harness/ 下）
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    ws = new WebSocket(`${proto}//${window.location.host}/app/deepseek-harness/api/ws`)
  } catch {
    ws = null
    scheduleReconnect()
    return
  }

  ws.onopen = () => {
    if (opened) reconnectListeners.forEach(fn => fn())
    opened = true
    wsConnected.value = true
  }

  ws.onmessage = (e) => {
    let msg: WSMessage
    try {
      msg = JSON.parse(e.data as string)
    } catch { return }
    if (msg.type === 'status' && msg.data) {
      statusData.value = msg.data as StatusData
    } else if (msg.type === 'log' && typeof msg.data === 'string') {
      logListeners.forEach(fn => fn(msg.data as string))
    }
  }

  ws.onclose = () => {
    ws = null
    wsConnected.value = false
    scheduleReconnect()
  }

  ws.onerror = () => {
    // 连接异常：主动 close 触发 onclose 统一重连
    ws?.close()
  }
}

function scheduleReconnect() {
  if (reconnectTimer !== null) return
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    connectWS()
  }, RECONNECT_DELAY)
}
