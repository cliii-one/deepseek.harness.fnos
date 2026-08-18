<template>
  <!-- WebUI 内嵌视图：管理面板内直接嵌入 DSH 聊天界面（fngateway 网关同源，无需额外鉴权） -->
  <div class="w-full h-[calc(100dvh-82px)] sm:h-[calc(100dvh-48px)] flex flex-col min-h-0 overflow-hidden">
    <n-card :bordered="false" class="flex-1 flex flex-col shadow-sm rounded-2xl min-h-0 overflow-hidden"
      content-style="display: flex; flex-direction: column; flex: 1; min-height: 0; overflow: hidden; padding: 0;">
      <!-- 标题与操作栏 -->
      <template #header>
        <span class="text-base sm:text-lg font-bold text-slate-800 dark:text-slate-100 tracking-tight">DeepSeek Harness WebUI</span>
      </template>

      <template #header-extra>
        <n-flex :size="8" align="center" :wrap="false">
          <!-- 重新加载内嵌页面 -->
          <n-button size="small" secondary title="重新加载" class="!px-2" @click="reloadFrame">
            <n-icon :size="16">
              <Refresh />
            </n-icon>
          </n-button>

          <!-- 新标签页打开：iframe 内受限时（弹窗/全屏/快捷键）可跳出独立页 -->
          <n-button size="small" secondary type="primary" title="在新标签页中打开" class="!px-2 sm:!px-2.5"
            @click="openExternal">
            <div class="flex items-center justify-center gap-1">
              <n-icon :size="16">
                <ExternalLink />
              </n-icon>
              <span class="hidden sm:inline">新标签页打开</span>
            </div>
          </n-button>
        </n-flex>
      </template>

      <!-- iframe 容器：撑满卡片并内部滚动 -->
      <div class="relative flex-1 min-h-0 flex flex-col overflow-hidden">
        <!-- 加载中遮罩（iframe onload 后淡出） -->
        <transition name="fade">
          <div v-if="!loaded" class="absolute inset-0 z-10 flex items-center justify-center bg-slate-50/80 dark:bg-[#12141a]/80 backdrop-blur-sm">
            <div class="flex items-center gap-2 text-slate-400 dark:text-slate-500">
              <n-spin :size="18" />
              <span class="text-sm">正在加载 Harness WebUI…</span>
            </div>
          </div>
        </transition>

        <iframe
          ref="frameRef"
          :src="webuiUrl"
          class="flex-1 min-h-0 w-full border-0"
          style="background: #fff;"
          @load="loaded = true"
        />
      </div>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { NCard, NFlex, NButton, NIcon, NSpin } from 'naive-ui'
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
