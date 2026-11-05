<template>
  <div class="compose-editor">
    <!-- Toolbar -->
    <div class="editor-toolbar">
      <el-button
        type="primary"
        :icon="DocumentChecked"
        :loading="saving"
        :disabled="!hasChanges || !projectName"
        @click="handleSave"
      >
        {{ t('common.save') }}
      </el-button>
      <el-upload
        :show-file-list="false"
        :before-upload="handleUpload"
        accept=".yml,.yaml"
      >
        <el-button :icon="Upload" :disabled="!projectName">
          {{ t('compose.upload') }}
        </el-button>
      </el-upload>
      <span v-if="hasChanges" class="unsaved-indicator">
        <el-icon><Warning /></el-icon>
        {{ t('compose.unsavedChanges') }}
      </span>
    </div>

    <!-- Editor -->
    <div class="editor-container">
      <el-input
        v-model="localContent"
        type="textarea"
        :placeholder="t('compose.editorPlaceholder')"
        :disabled="!projectName || loading"
        class="yaml-editor"
        @input="handleInput"
      />
    </div>

    <!-- Line numbers hint -->
    <div class="editor-footer">
      <span class="line-count">
        {{ t('compose.lines') }}: {{ lineCount }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { DocumentChecked, Upload, Warning } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useI18n } from '@/composables/useI18n'

const { t } = useI18n()

const props = defineProps<{
  projectName: string | null
  content: string
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'save', content: string): void
  (e: 'change', content: string): void
}>()

const localContent = ref('')
const originalContent = ref('')
const saving = ref(false)

// Computed
const hasChanges = computed(() => {
  return localContent.value !== originalContent.value
})

const lineCount = computed(() => {
  if (!localContent.value) return 0
  return localContent.value.split('\n').length
})

// Watch for external content changes
watch(
  () => props.content,
  (newContent) => {
    localContent.value = newContent
    originalContent.value = newContent
  },
  { immediate: true }
)

// Watch for project changes - reset content
watch(
  () => props.projectName,
  () => {
    if (!props.projectName) {
      localContent.value = ''
      originalContent.value = ''
    }
  }
)

// Methods
function handleInput() {
  emit('change', localContent.value)
}

async function handleSave() {
  if (!hasChanges.value || !props.projectName) return

  try {
    saving.value = true
    emit('save', localContent.value)
    originalContent.value = localContent.value
  } finally {
    saving.value = false
  }
}

function handleUpload(file: File): boolean {
  const reader = new FileReader()
  reader.onload = (e) => {
    const content = e.target?.result as string
    if (content) {
      localContent.value = content
      emit('change', content)
      ElMessage.success(t('compose.uploadSuccess'))
    }
  }
  reader.onerror = () => {
    ElMessage.error(t('compose.uploadFailed'))
  }
  reader.readAsText(file)
  return false // Prevent default upload
}

// Expose methods for parent component
defineExpose({
  hasChanges,
  resetChanges: () => {
    localContent.value = originalContent.value
  },
  setContent: (content: string) => {
    localContent.value = content
    originalContent.value = content
  },
})
</script>

<style scoped>
.compose-editor {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  height: 100%;
  overflow: hidden;
}

.editor-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border-bottom: 1px solid var(--el-border-color-light);
  background: var(--el-bg-color);
}

.unsaved-indicator {
  display: flex;
  align-items: center;
  gap: 4px;
  color: var(--el-color-warning);
  font-size: 13px;
}

.editor-container {
  flex: 1;
  min-height: 0;
  padding: 12px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.yaml-editor {
  flex: 1;
  min-height: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.yaml-editor :deep(.el-textarea__inner) {
  flex: 1;
  height: 100% !important;
  min-height: 0;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 13px;
  line-height: 1.6;
  tab-size: 2;
  resize: none;
  overflow: auto;
}

.editor-footer {
  display: flex;
  justify-content: flex-end;
  padding: 8px 12px;
  border-top: 1px solid var(--el-border-color-light);
  background: var(--el-bg-color);
}

.line-count {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>
