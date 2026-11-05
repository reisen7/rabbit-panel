<template>
  <div class="compose-actions">
    <!-- Action Buttons -->
    <div class="action-buttons">
      <el-tooltip
        :content="status === 'running' ? '项目已在运行中' : ''"
        :disabled="status !== 'running'"
        placement="top"
      >
        <el-button
          type="success"
          :icon="VideoPlay"
          :loading="loading && currentAction === 'up'"
          :disabled="!projectName || loading || status === 'running'"
          @click="handleAction('up')"
        >
          {{ t('compose.up') }}
        </el-button>
      </el-tooltip>
      <el-tooltip
        :content="status === 'stopped' ? '项目未运行' : ''"
        :disabled="status !== 'stopped'"
        placement="top"
      >
        <el-button
          type="danger"
          :icon="VideoPause"
          :loading="loading && currentAction === 'down'"
          :disabled="!projectName || loading || status === 'stopped'"
          @click="handleAction('down')"
        >
          {{ t('compose.down') }}
        </el-button>
      </el-tooltip>
      <el-tooltip
        :content="status === 'stopped' ? '项目未运行' : ''"
        :disabled="status !== 'stopped'"
        placement="top"
      >
        <el-button
          type="warning"
          :icon="RefreshRight"
          :loading="loading && currentAction === 'restart'"
          :disabled="!projectName || loading || status === 'stopped'"
          @click="handleAction('restart')"
        >
          {{ t('compose.restart') }}
        </el-button>
      </el-tooltip>
      <el-button
        :icon="Download"
        :loading="loading && currentAction === 'pull'"
        :disabled="!projectName || loading"
        @click="handleAction('pull')"
      >
        {{ t('compose.pull') }}
      </el-button>
      <el-button
        v-if="output"
        :icon="Close"
        text
        @click="handleClear"
      >
        {{ t('compose.clearOutput') }}
      </el-button>
    </div>

    <!-- Output Panel (inline, small) -->
    <div v-if="output || loading" class="output-panel">
      <div class="output-header">
        <span class="output-title">
          <el-icon><Monitor /></el-icon>
          {{ t('compose.output') }}
        </span>
      </div>
      <div class="output-content">
        <pre v-if="output">{{ output }}</pre>
        <div v-else-if="loading" class="loading-placeholder">
          <el-icon class="is-loading"><Loading /></el-icon>
          {{ t('compose.executing') }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import {
  VideoPlay,
  VideoPause,
  RefreshRight,
  Download,
  Close,
  Monitor,
  Loading,
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useI18n } from '@/composables/useI18n'

const { t } = useI18n()

type ComposeAction = 'up' | 'down' | 'restart' | 'pull'

const props = defineProps<{
  projectName: string | null
  output: string
  loading?: boolean
  status?: 'running' | 'partial' | 'stopped' | 'unknown'
}>()

const emit = defineEmits<{
  (e: 'action', action: ComposeAction): void
  (e: 'clear'): void
}>()

const currentAction = ref<ComposeAction | null>(null)

async function handleAction(action: ComposeAction) {
  if (!props.projectName) return

  currentAction.value = action
  try {
    emit('action', action)
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : t('compose.actionFailed')
    )
  }
}

function handleClear() {
  currentAction.value = null
  emit('clear')
}

// Reset current action when loading completes
defineExpose({
  resetAction: () => {
    currentAction.value = null
  },
})
</script>

<style scoped>
.compose-actions {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  gap: 12px;
}

.action-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.output-panel {
  border: 1px solid var(--el-border-color-light);
  border-radius: 4px;
  overflow: hidden;
}

.output-header {
  display: flex;
  align-items: center;
  padding: 6px 12px;
  background: var(--el-fill-color-light);
  border-bottom: 1px solid var(--el-border-color-light);
}

.output-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 500;
}

.output-content {
  max-height: 120px;
  overflow: auto;
  background: #1e1e1e;
}

.output-content pre {
  margin: 0;
  padding: 8px 12px;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 13px;
  line-height: 1;
  letter-spacing: 0;
  white-space: pre;
  word-break: break-all;
  color: #d4d4d4;
}

.loading-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 16px;
  color: #999;
  font-size: 12px;
}
</style>
