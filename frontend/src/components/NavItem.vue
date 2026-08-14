<template>
  <button :class="[
    'w-full flex items-center gap-3 px-3.5 py-2.5 rounded-xl text-sm font-medium transition-colors border',
    active
      ? 'bg-white text-fnos-blue shadow-sm font-semibold border-slate-100'
      : 'text-slate-600 hover:bg-slate-200/50 hover:text-slate-900 border-transparent'
  ]">
    <Icon :name="icon" :size="16" />
    <span>{{ label }}</span>
  </button>
</template>

<script setup lang="ts">
import Icon, { type IconName } from './Icon.vue'

defineProps<{ icon: IconName; label: string; active: boolean }>()
</script>
