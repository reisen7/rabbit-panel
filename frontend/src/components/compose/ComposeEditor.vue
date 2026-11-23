<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { AlertTriangle, Loader2, Save, Upload } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'

const { t } = useI18n()
const toast = useToast()

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
const fileInputRef = ref<HTMLInputElement>()

const hasChanges = computed(() => localContent.value !== originalContent.value)

const lineCount = computed(() => {
  if (!localContent.value) return 0
  return localContent.value.split('\n').length
})

watch(
  () => props.content,
  (newContent) => {
    localContent.value = newContent
    originalContent.value = newContent
  },
  { immediate: true }
)

watch(
  () => props.projectName,
  () => {
    if (!props.projectName) {
      localContent.value = ''
      originalContent.value = ''
    }
  }
)

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

function triggerUpload() {
  fileInputRef.value?.click()
}

function handleFileSelect(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) return

  const reader = new FileReader()
  reader.onload = (e) => {
    const content = e.target?.result as string
    if (content) {
      localContent.value = content
      emit('change', content)
      toast.success(t('compose.uploadSuccess'))
    }
  }
  reader.onerror = () => {
    toast.error(t('compose.uploadFailed'))
  }
  reader.readAsText(file)
  target.value = ''
}

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

<template>
  <div class="flex flex-col flex-1 min-h-0 h-full overflow-hidden">
    <div class="flex items-center gap-2.5 px-3 py-2.5 border-b bg-card">
      <Button
        size="sm"
        :disabled="!hasChanges || !projectName || saving"
        @click="handleSave"
      >
        <Loader2 v-if="saving" class="animate-spin" />
        <Save v-else />
        {{ t('common.save') }}
      </Button>
      <Button
        size="sm"
        variant="outline"
        :disabled="!projectName"
        @click="triggerUpload"
      >
        <Upload />
        {{ t('compose.upload') }}
      </Button>
      <input
        ref="fileInputRef"
        type="file"
        accept=".yml,.yaml"
        class="hidden"
        @change="handleFileSelect"
      />
      <span v-if="hasChanges" class="inline-flex items-center gap-1.5 text-xs text-amber-600 dark:text-amber-400">
        <AlertTriangle class="size-3.5" />
        {{ t('compose.unsavedChanges') }}
      </span>
    </div>

    <div class="flex-1 min-h-0 p-3 flex flex-col overflow-hidden">
      <Textarea
        v-model="localContent"
        :placeholder="t('compose.editorPlaceholder')"
        :disabled="!projectName || loading"
        class="flex-1 min-h-0 h-full resize-none font-mono text-[13px] leading-relaxed [tab-size:2]"
        @input="handleInput"
      />
    </div>

    <div class="flex items-center justify-end px-3 py-2 border-t bg-card">
      <span class="text-xs text-muted-foreground tabular-nums">
        {{ t('compose.lines') }}: {{ lineCount }}
      </span>
    </div>
  </div>
</template>
