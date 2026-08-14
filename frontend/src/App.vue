<template>
  <div class="flex h-screen overflow-hidden bg-fnos-bg">
    <aside class="w-56 shrink-0 bg-slate-50/80 border-r border-slate-200/60 p-4 flex flex-col justify-between">
      <nav class="space-y-1">
        <NavItem v-for="t in mainTabs" :key="t.key" :icon="t.icon" :label="t.label"
          :active="tab === t.key" @click="tab = t.key" />
      </nav>
      <div class="pt-4 border-t border-slate-200/60">
        <NavItem icon="settings" label="设置" :active="tab === 'settings'" @click="tab = 'settings'" />
      </div>
    </aside>

    <main class="flex-1 overflow-y-auto p-4 sm:p-6 lg:p-8">
      <component :is="currentView" />
    </main>

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
import { connectEvents } from './store'
import type { IconName } from './components/Icon.vue'

type TabKey = 'overview' | 'logs' | 'settings'

const views: Record<TabKey, Component> = { overview: Overview, logs: Logs, settings: Settings }

const mainTabs: { key: TabKey; label: string; icon: IconName }[] = [
  { key: 'overview', label: '概览', icon: 'grid' },
  { key: 'logs', label: '日志', icon: 'file' }
]

const tab = ref<TabKey>('overview')
const currentView = computed(() => views[tab.value])

onMounted(connectEvents)
</script>
