<template>
  <div class="w-full flex-1 flex flex-col gap-4 sm:gap-6">
    <!-- 同行页头与操作区 -->
    <div class="flex items-center justify-between gap-3 w-full">
      <h1 class="text-xl font-bold text-slate-800 tracking-tight">应用设置</h1>

      <!-- 右侧同行操作按钮组 -->
      <div class="flex items-center gap-2">
        <n-button
          v-if="isChanged"
          size="medium"
          :disabled="saving"
          @click="handleReset"
        >
          <template #icon>
            <n-icon>
              <X />
            </n-icon>
          </template>
          取消
        </n-button>

        <n-button
          type="primary"
          size="medium"
          :loading="saving"
          :disabled="!isChanged || loadError || !configLoaded"
          @click="handleSave"
          class="px-5 shadow-sm shadow-fnos-blue/20"
        >
          <template #icon v-if="!saving">
            <n-icon>
              <DeviceFloppy />
            </n-icon>
          </template>
          保存
        </n-button>
      </div>
    </div>

    <!-- 加载失败轻量状态 -->
    <template v-if="loadError && !configLoaded">
      <n-card :bordered="false" class="shadow-sm py-12 text-center">
        <n-empty description="配置加载失败">
          <template #extra>
            <n-button size="small" secondary @click="configStore.fetchConfig(true)">
              重新加载
            </n-button>
          </template>
        </n-empty>
      </n-card>
    </template>

    <!-- 配置表单内容 -->
    <template v-else>
      <n-form ref="formRef" :model="config" :rules="rules" label-placement="top" size="medium">
        <div class="flex flex-col gap-4 sm:gap-6">
          <!-- 核心服务网络与端口配置卡片 -->
          <n-card title="核心服务" :bordered="false" class="shadow-sm">
            <n-grid :cols="2" :x-gap="20" :y-gap="16" responsive="screen" item-responsive>
              <n-gi span="2 m:1">
                <n-form-item label="内部监听端口" path="server_port">
                  <template #label>
                    <div class="flex items-center gap-1.5">
                      <span>内部监听端口</span>
                      <n-tooltip trigger="hover">
                        <template #trigger>
                          <n-icon size="14" class="text-slate-400 cursor-help">
                            <Help />
                          </n-icon>
                        </template>
                        DeepSeek Harness 本地后端进程监听端口，默认 2298
                      </n-tooltip>
                    </div>
                  </template>
                  <n-input-number
                    v-model:value="config.server_port"
                    :min="1"
                    :max="65535"
                    placeholder="2298"
                    class="w-full"
                  />
                </n-form-item>
              </n-gi>

              <n-gi span="2 m:1">
                <n-form-item label="反向代理端口" path="proxy_port">
                  <template #label>
                    <div class="flex items-center gap-1.5">
                      <span>反向代理端口</span>
                      <n-tooltip trigger="hover">
                        <template #trigger>
                          <n-icon size="14" class="text-slate-400 cursor-help">
                            <Help />
                          </n-icon>
                        </template>
                        对外暴露的代理访问端口 (默认 2299)，用于 Web 客户端直连
                      </n-tooltip>
                    </div>
                  </template>
                  <n-input-number
                    v-model:value="config.proxy_port"
                    :min="1"
                    :max="65535"
                    placeholder="2299"
                    class="w-full"
                  />
                </n-form-item>
              </n-gi>

              <n-gi span="2 m:1">
                <n-form-item label="外部访问地址" path="reverse_proxy_url">
                  <template #label>
                    <div class="flex items-center gap-1.5">
                      <span>外部访问地址</span>
                      <n-tooltip trigger="hover">
                        <template #trigger>
                          <n-icon size="14" class="text-slate-400 cursor-help">
                            <Help />
                          </n-icon>
                        </template>
                        点击概览页「进入 Harness」时跳转的绝对 URL（如 https://dsh.nas.com）
                      </n-tooltip>
                    </div>
                  </template>
                  <n-input
                    v-model:value="config.reverse_proxy_url"
                    placeholder="例如 https://dsh.example.com:2299"
                    clearable
                  />
                </n-form-item>
              </n-gi>

              <n-gi span="2 m:1">
                <n-form-item label="访问控制密码" path="access_password">
                  <template #label>
                    <div class="flex items-center gap-1.5">
                      <span>访问控制密码</span>
                      <n-tooltip trigger="hover">
                        <template #trigger>
                          <n-icon size="14" class="text-slate-400 cursor-help">
                            <Help />
                          </n-icon>
                        </template>
                        反向代理端口的访问密码，留空则不开启访问校验
                      </n-tooltip>
                    </div>
                  </template>
                  <n-input
                    type="password"
                    show-password-on="click"
                    v-model:value="config.access_password"
                    placeholder="留空则不启用密码保护"
                    autocomplete="new-password"
                  />
                </n-form-item>
              </n-gi>
            </n-grid>
          </n-card>

          <!-- 外网代理卡片 -->
          <n-card title="网络代理" :bordered="false" class="shadow-sm">
            <n-form-item label="网络代理地址" path="network_proxy">
              <template #label>
                <div class="flex items-center gap-1.5">
                  <span>网络代理地址 (HTTP / SOCKS5)</span>
                  <n-tooltip trigger="hover">
                    <template #trigger>
                      <n-icon size="14" class="text-slate-400 cursor-help">
                        <Help />
                      </n-icon>
                    </template>
                    用于 Git Clone 拉取仓库，留空使用系统直连
                  </n-tooltip>
                </div>
              </template>
              <n-input
                v-model:value="config.network_proxy"
                placeholder="例如 http://192.168.1.100:7890 或 socks5://192.168.1.100:7890"
                clearable
              />
            </n-form-item>
          </n-card>
        </div>
      </n-form>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import {
  NCard,
  NForm,
  NFormItem,
  NGrid,
  NGi,
  NInput,
  NInputNumber,
  NEmpty,
  NTooltip,
  NIcon,
  NButton,
  useMessage,
  useDialog,
  type FormInst,
  type FormRules
} from 'naive-ui'
import { Help, DeviceFloppy, X } from '@vicons/tabler'
import { useConfigStore } from '../stores/config'
import { useSystemStore } from '../stores/system'

const configStore = useConfigStore()
const systemStore = useSystemStore()
const message = useMessage()
const dialog = useDialog()

const formRef = ref<FormInst | null>(null)

const {
  config,
  savedConfig,
  saving,
  loadError,
  lastErrorMessage,
  configLoaded,
  isChanged,
  isServerPortChanged
} = storeToRefs(configStore)

// 表单输入校验规则
const rules: FormRules = {
  server_port: [
    { required: true, type: 'number', message: '请输入内部监听端口', trigger: ['input', 'blur'] },
    {
      validator(_rule, value: number) {
        if (!value || value < 1 || value > 65535) {
          return new Error('端口范围必须在 1 ~ 65535 之间')
        }
        if (value === config.value.proxy_port) {
          return new Error('内部监听端口不能与反向代理端口相同')
        }
        return true
      },
      trigger: ['input', 'blur']
    }
  ],
  proxy_port: [
    { required: true, type: 'number', message: '请输入反向代理端口', trigger: ['input', 'blur'] },
    {
      validator(_rule, value: number) {
        if (!value || value < 1 || value > 65535) {
          return new Error('端口范围必须在 1 ~ 65535 之间')
        }
        if (value === config.value.server_port) {
          return new Error('反向代理端口不能与内部监听端口相同')
        }
        return true
      },
      trigger: ['input', 'blur']
    }
  ],
  network_proxy: [
    {
      validator(_rule, value: string) {
        if (!value || !value.trim()) return true
        const trimmed = value.trim()
        if (!/^(http|https|socks5|socks5h):\/\//i.test(trimmed)) {
          return new Error('代理地址需以 http://、https:// 或 socks5:// 开头')
        }
        return true
      },
      trigger: ['input', 'blur']
    }
  ],
  reverse_proxy_url: [
    {
      validator(_rule, value: string) {
        if (!value || !value.trim()) return true
        const trimmed = value.trim()
        if (!/^(http|https):\/\//i.test(trimmed)) {
          return new Error('外部访问地址需以 http:// 或 https:// 开头')
        }
        return true
      },
      trigger: ['input', 'blur']
    }
  ]
}

onMounted(async () => {
  if (!configLoaded.value) {
    await configStore.fetchConfig()
    if (loadError.value) {
      message.error('加载配置失败')
    }
  }
})

// 执行实际保存
async function executeSave() {
  const res = await configStore.saveConfig()
  if (res.success) {
    message.success('设置保存成功')
  } else {
    message.error(res.message || '保存设置失败')
  }
}

// 提交保存（含表单验证与重启二次确认）
async function handleSave() {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
  } catch {
    message.error('请检查表单填写是否正确')
    return
  }

  // 内部监听端口变更且服务处于运行中时二次确认
  if (isServerPortChanged.value && systemStore.isRunning) {
    dialog.warning({
      title: '确认保存并重启核心服务？',
      content: `检测到内部监听端口已由 ${savedConfig.value?.server_port || 2298} 变更为 ${config.value.server_port}。保存设置后，系统将自动重启 DeepSeek Harness 后端进程以应用新端口。当前所有正在执行的任务可能会短暂中断，是否确认继续？`,
      positiveText: '保存并重启',
      negativeText: '取消',
      onPositiveClick: async () => {
        await executeSave()
      }
    })
    return
  }

  await executeSave()
}

// 取消修改
function handleReset() {
  configStore.resetConfig()
  formRef.value?.restoreValidation()
  message.info('已取消修改')
}
</script>