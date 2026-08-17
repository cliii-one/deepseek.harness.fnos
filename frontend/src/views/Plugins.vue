<template>
  <div class="w-full flex-1 flex flex-col gap-4 sm:gap-6">
    <!-- 同行页头与操作区 -->
    <div class="flex items-center justify-between gap-3 w-full">
      <div class="flex items-baseline gap-2.5">
        <h1 class="text-xl font-bold text-slate-800 tracking-tight">插件管理</h1>
        <span v-if="plugins.length" class="text-xs text-slate-400 font-medium">
          已安装 {{ plugins.length }} 个
        </span>
      </div>

      <!-- 右侧同行操作区：插件变更重启提醒 -->
      <div v-auto-animate class="flex items-center gap-2">
        <n-popconfirm
          v-if="isRunning && needRestart"
          @positive-click="handleRestartService"
          positive-text="确认重启"
          negative-text="取消"
        >
          <template #trigger>
            <n-button
              type="warning"
              size="tiny"
              secondary
              :bordered="false"
              :loading="isRestarting"
              :disabled="isActionLocked && !isRestarting"
              class="!h-6 !px-2 rounded-md text-[11px] font-medium transition-transform duration-150 active:scale-95"
            >
              <span class="hidden sm:inline">插件已变更，等待重启</span>
              <span class="inline sm:hidden">等待重启</span>
            </n-button>
          </template>
          重启服务将短暂中断当前对话连接，确认立即重启吗？
        </n-popconfirm>
      </div>
    </div>

    <!-- 安装插件卡片 -->
    <n-card :bordered="false" class="shadow-sm">
      <template #header>
        <div class="flex items-center justify-between w-full">
          <span class="text-base font-bold text-slate-800">安装新插件</span>
          <n-button type="primary" :loading="busy" :disabled="!canInstall" @click="handleInstall" class="px-5">
            <template #icon v-if="!busy">
              <n-icon>
                <Plus />
              </n-icon>
            </template>
            <span>{{ busy ? '正在执行…' : '开始安装' }}</span>
          </n-button>
        </div>
      </template>

      <!-- Naive UI 原生 Tabs -->
      <n-tabs v-model:value="mode" type="segment" animated size="medium" :disabled="busy">
        <!-- 命令安装 Tab -->
        <n-tab-pane name="cmd" tab="命令安装">
          <div class="space-y-3 pt-3">
            <div class="space-y-1.5">
              <n-input :value="command" @update:value="pluginStore.setCommand" :disabled="busy"
                placeholder="例如: dsh plugin --profile web add dshmarket" clearable>
                <template #prefix>
                  <n-icon class="text-slate-400">
                    <Terminal2 />
                  </n-icon>
                </template>
              </n-input>
              <div class="flex items-center justify-between gap-2 text-[11px] text-slate-400 pl-1">
                <span class="truncate">支持: npm 包、@scoped 包、github:user/repo</span>
                <a
                  href="javascript:void(0)"
                  @click="openMarketplace"
                  class="text-fnos-blue hover:underline inline-flex items-center gap-0.5 shrink-0 select-none font-medium cursor-pointer"
                >
                  <span>浏览插件市场</span>
                  <n-icon :size="12">
                    <ExternalLink />
                  </n-icon>
                </a>
              </div>
            </div>

            <!-- 命令解析折叠动画 -->
            <div v-auto-animate>
              <n-alert v-if="command.trim()" :type="preview?.valid ? 'success' : 'error'" :show-icon="true" class="rounded-xl text-xs">
                {{ preview?.valid ? `将执行: ${preview.command}` : (preview?.reason || '解析中…') }}
              </n-alert>
            </div>
          </div>
        </n-tab-pane>

        <!-- 离线文件上传 Tab -->
        <n-tab-pane name="upload" tab="文件上传">
          <div class="space-y-3 pt-3">
            <n-alert type="warning" :show-icon="true" class="rounded-xl text-xs">
              安装脚本将在本机以宿主权限执行，请仅安装来自可信来源的插件包。
            </n-alert>

            <n-upload v-model:file-list="uploadFileList" :max="1" accept=".tgz,.zip" :show-file-list="true"
              :default-upload="false" @change="handleUploadChange" :disabled="busy">
              <n-upload-dragger class="!py-6 transition-all duration-200 hover:border-fnos-blue/60">
                <div class="flex flex-col items-center justify-center gap-2">
                  <div class="w-12 h-12 rounded-2xl bg-blue-50 text-fnos-blue flex items-center justify-center transition-transform duration-200 group-hover:scale-110">
                    <n-icon :size="26">
                      <Upload />
                    </n-icon>
                  </div>
                  <div class="text-sm font-medium text-slate-700">
                    点击或拖拽插件压缩包到此处
                  </div>
                  <div class="text-xs text-slate-400">
                    支持 .tgz / .zip 格式压缩包（上限 64MB）
                  </div>
                </div>
              </n-upload-dragger>
            </n-upload>
          </div>
        </n-tab-pane>
      </n-tabs>
    </n-card>

    <!-- 已安装插件列表卡片 -->
    <n-card :bordered="false" class="shadow-sm">
      <template #header>
        <div class="flex items-center justify-between w-full">
          <div class="flex items-center gap-2">
            <span class="text-base font-bold text-slate-800">已安装插件</span>
            <n-badge :value="plugins.length" type="info" />
          </div>
          <n-button secondary size="small" :loading="loading || busy" @click="handleRefresh" class="transition-transform duration-150 active:scale-95">
            <template #icon>
              <n-icon>
                <Refresh />
              </n-icon>
            </template>
            <span>刷新</span>
          </n-button>
        </div>
      </template>

      <!-- 列表与空状态自适应过渡容器 -->
      <div v-auto-animate>
        <!-- 空状态 -->
        <div v-if="!plugins.length" class="py-12 text-center">
          <n-empty :description="loading ? '正在获取插件列表…' : '暂无已安装插件'">
            <template #icon>
              <n-icon :size="48" class="text-slate-300">
                <Puzzle />
              </n-icon>
            </template>
            <template #extra>
              <span class="text-xs text-slate-400">在上方输入命令或上传压缩包进行安装</span>
            </template>
          </n-empty>
        </div>

        <!-- 插件列表 -->
        <n-list v-else hoverable class="divide-y divide-slate-100" v-auto-animate>
          <n-list-item v-for="p in plugins" :key="p.name" class="!py-3.5 transition-colors duration-150">
            <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 w-full min-w-0">
              <!-- 信息区：第一行与第二行（自适应收缩与截断保护） -->
              <div class="flex-1 min-w-0 space-y-1.5">
                <!-- 第一行：标题、版本、状态 -->
                <div class="flex items-center gap-2 w-full min-w-0">
                  <span class="text-sm font-bold text-slate-800 truncate min-w-0" :title="p.name">
                    {{ p.name }}
                  </span>
                  <n-tag v-if="p.version" size="tiny" round :bordered="false" class="shrink-0 font-mono">
                    v{{ p.version }}
                  </n-tag>
                  <n-tag :type="p.layer ? 'success' : 'default'" size="tiny" round :bordered="false" class="shrink-0">
                    {{ p.layer ? '运行中' : '已停用' }}
                  </n-tag>
                </div>

                <!-- 第二行：来源（单行超出截断与浮动提示） -->
                <div v-if="p.spec" class="text-xs text-slate-400 font-mono truncate w-full min-w-0 block" :title="p.spec">
                  {{ p.spec }}
                </div>
              </div>

              <!-- 第三行（移动端独立行 / 桌面端右侧对齐）：Switch 启停胶囊、更新按钮、卸载按钮 -->
              <div
                class="flex items-center justify-between sm:justify-end gap-2 w-full sm:w-auto shrink-0 pt-0.5 sm:pt-0">
                <!-- Switch 开关启停胶囊 -->
                <div
                  class="flex items-center gap-1.5 bg-slate-50 px-2.5 py-1 rounded-lg border border-slate-100 shrink-0 transition-colors duration-150 hover:bg-slate-100/70">
                  <span class="text-xs text-slate-500 select-none">{{ p.layer ? '已启用' : '已禁用' }}</span>
                  <n-switch size="small" :value="p.layer" :disabled="busy"
                    @update:value="(val) => handleToggle(p.name, val)" />
                </div>

                <!-- 操作按钮组 -->
                <div class="flex items-center gap-2 shrink-0">
                  <n-button size="small" secondary type="info" :disabled="busy" @click="handleUpdate(p.name)"
                    class="transition-transform duration-150 active:scale-95">
                    <template #icon>
                      <n-icon>
                        <Refresh />
                      </n-icon>
                    </template>
                    <span>更新</span>
                  </n-button>

                  <n-popconfirm @positive-click="handleUninstall(p.name)" positive-text="确认卸载" negative-text="取消">
                    <template #trigger>
                      <n-button size="small" secondary type="error" :disabled="busy"
                        class="transition-transform duration-150 active:scale-95">
                        <template #icon>
                          <n-icon>
                            <Trash />
                          </n-icon>
                        </template>
                        <span>卸载</span>
                      </n-button>
                    </template>
                    确定要卸载插件「{{ p.name }}」吗？
                  </n-popconfirm>
                </div>
              </div>
            </div>
          </n-list-item>
        </n-list>
      </div>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import {
  NCard,
  NTabs,
  NTabPane,
  NInput,
  NButton,
  NAlert,
  NBadge,
  NEmpty,
  NList,
  NListItem,
  NSwitch,
  NTag,
  NIcon,
  NUpload,
  NUploadDragger,
  NPopconfirm,
  useMessage,
  type UploadFileInfo
} from 'naive-ui'
import {
  Plus,
  Refresh,
  Trash,
  Upload,
  Terminal2,
  Puzzle,
  ExternalLink
} from '@vicons/tabler'
import { usePluginStore } from '../stores/plugin'
import { useSystemStore } from '../stores/system'
import { withAsyncLock } from '../utils/debounce'
import { trimSdk } from '../utils/trimSdk'

const pluginStore = usePluginStore()
const systemStore = useSystemStore()
const message = useMessage()

const openMarketplace = () => {
  trimSdk.openURL('https://awesome-dsh-plugin.com/zh', '_blank')
}

const {
  plugins,
  loading,
  pluginBusy: busy,
  needRestart,
  mode,
  command,
  file,
  preview,
  canInstall
} = storeToRefs(pluginStore)

const {
  isRunning,
  isActionLocked,
  activeAction
} = storeToRefs(systemStore)

const isRestarting = computed(() => activeAction.value === 'restart')

const handleRestartService = withAsyncLock(async () => {
  const res = await systemStore.sendAction('restart')
  if (res.success) {
    message.success(res.message || '重启指令已发送，正在等待服务就绪…')
  } else {
    message.error(res.message || '重启失败')
  }
})

const uploadFileList = ref<UploadFileInfo[]>([])

const handleUploadChange = (data: { fileList: UploadFileInfo[] }) => {
  uploadFileList.value = data.fileList
  const latest = data.fileList[data.fileList.length - 1]
  file.value = latest?.file ?? null
}

const handleRefresh = withAsyncLock(async () => {
  await pluginStore.fetchPlugins()
  message.success('插件列表已刷新')
})

const handleInstall = withAsyncLock(async () => {
  const res = await pluginStore.installPlugin()
  if (res.success) {
    message.success(res.message || '已开始执行插件安装')
    uploadFileList.value = []
  } else {
    message.error(res.message || '安装失败')
  }
})

const handleToggle = withAsyncLock(async (name: string, enabled: boolean) => {
  const res = await pluginStore.togglePlugin(name, enabled)
  if (res.success) {
    message.success(res.message || (enabled ? '已启用插件' : '已禁用插件'))
  } else {
    message.error(res.message || '操作失败')
  }
})

const handleUpdate = withAsyncLock(async (name: string) => {
  const res = await pluginStore.updatePlugin(name)
  if (res.success) {
    message.success(res.message || `已开始更新 ${name}`)
  } else {
    message.error(res.message || '更新失败')
  }
})

const handleUninstall = withAsyncLock(async (name: string) => {
  const res = await pluginStore.uninstallPlugin(name)
  if (res.success) {
    message.success(res.message || `已开始卸载 ${name}`)
  } else {
    message.error(res.message || '卸载失败')
  }
})

onMounted(() => {
  if (!pluginStore.plugins.length) {
    pluginStore.fetchPlugins()
  }
})
</script>

<style scoped>
:deep(.n-list-item__main) {
  min-width: 0;
  width: 100%;
}
</style>
