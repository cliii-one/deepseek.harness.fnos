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

    <!-- 全屏 iframe：DSH 界面本体（按访问模式解析地址） -->
    <iframe
      ref="frameRef"
      :src="webuiUrl"
      class="absolute inset-0 w-full h-full border-0"
      style="background: #fff;"
      allow="clipboard-write; fullscreen"
      @load="loaded = true"
    />

    <!-- 悬浮工具按钮组：右下角带文字药丸按钮，清晰可见但不遮挡内容 -->
    <div class="absolute bottom-5 right-5 z-30 flex items-center gap-2.5">
      <n-button size="small" round secondary @click="goManage"
        class="!shadow-lg shadow-black/10 backdrop-blur-md">
        <template #icon>
          <n-icon :size="15"><Dashboard /></n-icon>
        </template>
        管理面板
      </n-button>
      <n-button size="small" round secondary type="primary" @click="openExternal"
        class="!shadow-lg shadow-black/10 backdrop-blur-md">
        <template #icon>
          <n-icon :size="15"><ExternalLink /></n-icon>
        </template>
        新标签页
      </n-button>
      <n-button size="small" round secondary @click="reloadFrame"
        class="!shadow-lg shadow-black/10 backdrop-blur-md">
        <template #icon>
          <n-icon :size="15"><Refresh /></n-icon>
        </template>
        刷新
      </n-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NButton, NIcon, NSpin } from 'naive-ui'
import { Refresh, ExternalLink, Dashboard } from '@vicons/tabler'
import { configApi } from '../api'
import { useAppStore } from '../stores/app'

// 默认走飞牛网关前缀反代（同源内嵌）；custom/port 模式按配置解析
const DEFAULT_URL = '/app/deepseek-harness/fngateway/'

const loaded = ref(false)
const frameRef = ref<HTMLIFrameElement | null>(null)
const webuiUrl = ref(DEFAULT_URL)

// 侧边栏在 WebUI 视图已整体隐藏，通过悬浮按钮切回管理面板（概览）
const appStore = useAppStore()
function goManage() {
  appStore.setTab('overview')
}

// 按访问模式解析 WebUI 地址：fngateway 同源内嵌，custom 用外部地址，port 用代理端口
async function resolveWebUIUrl(): Promise<string> {
  try {
    const res = await configApi.getConfig()
    const cfg = res.success ? res.data : null
    const mode = cfg?.access_mode || (cfg?.reverse_proxy_url ? 'custom' : 'fngateway')

    if (mode === 'custom' && cfg?.reverse_proxy_url) {
      return cfg.reverse_proxy_url
    }
    if (mode === 'port') {
      const port = cfg?.proxy_port || 2299
      return `https://${window.location.hostname}:${port}/`
    }
  } catch (e) {
    console.error('WebUI: 读取访问模式失败，回退网关前缀', e)
  }
  return DEFAULT_URL
}

onMounted(async () => {
  webuiUrl.value = await resolveWebUIUrl()
})

// 重新加载内嵌页面：重置加载态并刷新 iframe
function reloadFrame() {
  loaded.value = false
  if (frameRef.value) {
    frameRef.value.src = webuiUrl.value
  }
}

// 新标签页打开独立 WebUI（iframe 内受限时的逃生通道）
function openExternal() {
  window.open(webuiUrl.value, '_blank')
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