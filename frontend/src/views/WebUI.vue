<template>
  <!-- WebUI 沉浸式视图：DSH 聊天界面铺满整个内容区，无任何覆盖元素 -->
  <div class="w-full h-full relative flex-1 min-h-0 overflow-hidden bg-white">

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
      src="/app/deepseek-harness/fngateway/"
      class="absolute inset-0 w-full h-full border-0"
      style="background: #fff;"
      allow="clipboard-write; fullscreen"
      @load="loaded = true"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { NSpin } from 'naive-ui'

// DSH 通过飞牛统一网关接入（gatewayPrefix /app/deepseek-harness），聊天界面全屏展示；
// 管理/刷新/新标签页等工具按钮已移入侧边栏底部（App.vue），WebUI 视图保持零遮挡
const loaded = ref(false)
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