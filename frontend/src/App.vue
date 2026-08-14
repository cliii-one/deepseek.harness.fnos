<template>
  <div class="flex flex-col sm:flex-row h-screen overflow-hidden bg-fnos-bg">
    <aside class="hidden sm:flex w-56 shrink-0 bg-slate-50/80 border-r border-slate-200/60 p-4 flex-col justify-between">
      <nav class="space-y-1">
        <NavItem v-for="t in mainTabs" :key="t.key" :icon="t.icon" :label="t.label"
          :active="tab === t.key" @click="tab = t.key" />
      </nav>
      <div class="pt-4 border-t border-slate-200/60">
        <NavItem icon="settings" label="设置" :active="tab === 'settings'" @click="tab = 'settings'" />
      </div>
    </aside>

    <main class="flex-1 overflow-y-auto p-4 sm:p-6 lg:p-8 pb-20 sm:pb-8">
      <component :is="currentView" />
    </main>

    <nav class="sm:hidden fixed bottom-0 left-0 right-0 bg-white/95 backdrop-blur-sm border-t border-slate-200/60 px-2 pt-1.5 pb-3 flex justify-around items-center z-50">
      <button v-for="t in mobileTabs" :key="t.key" @click="tab = t.key"
        :class="[
          'flex flex-col items-center gap-0.5 px-2 py-1.5 rounded-xl text-[11px] font-medium transition-colors flex-1',
          tab === t.key
            ? 'text-fnos-blue'
            : 'text-slate-500 hover:text-slate-700'
        ]">
        <Icon :name="t.icon" :size="22" />
        <span>{{ t.label }}</span>
      </button>
    </nav>

    <Toasts />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, type Component } from 'vue'
import Overview from './views/Overview.vue'
import Logs from './views/Logs.vue'
import Settings from './views/Settings.vue'
import NavItem from './components/NavItem.vue'
import Toasts from './components/Toasts.vue'
import Icon from './components/Icon.vue'
import { connectWS } from './store'
import type { IconName } from './components/Icon.vue'

type TabKey = 'overview' | 'logs' | 'settings'

const views: Record<TabKey, Component> = { overview: Overview, logs: Logs, settings: Settings }

const mainTabs: { key: TabKey; label: string; icon: IconName }[] = [
  { key: 'overview', label: '概览', icon: 'grid' },
  { key: 'logs', label: '日志', icon: 'file' }
]

const mobileTabs: { key: TabKey; label: string; icon: IconName }[] = [
  { key: 'overview', label: '概览', icon: 'grid' },
  { key: 'logs', label: '日志', icon: 'file' },
  { key: 'settings', label: '设置', icon: 'settings' }
]

const tab = ref<TabKey>('overview')
const currentView = computed(() => views[tab.value])

onMounted(connectWS)
</script>