import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { StatusData, RequestResult } from '../types/api'
import { systemApi, configApi } from '../api'

export const useSystemStore = defineStore('system', () => {
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

  const wsConnected = ref(true)
  const serverTimeOffset = ref(0)
  const actionBusy = ref(false)
  const currentTime = ref(Date.now())

  let clockTimer: ReturnType<typeof setInterval> | null = null

  function startClock() {
    if (clockTimer) return
    clockTimer = setInterval(() => {
      currentTime.value = Date.now()
    }, 1000)
  }

  function stopClock() {
    if (clockTimer) {
      clearInterval(clockTimer)
      clockTimer = null
    }
  }

  function setWsConnected(val: boolean) {
    wsConnected.value = val
  }

  function updateStatus(data: Partial<StatusData>) {
    statusData.value = {
      ...statusData.value,
      ...data
    }
    if (data.server_time) {
      serverTimeOffset.value = data.server_time * 1000 - Date.now()
    }
  }

  const isRunning = computed(() => statusData.value.status === 'running')
  const isBuilding = computed(() => statusData.value.status === 'building')

  const statusTagType = computed<'success' | 'info' | 'default'>(() => {
    if (isRunning.value) return 'success'
    if (isBuilding.value) return 'info'
    return 'default'
  })

  const statusLabel = computed(() => {
    if (isRunning.value) return '运行中'
    if (isBuilding.value) return '源码构建中'
    return '已停止'
  })

  function formatDuration(total: number): string {
    const h = Math.floor(total / 3600)
    const m = Math.floor((total % 3600) / 60)
    const s = total % 60
    if (h > 0) return `${h}小时${m}分${s}秒`
    if (m > 0) return `${m}分${s}秒`
    return `${s}秒`
  }

  const uptimeText = computed(() => {
    const s = statusData.value
    if (s.status !== 'running' || !s.started_at) return '-'
    const serverNow = currentTime.value + serverTimeOffset.value
    const secs = Math.max(0, Math.floor((serverNow - s.started_at * 1000) / 1000))
    return formatDuration(secs)
  })

  async function sendAction(action: string): Promise<RequestResult<StatusData>> {
    actionBusy.value = true
    try {
      const res = await systemApi.sendAction(action)
      if (res.success && res.data) {
        updateStatus(res.data)
      }
      return res
    } finally {
      actionBusy.value = false
    }
  }

  async function openHarnessApp(): Promise<void> {
    const res = await configApi.getConfig()
    if (res.success && res.data?.reverse_proxy_url) {
      window.open(res.data.reverse_proxy_url, '_blank')
      return
    }
    const appUrl = statusData.value.app_url
    if (appUrl?.startsWith(':')) {
      window.open(`https://${window.location.hostname}${appUrl}`, '_blank')
    }
  }

  return {
    statusData,
    wsConnected,
    serverTimeOffset,
    actionBusy,
    isRunning,
    isBuilding,
    statusTagType,
    statusLabel,
    uptimeText,
    startClock,
    stopClock,
    setWsConnected,
    updateStatus,
    sendAction,
    openHarnessApp
  }
})
