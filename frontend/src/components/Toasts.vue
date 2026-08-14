<template>
  <Teleport to="body">
    <div class="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 flex flex-col-reverse items-center gap-2 pointer-events-none">
      <div v-for="t in toasts" :key="t.id"
        class="inline-flex items-center gap-2.5 px-4 py-2.5 bg-white text-xs font-semibold rounded-full shadow-xl shadow-slate-200/80 border border-slate-200/80 max-w-sm">
        <Icon :name="meta[t.type].icon" :size="16" :class="meta[t.type].cls" />
        <span class="truncate text-slate-700">{{ t.message }}</span>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { toasts, type ToastType } from '../toast'
import Icon, { type IconName } from './Icon.vue'

const meta: Record<ToastType, { icon: IconName; cls: string }> = {
  info: { icon: 'info', cls: 'text-fnos-blue' },
  warning: { icon: 'warning', cls: 'text-amber-500' },
  loading: { icon: 'spinner', cls: 'text-fnos-blue' }
}
</script>
