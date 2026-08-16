import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { logApi } from '../api'
import type { RequestResult } from '../types/api'

export const useLogStore = defineStore('log', () => {
  const logLines = ref<string[]>([])
  const logAutoScroll = ref(true)
  const fetching = ref(false)

  const MAX_LOG_BYTES = 100 * 1024
  const MAX_LOG_LINES = 5000
  const FLUSH_INTERVAL = 80

  let pendingBuffer = ''
  let flushTimer: ReturnType<typeof setTimeout> | null = null
  const flushListeners = new Set<() => void>()

  function trimLogs() {
    const arr = logLines.value
    let total = 0
    for (let i = arr.length - 1; i >= 0; i--) {
      total += arr[i].length
      if (total > MAX_LOG_BYTES || arr.length - i > MAX_LOG_LINES) {
        arr.splice(0, i + 1)
        return
      }
    }
  }

  function flushPending() {
    flushTimer = null
    if (!pendingBuffer) return
    logLines.value.push(pendingBuffer)
    pendingBuffer = ''
    trimLogs()
    flushListeners.forEach((fn) => fn())
  }

  function appendChunk(chunk: string) {
    pendingBuffer += chunk
    if (flushTimer === null) {
      flushTimer = setTimeout(flushPending, FLUSH_INTERVAL)
    }
  }

  function onFlush(fn: () => void): () => void {
    flushListeners.add(fn)
    return () => {
      flushListeners.delete(fn)
    }
  }

  function setLogs(lines: string[]) {
    logLines.value = [...lines]
    trimLogs()
    flushListeners.forEach((fn) => fn())
  }

  const displayedText = computed(() => logLines.value.join(''))

  async function fetchLogs(): Promise<void> {
    fetching.value = true
    try {
      const res = await logApi.getLogs()
      if (res.success && res.data) {
        if (Array.isArray(res.data.lines) && res.data.lines.length > 0) {
          setLogs(res.data.lines)
        } else if (typeof res.data.content === 'string' && res.data.content) {
          setLogs([res.data.content])
        } else {
          setLogs([])
        }
      }
    } finally {
      fetching.value = false
    }
  }

  async function clearLogs(): Promise<RequestResult<boolean>> {
    const res = await logApi.clearLogs()
    if (res.success) {
      logLines.value = []
      pendingBuffer = ''
      if (flushTimer !== null) {
        clearTimeout(flushTimer)
        flushTimer = null
      }
    }
    return res
  }

  function downloadLogs(): void {
    window.open(logApi.getDownloadUrl(), '_blank')
  }

  return {
    logLines,
    logAutoScroll,
    fetching,
    displayedText,
    appendChunk,
    setLogs,
    fetchLogs,
    clearLogs,
    downloadLogs,
    onFlush
  }
})
