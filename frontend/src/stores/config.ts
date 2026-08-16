import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { configApi } from '../api'
import type { SettingsConfig, RequestResult } from '../types/api'

export const useConfigStore = defineStore('config', () => {
  const config = ref<SettingsConfig>({
    server_port: 2298,
    proxy_port: 2299,
    network_proxy: '',
    reverse_proxy_url: '',
    access_password: ''
  })

  const savedConfig = ref<SettingsConfig | null>(null)
  const loading = ref(false)
  const loadError = ref(false)
  const saveError = ref(false)
  const lastErrorMessage = ref('')
  const configLoaded = ref(false)

  let saveTimer: ReturnType<typeof setTimeout> | null = null

  const isChanged = computed(() => {
    if (!savedConfig.value) return false
    return JSON.stringify(config.value) !== JSON.stringify(savedConfig.value)
  })

  async function fetchConfig(force = false): Promise<void> {
    if (configLoaded.value && !force) return
    loading.value = true
    try {
      const res = await configApi.getConfig()
      if (res.success && res.data) {
        config.value = { ...res.data }
        savedConfig.value = { ...res.data }
        loadError.value = false
        lastErrorMessage.value = ''
        configLoaded.value = true
      } else {
        loadError.value = true
        lastErrorMessage.value = res.message || '加载配置失败'
      }
    } finally {
      loading.value = false
    }
  }

  async function saveConfig(): Promise<RequestResult<SettingsConfig>> {
    const res = await configApi.saveConfig(config.value)
    if (res.success && res.data) {
      savedConfig.value = { ...config.value }
      saveError.value = false
      lastErrorMessage.value = ''
    } else {
      saveError.value = true
      lastErrorMessage.value = res.message || '保存配置失败'
    }
    return res
  }

  function triggerAutoSave(onSuccess?: () => void, onError?: (msg: string) => void) {
    if (loadError.value) return
    if (!isChanged.value) {
      saveError.value = false
      return
    }

    // 端口合法性基础校验：未输入完成时不触发保存
    const sPort = config.value.server_port
    const pPort = config.value.proxy_port
    if (!sPort || sPort < 1 || sPort > 65535 || !pPort || pPort < 1 || pPort > 65535) {
      return
    }

    if (saveTimer) clearTimeout(saveTimer)
    saveTimer = setTimeout(async () => {
      const res = await saveConfig()
      if (res.success) {
        onSuccess?.()
      } else {
        onError?.(res.message)
      }
    }, 800)
  }

  return {
    config,
    savedConfig,
    loading,
    loadError,
    saveError,
    lastErrorMessage,
    configLoaded,
    isChanged,
    fetchConfig,
    saveConfig,
    triggerAutoSave
  }
})
