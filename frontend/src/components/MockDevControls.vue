<template>
  <Teleport to="body">
    <div
      v-if="isDev"
      ref="floatRef"
      class="fixed z-[99999] select-none font-sans touch-none"
      :style="{ left: `${position.x}px`, top: `${position.y}px` }"
    >
      <!-- 1. 悬浮胶囊球（支持任意拖拽，点击任意处均可展开/收起） -->
      <div
        @mousedown="startDrag"
        @touchstart="startDragTouch"
        @click="handleCapsuleClick"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-slate-900/90 hover:bg-slate-900 dark:bg-slate-800/90 dark:hover:bg-slate-800 text-white backdrop-blur-md shadow-xl border border-white/15 cursor-pointer select-none transition-all duration-150 hover:scale-105 active:scale-95 group"
      >
        <div class="w-2 h-2 rounded-full bg-emerald-400 animate-pulse shrink-0"></div>
        <span class="text-[11px] font-mono font-medium tracking-wide">MOCK</span>
        <div class="ml-0.5 text-slate-300 group-hover:text-white flex items-center">
          <n-icon :size="13">
            <component :is="expanded ? ChevronDown : ChevronUp" />
          </n-icon>
        </div>
      </div>

      <!-- 2. 悬浮展开面板（基于当前悬浮球自适应弹出，支持关闭） -->
      <div
        v-if="expanded"
        class="absolute mt-2 w-72 rounded-2xl bg-white/95 dark:bg-[#1c2028]/95 backdrop-blur-xl border border-slate-200/80 dark:border-white/10 shadow-2xl overflow-hidden transition-all duration-200"
        :class="dropdownAlignClass"
      >
        <!-- 头部 -->
        <div class="flex items-center justify-between px-3.5 py-2 bg-slate-50/80 dark:bg-white/[0.04] border-b border-slate-100 dark:border-white/[0.06]">
          <div class="flex items-center gap-1.5">
            <span class="text-xs font-bold text-slate-800 dark:text-slate-100">仿真事件触发器</span>
            <n-tag size="tiny" type="info" round :bordered="false" class="text-[10px]">Dev</n-tag>
          </div>
          <button
            type="button"
            @click="expanded = false"
            class="p-1 rounded-md text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors"
          >
            <n-icon :size="14"><X /></n-icon>
          </button>
        </div>

        <!-- 动作操作区 -->
        <div class="p-2.5 space-y-2.5 max-h-[70vh] overflow-y-auto">
          <!-- 1. 版本升级与构建流程仿真 -->
          <div class="space-y-1.5">
            <div class="text-[10px] font-semibold text-slate-400 dark:text-slate-500 uppercase tracking-wider">
              版本更新与构建状态
            </div>
            <div class="grid grid-cols-2 gap-1.5">
              <n-button size="tiny" type="info" secondary @click="simulateUpgradeBuilding" class="rounded-lg justify-start !px-2 text-[11px]">
                <template #icon><n-icon :size="12"><Download /></n-icon></template>
                更新构建中(带徽章)
              </n-button>
              <n-button size="tiny" type="success" secondary @click="simulateBuildSuccess" class="rounded-lg justify-start !px-2 text-[11px]">
                <template #icon><n-icon :size="12"><Check /></n-icon></template>
                构建成功(新版本)
              </n-button>
            </div>
            <div class="grid grid-cols-2 gap-1.5">
              <n-button size="tiny" type="warning" secondary @click="simulateStarting" class="rounded-lg justify-start !px-2 text-[11px]">
                <template #icon><n-icon :size="12"><PlayerPlay /></n-icon></template>
                服务启动中
              </n-button>
              <n-button size="tiny" secondary @click="simulateStopped" class="rounded-lg justify-start !px-2 text-[11px]">
                <template #icon><n-icon :size="12"><PlayerStop /></n-icon></template>
                服务已停止
              </n-button>
            </div>
          </div>

          <!-- 2. 插件管理与安装取消仿真 -->
          <div class="space-y-1.5 pt-1.5 border-t border-slate-100 dark:border-white/[0.06]">
            <div class="text-[10px] font-semibold text-slate-400 dark:text-slate-500 uppercase tracking-wider">
              插件管理与安装控制
            </div>
            <div class="grid grid-cols-2 gap-1.5">
              <n-button size="tiny" :type="pluginStore.pluginBusy ? 'error' : 'primary'" secondary @click="togglePluginBusy" class="rounded-lg justify-start !px-2 text-[11px]">
                <template #icon><n-icon :size="12"><Tools /></n-icon></template>
                {{ pluginStore.pluginBusy ? '结束安装状态' : '模拟正在安装(可取消)' }}
              </n-button>
              <n-button size="tiny" secondary @click="toggleRestart" class="rounded-lg justify-start !px-2 text-[11px]">
                <template #icon><n-icon :size="12"><AlertTriangle /></n-icon></template>
                {{ pluginStore.needRestart ? '清除重启提醒' : '触发重启提醒' }}
              </n-button>
            </div>
            <div class="grid grid-cols-2 gap-1.5">
              <n-button size="tiny" type="error" secondary @click="injectBroken(false)" class="rounded-lg justify-start !px-2 text-[11px]">
                <template #icon><n-icon :size="12"><Bug /></n-icon></template>
                注入单条报错
              </n-button>
              <n-button size="tiny" type="error" secondary @click="injectMultipleBroken" class="rounded-lg justify-start !px-2 text-[11px]">
                <template #icon><n-icon :size="12"><Flame /></n-icon></template>
                注入多异常(一键禁用)
              </n-button>
            </div>
            <div class="grid grid-cols-2 gap-1.5">
              <n-button size="tiny" type="success" secondary @click="healBroken" class="rounded-lg justify-start !px-2 text-[11px]">
                <template #icon><n-icon :size="12"><Check /></n-icon></template>
                清除所有报错
              </n-button>
              <n-button size="tiny" type="warning" secondary @click="injectExtremePlugin" class="rounded-lg justify-start !px-2 text-[11px]">
                <template #icon><n-icon :size="12"><Maximize /></n-icon></template>
                超长包名/路径
              </n-button>
            </div>
          </div>

          <!-- 3. 数据重置与空状态 -->
          <div class="space-y-1.5 pt-1.5 border-t border-slate-100 dark:border-white/[0.06]">
            <div class="text-[10px] font-semibold text-slate-400 dark:text-slate-500 uppercase tracking-wider">
              基础控制
            </div>
            <div class="grid grid-cols-2 gap-1.5">
              <n-button size="tiny" secondary type="warning" @click="emptyList" class="rounded-lg justify-start !px-2 text-[11px]">
                <template #icon><n-icon :size="12"><Trash /></n-icon></template>
                清空插件列表
              </n-button>
              <n-button size="tiny" secondary type="primary" @click="resetList" class="rounded-lg justify-start !px-2 text-[11px]">
                <template #icon><n-icon :size="12"><Refresh /></n-icon></template>
                重置默认数据
              </n-button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useMessage } from 'naive-ui'
import {
  X,
  Bug,
  Flame,
  Check,
  Maximize,
  AlertTriangle,
  Power,
  Trash,
  Refresh,
  ChevronDown,
  ChevronUp,
  Download,
  PlayerPlay,
  PlayerStop,
  Tools
} from '@vicons/tabler'
import { usePluginStore } from '../stores/plugin'
import { useSystemStore } from '../stores/system'
import type { ServiceStatus } from '../types/api'

const isDev = Boolean((import.meta as any).env?.DEV ?? true)
const expanded = ref(false)
const message = useMessage()
const pluginStore = usePluginStore()
const systemStore = useSystemStore()

// 悬浮球绝对坐标（支持任意拖拽）
const position = ref({ x: 20, y: 120 })
const isDragging = ref(false)
let dragStartX = 0
let dragStartY = 0
let initialPosX = 0
let initialPosY = 0

// 初始化停靠在右下角安全位置
onMounted(() => {
  if (typeof window !== 'undefined') {
    position.value = {
      x: Math.max(16, window.innerWidth - 130),
      y: Math.max(16, window.innerHeight - 80)
    }
  }
})

// 根据悬浮球在屏幕中的位置，智能决定弹出面板是朝左还是朝右、朝上还是朝下
const dropdownAlignClass = computed(() => {
  if (typeof window === 'undefined') return 'right-0'
  const isRightHalf = position.value.x > window.innerWidth / 2
  const isBottomHalf = position.value.y > window.innerHeight / 2
  
  const hAlign = isRightHalf ? 'right-0' : 'left-0'
  const vAlign = isBottomHalf ? 'bottom-full mb-2' : 'top-full mt-2'
  return `${hAlign} ${vAlign}`
})

function handleCapsuleClick(e: MouseEvent) {
  // 如果没有发生显著拖拽，直接切换展开收起
  if (!isDragging.value) {
    expanded.value = !expanded.value
  }
}

// 鼠标拖拽逻辑
function startDrag(e: MouseEvent) {
  isDragging.value = false
  dragStartX = e.clientX
  dragStartY = e.clientY
  initialPosX = position.value.x
  initialPosY = position.value.y

  window.addEventListener('mousemove', onMouseMove)
  window.addEventListener('mouseup', onMouseUp)
}

function onMouseMove(e: MouseEvent) {
  const dx = e.clientX - dragStartX
  const dy = e.clientY - dragStartY
  if (Math.abs(dx) > 4 || Math.abs(dy) > 4) {
    isDragging.value = true
  }
  
  const maxX = window.innerWidth - 90
  const maxY = window.innerHeight - 40
  position.value = {
    x: Math.min(Math.max(10, initialPosX + dx), maxX),
    y: Math.min(Math.max(10, initialPosY + dy), maxY)
  }
}

function onMouseUp(e: MouseEvent) {
  window.removeEventListener('mousemove', onMouseMove)
  window.removeEventListener('mouseup', onMouseUp)
  
  // 延迟一帧重置 isDragging 状态，防止触发同一微任务中的 click
  setTimeout(() => {
    isDragging.value = false
  }, 50)
}

// 触摸屏拖拽逻辑
function startDragTouch(e: TouchEvent) {
  const touch = e.touches[0]
  isDragging.value = false
  dragStartX = touch.clientX
  dragStartY = touch.clientY
  initialPosX = position.value.x
  initialPosY = position.value.y

  window.addEventListener('touchmove', onTouchMove, { passive: false })
  window.addEventListener('touchend', onTouchEnd)
}

function onTouchMove(e: TouchEvent) {
  const touch = e.touches[0]
  const dx = touch.clientX - dragStartX
  const dy = touch.clientY - dragStartY
  if (Math.abs(dx) > 4 || Math.abs(dy) > 4) {
    isDragging.value = true
    e.preventDefault()
  }

  const maxX = window.innerWidth - 90
  const maxY = window.innerHeight - 40
  position.value = {
    x: Math.min(Math.max(10, initialPosX + dx), maxX),
    y: Math.min(Math.max(10, initialPosY + dy), maxY)
  }
}

function onTouchEnd() {
  window.removeEventListener('touchmove', onTouchMove)
  window.removeEventListener('touchend', onTouchEnd)
  setTimeout(() => {
    isDragging.value = false
  }, 50)
}

onUnmounted(() => {
  window.removeEventListener('mousemove', onMouseMove)
  window.removeEventListener('mouseup', onMouseUp)
  window.removeEventListener('touchmove', onTouchMove)
  window.removeEventListener('touchend', onTouchEnd)
})

// === 仿真动作 ===
function simulateUpgradeBuilding() {
  systemStore.updateStatus({
    status: 'building',
    commit: '8e5f7a2',
    target_commit: '3d2c1b0',
    last_message: '检测到版本变更 (8e5f7a2 → 3d2c1b0)，正在同步依赖与编译构建...'
  })
  message.info('已模拟触发版本更新构建（标题旁可见 Commit 对比小徽章）')
}

function simulateBuildSuccess() {
  systemStore.updateStatus({
    status: 'running',
    version: '0.1.0-rc.6',
    commit: '3d2c1b0',
    target_commit: '',
    build_time: new Date().toLocaleDateString('zh-CN') + ' ' + new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }),
    last_message: ''
  })
  message.success('已模拟版本更新构建成功，服务恢复运行')
}

function simulateStarting() {
  systemStore.updateStatus({
    status: 'starting',
    last_message: '服务主进程已拉起 (PID=18492)，正在等待 Web 服务就绪...'
  })
  message.warning('已模拟服务启动中')
}

function simulateStopped() {
  systemStore.updateStatus({
    status: 'stopped',
    target_commit: '',
    last_message: ''
  })
  message.info('已模拟服务停止')
}

function togglePluginBusy() {
  pluginStore.pluginBusy = !pluginStore.pluginBusy
  if (pluginStore.pluginBusy) {
    message.warning('已进入插件安装状态（可在安装面板查看【取消】按钮）')
  } else {
    message.success('已恢复正常状态')
  }
}

function injectBroken(isLong = false) {
  const errorMsg = isLong
    ? 'FATAL_LOADER_ERROR: failed to apply loader entry 3e54f87a (dsh-vision-router): keyed slot "settings.plugin.item" requires options.key in schema definition from /vol1/@appdata/deepseek.harness/plugins/node_modules/dsh-vision-router/dist/index.cjs:142:95 with stack trace at CordisLoader.registerSlot()'
    : 'failed to apply loader entry 3e54f87a (dsh-vision-router): keyed slot "settings.plugin.item" requires options.key'

  const target = pluginStore.plugins.find(p => p.name === 'dsh-vision-router')
  if (target) {
    target.state = 'broken'
    target.errorReason = errorMsg
    target.layer = false
  } else {
    pluginStore.plugins.unshift({
      name: 'dsh-vision-router',
      version: '1.2.0',
      spec: 'dsh-vision-router@latest',
      state: 'broken',
      layer: false,
      entryIds: ['vision-router'],
      description: '多模态视觉路由插件，支持将图像问答、OCR、定位等任务路由至本地或远程视觉提供链。',
      author: 'ysr666',
      homepage: 'https://github.com/ysr666/dsh-vision-router',
      license: 'MIT',
      isProtected: false,
      hasBundle: true,
      errorReason: errorMsg
    })
  }
  message.error(`已注入插件崩溃报错 (${isLong ? '超长日志' : '标准诊断'})`)
}

function injectMultipleBroken() {
  injectBroken(false)
  const pkgName = 'dsh-plugin-latex-renderer'
  if (!pluginStore.plugins.find(p => p.name === pkgName)) {
    pluginStore.plugins.unshift({
      name: pkgName,
      version: '0.9.4',
      spec: 'dsh-plugin-latex-renderer@latest',
      state: 'broken',
      layer: false,
      entryIds: ['latex-renderer'],
      description: 'LaTeX 数学公式与图表即时渲染组件。',
      author: 'math-lab',
      license: 'MIT',
      isProtected: false,
      hasBundle: true,
      errorReason: 'Cannot find module "@deepseek-ai/dsh-math-core" required by /node_modules/dsh-plugin-latex-renderer'
    })
  }
  message.error('已注入多个异常插件（可在插件管理查看【一键禁用所有异常】）')
}

function healBroken() {
  pluginStore.plugins.forEach(p => {
    if (p.state === 'broken') {
      p.state = 'disabled'
      p.errorReason = undefined
    }
  })
  message.success('已清除所有插件报错')
}

function injectExtremePlugin() {
  const extremeName = '@dsh-extremely-long-scoped-community-package-name/advanced-multimodal-chain-router-extension'
  if (!pluginStore.plugins.find(p => p.name === extremeName)) {
    pluginStore.plugins.unshift({
      name: extremeName,
      version: '99.99.99-alpha.build.20260818.preview',
      spec: 'github:Very-Long-Organization-Account-Name/deepseek-harness-ultra-plugin-repository#path:/packages/sub-extension-client-ui-theme',
      state: 'live',
      layer: true,
      entryIds: ['extreme-router'],
      description: '这是一个为了严苛测试前端 CSS Flexbox 弹性截断与极限换行抗压能力而专门生成的极限超长插件描述文本。',
      author: 'Super-Long-Author-Name-From-Open-Source-Community-Contributor-Team',
      homepage: 'https://github.com/Very-Long-Organization-Account-Name/deepseek-harness-ultra-plugin-repository',
      license: 'Apache-2.0',
      isProtected: false,
      hasBundle: true
    })
    message.info('已注入极限文本插件')
  }
}

function toggleRestart() {
  if (pluginStore.needRestart) {
    pluginStore.clearRestartNeeded()
    message.info('已清除重启提醒')
  } else {
    pluginStore.markRestartNeeded()
    message.warning('已触发配置变更重启提醒')
  }
}

function emptyList() {
  pluginStore.plugins = []
  message.warning('已清空列表（空状态）')
}

function resetList() {
  pluginStore.fetchPlugins()
  message.success('已重置为默认仿真数据')
}
</script>
