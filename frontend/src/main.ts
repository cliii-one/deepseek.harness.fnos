import { createApp } from 'vue'
import { createPinia } from 'pinia'
import './style.css'
import App from './App.vue'

const app = createApp(App)

// 注册全局按钮/点击防抖指令 v-debounce（默认 500ms 冷却时间）
app.directive('debounce', {
  mounted(el: HTMLElement, binding) {
    const delay = typeof binding.value === 'number' ? binding.value : 500
    let timer: number | null = null
    el.addEventListener(
      'click',
      (e: Event) => {
        if (timer) {
          e.stopImmediatePropagation()
          e.preventDefault()
          return
        }
        timer = window.setTimeout(() => {
          timer = null
        }, delay)
      },
      true
    )
  }
})

app.use(createPinia()).mount('#app')