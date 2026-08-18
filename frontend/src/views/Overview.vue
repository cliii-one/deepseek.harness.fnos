<template>
  <div class="w-full flex-1 flex flex-col gap-4 sm:gap-6">
    <!-- 桌面端页头 -->
    <n-page-header class="hidden sm:block">
      <template #title>
        <div class="text-xl font-bold text-slate-800 dark:text-slate-100 tracking-tight">概览</div>
      </template>
    </n-page-header>

    <!-- 状态监控核心卡片 -->
    <n-card :bordered="false" class="shadow-sm rounded-2xl">
      <div class="flex flex-col py-1 sm:py-2">
        <!-- 上半部分：应用标题/版本 + 右侧主操作（进入 Harness） -->
        <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div class="space-y-1 min-w-0 flex-1">
            <div class="flex items-center gap-2.5 flex-wrap min-w-0">
              <span class="text-lg sm:text-xl font-bold text-slate-800 dark:text-slate-100 tracking-tight leading-tight truncate">
                {{ statusData.name || 'DeepSeek Harness' }}
              </span>
              <!-- 更新构建过程中的目标 Commit 动态小徽章 -->
              <n-tag
                v-if="isBuilding && statusData.target_commit && statusData.commit && statusData.commit !== statusData.target_commit"
                type="info"
                size="small"
                round
                :bordered="false"
                class="font-mono text-xs bg-blue-50 dark:bg-blue-950/50 text-blue-600 dark:text-blue-400 font-semibold px-2 flex items-center gap-1 shadow-sm shrink-0"
              >
                <template #icon>
                  <n-spin :size="10" class="mr-0.5" />
                </template>
                <span>{{ formatShortCommit(statusData.commit) }}</span>
                <span class="opacity-60">→</span>
                <span>{{ formatShortCommit(statusData.target_commit) }}</span>
              </n-tag>
            </div>
            <div class="text-xs sm:text-sm text-slate-400 dark:text-slate-500 flex items-center gap-1.5 sm:gap-2 flex-nowrap min-w-0">
              <span class="shrink-0">版本: {{ statusData.version || '-' }}</span>
              <span class="text-slate-200 dark:text-slate-700 shrink-0 select-none">|</span>
              <span class="shrink-0 font-mono">Commit: {{ statusData.commit || '-' }}</span>
              <span class="text-slate-200 dark:text-slate-700 shrink-0 select-none">|</span>
              <span
                class="min-w-0 truncate"
                :title="statusData.build_time ? `Build: ${statusData.build_time}` : ''"
              >
                Build: {{ statusData.build_time || '-' }}
              </span>
            </div>
          </div>

          <n-button type="primary" size="large" :disabled="!isRunning" @click="goWebUI"
            class="w-full sm:w-auto !h-12 px-7 shadow-sm shadow-fnos-blue/20 font-medium rounded-xl text-base transition-transform duration-150 active:scale-95">
            <template #icon>
              <n-icon :size="20">
                <Message />
              </n-icon>
            </template>
            <span>进入 Harness</span>
          </n-button>
        </div>

        <!-- 原生分割线 -->
        <n-divider class="!my-5 sm:!my-6" />

        <!-- 下半部分：3 列运行指标网格 -->
        <n-grid :cols="3" :x-gap="16" :y-gap="12" class="text-center" responsive="screen" item-responsive>
          <n-gi>
            <div
              class="py-3 sm:py-3.5 px-3 rounded-2xl bg-slate-50 dark:bg-white/[0.03] border border-slate-100/80 dark:border-white/[0.06] h-full flex flex-col justify-center transition-all duration-200 hover:border-slate-200/90 dark:hover:border-white/[0.12] hover:bg-slate-100/50 dark:hover:bg-white/[0.06]">
              <n-statistic label="运行状态">
                <template #default>
                  <div class="flex justify-center mt-1">
                    <n-tag :type="statusTagType" size="medium" round :bordered="false" class="font-medium transition-all duration-200">
                      <template #icon v-if="isBuilding || isStarting">
                        <n-spin :size="12" class="mr-1" />
                      </template>
                      {{ statusLabel }}
                    </n-tag>
                  </div>
                </template>
              </n-statistic>
            </div>
          </n-gi>

          <n-gi>
            <div
              class="py-3 sm:py-3.5 px-3 rounded-2xl bg-slate-50 dark:bg-white/[0.03] border border-slate-100/80 dark:border-white/[0.06] h-full flex flex-col justify-center transition-all duration-200 hover:border-slate-200/90 dark:hover:border-white/[0.12] hover:bg-slate-100/50 dark:hover:bg-white/[0.06]">
              <n-statistic label="运行时间">
                <template #default>
                  <div class="text-sm sm:text-base font-bold text-slate-700 dark:text-slate-200 truncate mt-0.5">
                    {{ uptimeText }}
                  </div>
                </template>
              </n-statistic>
            </div>
          </n-gi>

          <n-gi>
            <div
              class="py-3 sm:py-3.5 px-3 rounded-2xl bg-slate-50 dark:bg-white/[0.03] border border-slate-100/80 dark:border-white/[0.06] h-full flex flex-col justify-center transition-all duration-200 hover:border-slate-200/90 dark:hover:border-white/[0.12] hover:bg-slate-100/50 dark:hover:bg-white/[0.06]">
              <n-statistic label="进程 PID">
                <template #default>
                  <div class="text-sm sm:text-base font-bold text-slate-700 dark:text-slate-200 truncate mt-0.5"
                    :title="isRunning && statusData.pid ? String(statusData.pid) : '-'">
                    {{ isRunning && statusData.pid ? statusData.pid : '-' }}
                  </div>
                </template>
              </n-statistic>
            </div>
          </n-gi>
        </n-grid>
      </div>
    </n-card>

    <!-- 运行控制区 -->
    <div class="space-y-4">
      <h2 class="text-lg font-bold text-slate-800 dark:text-slate-100 tracking-tight">运行控制</h2>
      <n-grid v-auto-animate :cols="4" :x-gap="16" :y-gap="16" responsive="screen" item-responsive>
        <n-gi span="2 m:1" v-for="a in actionCards" :key="a.action">
          <n-tooltip trigger="hover" :disabled="isTouch || a.disabled || a.loading">
            <template #trigger>
              <div class="h-full">
                <!-- 若需要二次确认（停止、重建），接入 NPopconfirm -->
                <n-popconfirm
                  v-if="a.confirmText"
                  :disabled="a.disabled || a.loading"
                  @positive-click="handleAction(a.action)"
                  positive-text="确认"
                  negative-text="取消"
                >
                  <template #trigger>
                    <n-card
                      hoverable
                      :bordered="false"
                      class="cursor-pointer text-center interactive-card select-none !p-2 sm:!p-4 shadow-sm group rounded-2xl !h-full"
                      :class="{ 'opacity-50 !cursor-not-allowed !transform-none': a.disabled && !a.loading }"
                    >
                      <div class="flex flex-col items-center justify-center gap-2.5 py-3">
                        <div
                          class="w-12 h-12 rounded-2xl flex items-center justify-center transition-all duration-200 group-hover:scale-110 group-active:scale-95"
                          :class="a.iconBg"
                        >
                          <n-spin v-if="a.loading" :size="24" />
                          <n-icon v-else :size="24" :class="a.iconColor" class="transition-transform duration-200">
                            <component :is="a.icon" />
                          </n-icon>
                        </div>
                        <span class="text-sm font-medium text-slate-700 dark:text-slate-300 group-hover:text-slate-900 dark:group-hover:text-white transition-colors duration-150">
                          {{ a.label }}
                        </span>
                      </div>
                    </n-card>
                  </template>
                  {{ a.confirmText }}
                </n-popconfirm>

                <!-- 常规操作卡片（无确认提示） -->
                <n-card
                  v-else
                  hoverable
                  :bordered="false"
                  class="cursor-pointer text-center interactive-card select-none !p-2 sm:!p-4 shadow-sm group rounded-2xl !h-full"
                  :class="{ 'opacity-50 !cursor-not-allowed !transform-none': a.disabled && !a.loading }"
                  @click="!a.disabled && !a.loading && handleAction(a.action)"
                >
                  <div class="flex flex-col items-center justify-center gap-2.5 py-3">
                    <div
                      class="w-12 h-12 rounded-2xl flex items-center justify-center transition-all duration-200 group-hover:scale-110 group-active:scale-95"
                      :class="a.iconBg"
                    >
                      <n-spin v-if="a.loading" :size="24" />
                      <n-icon v-else :size="24" :class="a.iconColor" class="transition-transform duration-200">
                        <component :is="a.icon" />
                      </n-icon>
                    </div>
                    <span class="text-sm font-medium text-slate-700 dark:text-slate-300 group-hover:text-slate-900 dark:group-hover:text-white transition-colors duration-150">
                      {{ a.label }}
                    </span>
                  </div>
                </n-card>
              </div>
            </template>
            {{ a.desc }}
          </n-tooltip>
        </n-gi>
      </n-grid>
    </div>

    <!-- 底部状态通知区 -->
    <div v-auto-animate class="space-y-3">
      <!-- 实时构建进度 / 启动中 / 错误信息 -->
      <n-alert v-if="statusData.last_message" :type="isBuilding || isStarting ? 'info' : 'warning'" :show-icon="true" class="rounded-2xl shadow-sm">
        {{ statusData.last_message }}
      </n-alert>

      <!-- 实时连接断开提示 -->
      <n-alert v-if="!wsConnected" type="error" :show-icon="true" class="rounded-2xl shadow-sm">
        实时连接已断开，正在自动重连…
      </n-alert>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, type Component } from 'vue'
import { storeToRefs } from 'pinia'
import {
  NPageHeader,
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
  NTooltip,
  NPopconfirm,
  useMessage
} from 'naive-ui'
import {
  Message,
  PlayerPlay,
  PlayerStop,
  Refresh,
  Download,
  Tools
} from '@vicons/tabler'
import { useSystemStore } from '../stores/system'
import { useAppStore } from '../stores/app'
import { withAsyncLock } from '../utils/debounce'
import { useIsTouchDevice } from '../utils/device'

const systemStore = useSystemStore()
const appStore = useAppStore()
const message = useMessage()
const isTouch = useIsTouchDevice()

const {
  statusData,
  wsConnected,
  activeAction,
  isActionLocked,
  isRunning,
  isStarting,
  isBuilding,
  statusTagType,
  statusLabel,
  uptimeText
} = storeToRefs(systemStore)

// 进入 Harness：切换到内嵌 WebUI 视图（应用内沉浸式，不再新开浏览器标签）
function goWebUI() {
  appStore.setTab('webui')
}

function formatShortCommit(c?: string): string {
  if (!c || c === '-') return '-'
  return c.length > 7 ? c.slice(0, 7) : c
}

interface ActionCard {
  action: string
  icon: Component
  label: string
  desc: string
  iconBg: string
  iconColor: string
  disabled: boolean
  loading: boolean
  confirmText?: string
}

const isRestarting = computed(() => activeAction.value === 'restart')
const showStopCard = computed(() => isRunning.value || isRestarting.value)

const actionCards = computed<ActionCard[]>(() => [
  showStopCard.value
    ? {
      action: 'stop',
      icon: PlayerStop,
      label: '停止服务',
      desc: '终止 DeepSeek Harness 后台运行进程',
      iconBg: 'bg-rose-50 dark:bg-rose-950/30 group-hover:bg-rose-100 dark:group-hover:bg-rose-950/50',
      iconColor: 'text-rose-600 dark:text-rose-400',
      disabled: isActionLocked.value,
      loading: activeAction.value === 'stop',
      confirmText: '确定要停止 DeepSeek Harness 服务吗？'
    }
    : {
      action: 'start',
      icon: PlayerPlay,
      label: isStarting.value ? '服务启动中' : '启动服务',
      desc: isStarting.value ? '正在拉起服务主进程并等待就绪…' : '拉起 DeepSeek Harness 后台核心服务',
      iconBg: 'bg-emerald-50 dark:bg-emerald-950/30 group-hover:bg-emerald-100 dark:group-hover:bg-emerald-950/50',
      iconColor: 'text-emerald-600 dark:text-emerald-400',
      disabled: isActionLocked.value,
      loading: isStarting.value || activeAction.value === 'start'
    },
  {
    action: 'restart',
    icon: Refresh,
    label: '重启服务',
    desc: '热重启后台进程，即时生效最新配置或插件变更',
    iconBg: 'bg-amber-50 dark:bg-amber-950/30 group-hover:bg-amber-100 dark:group-hover:bg-amber-950/50',
    iconColor: 'text-amber-600 dark:text-amber-400',
    disabled: !isRestarting.value && (!isRunning.value || isActionLocked.value),
    loading: isRestarting.value
  },
  {
    action: 'upgrade',
    icon: Download,
    label: '拉取更新',
    desc: '检查远程代码更新，检测到新版本时自动同步依赖并构建',
    iconBg: 'bg-blue-50 dark:bg-blue-950/30 group-hover:bg-blue-100 dark:group-hover:bg-blue-950/50',
    iconColor: 'text-fnos-blue dark:text-blue-400',
    disabled: isActionLocked.value && activeAction.value !== 'upgrade',
    loading: activeAction.value === 'upgrade' || (isBuilding.value && activeAction.value !== 'rebuild')
  },
  {
    action: 'rebuild',
    icon: Tools,
    label: '强制重建',
    desc: '重新拉取全部依赖并完整编译，用于修复异常损坏的环境',
    iconBg: 'bg-purple-50 dark:bg-purple-950/30 group-hover:bg-purple-100 dark:group-hover:bg-purple-950/50',
    iconColor: 'text-purple-600 dark:text-purple-400',
    disabled: isActionLocked.value && activeAction.value !== 'rebuild',
    loading: activeAction.value === 'rebuild',
    confirmText: '强制重建将重新拉取依赖并编译，耗时较长，确定继续？'
  }
])

const handleAction = withAsyncLock(async (action: string) => {
  const res = await systemStore.sendAction(action)
  if (res.success) {
    message.success(res.message || '操作成功')
  } else {
    message.error(res.message || '操作失败')
  }
})
</script>