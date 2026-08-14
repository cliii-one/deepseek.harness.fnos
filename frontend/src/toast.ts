import { ref } from 'vue'

export type ToastType = 'info' | 'warning' | 'loading'

export interface Toast {
  id: number
  message: string
  type: ToastType
}

export const toasts = ref<Toast[]>([])

let nextId = 0

export function showToast(message: string, type: ToastType = 'info', duration = 4000) {
  const id = nextId++
  toasts.value.push({ id, message, type })
  if (duration > 0) {
    setTimeout(() => {
      toasts.value = toasts.value.filter(t => t.id !== id)
    }, duration)
  }
  return id
}

export function dismissToast(id: number) {
  toasts.value = toasts.value.filter(t => t.id !== id)
}
