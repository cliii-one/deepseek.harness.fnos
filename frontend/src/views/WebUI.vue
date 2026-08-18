<template>
  <!-- WebUI 沉浸式视图：DSH 聊天界面铺满整个内容区，工具按钮悬浮于左上角 -->
  <div class="w-full h-full relative flex-1 min-h-0 overflow-hidden bg-white">

    <!-- 左上角悬浮工具按钮：仅在 WebUI 视图显示（管理界面不出现） -->
    <div
      class="absolute left-4 top-4 z-30 flex items-center gap-1.5 p-1 rounded-xl bg-white/85 dark:bg-[#181b22]/85 backdrop-blur-md shadow-sm border border-slate-200/70 dark:border-white/10 opacity-60 hover:opacity-100 transition-opacity"
    >
      <n-tooltip placement="bottom">
        <template #trigger>
          <n-button quaternary circle size="small" title="返回管理面板" @click="goManage">
            <n-icon :size="17"><Dashboard /></n-icon>
          </n-button>
        </template>
        返回管理面板
      </n-tooltip>
      <n-tooltip placement="bottom">
        <template #trigger>
          <n-button quaternary circle size="small" title="在新标签页中打开" @click="openExternal">
            <n-icon :size="17"><ExternalLink /></n-icon>
          </n-button>
        </template>
        新标签页打开
      </n-tooltip>
      <n-tooltip placement="bottom">
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

    <!-- 全屏 iframe：DSH 界面本体，固定走飞牛统一网关（同源内嵌，无需额外鉴权） -->
    <iframe
      ref="frameRef"
      :src="WEBUI_URL"
      class="absolute inset-0 w-full h-full border-0"
      style="background: #fff;"
      allow="clipboard-write; fullscreen"
      @load="loaded = true"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { NButton, NIcon, NSpin, NTooltip } from 'naive-ui'
import { Refresh, ExternalLink, Dashboard } from '@vicons/tabler'
import { useAppStore } from '../stores/app'

// DSH 通过飞牛统一网关接入（gatewayPrefix /app/deepseek-harness），聊天界面全屏展示；
// 左侧栏在 WebUI 视图整体折叠，三个工具按钮悬浮于左上角提供返回/新标签/刷新能力
const WEBUI_URL = '/app/deepseek-harness/fngateway/'

const loaded = ref(false)
const frameRef = ref<HTMLIFrameElement | null>(null)
const appStore = useAppStore()

// 返回管理面板（概览）
function goManage() {
  appStore.setTab('overview')
}

// 新标签页打开独立 WebUI
function openExternal() {
  window.open(WEBUI_URL, '_blank')
}

// 重新加载内嵌页面（仅重置 iframe，不重载整个管理应用）
function reloadFrame() {
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