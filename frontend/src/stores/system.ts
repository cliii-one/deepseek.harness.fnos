import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { StatusData, RequestResult } from '../types/api'
import { systemApi, configApi } from '../api'
import { trimSdk } from '../utils/trimSdk'
import { usePluginStore } from './plugin'

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
  const activeAction = ref<string | null>(null)
  const currentTime = ref(Date.now())

  let clockTimer: ReturnType<typeof setInterval> | null = null
  let actionTimeoutTimer: ReturnType<typeof setTimeout> | null = null

  function clearActionLock() {
    activeAction.value = null
    if (actionTimeoutTimer) {
      clearTimeout(actionTimeoutTimer)
      actionTimeoutTimer = null
    }
  }

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
    const oldStatus = statusData.value.status
    statusData.value = {
      ...statusData.value,
      ...data
    }
    if (data.server_time) {
      serverTimeOffset.value = data.server_time * 1000 - Date.now()
    }
    const curStatus = statusData.value.status

    // 服务停止或重启恢复运行后清理插件待生效状态
    if (curStatus === 'stopped' || (oldStatus === 'starting' && curStatus === 'running')) {
      const pluginStore = usePluginStore()
      pluginStore.clearRestartNeeded()
    }

    // 状态流转完成时自动解除动作锁定
    if (activeAction.value) {
      if (activeAction.value === 'start') {
        if (curStatus === 'running' || (oldStatus === 'starting' && curStatus === 'stopped') || (oldStatus === 'building' && curStatus === 'stopped')) {
          clearActionLock()
        }
      } else if (activeAction.value === 'restart') {
        if (curStatus === 'running' || (oldStatus === 'starting' && curStatus === 'stopped')) {
          clearActionLock()
        }
      } else if (activeAction.value === 'upgrade' || activeAction.value === 'rebuild') {
        if (curStatus !== 'building') {
          clearActionLock()
        }
      } else if (activeAction.value === 'stop') {
        if (curStatus === 'stopped') {
          clearActionLock()
        }
      }
    }
  }

  const isRunning = computed(() => statusData.value.status === 'running')
  const isStarting = computed(() => statusData.value.status === 'starting')
  const isBuilding = computed(() => statusData.value.status === 'building')
  const isActionLocked = computed(() => Boolean(activeAction.value) || isBuilding.value || isStarting.value)

  const statusTagType = computed<'success' | 'warning' | 'info' | 'default'>(() => {
    if (isRunning.value) return 'success'
    if (isStarting.value) return 'warning'
    if (isBuilding.value) return 'info'
    return 'default'
  })

  const statusLabel = computed(() => {
    if (isRunning.value) return '运行中'
    if (isStarting.value) return '服务启动中'
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
    if (isActionLocked.value) {
      return { success: false, message: '当前有任务正在进行中，请稍候' }
    }

    activeAction.value = action

    // 超时兜底（最多锁定 60s）
    actionTimeoutTimer = setTimeout(() => {
      clearActionLock()
    }, 60000)

    try {
      const res = await systemApi.sendAction(action)
      if (!res.success) {
        clearActionLock()
      }
      return res
    } catch {
      clearActionLock()
    }
    return { success: false, message: '请求失败' }
  }

  async function openHarnessApp(): Promise<void> {
    const res = await configApi.getConfig()
    if (res.success && res.data?.reverse_proxy_url) {
      await trimSdk.openURL(res.data.reverse_proxy_url, '_blank')
      return
    }
    const appUrl = statusData.value.app_url
    if (appUrl?.startsWith(':')) {
      await trimSdk.openURL(`https://${window.location.hostname}${appUrl}`, '_blank')
    }
  }

  return {
    statusData,
    wsConnected,
    serverTimeOffset,
    activeAction,
    isActionLocked,
    isRunning,
    isStarting,
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
