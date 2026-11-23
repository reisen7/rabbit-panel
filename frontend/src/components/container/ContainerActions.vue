<script setup lang="ts">
import { ref, watch } from 'vue'
import {
  ChevronDown,
  FileText,
  FolderOpen,
  Loader2,
  Play,
  RotateCw,
  Settings,
  Square,
  Terminal,
  Trash2,
} from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { useConfirm } from '@/composables/useConfirm'
import { useContainerStore } from '@/stores/containers'
import type { ContainerInfo } from '@/types'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

const { t } = useI18n()
const { confirm } = useConfirm()
const containerStore = useContainerStore()

const props = withDefaults(
  defineProps<{
    container: ContainerInfo
    compact?: boolean
  }>(),
  {
    compact: false,
  }
)

const emit = defineEmits<{
  action: [id: string, action: 'start' | 'stop' | 'restart' | 'remove']
  logs: [id: string, name: string]
  terminal: [id: string, name: string]
  files: [id: string, name: string]
  config: [id: string, name: string]
}>()

const actionLoading = ref<string | null>(null)

watch(
  () => containerStore.actionInProgress,
  (inProgress) => {
    if (!inProgress) {
      actionLoading.value = null
    }
  }
)

function handleAction(action: 'start' | 'stop' | 'restart' | 'remove') {
  actionLoading.value = action
  emit('action', props.container.id, action)
}

async function confirmRemove() {
  const ok = await confirm({
    title: t('common.warning'),
    description: t('container.confirmRemove'),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    destructive: true,
  })
  if (ok) {
    handleAction('remove')
  }
}

async function handleCommand(command: string) {
  if (command === 'remove') {
    await confirmRemove()
    return
  }

  switch (command) {
    case 'start':
    case 'stop':
    case 'restart':
      handleAction(command)
      break
    case 'logs':
      handleViewLogs()
      break
    case 'terminal':
      handleOpenTerminal()
      break
    case 'files':
      handleOpenFiles()
      break
    case 'config':
      handleOpenConfig()
      break
  }
}

function handleViewLogs() {
  emit('logs', props.container.id, props.container.name)
}

function handleOpenTerminal() {
  emit('terminal', props.container.id, props.container.name)
}

function handleOpenFiles() {
  emit('files', props.container.id, props.container.name)
}

function handleOpenConfig() {
  emit('config', props.container.id, props.container.name)
}
</script>

<template>
  <TooltipProvider>
    <div class="flex items-center justify-center">
      <!-- Compact mode: dropdown menu -->
      <DropdownMenu v-if="compact">
        <DropdownMenuTrigger as-child>
          <Button variant="outline" size="sm">
            {{ t('common.actions') }}
            <ChevronDown class="size-3.5" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="w-44">
          <DropdownMenuItem v-if="container.state !== 'running'" @click="handleCommand('start')">
            <Play />
            {{ t('container.start') }}
          </DropdownMenuItem>
          <DropdownMenuItem v-if="container.state === 'running'" @click="handleCommand('stop')">
            <Square />
            {{ t('container.stop') }}
          </DropdownMenuItem>
          <DropdownMenuItem @click="handleCommand('restart')">
            <RotateCw />
            {{ t('container.restart') }}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem @click="handleCommand('logs')">
            <FileText />
            {{ t('container.logs') }}
          </DropdownMenuItem>
          <DropdownMenuItem v-if="container.state === 'running'" @click="handleCommand('terminal')">
            <Terminal />
            {{ t('container.terminal') }}
          </DropdownMenuItem>
          <DropdownMenuItem v-if="container.state === 'running'" @click="handleCommand('files')">
            <FolderOpen />
            {{ t('container.files') }}
          </DropdownMenuItem>
          <DropdownMenuItem @click="handleCommand('config')">
            <Settings />
            {{ t('container.config') }}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem class="text-destructive focus:text-destructive" @click="handleCommand('remove')">
            <Trash2 />
            {{ t('common.delete') }}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <!-- Normal mode: compact segmented icon buttons -->
      <div v-else class="inline-flex items-center rounded-md border bg-background shadow-2xs divide-x">
        <Tooltip>
          <TooltipTrigger as-child>
            <span>
              <Button
                variant="ghost"
                size="icon"
                class="rounded-none rounded-l-md text-emerald-600 dark:text-emerald-400"
                :disabled="container.state === 'running' || containerStore.actionInProgress"
                @click="handleAction('start')"
              >
                <Loader2 v-if="actionLoading === 'start'" class="animate-spin" />
                <Play v-else />
              </Button>
            </span>
          </TooltipTrigger>
          <TooltipContent>{{ t('container.start') }}</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger as-child>
            <span>
              <Button
                variant="ghost"
                size="icon"
                class="rounded-none text-amber-600 dark:text-amber-400"
                :disabled="container.state !== 'running' || containerStore.actionInProgress"
                @click="handleAction('stop')"
              >
                <Loader2 v-if="actionLoading === 'stop'" class="animate-spin" />
                <Square v-else />
              </Button>
            </span>
          </TooltipTrigger>
          <TooltipContent>{{ t('container.stop') }}</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger as-child>
            <span>
              <Button
                variant="ghost"
                size="icon"
                class="rounded-none"
                :disabled="containerStore.actionInProgress"
                @click="handleAction('restart')"
              >
                <Loader2 v-if="actionLoading === 'restart'" class="animate-spin" />
                <RotateCw v-else />
              </Button>
            </span>
          </TooltipTrigger>
          <TooltipContent>{{ t('container.restart') }}</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger as-child>
            <Button variant="ghost" size="icon" class="rounded-none" @click="handleViewLogs">
              <FileText />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{{ t('container.logs') }}</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger as-child>
            <span>
              <Button
                variant="ghost"
                size="icon"
                class="rounded-none"
                :disabled="container.state !== 'running'"
                @click="handleOpenTerminal"
              >
                <Terminal />
              </Button>
            </span>
          </TooltipTrigger>
          <TooltipContent>{{ t('container.terminal') }}</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger as-child>
            <span>
              <Button
                variant="ghost"
                size="icon"
                class="rounded-none"
                :disabled="container.state !== 'running'"
                @click="handleOpenFiles"
              >
                <FolderOpen />
              </Button>
            </span>
          </TooltipTrigger>
          <TooltipContent>{{ t('container.files') }}</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger as-child>
            <Button variant="ghost" size="icon" class="rounded-none" @click="handleOpenConfig">
              <Settings />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{{ t('container.config') }}</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger as-child>
            <span>
              <Button
                variant="ghost"
                size="icon"
                class="rounded-none rounded-r-md text-destructive hover:text-destructive"
                :disabled="containerStore.actionInProgress"
                @click="confirmRemove"
              >
                <Loader2 v-if="actionLoading === 'remove'" class="animate-spin" />
                <Trash2 v-else />
              </Button>
            </span>
          </TooltipTrigger>
          <TooltipContent>{{ t('common.delete') }}</TooltipContent>
        </Tooltip>
      </div>
    </div>
  </TooltipProvider>
</template>
