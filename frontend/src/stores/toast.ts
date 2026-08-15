import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface Toast {
  id: number
  message: string
}

let nextId = 0

export const useToastStore = defineStore('toast', () => {
  const toasts = ref<Toast[]>([])

  function showToast(message: string, duration = 3000) {
    const id = nextId++
    toasts.value.push({ id, message })
    if (duration > 0) {
      setTimeout(() => {
        toasts.value = toasts.value.filter(t => t.id !== id)
      }, duration)
    }
    return id
  }

  function dismissToast(id: number) {
    toasts.value = toasts.value.filter(t => t.id !== id)
  }

  return { toasts, showToast, dismissToast }
})