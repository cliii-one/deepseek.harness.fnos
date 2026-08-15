<template>
  <div class="space-y-6 max-w-5xl mx-auto">
    <h1 class="text-xl font-bold text-slate-800 tracking-tight">工作区</h1>

    <div v-if="!items.length"
      class="bg-white rounded-2xl p-10 shadow-sm border border-slate-100 text-center">
      <div class="w-16 h-16 bg-slate-50 rounded-full flex items-center justify-center mx-auto mb-4">
        <Icon name="workspace" :size="28" class="text-slate-400" />
      </div>
      <p class="text-slate-500 text-sm">暂无工作区数据</p>
      <p class="text-slate-400 text-xs mt-1">请先运行 DeepSeek Harness 并创建工作区</p>
    </div>

    <div v-else class="grid grid-cols-1 sm:grid-cols-2 gap-3">
      <button v-for="item in items" :key="item.workspaceId"
        @click="openWorkspace(item.path)"
        :title="`在 FNOS 文件管理 打开 ${item.title}`"
        class="bg-white hover:bg-slate-50 border border-slate-100 rounded-2xl p-4 text-left shadow-sm transition-colors group">
        <div class="flex items-start gap-3">
          <div class="w-10 h-10 bg-slate-50 rounded-xl flex items-center justify-center text-slate-600 group-hover:bg-blue-50 group-hover:text-fnos-blue transition-colors shrink-0">
            <Icon name="workspaceCard" :size="20" />
          </div>
          <div class="min-w-0 flex-1">
            <h3 class="text-sm font-semibold text-slate-800 truncate">{{ item.title }}</h3>
            <p class="text-xs text-slate-400 truncate mt-0.5" :title="item.path">{{ item.path }}</p>
            <div class="flex items-center gap-3 mt-2 text-[11px] text-slate-400">
              <span>{{ (item.sessionIds || []).length }} 个会话</span>
              <span v-if="item.updatedAt">更新于 {{ formatTime(item.updatedAt) }}</span>
            </div>
          </div>
        </div>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { TrimApp } from '@trimjs/web-app'
import { useAppStore } from '../stores/app'
import { useToastStore } from '../stores/toast'
import Icon from '../components/Icon.vue'

const appStore = useAppStore()
const toastStore = useToastStore()
const items = computed(() => (appStore.workspaceData.items || []).filter(Boolean))

let trimApp: TrimApp | null = null

const formatTime = (iso: string) => {
  try {
    const d = new Date(iso)
    return d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
  } catch {
    return iso
  }
}

const openWorkspace = async (path: string) => {
  if (!trimApp) return
  try {
    await trimApp.openFileManager(path)
  } catch (e: any) {
    toastStore.showToast(`打开文件管理器失败：${e?.message || e}`)
  }
}

onMounted(async () => {
  trimApp = new TrimApp()
  await trimApp.ready()
  await appStore.fetchWorkspaceData()
})
</script>