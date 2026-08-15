<template>
  <div class="space-y-6 max-w-3xl mx-auto">
    <div class="flex items-center justify-between">
      <h1 class="text-xl font-bold text-slate-800 tracking-tight">应用设置</h1>
      <div class="flex items-center gap-2">
        <span v-if="loadError"
          class="text-xs font-medium text-rose-600 bg-rose-50 px-3 py-1 rounded-full border border-rose-100">
          配置加载失败，已禁用保存
        </span>
        <span v-if="saveError"
          class="text-xs font-medium text-rose-600 bg-rose-50 px-3 py-1 rounded-full border border-rose-100">
          保存失败
        </span>
      </div>
    </div>

    <template v-if="configLoaded">
      <!-- 服务配置 -->
      <section class="bg-white rounded-2xl p-6 shadow-sm border border-slate-100 space-y-5">
        <h2 class="text-sm font-bold text-slate-800 border-b border-slate-100 pb-3">服务配置</h2>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div v-for="f in portFields" :key="f.key" class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700">{{ f.label }}</label>
            <input type="number" v-model.number="config[f.key]" :placeholder="f.placeholder" :class="inputCls">
            <p class="text-[11px] text-slate-400">{{ f.hint }}</p>
          </div>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700">反向代理地址</label>
            <input type="url" v-model="config.reverse_proxy_url" placeholder="例如 https://dsh.example.com:2299"
              :class="inputCls">
            <p class="text-[11px] text-slate-400">概览页「打开」按钮跳转地址。</p>
          </div>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700">访问密码</label>
            <input type="password" v-model="config.access_password" placeholder="留空则不启用" :class="inputCls"
              autocomplete="new-password">
            <p class="text-[11px] text-slate-400">反向代理端口访问密码。</p>
          </div>
        </div>
      </section>

      <!-- 网络代理 -->
      <section class="bg-white rounded-2xl p-6 shadow-sm border border-slate-100 space-y-5">
        <h2 class="text-sm font-bold text-slate-800 border-b border-slate-100 pb-3">代理配置</h2>
        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-700">网络代理 (HTTP / SOCKS5)</label>
          <input type="text" v-model="config.network_proxy"
            placeholder="例如 http://192.168.1.100:7890 或 socks5://192.168.1.100:7890" :class="inputCls">
          <p class="text-[11px] text-slate-400">用于拉取 Git 代码与 npm 下载，留空使用直连。</p>
        </div>
      </section>

    </template>

    <!-- 加载占位 -->
    <template v-else>
      <section class="bg-white rounded-2xl p-6 shadow-sm border border-slate-100 space-y-5 animate-pulse">
        <div class="h-5 w-32 bg-slate-200 rounded"></div>
        <div class="space-y-4 pt-3">
          <div v-for="i in 2" :key="i" class="space-y-1">
            <div class="h-3 w-28 bg-slate-200 rounded"></div>
            <div class="h-9 bg-slate-100 rounded-xl"></div>
          </div>
        </div>
      </section>

      <section class="bg-white rounded-2xl p-6 shadow-sm border border-slate-100 space-y-5 animate-pulse">
        <div class="h-5 w-24 bg-slate-200 rounded"></div>
        <div class="space-y-1 pt-3">
          <div class="h-3 w-36 bg-slate-200 rounded"></div>
          <div class="h-9 bg-slate-100 rounded-xl"></div>
          <div class="h-2.5 w-48 bg-slate-100 rounded"></div>
        </div>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { apiGet, apiPost } from '../api'
import { useAppStore, type SettingsConfig } from '../stores/app'

const appStore = useAppStore()

const config = ref<SettingsConfig>({
  server_port: 3080,
  proxy_port: 2299,
  network_proxy: '',
  reverse_proxy_url: '',
  access_password: ''
})

const inputCls = 'w-full px-3.5 py-2 bg-slate-50 border border-slate-200 rounded-xl text-sm text-slate-800 focus:outline-none focus:border-fnos-blue focus:bg-white transition-colors'

const portFields: { key: 'server_port' | 'proxy_port'; label: string; placeholder: string; hint: string }[] = [
  { key: 'server_port', label: '服务监听端口', placeholder: '3080', hint: 'DeepSeek Harness 内部监听端口 (默认 3080)' },
  { key: 'proxy_port', label: '反向代理端口', placeholder: '2299', hint: '处理后的对外访问端口 (默认 2299)，可用于反向代理。' }
]

const saveError = ref(false)
const loadError = ref(false)
const configLoaded = ref(false)
let skipWatch = true
let savedConfig: SettingsConfig | null = null

onMounted(async () => {
  if (appStore.settingsConfig) {
    config.value = { ...appStore.settingsConfig }
    savedConfig = appStore.savedSettingsConfig ? { ...appStore.savedSettingsConfig } : { ...config.value }
  } else {
    const res = await apiGet<Partial<SettingsConfig>>('config')
    if (res?.ok && res.data) {
      config.value = { ...config.value, ...res.data }
      savedConfig = { ...config.value }
      appStore.settingsConfig = { ...config.value }
      appStore.savedSettingsConfig = { ...savedConfig }
    } else {
      loadError.value = true
    }
  }
  skipWatch = false
  configLoaded.value = true
})

function configsEqual(a: SettingsConfig, b: SettingsConfig): boolean {
  return (
    a.server_port === b.server_port &&
    a.proxy_port === b.proxy_port &&
    a.network_proxy === b.network_proxy &&
    a.reverse_proxy_url === b.reverse_proxy_url &&
    a.access_password === b.access_password
  )
}

let saveTimer: ReturnType<typeof setTimeout> | null = null
watch(config, (cfg) => {
  appStore.settingsConfig = { ...cfg }
  if (skipWatch || loadError.value) return
  if (savedConfig && configsEqual(cfg, savedConfig)) return
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(async () => {
    const res = await apiPost<SettingsConfig>('config', cfg)
    saveError.value = !(res?.ok ?? false)
    if (res?.ok && res.data) {
      savedConfig = { ...res.data }
      appStore.settingsConfig = { ...res.data }
      appStore.savedSettingsConfig = { ...res.data }
    }
  }, 400)
}, { deep: true })
</script>