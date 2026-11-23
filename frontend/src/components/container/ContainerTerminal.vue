<script setup lang="ts">
import { ref, computed, watch, onUnmounted, nextTick } from 'vue'
import { AlertTriangle, Loader2, XCircle } from 'lucide-vue-next'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'
import { useI18n } from '@/composables/useI18n'
import { containerApi } from '@/api/containers'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

type ConnectionStatus = 'idle' | 'connecting' | 'connected' | 'disconnected' | 'error'

const props = defineProps<{
  visible: boolean
  containerId: string
  containerName: string
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
}>()

const { t } = useI18n()

const dialogVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
})

const terminalRef = ref<HTMLElement | null>(null)
let terminal: Terminal | null = null
let fitAddon: FitAddon | null = null
let ws: WebSocket | null = null
let resizeObserver: ResizeObserver | null = null

const connectionStatus = ref<ConnectionStatus>('idle')
const errorMessage = ref('')

const statusText = computed(() => {
  switch (connectionStatus.value) {
    case 'connecting':
      return t('container.terminalConnecting')
    case 'disconnected':
      return t('container.terminalDisconnected')
    case 'error':
      return errorMessage.value || t('container.terminalError')
    default:
      return ''
  }
})

function initTerminal() {
  if (!terminalRef.value) return

  terminal = new Terminal({
    cursorBlink: true,
    cursorStyle: 'block',
    fontSize: 14,
    fontFamily: 'Menlo, Monaco, "Courier New", monospace',
    theme: {
      background: '#1e1e1e',
      foreground: '#d4d4d4',
      cursor: '#d4d4d4',
      cursorAccent: '#1e1e1e',
      selectionBackground: '#264f78',
      black: '#000000',
      red: '#cd3131',
      green: '#0dbc79',
      yellow: '#e5e510',
      blue: '#2472c8',
      magenta: '#bc3fbc',
      cyan: '#11a8cd',
      white: '#e5e5e5',
      brightBlack: '#666666',
      brightRed: '#f14c4c',
      brightGreen: '#23d18b',
      brightYellow: '#f5f543',
      brightBlue: '#3b8eea',
      brightMagenta: '#d670d6',
      brightCyan: '#29b8db',
      brightWhite: '#e5e5e5',
    },
    allowProposedApi: true,
  })

  fitAddon = new FitAddon()
  terminal.loadAddon(fitAddon)
  terminal.loadAddon(new WebLinksAddon())

  terminal.open(terminalRef.value)
  fitAddon.fit()

  terminal.onData((data) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(data)
    }
  })

  terminal.onResize(({ cols, rows }) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'resize', cols, rows }))
    }
  })

  resizeObserver = new ResizeObserver(() => {
    if (fitAddon) {
      fitAddon.fit()
    }
  })
  resizeObserver.observe(terminalRef.value)
}

function connect() {
  if (!props.containerId) return

  connectionStatus.value = 'connecting'
  errorMessage.value = ''

  const wsUrl = containerApi.getTerminalWsUrl(props.containerId)
  ws = new WebSocket(wsUrl)

  ws.onopen = () => {
    connectionStatus.value = 'connected'
    terminal?.focus()

    setTimeout(() => {
      if (terminal && ws && ws.readyState === WebSocket.OPEN) {
        ws.send(
          JSON.stringify({
            type: 'resize',
            cols: terminal.cols,
            rows: terminal.rows,
          })
        )
      }
    }, 100)
  }

  ws.onmessage = (event) => {
    if (event.data instanceof Blob) {
      event.data.text().then((text) => {
        terminal?.write(text)
      })
    } else if (typeof event.data === 'string') {
      try {
        const message = JSON.parse(event.data)
        if (message.type === 'error') {
          errorMessage.value = message.data || t('container.terminalError')
          connectionStatus.value = 'error'
          return
        }
      } catch {
        // Not JSON, treat as plain text output
      }
      terminal?.write(event.data)
    }
  }

  ws.onerror = (error) => {
    console.error('[Terminal] WebSocket error:', error)
    errorMessage.value = t('container.terminalError')
    connectionStatus.value = 'error'
  }

  ws.onclose = (event) => {
    console.log('[Terminal] WebSocket closed:', event.code, event.reason)
    if (connectionStatus.value === 'connected') {
      connectionStatus.value = 'disconnected'
    }
    if (!event.wasClean && connectionStatus.value !== 'error') {
      errorMessage.value = t('container.terminalDisconnected')
      connectionStatus.value = 'disconnected'
    }
  }
}

function disconnect() {
  if (ws) {
    ws.close()
    ws = null
  }
}

function reconnect() {
  disconnect()
  terminal?.clear()
  connect()
}

async function handleOpen() {
  await nextTick()
  initTerminal()
  connect()
}

function cleanupTerminal() {
  disconnect()

  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }

  if (terminal) {
    terminal.dispose()
    terminal = null
  }

  fitAddon = null
  connectionStatus.value = 'idle'
}

watch(
  () => props.visible,
  (val) => {
    if (val) {
      handleOpen()
    } else {
      cleanupTerminal()
    }
  },
  { immediate: true }
)

watch(
  () => props.containerId,
  (newId, oldId) => {
    if (newId !== oldId && props.visible) {
      disconnect()
      terminal?.clear()
      connect()
    }
  }
)

onUnmounted(() => {
  cleanupTerminal()
})
</script>

<template>
  <Dialog v-model:open="dialogVisible">
    <DialogContent
      class="sm:max-w-[85vw] max-w-[1100px] p-0 overflow-hidden gap-0"
      @interact-outside.prevent
      @escape-key-down.prevent
    >
      <DialogHeader class="px-5 py-3.5 border-b">
        <DialogTitle>{{ t('container.terminal') + ' - ' + containerName }}</DialogTitle>
      </DialogHeader>

      <div class="relative h-[520px] bg-[#1e1e1e]">
        <div ref="terminalRef" class="size-full p-2.5 box-border terminal-wrapper" />

        <div
          v-if="connectionStatus !== 'connected'"
          class="absolute inset-0 bg-[#1e1e1e]/90 flex items-center justify-center z-10"
        >
          <div class="flex flex-col items-center gap-3 text-center text-zinc-200 px-6">
            <Loader2 v-if="connectionStatus === 'connecting'" class="size-8 animate-spin text-zinc-300" />
            <XCircle v-else-if="connectionStatus === 'disconnected'" class="size-8 text-zinc-400" />
            <AlertTriangle v-else-if="connectionStatus === 'error'" class="size-8 text-red-400" />
            <p class="text-sm text-zinc-300">{{ statusText }}</p>
            <Button
              v-if="connectionStatus === 'disconnected' || connectionStatus === 'error'"
              size="sm"
              @click="reconnect"
            >
              {{ t('container.reconnect') }}
            </Button>
          </div>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>

<style scoped>
.terminal-wrapper :deep(.xterm) {
  height: 100%;
}

.terminal-wrapper :deep(.xterm-viewport) {
  overflow-y: auto !important;
}
</style>
