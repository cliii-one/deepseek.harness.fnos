<template>
  <div class="w-full max-w-7xl mx-auto flex flex-col gap-4 h-[calc(100vh-140px)] sm:h-[calc(100vh-100px)]">
    <!-- 头部工具栏 -->
    <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 shrink-0">
      <h1 class="text-xl font-bold text-slate-800 tracking-tight">运行日志</h1>

      <div class="flex flex-wrap items-center gap-2">
        <label class="flex items-center gap-2 text-xs font-medium text-slate-600 cursor-pointer bg-white px-3 py-1.5 rounded-lg border border-slate-200">
          <input type="checkbox" v-model="autoScroll" class="rounded text-fnos-blue focus:ring-0">
          <span>自动滚动</span>
        </label>

        <button v-for="t in tools" :key="t.label" @click="t.onClick" :class="[
          'px-3 py-1.5 bg-white border rounded-lg text-xs font-medium flex items-center gap-1.5 transition-colors',
          t.danger
            ? 'border-slate-200 text-slate-700 hover:bg-rose-50 hover:border-rose-200 hover:text-rose-600'
            : 'border-slate-200 text-slate-700 hover:bg-slate-50'
        ]">
          <Icon :name="t.icon" :size="14" class="text-slate-500" />
          <span>{{ t.label }}</span>
        </button>
      </div>
    </div>

    <!-- 日志终端 -->
    <div ref="logContainer"
      class="flex-1 bg-[#1e222b] rounded-2xl p-4 shadow-inner font-mono text-xs text-slate-200 overflow-y-auto leading-relaxed border border-slate-800">
      <pre v-if="logText" class="whitespace-pre-wrap break-all">{{ logText }}</pre>
      <div v-else class="text-slate-500 text-center py-20 font-sans">暂无运行日志数据</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { onLogChunk, onReconnect } from '../store'
import { apiGet, apiDelete } from '../api'
import { showToast } from '../toast'
import Icon, { type IconName } from '../components/Icon.vue'

const MAX_LOG_LENGTH = 300 * 1024

const logText = ref('')
const autoScroll = ref(true)
const logContainer = ref<HTMLElement | null>(null)
let unsubscribe: (() => void) | null = null

const trimLog = () => {
  if (logText.value.length > MAX_LOG_LENGTH) {
    const cut = logText.value.indexOf('\n', logText.value.length - MAX_LOG_LENGTH)
    logText.value = logText.value.slice(cut > 0 ? cut + 1 : -MAX_LOG_LENGTH)
  }
}

const scrollToBottom = () => {
  nextTick(() => {
    if (logContainer.value && autoScroll.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  })
}

const fetchLogs = async () => {
  const data = await apiGet<string>('logs')
  if (data !== null) {
    logText.value = data
    trimLog()
    scrollToBottom()
  }
}

const clearLogs = async () => {
  if (!confirm('确定要清空所有运行日志吗？')) return
  if (await apiDelete('logs') !== null) {
    logText.value = ''
  } else {
    showToast('清空日志失败', 'warning')
  }
}

const downloadLogs = () => {
  window.open('api/logs/download', '_blank')
}

const tools: { label: string; icon: IconName; onClick: () => void; danger?: boolean }[] = [
  { label: '下载', icon: 'download', onClick: downloadLogs },
  { label: '清空', icon: 'trash', onClick: clearLogs, danger: true }
]

let offReconnect: (() => void) | null = null

onMounted(() => {
  fetchLogs()
  unsubscribe = onLogChunk((chunk) => {
    logText.value += chunk
    trimLog()
    scrollToBottom()
  })
  // SSE 断线重连后重新拉取全量日志，补齐断线期间遗漏的内容
  offReconnect = onReconnect(fetchLogs)
})

onUnmounted(() => {
  unsubscribe?.()
  offReconnect?.()
})
</script>