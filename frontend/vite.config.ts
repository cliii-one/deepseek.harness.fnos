import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  base: './',
  server: {
    port: 3000,
    proxy: {
      '/app/deepseek-harness/api': {
        target: 'http://localhost:20378',
        changeOrigin: true
      },
      '/api': {
        target: 'http://localhost:20378',
        changeOrigin: true
      }
    }
  }
})
