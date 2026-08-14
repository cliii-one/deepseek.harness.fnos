<template>
  <div class="space-y-6 max-w-7xl mx-auto">
    <h1 class="text-xl font-bold text-slate-800 tracking-tight">概览</h1>

    <!-- 状态卡片 -->
    <div class="bg-white rounded-2xl p-5 sm:p-6 shadow-sm border border-slate-100">
      <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div class="flex items-center gap-4">
          <div class="w-12 h-12 sm:w-14 sm:h-14 bg-slate-100 rounded-2xl flex items-center justify-center text-slate-600 shadow-inner shrink-0">
            <Icon name="monitor" :size="30" />
          </div>
          <div>
            <h2 class="text-base sm:text-lg font-bold text-slate-800">{{ statusData.name }}</h2>
            <p class="text-xs sm:text-sm text-slate-400 mt-0.5">
              版本: {{ statusData.version }} <span class="text-slate-300 mx-1">|</span> Commit: {{ statusData.commit }}
            </p>
          </div>
        </div>

        <button @click="openApp" :disabled="!isRunning"
          class="w-full sm:w-auto px-5 py-2.5 bg-fnos-blue hover:bg-fnos-blue-hover disabled:bg-slate-300 disabled:cursor-not-allowed text-white font-medium rounded-xl text-sm transition-colors shadow-sm flex items-center justify-center gap-2 shrink-0">
          <Icon name="external" :size="16" />
          <span>打开</span>
        </button>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 mt-6 pt-6 border-t border-slate-100 text-sm">
        <div>
          <div class="text-slate-400 text-xs font-medium mb-1.5">运行状态</div>
          <span class="inline-flex items-center px-3 py-0.5 rounded-full text-xs font-medium border" :class="statusMeta.cls">
            <Icon v-if="statusMeta.spin" name="spinner" :size="12" class="mr-1.5" />
            <span v-else class="w-1.5 h-1.5 rounded-full mr-1.5" :class="statusMeta.dot" />
            {{ statusMeta.label }}
          </span>
        </div>
        <div v-for="f in infoFields" :key="f.label">
          <div class="text-slate-400 text-xs font-medium mb-1">{{ f.label }}</div>
          <div class="text-slate-800 font-medium text-base">{{ f.value }}</div>
        </div>
      </div>

      <!-- 实时消息（构建进度 / 异常原因） -->
      <div v-if="statusData.last_message"
        class="mt-4 px-4 py-2.5 rounded-xl text-xs font-medium flex items-center gap-2 border"
        :class="isBuilding ? 'bg-blue-50 text-blue-600 border-blue-100' : 'bg-amber-50 text-amber-600 border-amber-100'">
        <Icon :name="isBuilding ? 'spinner' : 'warning'" :size="14" />
        <span class="truncate">{{ statusData.last_message }}</span>
      </div>
    </div>

    <!-- 运行操作 -->
    <div class="space-y-4">
      <h2 class="text-lg font-bold text-slate-800 tracking-tight">运行操作</h2>
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 sm:gap-5">
        <button v-for="a in actionCards" :key="a.label" @click="doAction(a.action)" :disabled="a.disabled"
          class="bg-white hover:bg-slate-50 border border-slate-100 rounded-2xl p-6 sm:p-7 flex flex-col items-center justify-center gap-3 transition-colors shadow-sm group disabled:opacity-50 disabled:cursor-not-allowed">
          <div class="w-12 h-12 bg-slate-50 rounded-full flex items-center justify-center text-slate-600 transition-colors" :class="a.hover">
            <Icon :name="a.icon" :size="22" />
          </div>
          <span class="text-sm font-medium text-slate-700 group-hover:text-slate-900">{{ a.label }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { statusData, type StatusData } from '../store'
import { apiGet, apiPost } from '../api'
import { showToast } from '../toast'
import Icon, { type IconName } from '../components/Icon.vue'

const loading = ref(false)

const isRunning = computed(() => statusData.value.status === 'running')
const isBuilding = computed(() => statusData.value.status === 'building')

const statusMeta = computed(() => {
  if (isRunning.value) return { label: '运行中', cls: 'bg-emerald-50 text-emerald-600 border-emerald-100', dot: 'bg-emerald-500' }
  if (isBuilding.value) return { label: '源码构建中', cls: 'bg-blue-50 text-blue-600 border-blue-100', spin: true }
  return { label: '已停止', cls: 'bg-slate-100 text-slate-500 border-slate-200', dot: 'bg-slate-400' }
})

const infoFields = computed(() => [
  { label: '运行时间', value: statusData.value.uptime },
  { label: '构建时间', value: statusData.value.build_time }
])

interface ActionCard {
  action: string
  icon: IconName
  label: string
  hover: string
  disabled: boolean
}

const actionCards = computed<ActionCard[]>(() => [
  isRunning.value
    ? { action: 'stop', icon: 'stop', label: '停止服务', hover: 'group-hover:bg-rose-50 group-hover:text-rose-600', disabled: loading.value }
    : { action: 'start', icon: 'play', label: '启动服务', hover: 'group-hover:bg-blue-50 group-hover:text-fnos-blue', disabled: loading.value || isBuilding.value },
  { action: 'restart', icon: 'refresh', label: '重启服务', hover: 'group-hover:bg-amber-50 group-hover:text-amber-600', disabled: loading.value || !isRunning.value },
  { action: 'upgrade', icon: 'upload', label: '更新构建', hover: 'group-hover:bg-blue-50 group-hover:text-fnos-blue', disabled: loading.value || isBuilding.value }
])

const openApp = async () => {
  const cfg = await apiGet<{ reverse_proxy_url?: string }>('config')
  if (cfg?.reverse_proxy_url) {
    window.open(cfg.reverse_proxy_url, '_blank')
    return
  }
  const appUrl = statusData.value.app_url
  if (appUrl?.startsWith(':')) {
    window.open(`https://${window.location.hostname}${appUrl}`, '_blank')
  }
}

const doAction = async (action: string) => {
  loading.value = true
  if (action === 'upgrade') {
    showToast('已触发更新构建，等待后台响应...', 'loading')
  }
  const data = await apiPost<StatusData>('action', { action })
  if (data) {
    // upgrade 是异步任务，HTTP 响应仅代表"已触发"，状态以 SSE 推送为准
    if (action !== 'upgrade') {
      statusData.value = data
    }
  } else {
    showToast('操作失败，请查看运行日志', 'warning')
  }
  loading.value = false
}
</script>