import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { pluginApi } from '../api'
import type { PluginItem, PluginStatus, PreviewResult, RequestResult } from '../types/api'

export const usePluginStore = defineStore('plugin', () => {
  const plugins = ref<PluginItem[]>([])
  const loading = ref(false)
  const pluginBusy = ref(false)

  const mode = ref<'cmd' | 'upload'>('cmd')
  const command = ref('')
  const file = ref<File | null>(null)
  const preview = ref<PreviewResult | null>(null)

  let previewTimer: ReturnType<typeof setTimeout> | null = null

  function setCommand(cmd: string) {
    command.value = cmd
    if (previewTimer) clearTimeout(previewTimer)
    const trimmed = cmd.trim()
    if (!trimmed) {
      preview.value = null
      return
    }
    previewTimer = setTimeout(async () => {
      const res = await pluginApi.preview(trimmed)
      if (res.success && res.data) {
        preview.value = {
          valid: res.data.valid ?? res.data.ok ?? false,
          command: res.data.command,
          reason: res.data.reason,
          verb: res.data.verb,
          profile: res.data.profile,
          specs: res.data.specs
        }
      } else {
        preview.value = {
          valid: false,
          reason: res.message || '命令解析失败'
        }
      }
    }, 200)
  }

  const canInstall = computed(() => {
    if (pluginBusy.value) return false
    if (mode.value === 'cmd') {
      return Boolean(command.value.trim() && preview.value?.valid)
    }
    return Boolean(file.value)
  })

  async function fetchPlugins(): Promise<void> {
    loading.value = true
    try {
      const res = await pluginApi.getList()
      if (res.success && res.data && Array.isArray(res.data.plugins)) {
        plugins.value = res.data.plugins
      }
    } finally {
      loading.value = false
    }
  }

  function updatePluginStatus(s: PluginStatus) {
    const wasBusy = pluginBusy.value
    pluginBusy.value = s.running
    if (wasBusy && !s.running) {
      fetchPlugins()
    }
  }

  async function installPlugin(): Promise<RequestResult<unknown>> {
    if (!canInstall.value) {
      return { success: false, message: '请先输入有效命令或选择上传文件' }
    }
    if (mode.value === 'cmd') {
      const res = await pluginApi.run(command.value.trim())
      if (res.success) {
        command.value = ''
        preview.value = null
      }
      return res
    } else {
      if (!file.value) {
        return { success: false, message: '请选择插件压缩包' }
      }
      const res = await pluginApi.upload(file.value)
      if (res.success) {
        file.value = null
      }
      return res
    }
  }

  async function togglePlugin(name: string, enable: boolean): Promise<RequestResult<unknown>> {
    const res = await pluginApi.toggle(name, enable)
    if (res.success) {
      await fetchPlugins()
    }
    return res
  }

  async function updatePlugin(name: string): Promise<RequestResult<unknown>> {
    return pluginApi.run(`dsh plugin --profile web update ${name}`)
  }

  async function uninstallPlugin(name: string): Promise<RequestResult<unknown>> {
    return pluginApi.run(`dsh plugin --profile web remove ${name}`)
  }

  return {
    plugins,
    loading,
    pluginBusy,
    mode,
    command,
    file,
    preview,
    canInstall,
    setCommand,
    fetchPlugins,
    updatePluginStatus,
    installPlugin,
    togglePlugin,
    updatePlugin,
    uninstallPlugin
  }
})
