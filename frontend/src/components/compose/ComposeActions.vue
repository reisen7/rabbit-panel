<script setup lang="ts">
import { ref } from 'vue'
import { Download, Loader2, Play, RotateCw, Square, Terminal, X } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

const { t } = useI18n()
const toast = useToast()

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
    toast.error(error instanceof Error ? error.message : t('compose.actionFailed'))
  }
}

function handleClear() {
  currentAction.value = null
  emit('clear')
}

defineExpose({
  resetAction: () => {
    currentAction.value = null
  },
})
</script>

<template>
  <TooltipProvider>
    <div class="flex flex-col shrink-0 gap-3">
      <div class="flex flex-wrap items-center gap-2">
        <Tooltip>
          <TooltipTrigger as-child>
            <span>
              <Button
                size="sm"
                class="bg-emerald-600 text-white hover:bg-emerald-600/90"
                :disabled="!projectName || loading || status === 'running'"
                @click="handleAction('up')"
              >
                <Loader2 v-if="loading && currentAction === 'up'" class="animate-spin" />
                <Play v-else />
                {{ t('compose.up') }}
              </Button>
            </span>
          </TooltipTrigger>
          <TooltipContent v-if="status === 'running'">项目已在运行中</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger as-child>
            <span>
              <Button
                size="sm"
                variant="destructive"
                :disabled="!projectName || loading || status === 'stopped'"
                @click="handleAction('down')"
              >
                <Loader2 v-if="loading && currentAction === 'down'" class="animate-spin" />
                <Square v-else />
                {{ t('compose.down') }}
              </Button>
            </span>
          </TooltipTrigger>
          <TooltipContent v-if="status === 'stopped'">项目未运行</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger as-child>
            <span>
              <Button
                size="sm"
                variant="outline"
                class="text-amber-600 dark:text-amber-400"
                :disabled="!projectName || loading || status === 'stopped'"
                @click="handleAction('restart')"
              >
                <Loader2 v-if="loading && currentAction === 'restart'" class="animate-spin" />
                <RotateCw v-else />
                {{ t('compose.restart') }}
              </Button>
            </span>
          </TooltipTrigger>
          <TooltipContent v-if="status === 'stopped'">项目未运行</TooltipContent>
        </Tooltip>

        <Button
          size="sm"
          variant="outline"
          :disabled="!projectName || loading"
          @click="handleAction('pull')"
        >
          <Loader2 v-if="loading && currentAction === 'pull'" class="animate-spin" />
          <Download v-else />
          {{ t('compose.pull') }}
        </Button>

        <Button v-if="output" size="sm" variant="ghost" @click="handleClear">
          <X />
          {{ t('compose.clearOutput') }}
        </Button>
      </div>

      <div v-if="output || loading" class="rounded-lg border overflow-hidden">
        <div class="flex items-center gap-1.5 px-3 py-1.5 bg-muted/50 border-b text-xs font-medium">
          <Terminal class="size-3.5 text-muted-foreground" />
          <span>{{ t('compose.output') }}</span>
        </div>
        <div class="max-h-[140px] overflow-auto bg-zinc-950 text-zinc-100">
          <pre
            v-if="output"
            class="m-0 p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap break-all"
          >{{ output }}</pre>
          <div v-else-if="loading" class="flex items-center justify-center gap-2 p-4 text-xs text-zinc-400">
            <Loader2 class="size-4 animate-spin" />
            <span>{{ t('compose.executing') }}</span>
          </div>
        </div>
      </div>
    </div>
  </TooltipProvider>
</template>
