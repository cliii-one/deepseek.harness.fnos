<template>
  <Teleport to="body">
    <div class="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 flex flex-col-reverse items-center gap-2 pointer-events-none">
      <div v-for="t in toasts" :key="t.id"
        class="px-4 py-2 bg-slate-800/90 text-white text-xs rounded shadow-md max-w-sm">
        {{ t.message }}
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { toasts } from '../toast'
</script>
