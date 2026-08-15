import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiGet } from '../api'

export interface StatusData {
  name: string
  version: string
  commit: string
  status: string
  uptime: string
  started_at: number
  build_time: string
  app_url: string
  last_message: string
}

export interface WorkspaceItem {
  workspaceId: string
  path: string
  title: string
  sessionIds: string[]
  createdAt: string
  updatedAt: string
}

export interface WorkspaceData {
  items: WorkspaceItem[]
  archivedSessionIds: string[]
}

export interface PluginStatus {
  running: boolean
  ok?: boolean
  message?: string
}

const logListeners = new Set<(chunk: string) => void>()
const reconnectListeners = new Set<() => void>()
const pluginListeners = new Set<(s: PluginStatus) => void>()

export const useAppStore = defineStore('app', () => {
  const statusData = ref<StatusData>({
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

  const workspaceData = ref<WorkspaceData>({
    items: [],
    archivedSessionIds: []
  })

  const wsConnected = ref(true)
  const currentTab = ref('overview')
  const pluginBusy = ref(false)

  function setTab(tab: string) {
    currentTab.value = tab
  }

  let ws: WebSocket | null = null
  let opened = false
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  const RECONNECT_DELAY = 3000

  async function fetchWorkspaceData() {
    const res = await apiGet<WorkspaceData>('workspace/list')
    if (res?.ok && res.data) {
      workspaceData.value = res.data
    }
  }

  function connectWS() {
    if (ws) return
    try {
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
      let msg: { type: string; data?: unknown }
      try {
        msg = JSON.parse(e.data as string)
      } catch { return }
      if (msg.type === 'status' && msg.data) {
        statusData.value = msg.data as StatusData
      } else if (msg.type === 'workspace' && msg.data) {
        workspaceData.value = msg.data as WorkspaceData
      } else if (msg.type === 'log' && typeof msg.data === 'string') {
        logListeners.forEach(fn => fn(msg.data as string))
      } else if (msg.type === 'plugin' && msg.data) {
        const s = msg.data as PluginStatus
        pluginBusy.value = s.running
        pluginListeners.forEach(fn => fn(s))
      }
    }

    ws.onclose = () => {
      ws = null
      wsConnected.value = false
      scheduleReconnect()
    }

    ws.onerror = () => {
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

  function onLogChunk(fn: (chunk: string) => void): () => void {
    logListeners.add(fn)
    return () => { logListeners.delete(fn) }
  }

  function onReconnect(fn: () => void): () => void {
    reconnectListeners.add(fn)
    return () => { reconnectListeners.delete(fn) }
  }

  function onPluginEvent(fn: (s: PluginStatus) => void): () => void {
    pluginListeners.add(fn)
    return () => { pluginListeners.delete(fn) }
  }

  return {
    statusData,
    workspaceData,
    wsConnected,
    currentTab,
    pluginBusy,
    setTab,
    connectWS,
    fetchWorkspaceData,
    onLogChunk,
    onReconnect,
    onPluginEvent
  }
})
