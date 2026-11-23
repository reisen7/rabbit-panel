<script setup lang="ts">
import { ref, computed, watch, nextTick, onUnmounted } from 'vue'
import { CheckCircle2, Search, Trash2, XCircle } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { containerApi } from '@/api/containers'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

const { t } = useI18n()

const props = defineProps<{
  visible: boolean
  containerId: string
  containerName: string
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
}>()

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
})

const logs = ref<string[]>([])
const searchQuery = ref('')
const autoScroll = ref(true)
const isConnected = ref(false)
const logsRef = ref<HTMLElement>()
const showAllLogs = ref(false)

let eventSource: EventSource | null = null

const filteredLogs = computed(() => {
  if (!searchQuery.value) {
    return logs.value
  }
  const query = searchQuery.value.toLowerCase()
  return logs.value.filter((log) => log.toLowerCase().includes(query))
})

function highlightSearch(text: string): string {
  if (!searchQuery.value) {
    return escapeHtml(text)
  }

  const query = searchQuery.value
  const escapedText = escapeHtml(text)
  const escapedQuery = escapeHtml(query)

  const regex = new RegExp(`(${escapeRegex(escapedQuery)})`, 'gi')
  return escapedText.replace(regex, '<mark class="highlight">$1</mark>')
}

function escapeHtml(text: string): string {
  const div = document.createElement('div')
  div.textContent = text
  return div.innerHTML
}

function escapeRegex(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

async function scrollToBottom() {
  if (autoScroll.value && logsRef.value) {
    await nextTick()
    logsRef.value.scrollTop = logsRef.value.scrollHeight
  }
}

watch(logs, scrollToBottom, { deep: true })

function handleOpen() {
  if (!props.containerId) return
  showAllLogs.value = false
  connectLogs()
}

function connectLogs() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }

  logs.value = []
  isConnected.value = false

  if (showAllLogs.value) {
    loadAllLogs()
    return
  }

  eventSource = containerApi.logs(props.containerId, 100, true)

  eventSource.onopen = () => {
    isConnected.value = true
  }

  eventSource.onmessage = (event) => {
    if (event.data) {
      const logLine = event.data.replace(/\\n/g, '\n').replace(/\\r/g, '\r')
      logs.value.push(logLine)

      if (!showAllLogs.value && logs.value.length > 10000) {
        logs.value = logs.value.slice(-5000)
      }
    }
  }

  eventSource.onerror = () => {
    isConnected.value = false
    eventSource?.close()
    eventSource = null
  }
}

async function loadAllLogs() {
  try {
    logs.value = await containerApi.logsOnce(props.containerId, 'all')
    isConnected.value = true
  } catch {
    isConnected.value = false
  }
}

function handleModeChange() {
  connectLogs()
}

function cleanupLogs() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
  isConnected.value = false
}

function handleClose() {
  cleanupLogs()
  dialogVisible.value = false
}

function clearLogs() {
  logs.value = []
}

watch(
  () => props.visible,
  (val) => {
    if (val) {
      handleOpen()
    } else {
      cleanupLogs()
    }
  },
  { immediate: true }
)

onUnmounted(() => {
  cleanupLogs()
})
</script>

<template>
  <Dialog v-model:open="dialogVisible">
    <DialogContent class="sm:max-w-[900px] max-h-[90vh] flex flex-col" @interact-outside.prevent>
      <DialogHeader>
        <DialogTitle>{{ `${t('container.logs')} - ${containerName}` }}</DialogTitle>
      </DialogHeader>

      <div class="flex flex-col gap-3 flex-1 min-h-0">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="relative w-full sm:w-64">
            <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
            <Input
              v-model="searchQuery"
              :placeholder="t('container.searchLogs')"
              class="pl-8 h-8"
            />
          </div>

          <div class="flex flex-wrap items-center gap-4">
            <div class="flex items-center gap-2">
              <span class="text-xs text-muted-foreground">实时日志</span>
              <Switch v-model="showAllLogs" @update:model-value="handleModeChange" />
              <span class="text-xs text-muted-foreground">全部日志</span>
            </div>

            <div class="flex items-center gap-2">
              <Checkbox id="logs-auto-scroll" v-model="autoScroll" :disabled="showAllLogs" />
              <Label for="logs-auto-scroll" class="text-xs font-normal cursor-pointer">
                Auto-scroll
              </Label>
            </div>

            <Button variant="outline" size="sm" @click="clearLogs">
              <Trash2 />
              Clear
            </Button>
          </div>
        </div>

        <div
          ref="logsRef"
          class="h-[420px] overflow-y-auto rounded-lg border bg-zinc-950 p-3 font-mono text-xs leading-relaxed text-zinc-200"
        >
          <div v-if="filteredLogs.length === 0" class="py-16 text-center text-zinc-500">
            {{ t('container.noLogs') }}
          </div>
          <div
            v-for="(log, index) in filteredLogs"
            :key="index"
            class="whitespace-pre-wrap break-all px-1 py-0.5 rounded hover:bg-zinc-900/80"
            v-html="highlightSearch(log)"
          />
        </div>

        <div class="flex items-center justify-between text-xs text-muted-foreground">
          <span
            v-if="showAllLogs"
            class="inline-flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400"
          >
            <CheckCircle2 class="size-3.5" />
            History Loaded
          </span>
          <span
            v-else-if="isConnected"
            class="inline-flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400"
          >
            <CheckCircle2 class="size-3.5" />
            Connected
          </span>
          <span v-else class="inline-flex items-center gap-1.5 text-destructive">
            <XCircle class="size-3.5" />
            Disconnected
          </span>

          <span class="tabular-nums">{{ logs.length }} lines</span>
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="handleClose">{{ t('common.close') }}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<style scoped>
:deep(.highlight) {
  background: rgba(245, 158, 11, 0.35);
  color: #fde68a;
  padding: 0 2px;
  border-radius: 2px;
}
</style>
