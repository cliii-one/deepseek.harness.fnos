<template>
  <div class="w-full flex-1 flex flex-col gap-4 sm:gap-6">
    <!-- 桌面端页头 -->
    <n-page-header class="hidden sm:block">
      <template #title>
        <div class="text-xl font-bold text-slate-800 tracking-tight">应用设置</div>
      </template>
      <template #extra>
        <n-space :size="8">
          <n-tag v-if="loadError" type="error" size="small" round :bordered="false">
            配置加载失败，已禁用保存
          </n-tag>
          <n-tooltip v-if="saveError" trigger="hover">
            <template #trigger>
              <n-tag type="error" size="small" round :bordered="false" class="cursor-help">
                保存失败
              </n-tag>
            </template>
            {{ lastErrorMessage || '保存失败，请检查端口是否被占用或格式是否有误' }}
          </n-tooltip>
        </n-space>
      </template>
    </n-page-header>

    <!-- 移动端错误提示栏 -->
    <div v-if="loadError || saveError" class="sm:hidden">
      <n-alert type="error" :show-icon="true" class="rounded-xl">
        {{ loadError ? '配置加载失败，已禁用自动保存' : (lastErrorMessage || '配置保存失败，请检查设置') }}
      </n-alert>
    </div>

    <template v-if="configLoaded">
      <!-- 服务网络端口配置卡片 -->
      <n-card title="核心服务" :bordered="false" class="shadow-sm">
        <n-form :model="config" label-placement="top" size="medium">
          <n-grid :cols="2" :x-gap="20" :y-gap="16" responsive="screen" item-responsive>
            <n-gi span="2 m:1">
              <n-form-item>
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
                <n-input-number v-model:value="config.server_port" :min="1" :max="65535" :update-value-on-input="false"
                  placeholder="2298" class="w-full" />
              </n-form-item>
            </n-gi>

            <n-gi span="2 m:1">
              <n-form-item>
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
                <n-input-number v-model:value="config.proxy_port" :min="1" :max="65535" :update-value-on-input="false"
                  placeholder="2299" class="w-full" />
              </n-form-item>
            </n-gi>

            <n-gi span="2 m:1">
              <n-form-item>
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
                <n-input v-model:value="config.reverse_proxy_url" :update-value-on-input="false"
                  placeholder="例如 https://dsh.example.com:2299" clearable />
              </n-form-item>
            </n-gi>

            <n-gi span="2 m:1">
              <n-form-item>
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
                <n-input type="password" show-password-on="click" v-model:value="config.access_password"
                  :update-value-on-input="false" placeholder="留空则不启用密码保护" autocomplete="new-password" />
              </n-form-item>
            </n-gi>
          </n-grid>
        </n-form>
      </n-card>

      <!-- 外网代理卡片 -->
      <n-card title="网络代理" :bordered="false" class="shadow-sm">
        <n-form :model="config" label-placement="top" size="medium">
          <n-form-item>
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
            <n-input v-model:value="config.network_proxy" :update-value-on-input="false"
              placeholder="例如 http://192.168.1.100:7890 或 socks5://192.168.1.100:7890" clearable />
          </n-form-item>
        </n-form>
      </n-card>
    </template>

    <!-- 骨架占位屏 -->
    <template v-else>
      <n-card :bordered="false" class="space-y-4 shadow-sm">
        <n-skeleton text style="width: 25%" />
        <n-skeleton text :repeat="4" />
      </n-card>
      <n-card :bordered="false" class="space-y-4 shadow-sm">
        <n-skeleton text style="width: 20%" />
        <n-skeleton text :repeat="2" />
      </n-card>
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted, watch } from 'vue'
import { storeToRefs } from 'pinia'
import {
  NPageHeader,
  NCard,
  NForm,
  NFormItem,
  NGrid,
  NGi,
  NInput,
  NInputNumber,
  NTag,
  NSpace,
  NAlert,
  NTooltip,
  NSkeleton,
  NIcon,
  useMessage
} from 'naive-ui'
import { Help } from '@vicons/tabler'
import { useConfigStore } from '../stores/config'

const configStore = useConfigStore()
const message = useMessage()

const { config, loadError, saveError, lastErrorMessage, configLoaded } = storeToRefs(configStore)

onMounted(async () => {
  await configStore.fetchConfig()
  if (loadError.value) {
    message.error('加载配置失败')
  }
})

watch(
  config,
  () => {
    configStore.triggerAutoSave(
      () => {
        message.success('设置已自动保存')
      },
      (errMsg) => {
        message.error(errMsg || '保存失败')
      }
    )
  },
  { deep: true }
)
</script>