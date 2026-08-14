import { ref } from 'vue'

export interface Toast {
  id: number
  message: string
}

export const toasts = ref<Toast[]>([])

let nextId = 0

/** 简单文字提示，默认 3 秒后自动消失 */
export function showToast(message: string, duration = 3000) {
  const id = nextId++
  toasts.value.push({ id, message })
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
