<template>
  <div class="space-y-6 max-w-4xl mx-auto">
    <h1 class="text-xl font-bold text-slate-800 tracking-tight">插件管理</h1>

    <!-- 安装插件 -->
    <section class="bg-white rounded-2xl p-6 shadow-sm border border-slate-100 space-y-5">
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-slate-100 pb-3">
        <h2 class="text-sm font-bold text-slate-800">安装插件</h2>
        <button @click="install" :disabled="!canInstall"
          class="px-5 py-2 bg-fnos-blue hover:bg-fnos-blue-hover disabled:bg-slate-300 disabled:cursor-not-allowed text-white font-medium rounded-xl text-sm transition-colors shadow-sm flex items-center gap-2 self-end sm:self-auto">
          <Icon v-if="busy" name="spinner" :size="14" />
          <span>{{ busy ? '正在执行…' : '安装' }}</span>
        </button>
      </div>

      <div class="flex items-center gap-6">
        <label class="flex items-center gap-2 text-sm font-medium text-slate-700 cursor-pointer">
          <input type="radio" value="cmd" v-model="mode" class="text-fnos-blue focus:ring-0" :disabled="busy">
          <span>命令</span>
        </label>
        <label class="flex items-center gap-2 text-sm font-medium text-slate-700 cursor-pointer">
          <input type="radio" value="upload" v-model="mode" class="text-fnos-blue focus:ring-0" :disabled="busy">
          <span>上传</span>
        </label>
      </div>

      <!-- 命令模式 -->
      <div v-if="mode === 'cmd'" class="space-y-3">
        <div class="space-y-1">
          <input type="text" v-model="command" :disabled="busy"
            placeholder="dsh plugin add 包名"
            class="w-full px-3.5 py-2 bg-slate-50 border border-slate-200 rounded-xl text-sm font-mono text-slate-800 focus:outline-none focus:border-fnos-blue focus:bg-white transition-colors disabled:opacity-50">
          <p class="text-[11px] text-slate-400">例：dsh plugin --profile web add github:user/hello</p>
        </div>

        <div v-if="command.trim()" :class="[
          'px-3.5 py-2 rounded-xl text-xs font-medium flex items-center gap-2 border',
          preview?.ok
            ? 'bg-emerald-50 text-emerald-600 border-emerald-100'
            : 'bg-rose-50 text-rose-600 border-rose-100'
        ]">
          <Icon :name="preview?.ok ? 'info' : 'warning'" :size="14" />
          <span class="truncate">{{ preview?.ok ? `将执行: ${preview.command}` : preview?.reason }}</span>
        </div>
      </div>

      <!-- 上传模式 -->
      <div v-else class="space-y-3">
        <div class="px-3.5 py-2.5 rounded-xl text-xs font-medium flex items-start gap-2 border bg-amber-50 text-amber-600 border-amber-100">
          <Icon name="warning" :size="14" class="mt-0.5 shrink-0" />
          <span>安装脚本将在本机执行，请仅安装可信来源的包。</span>
        </div>

        <div class="flex flex-col sm:flex-row sm:items-center gap-3">
          <input ref="fileInput" type="file" accept=".tgz,.zip" class="hidden" :disabled="busy" @change="onFileChange">
          <button @click="fileInput?.click()" :disabled="busy"
            class="px-4 py-2 bg-white border border-slate-200 rounded-xl text-sm font-medium text-slate-700 hover:bg-slate-50 transition-colors flex items-center gap-2 disabled:opacity-50 shrink-0">
            <Icon name="upload" :size="16" class="text-slate-500" />
            <span>选择文件</span>
          </button>
          <span class="text-sm text-slate-500 truncate">{{ file ? file.name : '支持 .tgz / .zip 压缩包（上限 64MB）' }}</span>
        </div>
      </div>
    </section>

    <!-- 已安装插件 -->
    <section class="bg-white rounded-2xl p-6 shadow-sm border border-slate-100 space-y-4">
      <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <h2 class="text-sm font-bold text-slate-800">已安装插件 ({{ plugins.length }})</h2>
        <button @click="refresh" :disabled="busy"
          class="px-3 py-1.5 bg-white border border-slate-200 rounded-lg text-xs font-medium text-slate-700 hover:bg-slate-50 transition-colors flex items-center gap-1.5 disabled:opacity-50">
          <Icon :name="busy ? 'spinner' : 'refresh'" :size="14" class="text-slate-500" />
          <span>刷新</span>
        </button>
      </div>

      <!-- 空态 -->
      <div v-if="!plugins.length" class="py-10 text-center">
        <div class="w-14 h-14 bg-slate-50 rounded-full flex items-center justify-center mx-auto mb-3">
          <Icon name="box" :size="26" class="text-slate-400" />
        </div>
        <p class="text-slate-500 text-sm">暂无已安装插件</p>
        <p class="text-slate-400 text-xs mt-1">在上方输入命令或上传 .tgz / .zip 进行安装</p>
      </div>

      <!-- 插件列表 -->
      <ul v-else class="divide-y divide-slate-100">
        <li v-for="p in plugins" :key="p.name" class="py-3.5 flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4">
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="text-sm font-semibold text-slate-800 truncate">{{ p.name }}</span>
              <span v-if="p.version" class="text-[11px] text-slate-400 shrink-0">v{{ p.version }}</span>
              <span v-if="!p.layer"
                class="text-[10px] font-medium text-amber-600 bg-amber-50 border border-amber-100 px-2 py-0.5 rounded-full shrink-0">
                未激活层
              </span>
            </div>
            <p v-if="p.spec" class="text-[11px] text-slate-400 mt-0.5 truncate">依赖声明: {{ p.spec }}</p>
          </div>
          <div class="flex items-center gap-2 shrink-0">
            <button v-if="p.layer" @click="toggle(p.name, false)" :disabled="busy"
              class="px-2.5 py-1 bg-white border border-slate-200 rounded-lg text-xs font-medium text-slate-700 hover:bg-amber-50 hover:text-amber-600 hover:border-amber-200 transition-colors flex items-center gap-1 disabled:opacity-50">
              <Icon name="stop" :size="12" />
              <span>禁用</span>
            </button>
            <button v-else @click="toggle(p.name, true)" :disabled="busy"
              class="px-2.5 py-1 bg-white border border-slate-200 rounded-lg text-xs font-medium text-slate-700 hover:bg-emerald-50 hover:text-emerald-600 hover:border-emerald-200 transition-colors flex items-center gap-1 disabled:opacity-50">
              <Icon name="play" :size="12" />
              <span>启用</span>
            </button>
            <button @click="quickOp('update', p.name)" :disabled="busy"
              class="px-2.5 py-1 bg-white border border-slate-200 rounded-lg text-xs font-medium text-slate-700 hover:bg-blue-50 hover:text-fnos-blue hover:border-blue-200 transition-colors flex items-center gap-1 disabled:opacity-50">
              <Icon name="refresh" :size="12" />
              <span>更新</span>
            </button>
            <button @click="uninstall(p.name)" :disabled="busy"
              class="px-2.5 py-1 bg-white border border-slate-200 rounded-lg text-xs font-medium text-slate-700 hover:bg-rose-50 hover:text-rose-600 hover:border-rose-200 transition-colors flex items-center gap-1 disabled:opacity-50">
              <Icon name="trash" :size="12" />
              <span>卸载</span>
            </button>
          </div>
        </li>
      </ul>

      <p v-if="builtin.length" class="text-[11px] text-slate-400 border-t border-slate-100 pt-3">
        内置层：{{ builtin.join(' · ') }}（随应用升级自动更新，无需管理）
      </p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useAppStore, type PluginStatus } from '../stores/app'
import { useToastStore } from '../stores/toast'
import { apiGet, apiPost, apiUpload } from '../api'
import Icon from '../components/Icon.vue'

const appStore = useAppStore()
const toastStore = useToastStore()

interface PluginPreview {
  ok: boolean
  command?: string
  reason?: string
}

interface PluginItem {
  name: string
  version: string
  spec: string
  layer: boolean
}

interface PluginList {
  profile: string
  plugins: PluginItem[]
  builtin: string[]
  bundles: string[]
}

const { pluginCmd: command, pluginMode: mode, pluginFile: file } = storeToRefs(appStore)
const preview = ref<PluginPreview | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const busy = computed(() => appStore.pluginBusy)

const plugins = ref<PluginItem[]>([])
const builtin = ref<string[]>([])

const canInstall = computed(() => {
  if (busy.value) return false
  if (mode.value === 'cmd') return command.value.trim() !== '' && preview.value?.ok === true
  return file.value !== null
})

// 命令预览：输入防抖 300ms 调用后端解析
let previewTimer: ReturnType<typeof setTimeout> | null = null
watch(command, (val) => {
  if (previewTimer) clearTimeout(previewTimer)
  if (!val.trim()) {
    preview.value = null
    return
  }
  previewTimer = setTimeout(async () => {
    const res = await apiPost<PluginPreview>('plugins/preview', { command: val })
    if (res?.ok) {
      preview.value = res.data
    } else {
      preview.value = { ok: false, reason: res?.message ?? '解析失败' }
    }
  }, 300)
})

const onFileChange = (e: Event) => {
  const input = e.target as HTMLInputElement
  file.value = input.files?.[0] ?? null
}

const refresh = async () => {
  const res = await apiGet<PluginList>('plugins')
  if (res?.ok && res.data) {
    plugins.value = res.data.plugins || []
    builtin.value = res.data.builtin || []
  }
}

// 插件操作完成后：刷新列表 + toast 结果（busy 由 store 随 WS 消息维护）
const onPluginEvent = (s: PluginStatus) => {
  if (s.running) return
  refresh()
  if (s.ok === false) {
    toastStore.showToast(s.message || '插件操作失败', 5000)
  } else if (s.message) {
    const msg = s.message.length > 300 ? '插件操作完成，详见运行日志' : s.message
    toastStore.showToast(msg, 4000)
  }
}

// 操作发起后主动同步一次后端状态，避免快操作的完成事件先于本响应到达
const startOp = async (msg: string) => {
  appStore.pluginBusy = true
  toastStore.showToast(msg)
  const res = await apiGet<PluginStatus>('plugins/status')
  if (res?.ok && res.data) onPluginEvent(res.data)
}

const install = async () => {
  if (!canInstall.value) return
  if (mode.value === 'cmd') {
    const res = await apiPost<{ command: string }>('plugins/run', { command: command.value })
    if (!res) {
      toastStore.showToast('网络连接失败')
    } else if (res.ok) {
      startOp('插件操作已开始，请查看运行日志')
    } else {
      toastStore.showToast(res.message)
    }
    return
  }
  if (!file.value) return
  const res = await apiUpload<{ name: string }>('plugins/upload', file.value)
  if (!res) {
    toastStore.showToast('网络连接失败')
  } else if (res.ok) {
    startOp(`插件 ${res.data.name} 安装已开始，请查看运行日志`)
    file.value = null
    if (fileInput.value) fileInput.value.value = ''
  } else {
    toastStore.showToast(res.message)
  }
}

const quickOp = async (verb: 'update', name: string) => {
  if (busy.value) return
  const res = await apiPost<{ command: string }>('plugins/run', { command: `dsh plugin ${verb} ${name}` })
  if (!res) {
    toastStore.showToast('网络连接失败')
  } else if (res.ok) {
    startOp('插件操作已开始，请查看运行日志')
  } else {
    toastStore.showToast(res.message)
  }
}

const uninstall = async (name: string) => {
  if (busy.value) return
  if (!confirm(`确定要卸载插件 ${name} 吗？`)) return
  const res = await apiPost<{ command: string }>('plugins/run', { command: `dsh plugin remove ${name}` })
  if (!res) {
    toastStore.showToast('网络连接失败')
  } else if (res.ok) {
    startOp('插件操作已开始，请查看运行日志')
  } else {
    toastStore.showToast(res.message)
  }
}

const toggle = async (name: string, enabled: boolean) => {
  if (busy.value) return
  const res = await apiPost<{ name: string; enabled: boolean }>('plugins/toggle', { name, enabled })
  if (!res) {
    toastStore.showToast('网络连接失败')
  } else if (res.ok) {
    startOp(enabled ? `正在启用 ${name}…` : `正在禁用 ${name}…`)
  } else {
    toastStore.showToast(res.message)
  }
}

let offPlugin: (() => void) | null = null

const syncPluginStatus = async () => {
  const res = await apiGet<PluginStatus>('plugins/status')
  if (res?.ok && res.data) appStore.pluginBusy = res.data.running
}

onMounted(() => {
  refresh()
  offPlugin = appStore.onPluginEvent(onPluginEvent)
  // 切回本页时同步操作状态：进行中则保持指示，已结束则复位
  syncPluginStatus()
})
onUnmounted(() => {
  offPlugin?.()
})
</script>
