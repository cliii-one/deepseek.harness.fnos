<template>
  <div class="w-full flex-1 flex flex-col gap-4 sm:gap-6">
    <!-- 页头 -->
    <div class="flex items-center justify-between gap-3 w-full">
      <h1 class="text-xl font-bold text-slate-800 dark:text-slate-100 tracking-tight">应用设置</h1>
    </div>

    <!-- 访问方式：只读说明卡（统一网关接入，无需可编辑配置） -->
    <n-card :bordered="false" class="shadow-sm rounded-2xl">
      <n-alert type="info" :show-icon="true" class="rounded-xl mb-3">
        当前访问方式：<b>飞牛统一网关</b>
      </n-alert>
      <p class="text-sm leading-relaxed text-slate-500 dark:text-slate-400">
        DeepSeek Harness 通过飞牛统一网关接入（路径
        <code class="px-1 py-0.5 rounded bg-slate-100 dark:bg-white/10 text-slate-700 dark:text-slate-300 font-mono text-xs">/app/deepseek-harness</code>），
        由飞牛系统统一管理登录态与会话鉴权，应用自身无需暴露端口、配置反向代理或访问密码。
      </p>
      <p class="text-sm leading-relaxed text-slate-500 dark:text-slate-400 mt-2">
        桌面入口打开后默认为概览页，点击「进入 Harness」即可在应用内使用 DSH 聊天界面；也可通过侧边栏底部的
        「新标签页」按钮在独立浏览器窗口中打开。
      </p>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { NCard, NAlert } from 'naive-ui'

// 极简只读设置页：统一网关接入，无可编辑配置项。
// 原端口/访问模式/反向代理/访问密码等配置项随 0.2.9 移除（只保留飞牛统一网关一条链路）。
</script>