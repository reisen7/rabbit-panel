<template>
  <el-dialog
    v-model="dialogVisible"
    :title="t('container.create')"
    width="1100px"
    :close-on-click-modal="false"
    :before-close="handleBeforeClose"
    @close="handleClose"
  >
    <div class="create-container-layout">
      <div class="form-panel">
        <el-form
          ref="formRef"
          :model="form"
          :rules="rules"
          label-width="120px"
          label-position="right"
        >
          <!-- Image -->
          <el-form-item :label="t('container.image')" prop="image">
            <el-input v-model="form.image" placeholder="nginx:latest" />
          </el-form-item>

          <!-- Container Name -->
          <el-form-item :label="t('container.containerName')" prop="name">
            <el-input v-model="form.name" placeholder="my-container" />
          </el-form-item>

          <!-- Restart Policy -->
          <el-form-item :label="t('container.restartPolicy')" prop="restart">
            <el-select v-model="form.restart" style="width: 100%">
              <el-option value="no" :label="t('container.restartPolicies.no')" />
              <el-option value="always" :label="t('container.restartPolicies.always')" />
              <el-option value="on-failure" :label="t('container.restartPolicies.onFailure')" />
              <el-option value="unless-stopped" :label="t('container.restartPolicies.unlessStopped')" />
            </el-select>
          </el-form-item>

          <!-- Network -->
          <el-form-item :label="t('container.network')" prop="network">
            <el-input v-model="form.network" placeholder="bridge" />
          </el-form-item>

          <!-- Port Mappings -->
          <el-form-item :label="t('container.ports')">
            <div class="dynamic-list">
              <div v-for="(port, index) in form.ports" :key="index" class="dynamic-item">
                <el-input
                  v-model="port.host"
                  :placeholder="t('container.hostPort')"
                  style="width: 120px"
                />
                <span class="separator">:</span>
                <el-input
                  v-model="port.container"
                  :placeholder="t('container.containerPort')"
                  style="width: 120px"
                />
                <el-button
                  type="danger"
                  :icon="Delete"
                  circle
                  size="small"
                  @click="removePort(index)"
                />
              </div>
              <el-button type="primary" text @click="addPort">
                <el-icon><Plus /></el-icon>
                {{ t('container.addPort') }}
              </el-button>
            </div>
          </el-form-item>

          <!-- Volume Mappings -->
          <el-form-item :label="t('container.volumes')">
            <div class="dynamic-list">
              <div v-for="(volume, index) in form.volumes" :key="index" class="dynamic-item">
                <el-input
                  v-model="volume.host"
                  :placeholder="t('container.hostPath')"
                  style="width: 180px"
                />
                <span class="separator">:</span>
                <el-input
                  v-model="volume.container"
                  :placeholder="t('container.containerPath')"
                  style="width: 180px"
                />
                <el-button
                  type="danger"
                  :icon="Delete"
                  circle
                  size="small"
                  @click="removeVolume(index)"
                />
              </div>
              <el-button type="primary" text @click="addVolume">
                <el-icon><Plus /></el-icon>
                {{ t('container.addVolume') }}
              </el-button>
            </div>
          </el-form-item>

          <!-- Environment Variables -->
          <el-form-item :label="t('container.envVars')">
            <div class="dynamic-list">
              <div v-for="(env, index) in form.envs" :key="index" class="dynamic-item">
                <el-input
                  v-model="env.key"
                  :placeholder="t('container.key')"
                  style="width: 150px"
                />
                <span class="separator">=</span>
                <el-input
                  v-model="env.value"
                  :placeholder="t('container.value')"
                  style="width: 200px"
                />
                <el-button
                  type="danger"
                  :icon="Delete"
                  circle
                  size="small"
                  @click="removeEnv(index)"
                />
              </div>
              <el-button type="primary" text @click="addEnv">
                <el-icon><Plus /></el-icon>
                {{ t('container.addEnv') }}
              </el-button>
            </div>
          </el-form-item>
        </el-form>
      </div>

      <div class="log-panel">
        <div class="log-header">
          <span>{{ t('container.output') }}</span>
          <span class="log-status">{{ isStreaming ? t('container.building') : '-' }}</span>
        </div>
        <div ref="logPanelRef" class="log-content">
          <div v-if="streamLogs.length === 0" class="log-empty">
            {{ t('container.output') }}
          </div>
          <div
            v-for="(line, index) in streamLogs"
            :key="index"
            :class="['log-line', line.type]"
          >
            {{ line.message }}
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <el-button v-if="loading || isStreaming" @click="handleMinimize">{{ t('update.minimize') }}</el-button>
      <el-button v-if="loading || isStreaming" type="danger" @click="handleCancelTask">{{ t('container.cancelTask') }}</el-button>
      <el-button v-else @click="handleClose">{{ t('common.close') }}</el-button>
      <el-button type="primary" :loading="loading" @click="handleSubmit">
        {{ t('common.confirm') }}
      </el-button>
    </template>
  </el-dialog>

  <transition name="fade">
    <div v-if="minimized && hasTaskState" class="mini-create-task" @click="restoreDialog">
      <div class="mini-header">
        <strong>{{ t('container.create') }}</strong>
        <span>{{ isStreaming ? t('container.building') : t('common.refresh') }}</span>
      </div>
      <div class="mini-body">
        <div class="mini-log">{{ latestLogLine }}</div>
      </div>
      <button v-if="loading || isStreaming" class="mini-cancel" @click.stop="handleCancelTask">×</button>
    </div>
  </transition>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, nextTick } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage } from 'element-plus'
import { Plus, Delete } from '@element-plus/icons-vue'
import { useI18n } from '@/composables/useI18n'
import { containerApi } from '@/api/containers'

const { t } = useI18n()

const props = defineProps<{
  visible: boolean
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
  created: []
}>()

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
})

const formRef = ref<FormInstance>()
const loading = ref(false)
const isStreaming = ref(false)
const streamLogs = ref<Array<{ type: string; message: string }>>([])
const logPanelRef = ref<HTMLElement | null>(null)
const minimized = ref(false)
const restoringFromMinimize = ref(false)
const abortController = ref<AbortController | null>(null)

interface PortMapping {
  host: string
  container: string
}

interface VolumeMapping {
  host: string
  container: string
}

interface EnvVar {
  key: string
  value: string
}

interface FormData {
  image: string
  name: string
  restart: string
  network: string
  ports: PortMapping[]
  volumes: VolumeMapping[]
  envs: EnvVar[]
}

const initialForm = (): FormData => ({
  image: '',
  name: '',
  restart: 'no',
  network: 'bridge',
  ports: [],
  volumes: [],
  envs: [],
})

const form = reactive<FormData>(initialForm())

const hasTaskState = computed(() => loading.value || isStreaming.value || streamLogs.value.length > 0)
const latestLogLine = computed(() => {
  if (streamLogs.value.length === 0) return t('container.output')
  return streamLogs.value[streamLogs.value.length - 1]?.message || t('container.output')
})

const rules: FormRules = {
  image: [
    { required: true, message: () => t('container.image') + ' is required', trigger: 'blur' },
  ],
}

// Reset form when dialog opens
watch(() => props.visible, (val) => {
  if (val) {
    if (restoringFromMinimize.value) {
      restoringFromMinimize.value = false
      return
    }
    Object.assign(form, initialForm())
    formRef.value?.resetFields()
    streamLogs.value = []
    isStreaming.value = false
    minimized.value = false
  }
})

watch(streamLogs, async () => {
  await nextTick()
  if (logPanelRef.value) {
    logPanelRef.value.scrollTop = logPanelRef.value.scrollHeight
  }
}, { deep: true })

// Port mapping methods
function addPort() {
  form.ports.push({ host: '', container: '' })
}

function removePort(index: number) {
  form.ports.splice(index, 1)
}

// Volume mapping methods
function addVolume() {
  form.volumes.push({ host: '', container: '' })
}

function removeVolume(index: number) {
  form.volumes.splice(index, 1)
}

// Environment variable methods
function addEnv() {
  form.envs.push({ key: '', value: '' })
}

function removeEnv(index: number) {
  form.envs.splice(index, 1)
}

function handleBeforeClose(done: () => void) {
  if (loading.value || isStreaming.value) {
    handleMinimize()
    done()
    return
  }
  done()
}

function handleMinimize() {
  minimized.value = true
  dialogVisible.value = false
}

function handleCancelTask() {
  abortController.value?.abort()
  abortController.value = null
  streamLogs.value.push({ type: 'error', message: '已取消创建任务' })
  loading.value = false
  isStreaming.value = false
}

function restoreDialog() {
  restoringFromMinimize.value = true
  minimized.value = false
  dialogVisible.value = true
}

function handleClose() {
  if (loading.value || isStreaming.value) {
    handleMinimize()
    return
  }
  dialogVisible.value = false
}

async function handleSubmit() {
  if (!formRef.value) return

  try {
    await formRef.value.validate()
  } catch {
    return
  }

  loading.value = true
  isStreaming.value = true
  streamLogs.value = []

  try {
    // Filter out empty entries
    const ports = form.ports.filter((p) => p.host && p.container)
    const volumes = form.volumes.filter((v) => v.host && v.container)
    const envs = form.envs.filter((e) => e.key)

    abortController.value = new AbortController()
    await containerApi.createStream({
      image: form.image,
      name: form.name,
      restart: form.restart,
      network: form.network,
      ports,
      volumes,
      env: envs,
    }, (entry) => {
      streamLogs.value.push(entry)
    }, abortController.value.signal)
    ElMessage.success(t('container.createSuccess'))
    minimized.value = false

    emit('created')
    handleClose()
  } catch (error: any) {
    if (error?.name === 'AbortError') {
      // cancelled by user
    }
  } finally {
    abortController.value = null
    loading.value = false
    isStreaming.value = false
  }
}
</script>

<style scoped>
.create-container-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 420px;
  gap: 16px;
  align-items: stretch;
}

.form-panel {
  min-width: 0;
}

.log-panel {
  display: flex;
  flex-direction: column;
  min-height: 100%;
  border: 1px solid var(--el-border-color);
  border-radius: 6px;
  overflow: hidden;
  background: #111;
}

.log-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 12px;
  background: var(--el-fill-color-light);
  border-bottom: 1px solid var(--el-border-color);
  font-weight: 500;
}

.log-status {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.log-content {
  flex: 1;
  min-height: 520px;
  max-height: 520px;
  overflow-y: auto;
  padding: 12px;
  background: #111;
  color: #d4d4d4;
  font-family: Consolas, Monaco, 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.5;
}

.log-empty {
  color: #888;
}

.log-line {
  white-space: pre-wrap;
  word-break: break-all;
  margin-bottom: 4px;
}

.log-line.error {
  color: #f56c6c;
}

.log-line.success {
  color: #67c23a;
}

.dynamic-list {
  width: 100%;
}

.dynamic-item {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}

.separator {
  color: var(--el-text-color-secondary);
  font-weight: bold;
}

.mini-create-task {
  position: fixed;
  right: 24px;
  bottom: 96px;
  width: 360px;
  z-index: 2200;
  padding: 14px 44px 14px 16px;
  border-radius: 16px;
  background: rgba(18, 22, 33, 0.94);
  color: #fff;
  box-shadow: 0 16px 40px rgba(0,0,0,0.24);
  cursor: pointer;
  backdrop-filter: blur(12px);
}

.mini-cancel {
  position: absolute;
  top: 10px;
  right: 10px;
  width: 22px;
  height: 22px;
  border: 0;
  border-radius: 999px;
  background: rgba(255,255,255,0.14);
  color: #fff;
  cursor: pointer;
}

.mini-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
  padding-right: 8px;
}

.mini-body {
  font-size: 12px;
  color: rgba(255,255,255,0.78);
  padding-right: 8px;
}

.mini-log {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.fade-enter-active, .fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from, .fade-leave-to {
  opacity: 0;
}

@media (max-width: 1200px) {
  .create-container-layout {
    grid-template-columns: 1fr;
  }

  .log-content {
    min-height: 220px;
    max-height: 220px;
  }

  .mini-create-task {
    right: 16px;
    left: 16px;
    width: auto;
    bottom: 96px;
  }
}
</style>
