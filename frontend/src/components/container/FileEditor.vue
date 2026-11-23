<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Loader2 } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { containerApi } from '@/api/containers'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'

const props = defineProps<{
  visible: boolean
  containerId: string
  filePath: string
  fileName: string
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
  saved: []
}>()

const { t } = useI18n()
const toast = useToast()

const dialogVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
})

const content = ref('')
const loading = ref(false)
const saving = ref(false)

async function loadContent() {
  if (!props.containerId || !props.filePath) return

  loading.value = true
  try {
    const result = await containerApi.fileRead(props.containerId, props.filePath)
    content.value = result.content
  } catch {
    toast.error(t('file.loadError'))
    dialogVisible.value = false
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  if (!props.containerId || !props.filePath) return

  saving.value = true
  try {
    await containerApi.fileWrite(props.containerId, props.filePath, content.value)
    toast.success(t('file.saveSuccess'))
    emit('saved')
    dialogVisible.value = false
  } catch {
    // Error handled by interceptor
  } finally {
    saving.value = false
  }
}

watch(
  () => props.visible,
  (val) => {
    if (val) {
      content.value = ''
      loadContent()
    }
  },
  { immediate: true }
)

watch(
  () => props.filePath,
  () => {
    if (props.visible && props.filePath) {
      loadContent()
    }
  }
)
</script>

<template>
  <Dialog v-model:open="dialogVisible">
    <DialogContent class="sm:max-w-[75vw] max-w-[1000px] max-h-[90vh] flex flex-col" @interact-outside.prevent>
      <DialogHeader>
        <DialogTitle>{{ t('file.edit') + ' - ' + fileName }}</DialogTitle>
      </DialogHeader>

      <div class="min-h-[420px] flex-1 flex flex-col">
        <div
          v-if="loading"
          class="flex-1 flex flex-col items-center justify-center gap-2 text-muted-foreground"
        >
          <Loader2 class="size-7 animate-spin" />
          <p class="text-sm">{{ t('common.loading') }}</p>
        </div>

        <Textarea
          v-else
          v-model="content"
          rows="20"
          :placeholder="t('file.fileContent')"
          class="flex-1 min-h-[420px] font-mono text-[13px] leading-relaxed resize-y"
        />
      </div>

      <DialogFooter class="gap-2">
        <Button variant="outline" @click="dialogVisible = false">{{ t('common.cancel') }}</Button>
        <Button :disabled="saving || loading" @click="handleSave">
          <Loader2 v-if="saving" class="animate-spin" />
          {{ saving ? t('file.saving') : t('common.save') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
