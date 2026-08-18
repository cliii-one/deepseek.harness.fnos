<template>
  <!-- WebUI 沉浸式视图：DSH 聊天界面铺满整个内容区，无卡片/标题装饰 -->
  <div class="w-full h-full relative flex-1 min-h-0 overflow-hidden">

    <!-- 加载中遮罩（iframe onload 后淡出） -->
    <transition name="fade">
      <div v-if="!loaded" class="absolute inset-0 z-20 flex items-center justify-center bg-slate-50/80 dark:bg-[#12141a]/80 backdrop-blur-sm">
        <div class="flex flex-col items-center gap-3 text-slate-400 dark:text-slate-500">
          <n-spin :size="22" />
          <span class="text-sm">正在加载 DeepSeek Harness…</span>
        </div>
      </div>
    </transition>

    <!-- 全屏 iframe：DSH 界面本体 -->
    <iframe
      ref="frameRef"
      :src="webuiUrl"
      class="absolute inset-0 w-full h-full border-0"
      style="background: #fff;"
      allow="clipboard-write; fullscreen"
      @load="loaded = true"
    />

    <!-- 悬浮工具按钮组：右下角小圆钮，平时半透明，悬停不打扰 -->
    <div class="absolute bottom-4 right-4 z-30 flex items-center gap-2 opacity-60 hover:opacity-100 transition-opacity">
      <n-tooltip placement="top">
        <template #trigger>
          <n-button circle size="small" secondary type="primary" title="在新标签页中打开" @click="openExternal">
            <n-icon :size="16"><ExternalLink /></n-icon>
          </n-button>
        </template>
        新标签页打开
      </n-tooltip>
      <n-tooltip placement="top">
        <template #trigger>
          <n-button circle size="small" secondary title="重新加载" @click="reloadFrame">
            <n-icon :size="16"><Refresh /></n-icon>
          </n-button>
        </template>
        重新加载
      </n-tooltip>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { NButton, NIcon, NSpin, NTooltip } from 'naive-ui'
import { Refresh, ExternalLink } from '@vicons/tabler'

// DSH WebUI 通过飞牛网关前缀反代（同源），与 fnpack/app/ui/config 中入口 URL 保持一致
const webuiUrl = '/app/deepseek-harness/fngateway/'

const loaded = ref(false)
const frameRef = ref<HTMLIFrameElement | null>(null)

// 重新加载内嵌页面：重置加载态并刷新 iframe
function reloadFrame() {
  loaded.value = false
  if (frameRef.value) {
    frameRef.value.src = webuiUrl
  }
}

// 新标签页打开独立 WebUI（iframe 内受限时的逃生通道）
function openExternal() {
  window.open(webuiUrl, '_blank')
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