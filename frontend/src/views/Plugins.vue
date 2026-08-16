<template>
  <div class="w-full flex-1 flex flex-col gap-4 sm:gap-6">
    <!-- 桌面端页头 -->
    <n-page-header class="hidden sm:block" :subtitle="plugins.length ? `已安装 ${plugins.length} 个插件` : ''">
      <template #title>
        <div class="text-xl font-bold text-slate-800 tracking-tight">插件管理</div>
      </template>
    </n-page-header>

    <!-- 安装插件卡片 -->
    <n-card :bordered="false" class="shadow-sm">
      <template #header>
        <div class="flex items-center justify-between w-full">
          <span class="text-base font-bold text-slate-800">安装新插件</span>
          <n-button
            type="primary"
            v-debounce
            :loading="busy"
            :disabled="!canInstall"
            @click="handleInstall"
            class="px-5"
          >
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
              <n-input
                :value="command"
                @update:value="pluginStore.setCommand"
                :disabled="busy"
                placeholder="例如: dsh plugin --profile web add dshmarket"
                clearable
              >
                <template #prefix>
                  <n-icon class="text-slate-400">
                    <Terminal2 />
                  </n-icon>
                </template>
              </n-input>
              <div class="text-[11px] text-slate-400 pl-1">
                支持: npm 包名、@scoped 包、github:user/repo 简写及带引号的 Monorepo 子路径
              </div>
            </div>

            <!-- 命令解析折叠动画 -->
            <n-collapse-transition :show="!!command.trim()">
              <n-alert
                :type="preview?.valid ? 'success' : 'error'"
                :show-icon="true"
                class="rounded-xl text-xs"
              >
                {{ preview?.valid ? `将执行: ${preview.command}` : (preview?.reason || '解析中…') }}
              </n-alert>
            </n-collapse-transition>
          </div>
        </n-tab-pane>

        <!-- 离线文件上传 Tab -->
        <n-tab-pane name="upload" tab="文件上传">
          <div class="space-y-3 pt-3">
            <n-alert type="warning" :show-icon="true" class="rounded-xl text-xs">
              安装脚本将在本机以宿主权限执行，请仅安装来自可信来源的插件包。
            </n-alert>

            <n-upload
              v-model:file-list="uploadFileList"
              :max="1"
              accept=".tgz,.zip"
              :show-file-list="true"
              :default-upload="false"
              @change="handleUploadChange"
              :disabled="busy"
            >
              <n-upload-dragger class="!py-6">
                <div class="flex flex-col items-center justify-center gap-2">
                  <div class="w-12 h-12 rounded-2xl bg-blue-50 text-fnos-blue flex items-center justify-center">
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
          <n-button
            secondary
            v-debounce
            size="small"
            :loading="loading || busy"
            @click="handleRefresh"
          >
            <template #icon>
              <n-icon>
                <Refresh />
              </n-icon>
            </template>
            <span>刷新</span>
          </n-button>
        </div>
      </template>

      <!-- 加载骨架屏 -->
      <div v-if="loading" class="space-y-4 py-2">
        <div v-for="i in 3" :key="i" class="space-y-2 p-3 bg-slate-50 rounded-xl">
          <n-skeleton text style="width: 40%" />
          <n-skeleton text :repeat="2" />
        </div>
      </div>

      <!-- 空状态 -->
      <div v-else-if="!plugins.length" class="py-12 text-center">
        <n-empty description="暂无已安装插件">
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
      <n-list v-else hoverable class="divide-y divide-slate-100">
        <n-list-item v-for="p in plugins" :key="p.name" class="!py-3.5">
          <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 w-full min-w-0">
            <!-- 信息区：第一行与第二行（自适应收缩与截断保护） -->
            <div class="flex-1 min-w-0 space-y-1.5">
              <!-- 第一行：标题、版本、状态 -->
              <div class="flex items-center gap-2 w-full min-w-0">
                <span
                  class="text-sm font-bold text-slate-800 truncate min-w-0"
                  :title="p.name"
                >
                  {{ p.name }}
                </span>
                <n-tag v-if="p.version" size="tiny" round :bordered="false" class="shrink-0">
                  v{{ p.version }}
                </n-tag>
                <n-tag
                  :type="p.layer ? 'success' : 'default'"
                  size="tiny"
                  round
                  :bordered="false"
                  class="shrink-0"
                >
                  {{ p.layer ? '运行中' : '已停用' }}
                </n-tag>
              </div>

              <!-- 第二行：来源（单行超出截断与浮动提示） -->
              <div
                v-if="p.spec"
                class="text-xs text-slate-400 font-mono truncate w-full min-w-0 block"
                :title="p.spec"
              >
                {{ p.spec }}
              </div>
            </div>

            <!-- 第三行（移动端独立行 / 桌面端右侧对齐）：Switch 启停胶囊、更新按钮、卸载按钮 -->
            <div class="flex items-center justify-between sm:justify-end gap-2 w-full sm:w-auto shrink-0 pt-0.5 sm:pt-0">
              <!-- Switch 开关启停胶囊 -->
              <div class="flex items-center gap-1.5 bg-slate-50 px-2.5 py-1 rounded-lg border border-slate-100 shrink-0">
                <span class="text-xs text-slate-500 select-none">{{ p.layer ? '已启用' : '已禁用' }}</span>
                <n-switch
                  size="small"
                  :value="p.layer"
                  :disabled="busy"
                  @update:value="(val) => handleToggle(p.name, val)"
                />
              </div>

              <!-- 操作按钮组 -->
              <div class="flex items-center gap-2 shrink-0">
                <n-button
                  size="small"
                  secondary
                  v-debounce
                  type="info"
                  :disabled="busy"
                  @click="handleUpdate(p.name)"
                >
                  <template #icon>
                    <n-icon>
                      <Refresh />
                    </n-icon>
                  </template>
                  <span>更新</span>
                </n-button>

                <n-popconfirm
                  @positive-click="handleUninstall(p.name)"
                  positive-text="确认卸载"
                  negative-text="取消"
                >
                  <template #trigger>
                    <n-button
                      size="small"
                      secondary
                      v-debounce
                      type="error"
                      :disabled="busy"
                    >
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
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import {
  NPageHeader,
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
  NSkeleton,
  NIcon,
  NUpload,
  NUploadDragger,
  NPopconfirm,
  NCollapseTransition,
  useMessage,
  type UploadFileInfo
} from 'naive-ui'
import {
  Plus,
  Refresh,
  Trash,
  Upload,
  Terminal2,
  Puzzle
} from '@vicons/tabler'
import { usePluginStore } from '../stores/plugin'
import { withAsyncLock } from '../utils/debounce'

const pluginStore = usePluginStore()
const message = useMessage()

const {
  plugins,
  loading,
  pluginBusy: busy,
  mode,
  command,
  file,
  preview,
  canInstall
} = storeToRefs(pluginStore)

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
    message.success('已开始执行插件安装')
    uploadFileList.value = []
  } else {
    message.error(res.message || '安装失败')
  }
})

const handleToggle = withAsyncLock(async (name: string, enabled: boolean) => {
  const res = await pluginStore.togglePlugin(name, enabled)
  if (res.success) {
    message.success(enabled ? '已启用插件' : '已禁用插件')
  } else {
    message.error(res.message || '操作失败')
  }
})

const handleUpdate = withAsyncLock(async (name: string) => {
  const res = await pluginStore.updatePlugin(name)
  if (res.success) {
    message.success(`已开始更新 ${name}`)
  } else {
    message.error(res.message || '更新失败')
  }
})

const handleUninstall = withAsyncLock(async (name: string) => {
  const res = await pluginStore.uninstallPlugin(name)
  if (res.success) {
    message.success(`已开始卸载 ${name}`)
  } else {
    message.error(res.message || '卸载失败')
  }
})

onMounted(() => {
  pluginStore.fetchPlugins()
})
</script>

<style scoped>
:deep(.n-list-item__main) {
  min-width: 0;
  width: 100%;
}
</style>

