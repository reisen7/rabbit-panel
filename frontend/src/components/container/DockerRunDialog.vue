<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { ChevronDown, ChevronRight, Loader2 } from 'lucide-vue-next'
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
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

const { t } = useI18n()
const toast = useToast()

const props = defineProps<{
  visible: boolean
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
  created: []
}>()

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => {
    if (!val && isExecuting.value) return
    emit('update:visible', val)
  },
})

const command = ref('')
const isExecuting = ref(false)
const output = ref<Array<{ type: string; message: string }>>([])
const outputRef = ref<HTMLElement>()
const previewOpen = ref(false)

interface ParsedCommand {
  image: string
  name: string
  ports: Array<{ host: string; container: string }>
  volumes: Array<{ host: string; container: string }>
  envs: Array<{ key: string; value: string }>
  restart: string
  network: string
}

const parsedCommand = computed((): ParsedCommand | null => {
  const cmd = command.value.trim()
  if (!cmd || !cmd.startsWith('docker run')) {
    return null
  }

  const result: ParsedCommand = {
    image: '',
    name: '',
    ports: [],
    volumes: [],
    envs: [],
    restart: '',
    network: '',
  }

  const args = parseCommandArgs(cmd)
  const nonFlagArgs: string[] = []

  let i = 2
  while (i < args.length) {
    const arg = args[i]
    if (!arg) {
      i++
      continue
    }

    if (arg === '-d' || arg === '--detach' || arg === '-it' || arg === '-i' || arg === '-t' || arg === '--rm') {
      i++
      continue
    }

    if (arg === '--name' && i + 1 < args.length) {
      result.name = args[i + 1] || ''
      i += 2
      continue
    }

    if ((arg === '-p' || arg === '--publish') && i + 1 < args.length) {
      const portMapping = args[i + 1] || ''
      const parts = portMapping.split(':')
      if (parts.length >= 2) {
        const hostPart = parts[0] || ''
        const containerPart = parts[parts.length - 1] || ''
        result.ports.push({
          host: hostPart,
          container: containerPart.split('/')[0] || '',
        })
      }
      i += 2
      continue
    }

    if ((arg === '-v' || arg === '--volume') && i + 1 < args.length) {
      const volumeMapping = args[i + 1] || ''
      const parts = volumeMapping.split(':')
      if (parts.length >= 2) {
        result.volumes.push({
          host: parts[0] || '',
          container: parts[1] || '',
        })
      }
      i += 2
      continue
    }

    if ((arg === '-e' || arg === '--env') && i + 1 < args.length) {
      const envVar = args[i + 1] || ''
      const eqIndex = envVar.indexOf('=')
      if (eqIndex > 0) {
        result.envs.push({
          key: envVar.substring(0, eqIndex),
          value: envVar.substring(eqIndex + 1),
        })
      }
      i += 2
      continue
    }

    if (arg === '--restart' && i + 1 < args.length) {
      result.restart = args[i + 1] || ''
      i += 2
      continue
    }

    if ((arg === '--network' || arg === '--net') && i + 1 < args.length) {
      result.network = args[i + 1] || ''
      i += 2
      continue
    }

    if (arg.startsWith('--') && arg.includes('=')) {
      const eqIndex = arg.indexOf('=')
      const flagName = arg.substring(2, eqIndex)
      const flagValue = arg.substring(eqIndex + 1)

      if (flagName === 'name') {
        result.name = flagValue
      } else if (flagName === 'restart') {
        result.restart = flagValue
      } else if (flagName === 'network' || flagName === 'net') {
        result.network = flagValue
      }
      i++
      continue
    }

    const nextArg = args[i + 1]
    if (arg.startsWith('-') && arg.length === 2 && i + 1 < args.length && nextArg && !nextArg.startsWith('-')) {
      i += 2
      continue
    }

    if (arg.startsWith('--') && i + 1 < args.length && nextArg && !nextArg.startsWith('-')) {
      i += 2
      continue
    }

    if (arg.startsWith('-')) {
      i++
      continue
    }

    nonFlagArgs.push(arg)
    i++
  }

  if (nonFlagArgs.length > 0) {
    result.image = nonFlagArgs[0] || ''
  }

  return result.image ? result : null
})

function parseCommandArgs(cmd: string): string[] {
  const normalizedCmd = cmd
    .replace(/\\\r?\n/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()

  const args: string[] = []
  let current = ''
  let inQuote = false
  let quoteChar = ''

  for (let i = 0; i < normalizedCmd.length; i++) {
    const char = normalizedCmd[i]

    if (inQuote) {
      if (char === quoteChar) {
        inQuote = false
      } else {
        current += char
      }
    } else if (char === '"' || char === "'") {
      inQuote = true
      quoteChar = char
    } else if (char === ' ' || char === '\t') {
      if (current) {
        args.push(current)
        current = ''
      }
    } else {
      current += char
    }
  }

  if (current) {
    args.push(current)
  }

  return args
}

watch(
  () => props.visible,
  (val) => {
    if (val) {
      command.value = ''
      output.value = []
      isExecuting.value = false
      previewOpen.value = false
    }
  }
)

watch(
  output,
  async () => {
    await nextTick()
    if (outputRef.value) {
      outputRef.value.scrollTop = outputRef.value.scrollHeight
    }
  },
  { deep: true }
)

function handleClose() {
  if (!isExecuting.value) {
    dialogVisible.value = false
  }
}

async function handleExecute() {
  const cmd = command.value.trim()
  if (!cmd) return

  if (!cmd.startsWith('docker run')) {
    toast.error('Only docker run commands are supported')
    return
  }

  isExecuting.value = true
  output.value = []

  try {
    await containerApi.createRawStream(
      cmd,
      (data) => {
        output.value.push({
          type: data.type || 'log',
          message: data.message || data.container_id || '',
        })
      },
      (error) => {
        output.value.push({ type: 'error', message: error })
      },
      () => {
        isExecuting.value = false
        const lastOutput = output.value[output.value.length - 1]
        if (lastOutput?.type === 'success') {
          toast.success(t('container.createSuccess'))
          emit('created')
        }
      }
    )
  } catch {
    isExecuting.value = false
  }
}
</script>

<template>
  <Dialog v-model:open="dialogVisible">
    <DialogContent class="sm:max-w-[700px] max-h-[88vh] flex flex-col" @interact-outside.prevent>
      <DialogHeader>
        <DialogTitle>{{ t('container.dockerRun') }}</DialogTitle>
      </DialogHeader>

      <div class="flex-1 overflow-y-auto space-y-4 pr-1 py-1">
        <div class="space-y-1.5">
          <Label for="docker-run-cmd">docker run</Label>
          <Textarea
            id="docker-run-cmd"
            v-model="command"
            rows="4"
            :placeholder="t('container.commandPlaceholder')"
            :disabled="isExecuting"
            class="font-mono text-xs leading-relaxed"
          />
          <p class="text-xs text-muted-foreground">{{ t('container.commandHelp') }}</p>
        </div>

        <div v-if="parsedCommand" class="rounded-lg border overflow-hidden">
          <button
            type="button"
            class="w-full flex items-center justify-between px-3.5 py-2.5 bg-muted/40 text-sm font-medium hover:bg-muted/70 transition-colors"
            @click="previewOpen = !previewOpen"
          >
            <span>Preview</span>
            <ChevronDown v-if="previewOpen" class="size-4 text-muted-foreground" />
            <ChevronRight v-else class="size-4 text-muted-foreground" />
          </button>
          <div v-if="previewOpen" class="p-3.5 space-y-2 border-t text-xs">
            <div v-if="parsedCommand.image" class="flex gap-3">
              <span class="min-w-[100px] font-medium text-muted-foreground">{{ t('container.image') }}:</span>
              <span class="font-mono break-all">{{ parsedCommand.image }}</span>
            </div>
            <div v-if="parsedCommand.name" class="flex gap-3">
              <span class="min-w-[100px] font-medium text-muted-foreground">{{ t('container.containerName') }}:</span>
              <span class="font-mono break-all">{{ parsedCommand.name }}</span>
            </div>
            <div v-if="parsedCommand.ports.length" class="flex gap-3">
              <span class="min-w-[100px] font-medium text-muted-foreground">{{ t('container.ports') }}:</span>
              <span class="font-mono break-all">
                {{ parsedCommand.ports.map((p) => `${p.host}:${p.container}`).join(', ') }}
              </span>
            </div>
            <div v-if="parsedCommand.volumes.length" class="flex gap-3">
              <span class="min-w-[100px] font-medium text-muted-foreground">{{ t('container.volumes') }}:</span>
              <span class="font-mono break-all">
                {{ parsedCommand.volumes.map((v) => `${v.host}:${v.container}`).join(', ') }}
              </span>
            </div>
            <div v-if="parsedCommand.envs.length" class="flex gap-3">
              <span class="min-w-[100px] font-medium text-muted-foreground">{{ t('container.envVars') }}:</span>
              <span class="font-mono break-all">
                {{ parsedCommand.envs.map((e) => `${e.key}=${e.value}`).join(', ') }}
              </span>
            </div>
            <div v-if="parsedCommand.restart" class="flex gap-3">
              <span class="min-w-[100px] font-medium text-muted-foreground">{{ t('container.restartPolicy') }}:</span>
              <span class="font-mono">{{ parsedCommand.restart }}</span>
            </div>
            <div v-if="parsedCommand.network" class="flex gap-3">
              <span class="min-w-[100px] font-medium text-muted-foreground">{{ t('container.network') }}:</span>
              <span class="font-mono">{{ parsedCommand.network }}</span>
            </div>
          </div>
        </div>

        <div v-if="output.length > 0" class="rounded-lg border overflow-hidden">
          <div class="px-3.5 py-2 bg-muted/40 border-b text-xs font-medium">
            {{ t('container.output') }}
          </div>
          <div
            ref="outputRef"
            class="max-h-[220px] overflow-y-auto p-3 bg-zinc-950 text-zinc-200 font-mono text-xs leading-relaxed space-y-0.5"
          >
            <div
              v-for="(line, index) in output"
              :key="index"
              class="whitespace-pre-wrap break-all"
              :class="{
                'text-red-400': line.type === 'error',
                'text-emerald-400': line.type === 'success',
              }"
            >
              {{ line.message }}
            </div>
          </div>
        </div>
      </div>

      <DialogFooter class="gap-2 pt-2">
        <Button type="button" variant="outline" :disabled="isExecuting" @click="handleClose">
          {{ t('common.cancel') }}
        </Button>
        <Button :disabled="!command.trim() || isExecuting" @click="handleExecute">
          <Loader2 v-if="isExecuting" class="animate-spin" />
          {{ t('container.execute') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
