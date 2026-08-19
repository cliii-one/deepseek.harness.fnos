<template>
  <!-- WebUI 沉浸式视图：DSH 聊天界面铺满整个内容区，工具按钮悬浮于右下角（不遮挡左上角 logo） -->
  <div class="w-full h-full relative flex-1 min-h-0 overflow-hidden bg-white">

    <!-- 悬浮工具按钮：默认右下角，可按住空白处自由拖动改变位置（位置记忆在 localStorage） -->
    <div
      class="fixed z-30 flex items-center gap-1.5 p-1 rounded-xl bg-white/85 dark:bg-[#181b22]/85 backdrop-blur-md shadow-md border border-slate-200/70 dark:border-white/10 opacity-60 hover:opacity-100 transition-opacity cursor-grab select-none"
      :class="{ '!cursor-grabbing !opacity-100': dragging }"
      :style="{ left: pos.x + 'px', top: pos.y + 'px' }"
      title="按住空白处可拖动位置"
      @mousedown.prevent="onMouseDown"
    >
      <n-tooltip placement="top">
        <template #trigger>
          <n-button quaternary circle size="small" title="返回管理面板" @click="goManage">
            <n-icon :size="17"><Dashboard /></n-icon>
          </n-button>
        </template>
        返回管理面板
      </n-tooltip>
      <n-tooltip placement="top">
        <template #trigger>
          <n-button quaternary circle size="small" title="在新标签页中打开" @click="openExternal">
            <n-icon :size="17"><ExternalLink /></n-icon>
          </n-button>
        </template>
        新标签页打开
      </n-tooltip>
      <n-tooltip placement="top">
        <template #trigger>
          <n-button quaternary circle size="small" title="重新加载" @click="reloadFrame">
            <n-icon :size="17"><Refresh /></n-icon>
          </n-button>
        </template>
        重新加载
      </n-tooltip>
    </div>

    <!-- 加载中遮罩（iframe onload 后淡出） -->
    <transition name="fade">
      <div v-if="!loaded" class="absolute inset-0 z-20 flex items-center justify-center bg-slate-50/80 dark:bg-[#12141a]/80 backdrop-blur-sm">
        <div class="flex flex-col items-center gap-3 text-slate-400 dark:text-slate-500">
          <n-spin :size="22" />
          <span class="text-sm">正在加载 DeepSeek Harness…</span>
        </div>
      </div>
    </transition>

    <!-- 全屏 iframe：DSH 界面本体，固定走飞牛统一网关（同源内嵌，无需额外鉴权）。
         拖动工具条期间临时禁用指针事件，避免 iframe 吞掉 mouseup 导致拖拽卡死 -->
    <iframe
      ref="frameRef"
      :src="WEBUI_URL"
      class="absolute inset-0 w-full h-full border-0"
      :class="{ 'pointer-events-none': dragging }"
      style="background: #fff;"
      allow="clipboard-write; fullscreen"
      @load="loaded = true"
    />
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { NButton, NIcon, NSpin, NTooltip } from 'naive-ui'
import { Refresh, ExternalLink, Dashboard } from '@vicons/tabler'
import { useAppStore } from '../stores/app'

// DSH 通过飞牛统一网关接入（gatewayPrefix /app/deepseek-harness），聊天界面全屏展示；
// 左侧栏在 WebUI 视图整体折叠，工具按钮悬浮（默认右下角，可自由拖拽改变位置）
const WEBUI_URL = '/app/deepseek-harness/fngateway/'

const loaded = ref(false)
const frameRef = ref<HTMLIFrameElement | null>(null)
const appStore = useAppStore()

// —— 工具条自由拖拽 ——
const TOOLBAR_W = 110 // 按钮组估算宽度（用于位置边界约束）
const TOOLBAR_H = 40 // 按钮组估算高度
const POS_KEY = 'dsh_webui_toolbar_pos' // 位置持久化键（localStorage）

interface Vec2 {
  x: number
  y: number
}

const pos = ref<Vec2>({ x: 0, y: 0 })
const dragging = ref(false)
// 拖动结束时抑制一次按钮点击（移动超过阈值视为拖动，不触发按钮动作）
const suppressClick = ref(false)

// 位置限制在视口内，防止拖出屏幕
function clampPos(p: Vec2): Vec2 {
  const maxX = Math.max(0, window.innerWidth - TOOLBAR_W)
  const maxY = Math.max(0, window.innerHeight - TOOLBAR_H)
  return { x: Math.min(Math.max(0, p.x), maxX), y: Math.min(Math.max(0, p.y), maxY) }
}

// 恢复上次位置；无记录时默认右下角（留 16px 边距）
function loadPos(): Vec2 {
  try {
    const raw = localStorage.getItem(POS_KEY)
    if (raw) {
      const p = JSON.parse(raw) as Vec2
      if (typeof p?.x === 'number' && typeof p?.y === 'number') {
        return clampPos(p)
      }
    }
  } catch {
    /* 存储数据损坏时忽略，回退默认位置 */
  }
  return { x: Math.max(0, window.innerWidth - TOOLBAR_W - 16), y: Math.max(0, window.innerHeight - TOOLBAR_H - 16) }
}

onMounted(() => {
  pos.value = loadPos()
})

// 拖拽基准：鼠标按下点 + 工具条起始位置 + 是否已移动（>5px 视为拖动）
let dragBase = { mx: 0, my: 0, ox: 0, oy: 0, moved: false }

function onMouseDown(e: MouseEvent) {
  if (e.button !== 0) return
  // 从按钮上按下不启动拖拽，保留按钮点击能力；空白处以拖拽为准
  const target = e.target as HTMLElement
  if (target.closest('button')) return
  dragBase = { mx: e.clientX, my: e.clientY, ox: pos.value.x, oy: pos.value.y, moved: false }
  dragging.value = true
  window.addEventListener('mousemove', onMouseMove)
  window.addEventListener('mouseup', onMouseUp)
}

function onMouseMove(e: MouseEvent) {
  if (!dragging.value) return
  const dx = e.clientX - dragBase.mx
  const dy = e.clientY - dragBase.my
  if (!dragBase.moved && Math.hypot(dx, dy) > 5) {
    dragBase.moved = true
  }
  if (dragBase.moved) {
    pos.value = clampPos({ x: dragBase.ox + dx, y: dragBase.oy + dy })
  }
}

function onMouseUp() {
  if (!dragging.value) return
  dragging.value = false
  window.removeEventListener('mousemove', onMouseMove)
  window.removeEventListener('mouseup', onMouseUp)
  if (dragBase.moved) {
    // 真实拖动：保存位置，并抑制随后触发的按钮点击
    suppressClick.value = true
    setTimeout(() => {
      suppressClick.value = false
    }, 80)
    localStorage.setItem(POS_KEY, JSON.stringify(pos.value))
  }
}

onBeforeUnmount(() => {
  window.removeEventListener('mousemove', onMouseMove)
  window.removeEventListener('mouseup', onMouseUp)
})

// 返回管理面板（概览）
function goManage() {
  if (suppressClick.value) return
  appStore.setTab('overview')
}

// 新标签页打开独立 WebUI
function openExternal() {
  if (suppressClick.value) return
  window.open(WEBUI_URL, '_blank')
}

// 重新加载内嵌页面（仅重置 iframe，不重载整个管理应用）
function reloadFrame() {
  if (suppressClick.value) return
  loaded.value = false
  if (frameRef.value) {
    frameRef.value.src = WEBUI_URL
  }
}
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.25s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>