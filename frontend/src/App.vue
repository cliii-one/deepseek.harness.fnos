<template>
  <n-config-provider :theme="currentTheme" :theme-overrides="currentThemeOverrides">
    <n-global-style />
    <n-loading-bar-provider>
      <n-dialog-provider>
        <n-notification-provider>
          <n-message-provider>
            <!-- 整体视口容器：Naive UI 原生 Layout -->
            <n-layout has-sider position="absolute" class="h-screen w-screen bg-[#f5f7fa] dark:bg-[#12141a]">
              <!-- 桌面端侧边栏 Sider：Naive UI 原生 NLayoutSider；进入 WebUI 时整体折叠（沉浸式全屏） -->
              <n-layout-sider
                v-if="tab !== 'webui'"
                bordered
                :width="240"
                :native-scrollbar="false"
                class="hidden sm:block select-none z-10"
                content-style="display: flex; flex-direction: column; justify-content: space-between; height: 100%;"
              >
                <!-- 顶部品牌与主导航菜单 -->
                <div class="flex flex-col gap-3 p-3">
                  <!-- 应用品牌标题卡片 -->
                  <div class="flex items-center gap-3 px-3 py-3 rounded-2xl bg-slate-50 dark:bg-white/[0.04] border border-slate-100/80 dark:border-white/[0.06] transition-all duration-200 hover:border-slate-200 dark:hover:border-white/[0.12] hover:bg-slate-100/50 dark:hover:bg-white/[0.07]">
                    <img src="/favicon.svg" alt="logo" class="w-8 h-8 rounded-xl object-contain shrink-0 transition-transform duration-200 hover:scale-105" />
                    <div class="min-w-0 flex-1">
                      <div class="text-sm font-bold text-slate-800 dark:text-slate-100 leading-tight truncate">DeepSeek</div>
                      <div class="text-[11px] text-slate-400 dark:text-slate-500 font-medium truncate mt-0.5">
                        Harness 管理器
                      </div>
                    </div>
                  </div>

                  <!-- 主导航菜单 -->
                  <n-menu
                    :value="tab"
                    :options="menuOptions"
                    @update:value="handleMenuSelect"
                  />
                </div>
              </n-layout-sider>

                <!-- 右侧主界面（滚动 Content + 移动端 Footer） -->
                <n-layout
                  class="h-full flex-1 flex flex-col overflow-hidden bg-[#f5f7fa] dark:bg-[#12141a]"
                  content-style="display: flex; flex-direction: column; height: 100%; flex: 1;"
                >
                  <!-- 主内容滚动区域：Naive UI 原生 NLayoutContent -->
                  <n-layout-content
                    :native-scrollbar="false"
                    :content-class="contentClass"
                    content-style="min-height: 100%; display: flex; flex-direction: column; align-items: center;"
                    class="flex-1"
                  >
                    <!-- 宽度约束容器：WebUI 视图全宽全屏（沉浸式），管理视图保持居中限宽 -->
                    <div :class="tab === 'webui' ? 'w-full flex-1 flex flex-col min-h-0' : 'w-full max-w-6xl flex-1 flex flex-col min-h-0'">
                      <Transition name="view-fade-slide" mode="out-in">
                        <KeepAlive>
                          <component :is="currentView" :key="tab" />
                        </KeepAlive>
                      </Transition>
                    </div>
                    <n-back-top v-if="tab !== 'webui'" :bottom="70" :right="20" class="sm:!bottom-8 sm:!right-8" />
                  </n-layout-content>

                <!-- 移动端底部导航 Tabbar：进入 WebUI 时同样隐藏（沉浸式全屏） -->
                <n-layout-footer
                  v-if="tab !== 'webui'"
                  bordered
                  position="absolute"
                  class="sm:hidden z-50 !bg-white/95 dark:!bg-[#181b22]/95 !backdrop-blur-md px-1 pt-1 shadow-lg mobile-tabbar-footer"
                >
                  <n-flex justify="space-around" align="center" :wrap="false" class="w-full">
                    <n-button
                      v-for="t in mobileTabs"
                      :key="t.key"
                      text
                      :type="tab === t.key ? 'primary' : 'default'"
                      @click="tab = t.key"
                      class="flex-1 !py-1 !px-0 transition-transform duration-150 active:scale-90"
                    >
                      <div class="flex flex-col items-center gap-0.5 select-none">
                        <n-icon :size="20" class="transition-transform duration-200" :class="tab === t.key ? 'scale-110' : 'scale-100'">
                          <component :is="t.icon" />
                        </n-icon>
                        <span
                          class="text-[11px] leading-tight transition-colors duration-150"
                          :class="tab === t.key ? 'font-bold text-fnos-blue dark:text-blue-400' : 'font-normal text-slate-500 dark:text-slate-400'"
                        >
                          {{ t.label }}
                        </span>
                      </div>
                    </n-button>
                  </n-flex>
                </n-layout-footer>
              </n-layout>
            </n-layout>

            <!-- 仅在开发环境挂载的仿真调试控制悬浮窗（生产构建通过 DCE 彻底排除） -->
            <component :is="MockDevControls" v-if="isDev && MockDevControls" />
          </n-message-provider>
        </n-notification-provider>
      </n-dialog-provider>
    </n-loading-bar-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import { h, ref, computed, watch, onMounted, defineAsyncComponent, type Component } from 'vue'

// 仅在开发模式动态加载仿真控制组件，生产构建由 Vite/Rollup 死代码消除 (DCE) 彻底剔除
const isDev = Boolean(import.meta.env.DEV)
const MockDevControls = isDev
  ? defineAsyncComponent(() => import('./components/MockDevControls.vue'))
  : null
import {
  NConfigProvider,
  NGlobalStyle,
  NMessageProvider,
  NDialogProvider,
  NNotificationProvider,
  NLoadingBarProvider,
  NLayout,
  NLayoutSider,
  NLayoutContent,
  NLayoutFooter,
  NMenu,
  NFlex,
  NButton,
  NBackTop,
  NIcon,
  darkTheme,
  useOsTheme,
  type MenuOption
} from 'naive-ui'
import {
  Dashboard,
  Folder,

  FileText
} from '@vicons/tabler'
import { getThemeOverrides } from './theme'
import Overview from './views/Overview.vue'
import Logs from './views/Logs.vue'
import Workspace from './views/Workspace.vue'

import WebUI from './views/WebUI.vue'
import { useAppStore } from './stores/app'
import { trimSdk } from './utils/trimSdk'

const appStore = useAppStore()
const osTheme = useOsTheme()
const themeMode = ref<'light' | 'dark'>(osTheme.value === 'dark' ? 'dark' : 'light')

// 监听系统偏好自动切换（当非宿主覆盖时）
watch(osTheme, (newOsTheme) => {
  if (trimSdk.isStandaloneWeb || !trimSdk.isWeb) {
    themeMode.value = newOsTheme === 'dark' ? 'dark' : 'light'
  }
})

// 同步 HTML 根节点 class 与 dataset
watch(themeMode, (mode) => {
  document.documentElement.dataset.theme = mode
  if (mode === 'dark') {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}, { immediate: true })

const currentTheme = computed(() => (themeMode.value === 'dark' ? darkTheme : null))
const currentThemeOverrides = computed(() => getThemeOverrides(themeMode.value))

type TabKey = 'overview' | 'workspace' | 'webui' | 'logs'

const tabLabels: Record<TabKey, string> = {
  overview: '概览 · DeepSeek Harness',
  workspace: '工作区 · DeepSeek Harness',
  webui: 'DeepSeek Harness WebUI',
  logs: '运行日志 · DeepSeek Harness'
}

const views: Record<TabKey, Component> = {
  overview: Overview,
  workspace: Workspace,
  webui: WebUI,
  logs: Logs
}

function renderIcon(icon: Component) {
  return () => h(NIcon, null, { default: () => h(icon) })
}

const menuOptions: MenuOption[] = [
  { key: 'overview', label: '概览', icon: renderIcon(Dashboard) },
  { key: 'workspace', label: '工作区', icon: renderIcon(Folder) },
  { key: 'logs', label: '运行日志', icon: renderIcon(FileText) }
]

const mobileTabs = [
  { key: 'overview' as TabKey, label: '概览', icon: Dashboard },
  { key: 'workspace' as TabKey, label: '工作区', icon: Folder },
  { key: 'logs' as TabKey, label: '日志', icon: FileText }
]

const tab = computed<TabKey>({
  get: () => appStore.currentTab as TabKey,
  set: (v) => appStore.setTab(v)
})

watch(tab, (newTab) => {
  trimSdk.setTitle(tabLabels[newTab] || 'DeepSeek Harness')
}, { immediate: true })

const handleMenuSelect = (key: string) => {
  tab.value = key as TabKey
}

const currentView = computed(() => views[tab.value])

// WebUI 视图沉浸式全屏：清除滚动区域的内边距与背景
const contentClass = computed(() =>
  tab.value === 'webui' ? 'app-content-scroll app-content-full' : 'app-content-scroll'
)

onMounted(() => {
  appStore.init()
  trimSdk.initPlatformTheme((theme) => {
    themeMode.value = theme
  })
})
</script>

<style scoped>
:deep(.app-content-scroll) {
  padding: 14px 14px calc(68px + env(safe-area-inset-bottom, 0px)) 14px;
}
/* WebUI 沉浸式视图：铺满内容区，无内边距 */
:deep(.app-content-full) {
  padding: 0 !important;
}
.mobile-tabbar-footer {
  padding-bottom: calc(8px + env(safe-area-inset-bottom, 0px));
}
@media (min-width: 640px) {
  :deep(.app-content-scroll) {
    padding: 24px;
  }
}
</style>