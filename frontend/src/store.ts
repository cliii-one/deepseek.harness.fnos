import { ref } from 'vue'

export interface StatusData {
  name: string
  version: string
  commit: string
  status: string
  uptime: string
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

/** SSE 断线重连成功时触发（首次连接不触发），用于补拉断线期间遗漏的数据 */
export function onReconnect(fn: () => void): () => void {
  reconnectListeners.add(fn)
  return () => { reconnectListeners.delete(fn) }
}

let es: EventSource | null = null
let opened = false

export function connectEvents() {
  if (es) return
  es = new EventSource('api/events')

  es.onopen = () => {
    if (opened) reconnectListeners.forEach(fn => fn())
    opened = true
  }

  es.addEventListener('status', (e) => {
    try {
      statusData.value = JSON.parse((e as MessageEvent).data)
    } catch { /* ignore */ }
  })

  es.addEventListener('log', (e) => {
    try {
      const chunk = JSON.parse((e as MessageEvent).data) as string
      logListeners.forEach(fn => fn(chunk))
    } catch { /* ignore */ }
  })
}
