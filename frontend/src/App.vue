<template>
  <n-config-provider :theme-overrides="themeOverrides">
    <n-loading-bar-provider>
      <n-dialog-provider>
        <n-notification-provider>
          <n-message-provider>
            <!-- 整体视口容器：Naive UI 原生 Layout -->
            <n-layout has-sider position="absolute" class="h-screen w-screen bg-[#f5f7fa]">
              <!-- 桌面端侧边栏 Sider：Naive UI 原生 NLayoutSider -->
              <n-layout-sider
                bordered
                :width="240"
                :native-scrollbar="false"
                class="hidden sm:block select-none z-10"
                content-style="display: flex; flex-direction: column; justify-content: space-between; height: 100%;"
              >
                <!-- 顶部品牌与主导航菜单 -->
                <div class="flex flex-col gap-3 p-3">
                  <!-- 应用品牌标题卡片 -->
                  <div class="flex items-center gap-3 px-3 py-3 rounded-2xl bg-slate-50 border border-slate-100/80">
                    <img src="/favicon.svg" alt="logo" class="w-8 h-8 rounded-xl object-contain shrink-0" />
                    <div class="min-w-0 flex-1">
                      <div class="text-sm font-bold text-slate-800 leading-tight truncate">DeepSeek</div>
                      <div class="text-[11px] text-slate-400 font-medium truncate mt-0.5">
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

                <!-- 底部设置项：统一采用 NMenu 驱动交互与主题高亮 -->
                <div class="p-3">
                  <n-divider class="!my-2" />
                  <n-menu
                    :value="tab"
                    :options="settingsMenuOptions"
                    @update:value="handleMenuSelect"
                  />
                </div>
              </n-layout-sider>

                <!-- 右侧主界面（滚动 Content + 移动端 Footer） -->
                <n-layout
                  class="h-full flex-1 flex flex-col overflow-hidden bg-[#f5f7fa]"
                  content-style="display: flex; flex-direction: column; height: 100%; flex: 1;"
                >
                  <!-- 主内容滚动区域：Naive UI 原生 NLayoutContent -->
                  <n-layout-content
                    :native-scrollbar="false"
                    content-class="app-content-scroll"
                    content-style="min-height: 100%; display: flex; flex-direction: column; align-items: center;"
                    class="flex-1"
                  >
                    <!-- 全局统一宽度约束容器 -->
                    <div class="w-full max-w-6xl flex-1 flex flex-col min-h-0">
                      <component :is="currentView" />
                    </div>
                    <n-back-top :bottom="70" :right="20" class="sm:!bottom-8 sm:!right-8" />
                  </n-layout-content>

                <!-- 移动端底部导航 Tabbar：全套纯正 Naive UI 组件 -->
                <n-layout-footer
                  bordered
                  position="absolute"
                  class="sm:hidden z-50 !bg-white/95 !backdrop-blur-md px-1 pt-1 pb-2 shadow-lg"
                >
                  <n-flex justify="space-around" align="center" :wrap="false" class="w-full">
                    <n-button
                      v-for="t in mobileTabs"
                      :key="t.key"
                      text
                      v-debounce="200"
                      :type="tab === t.key ? 'primary' : 'default'"
                      @click="tab = t.key"
                      class="flex-1 !py-1 !px-0"
                    >
                      <div class="flex flex-col items-center gap-0.5">
                        <n-icon :size="20">
                          <component :is="t.icon" />
                        </n-icon>
                        <span
                          class="text-[11px] leading-tight"
                          :class="tab === t.key ? 'font-bold text-fnos-blue' : 'font-normal text-slate-500'"
                        >
                          {{ t.label }}
                        </span>
                      </div>
                    </n-button>
                  </n-flex>
                </n-layout-footer>
              </n-layout>
            </n-layout>
          </n-message-provider>
        </n-notification-provider>
      </n-dialog-provider>
    </n-loading-bar-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import { h, computed, onMounted, type Component } from 'vue'
import {
  NConfigProvider,
  NMessageProvider,
  NDialogProvider,
  NNotificationProvider,
  NLoadingBarProvider,
  NLayout,
  NLayoutSider,
  NLayoutContent,
  NLayoutFooter,
  NMenu,
  NDivider,
  NFlex,
  NButton,
  NBackTop,
  NIcon,
  type MenuOption
} from 'naive-ui'
import {
  Dashboard,
  Folder,
  Puzzle,
  FileText,
  Settings
} from '@vicons/tabler'
import { themeOverrides } from './theme'
import Overview from './views/Overview.vue'
import Logs from './views/Logs.vue'
import SettingsView from './views/Settings.vue'
import Workspace from './views/Workspace.vue'
import Plugins from './views/Plugins.vue'
import { useAppStore } from './stores/app'

const appStore = useAppStore()

type TabKey = 'overview' | 'workspace' | 'logs' | 'plugins' | 'settings'

const views: Record<TabKey, Component> = {
  overview: Overview,
  workspace: Workspace,
  logs: Logs,
  plugins: Plugins,
  settings: SettingsView
}

function renderIcon(icon: Component) {
  return () => h(NIcon, null, { default: () => h(icon) })
}

const menuOptions: MenuOption[] = [
  { key: 'overview', label: '概览', icon: renderIcon(Dashboard) },
  { key: 'workspace', label: '工作区', icon: renderIcon(Folder) },
  { key: 'plugins', label: '插件管理', icon: renderIcon(Puzzle) },
  { key: 'logs', label: '运行日志', icon: renderIcon(FileText) }
]

const settingsMenuOptions: MenuOption[] = [
  { key: 'settings', label: '应用设置', icon: renderIcon(Settings) }
]

const mobileTabs = [
  { key: 'overview' as TabKey, label: '概览', icon: Dashboard },
  { key: 'workspace' as TabKey, label: '工作区', icon: Folder },
  { key: 'plugins' as TabKey, label: '插件', icon: Puzzle },
  { key: 'logs' as TabKey, label: '日志', icon: FileText },
  { key: 'settings' as TabKey, label: '设置', icon: Settings }
]

const tab = computed<TabKey>({
  get: () => appStore.currentTab as TabKey,
  set: (v) => appStore.setTab(v)
})

const handleMenuSelect = (key: string) => {
  tab.value = key as TabKey
}

const currentView = computed(() => views[tab.value])

onMounted(() => appStore.init())
</script>

<style scoped>
:deep(.app-content-scroll) {
  padding: 14px 14px 68px 14px;
}
@media (min-width: 640px) {
  :deep(.app-content-scroll) {
    padding: 24px;
  }
}
</style>