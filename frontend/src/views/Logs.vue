<template>
  <div class="w-full h-[calc(100dvh-82px)] sm:h-[calc(100dvh-48px)] flex flex-col min-h-0 overflow-hidden">
    <!-- 原生卡片包裹的日志视图：固定高度并撑满可用纵向空间 -->
    <n-card :bordered="false" class="flex-1 flex flex-col shadow-sm rounded-2xl min-h-0 overflow-hidden"
      content-style="display: flex; flex-direction: column; flex: 1; min-height: 0; overflow: hidden; padding-top: 0;">
      <!-- 标题与操作栏 -->
      <template #header>
        <span class="text-base sm:text-lg font-bold text-slate-800 tracking-tight">运行日志</span>
      </template>

      <template #header-extra>
        <n-flex :size="8" align="center" :wrap="false">
          <!-- 自动滚动开关 -->
          <n-button size="small" secondary v-debounce @click="autoScroll = !autoScroll">
            <div class="flex items-center gap-1.5">
              <span>自动滚动</span>
              <n-switch :value="autoScroll" size="small" @click.stop="autoScroll = !autoScroll" />
            </div>
          </n-button>

          <!-- 下载日志按钮 -->
          <n-button size="small" secondary v-debounce @click="logStore.downloadLogs">
            <template #icon>
              <n-icon>
                <Download />
              </n-icon>
            </template>
            <span class="hidden sm:inline">下载</span>
          </n-button>

          <!-- 清空日志按钮 -->
          <n-popconfirm @positive-click="handleClear" positive-text="确认清空" negative-text="取消">
            <template #trigger>
              <n-button size="small" secondary v-debounce type="error">
                <template #icon>
                  <n-icon>
                    <Trash />
                  </n-icon>
                </template>
                <span class="hidden sm:inline">清空</span>
              </n-button>
            </template>
            确定要清空所有运行日志吗？
          </n-popconfirm>
        </n-flex>
      </template>

      <!-- 日志容器与悬浮回到底部按钮 -->
      <div ref="logContainerRef" class="relative flex-1 min-h-0 flex flex-col overflow-hidden">
        <!-- Naive UI 原生日志组件：自适应撑满卡片高度并在内部滚动，支持 highlight.js 语法高亮与鼠标划选复制 -->
        <n-log ref="logInstRef" :log="displayedText" :hljs="hljs" language="harness-log"
          :font-size="12" :line-height="1.5" trim
          class="flex-1 min-h-0 bg-slate-50 rounded-xl p-3 sm:p-4 border border-slate-100/80 select-text cursor-text overflow-hidden"
          style="height: 100%;" />

        <!-- 正中央优雅居中加载遮罩 -->
        <transition name="fade">
          <div
            v-if="fetching"
            class="absolute inset-0 z-20 flex flex-col items-center justify-center bg-slate-50/85 backdrop-blur-[1px] rounded-xl pointer-events-none"
          >
            <n-spin size="medium">
              <template #description>
                <span class="text-xs text-slate-500 font-medium mt-2">正在获取运行日志…</span>
              </template>
            </n-spin>
          </div>
        </transition>

        <!-- 悬浮回到底部按钮 -->
        <transition name="fade-scale">
          <div v-show="showScrollToBottom" class="absolute right-3.5 bottom-3.5 sm:right-5 sm:bottom-5 z-10">
            <n-tooltip trigger="hover" placement="left">
              <template #trigger>
                <n-button circle secondary v-debounce type="primary" size="medium" @click="manualScrollToBottom"
                  class="!shadow-md !bg-white/90 !backdrop-blur-sm hover:!bg-white !border !border-slate-200/80 transition-all hover:scale-105">
                  <template #icon>
                    <n-icon :size="18">
                      <ArrowDown />
                    </n-icon>
                  </template>
                </n-button>
              </template>
              回到底部
            </n-tooltip>
          </div>
        </transition>
      </div>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, onActivated, nextTick } from 'vue'
import { storeToRefs } from 'pinia'
import {
  NCard,
  NSwitch,
  NButton,
  NPopconfirm,
  NFlex,
  NIcon,
  NTooltip,
  NLog,
  NSpin,
  useMessage,
  type LogInst
} from 'naive-ui'
import { Download, Trash, ArrowDown } from '@vicons/tabler'
import hljs from 'highlight.js/lib/core'
import { useLogStore } from '../stores/log'
import { withAsyncLock } from '../utils/debounce'

// 注册专用的 harness-log 日志高亮规则
hljs.registerLanguage('harness-log', () => ({
  contains: [
    // 错误与致命异常
    {
      className: 'type',
      begin: /\[FATAL\]|\[ERROR\]/
    },
    // 警告级别
    {
      className: 'keyword',
      begin: /\[WARN\]|\[WARNING\]/
    },
    // 信息级别
    {
      className: 'meta',
      begin: /\[INFO\]/
    },
    // 时间戳 (如 2026/08/16 13:08:24 或 2026-08-16 13:08:24)
    {
      className: 'comment',
      begin: /\d{4}[-/]\d{2}[-/]\d{2}(?:[ T]\d{2}:\d{2}:\d{2}(?:\.\d+)?)?/
    },
    // URL 地址
    {
      className: 'link',
      begin: /https?:\/\/[^\s]+/
    },
    // 路径
    {
      className: 'string',
      begin: /(?:\/[\w.-]+)+\/?/
    },
    // 数字与 PID / 端口等
    {
      className: 'number',
      begin: /\b\d+\b/
    }
  ]
}))

const logStore = useLogStore()
const message = useMessage()
const { displayedText, logAutoScroll: autoScroll, fetching } = storeToRefs(logStore)

const logInstRef = ref<LogInst | null>(null)
const logContainerRef = ref<HTMLElement | null>(null)
const showScrollToBottom = ref(false)
let scrollEl: HTMLElement | null = null
let offFlush: (() => void) | null = null

const handleScroll = () => {
  if (!scrollEl) return
  const distanceFromBottom = scrollEl.scrollHeight - scrollEl.scrollTop - scrollEl.clientHeight
  showScrollToBottom.value = distanceFromBottom > 40
}

const scrollToBottom = () => {
  if (!autoScroll.value) return
  nextTick(() => {
    logInstRef.value?.scrollTo({ position: 'bottom', silent: true })
    showScrollToBottom.value = false
  })
}

const manualScrollToBottom = () => {
  nextTick(() => {
    logInstRef.value?.scrollTo({ position: 'bottom', silent: false })
    showScrollToBottom.value = false
  })
}

const handleClear = withAsyncLock(async () => {
  const res = await logStore.clearLogs()
  if (res.success) {
    message.success('日志已清空')
  } else {
    message.error(res.message || '清空日志失败')
  }
})

onMounted(() => {
  if (!logStore.logLines.length) {
    logStore.fetchLogs().then(() => {
      scrollToBottom()
    })
  } else {
    scrollToBottom()
  }

  offFlush = logStore.onFlush(() => {
    scrollToBottom()
  })

  // 绑定日志滚动容器的实时位置监听
  nextTick(() => {
    scrollEl = logContainerRef.value?.querySelector('.n-scrollbar-container') as HTMLElement | null
    scrollEl?.addEventListener('scroll', handleScroll, { passive: true })
  })
})

// 切回日志页面时自动平滑同步滚动位置
onActivated(() => {
  if (autoScroll.value) {
    scrollToBottom()
  }
})

onUnmounted(() => {
  offFlush?.()
  scrollEl?.removeEventListener('scroll', handleScroll)
})
</script>

<style scoped>
:deep(.hljs-type) {
  color: #ef4444;
  font-weight: 600;
}

:deep(.hljs-keyword) {
  color: #f59e0b;
  font-weight: 600;
}

:deep(.hljs-meta) {
  color: #2563eb;
  font-weight: 600;
}

:deep(.hljs-comment) {
  color: #94a3b8;
}

:deep(.hljs-number) {
  color: #0891b2;
}

:deep(.hljs-string) {
  color: #059669;
}

:deep(.hljs-link) {
  color: #4f46e5;
  text-decoration: underline;
}

/* 居中加载遮罩淡入淡出动效 */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* 悬浮按钮过渡动效 */
.fade-scale-enter-active,
.fade-scale-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.fade-scale-enter-from,
.fade-scale-leave-to {
  opacity: 0;
  transform: scale(0.85) translateY(4px);
}
</style>