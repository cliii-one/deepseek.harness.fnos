<template>
  <div class="w-full flex-1 flex flex-col gap-4 sm:gap-6">
    <!-- 桌面端页头 -->
    <div class="hidden sm:flex items-center justify-between">
      <h1 class="text-xl font-bold text-slate-800 tracking-tight">概览</h1>
    </div>

    <!-- 状态监控核心卡片 -->
    <n-card :bordered="false" class="shadow-sm rounded-2xl">
      <div class="flex flex-col">
        <!-- 上半部分：应用标题/版本 + 右侧主操作（进入 Harness） -->
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div>
            <div class="text-base sm:text-lg font-bold text-slate-800 tracking-tight leading-tight">
              {{ statusData.name || 'DeepSeek Harness' }}
            </div>
            <div class="text-xs text-slate-400 flex items-center gap-2 mt-1">
              <span>版本: {{ statusData.version || '0.1.0' }}</span>
              <span class="text-slate-200">|</span>
              <span>Commit: {{ statusData.commit || '-' }}</span>
            </div>
          </div>

          <n-button
            type="primary"
            size="large"
            v-debounce
            :disabled="!isRunning"
            @click="systemStore.openHarnessApp"
            class="w-full sm:w-auto px-6 shadow-sm shadow-fnos-blue/20 font-medium"
          >
            <template #icon>
              <n-icon :size="18">
                <ExternalLink />
              </n-icon>
            </template>
            <span>进入 Harness</span>
          </n-button>
        </div>

        <!-- 原生分割线 -->
        <n-divider class="!my-4 sm:!my-5" />

        <!-- 下半部分：3 列运行指标网格 -->
        <n-grid :cols="3" :x-gap="8" class="text-center">
          <n-gi>
            <n-statistic label="运行状态">
              <template #default>
                <div class="flex justify-center mt-0.5">
                  <n-tag :type="statusTagType" size="small" round :bordered="false">
                    <template #icon v-if="isBuilding">
                      <n-spin :size="12" class="mr-1" />
                    </template>
                    {{ statusLabel }}
                  </n-tag>
                </div>
              </template>
            </n-statistic>
          </n-gi>

          <n-gi>
            <n-statistic label="运行时间">
              <template #default>
                <div class="text-sm sm:text-base font-semibold text-slate-700 mt-0.5">
                  {{ uptimeText }}
                </div>
              </template>
            </n-statistic>
          </n-gi>

          <n-gi>
            <n-statistic label="构建时间">
              <template #default>
                <div
                  class="text-xs sm:text-sm font-medium text-slate-600 mt-0.5 truncate"
                  :title="statusData.build_time || '-'"
                >
                  {{ statusData.build_time || '-' }}
                </div>
              </template>
            </n-statistic>
          </n-gi>
        </n-grid>

        <!-- 实时构建进度 / 错误信息 -->
        <div v-if="statusData.last_message" class="mt-4">
          <n-alert :type="isBuilding ? 'info' : 'warning'" :show-icon="true">
            {{ statusData.last_message }}
          </n-alert>
        </div>

        <!-- 实时连接断开提示 -->
        <div v-if="!wsConnected" class="mt-4">
          <n-alert type="error" :show-icon="true">
            实时连接已断开，正在自动重连…
          </n-alert>
        </div>
      </div>
    </n-card>

    <!-- 运行操作区 -->
    <div class="space-y-4">
      <h2 class="text-lg font-bold text-slate-800 tracking-tight">运行控制</h2>
      <n-grid :cols="4" :x-gap="16" :y-gap="16" responsive="screen" item-responsive>
        <n-gi span="2 m:1" v-for="a in actionCards" :key="a.label">
          <!-- 若需要二次确认（停止、重建），接入 NPopconfirm -->
          <n-popconfirm
            v-if="a.confirmText && !a.disabled"
            @positive-click="handleAction(a.action)"
            :positive-text="'确认'"
            :negative-text="'取消'"
          >
            <template #trigger>
              <n-card
                hoverable
                v-debounce
                :bordered="false"
                class="cursor-pointer text-center transition-all !p-2 sm:!p-4 shadow-sm group rounded-2xl !h-full"
                :class="{ 'opacity-50 !cursor-not-allowed': a.disabled }"
              >
                <div class="flex flex-col items-center justify-center gap-2.5 py-3">
                  <div
                    class="w-12 h-12 rounded-2xl flex items-center justify-center transition-colors"
                    :class="a.iconBg"
                  >
                    <n-icon :size="24" :class="a.iconColor">
                      <component :is="a.icon" />
                    </n-icon>
                  </div>
                  <span class="text-sm font-medium text-slate-700 group-hover:text-slate-900">
                    {{ a.label }}
                  </span>
                </div>
              </n-card>
            </template>
            {{ a.confirmText }}
          </n-popconfirm>

          <!-- 常规操作卡片 -->
          <n-card
            v-else
            hoverable
            v-debounce
            :bordered="false"
            class="cursor-pointer text-center transition-all !p-2 sm:!p-4 shadow-sm group rounded-2xl !h-full"
            :class="{ 'opacity-50 !cursor-not-allowed': a.disabled }"
            @click="!a.disabled && handleAction(a.action)"
          >
            <div class="flex flex-col items-center justify-center gap-2.5 py-3">
              <div
                class="w-12 h-12 rounded-2xl flex items-center justify-center transition-colors"
                :class="a.iconBg"
              >
                <n-icon :size="24" :class="a.iconColor">
                  <component :is="a.icon" />
                </n-icon>
              </div>
              <span class="text-sm font-medium text-slate-700 group-hover:text-slate-900">
                {{ a.label }}
              </span>
            </div>
          </n-card>
        </n-gi>
      </n-grid>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, type Component } from 'vue'
import { storeToRefs } from 'pinia'
import {
  NCard,
  NDivider,
  NButton,
  NTag,
  NGrid,
  NGi,
  NStatistic,
  NAlert,
  NSpin,
  NIcon,
  NPopconfirm,
  useMessage
} from 'naive-ui'
import {
  ExternalLink,
  PlayerPlay,
  PlayerStop,
  Refresh,
  Download,
  Tools
} from '@vicons/tabler'
import { useSystemStore } from '../stores/system'
import { withAsyncLock } from '../utils/debounce'

const systemStore = useSystemStore()
const message = useMessage()

const {
  statusData,
  wsConnected,
  actionBusy: loading,
  isRunning,
  isBuilding,
  statusTagType,
  statusLabel,
  uptimeText
} = storeToRefs(systemStore)

interface ActionCard {
  action: string
  icon: Component
  label: string
  iconBg: string
  iconColor: string
  disabled: boolean
  confirmText?: string
}

const actionCards = computed<ActionCard[]>(() => [
  isRunning.value
    ? {
        action: 'stop',
        icon: PlayerStop,
        label: '停止服务',
        iconBg: 'bg-rose-50 group-hover:bg-rose-100',
        iconColor: 'text-rose-600',
        disabled: loading.value,
        confirmText: '确定要停止 DeepSeek Harness 服务吗？'
      }
    : {
        action: 'start',
        icon: PlayerPlay,
        label: '启动服务',
        iconBg: 'bg-emerald-50 group-hover:bg-emerald-100',
        iconColor: 'text-emerald-600',
        disabled: loading.value || isBuilding.value
      },
  {
    action: 'restart',
    icon: Refresh,
    label: '重启服务',
    iconBg: 'bg-amber-50 group-hover:bg-amber-100',
    iconColor: 'text-amber-600',
    disabled: loading.value || !isRunning.value || isBuilding.value
  },
  {
    action: 'upgrade',
    icon: Download,
    label: '拉取更新',
    iconBg: 'bg-blue-50 group-hover:bg-blue-100',
    iconColor: 'text-fnos-blue',
    disabled: loading.value || isBuilding.value
  },
  {
    action: 'rebuild',
    icon: Tools,
    label: '强制重建',
    iconBg: 'bg-purple-50 group-hover:bg-purple-100',
    iconColor: 'text-purple-600',
    disabled: loading.value || isBuilding.value,
    confirmText: '强制重建将重新拉取依赖并编译，耗时较长，确定继续？'
  }
])

const handleAction = withAsyncLock(async (action: string) => {
  const res = await systemStore.sendAction(action)
  if (res.success) {
    message.success('指令已发送')
  } else {
    message.error(res.message || '操作失败')
  }
})
</script>