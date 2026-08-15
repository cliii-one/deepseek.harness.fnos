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
      <pre v-if="lines.length" class="whitespace-pre-wrap break-all">{{ displayedText }}</pre>
      <div v-else class="text-slate-500 text-center py-20 font-sans">暂无运行日志数据</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { storeToRefs } from 'pinia'
import { useAppStore } from '../stores/app'
import { useToastStore } from '../stores/toast'
import { apiGet, apiDelete } from '../api'
import Icon, { type IconName } from '../components/Icon.vue'

const appStore = useAppStore()
const toastStore = useToastStore()
const { logLines: lines, logAutoScroll: autoScroll } = storeToRefs(appStore)

const FLUSH_INTERVAL = 80

const logContainer = ref<HTMLElement | null>(null)
let unsubscribe: (() => void) | null = null
let offReconnect: (() => void) | null = null

let pendingBuffer = ''
let flushTimer: ReturnType<typeof setTimeout> | null = null
let skipScroll = false
/** 全量拉取中：WS 增量先入缓冲，返回后与全量合并 */
let fetching = false

const displayedText = computed(() => lines.value.join(''))

const scrollToBottom = () => {
  if (skipScroll) {
    skipScroll = false
    return
  }
  nextTick(() => {
    if (logContainer.value && autoScroll.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  })
}

const flush = () => {
  flushTimer = null
  if (!pendingBuffer) return
  const chunk = pendingBuffer
  pendingBuffer = ''

  const newLines = chunk.split(/(?<=\n)/)
  for (const l of newLines) {
    if (l) appStore.appendLog(l)
  }
  scrollToBottom()
}

const scheduleFlush = () => {
  if (flushTimer === null) {
    flushTimer = setTimeout(flush, FLUSH_INTERVAL)
  }
}

const appendChunk = (chunk: string) => {
  pendingBuffer += chunk
  if (!fetching) scheduleFlush()
}

const fetchLogs = async () => {
  fetching = true
  if (flushTimer !== null) {
    clearTimeout(flushTimer)
    flushTimer = null
  }
  pendingBuffer = ''
  const res = await apiGet<string>('logs')
  const allLines = (res?.ok && typeof res.data === 'string')
    ? res.data.split(/(?<=\n)/).filter(l => l.length > 0)
    : []
  // 拼接拉取窗口内的 WS 增量
  const extraLines = pendingBuffer.split(/(?<=\n)/).filter(l => l.length > 0)
  pendingBuffer = ''
  appStore.setLogs([...allLines, ...extraLines])
  scrollToBottom()
  fetching = false
}

const clearLogs = async () => {
  if (!confirm('确定要清空所有运行日志吗？')) return
  const res = await apiDelete<boolean>('logs')
  if (!res) {
    toastStore.showToast('网络连接失败')
  } else if (res.ok) {
    pendingBuffer = ''
    if (flushTimer !== null) {
      clearTimeout(flushTimer)
      flushTimer = null
    }
    appStore.clearLogs()
  } else {
    toastStore.showToast(res.message)
  }
}

const downloadLogs = () => {
  window.open('api/logs/download', '_blank')
}

const tools: { label: string; icon: IconName; onClick: () => void; danger?: boolean }[] = [
  { label: '下载', icon: 'download', onClick: downloadLogs },
  { label: '清空', icon: 'trash', onClick: clearLogs, danger: true }
]

onMounted(() => {
  fetchLogs()
  unsubscribe = appStore.onLogChunk(appendChunk)
  offReconnect = appStore.onReconnect(fetchLogs)
})

onUnmounted(() => {
  unsubscribe?.()
  offReconnect?.()
  if (flushTimer !== null) {
    clearTimeout(flushTimer)
    flushTimer = null
  }
})
</script>