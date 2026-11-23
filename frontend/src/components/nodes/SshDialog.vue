<script setup lang="ts">
import { ref, computed, watch, onUnmounted, nextTick } from 'vue'
import { AlertTriangle, ArrowLeft, Folder, Loader2, TerminalSquare, Trash2, X, XCircle } from 'lucide-vue-next'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'
import { toast } from 'vue-sonner'
import { useI18n } from '@/composables/useI18n'
import { sshApi, type SshCredential, type SshConnParams } from '@/api/ssh'
import SshFileExplorer from '@/components/nodes/SshFileExplorer.vue'
import type { NodeInfo } from '@/types'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

type ConnectionStatus = 'idle' | 'connecting' | 'connected' | 'disconnected' | 'error'

interface ConnectPayload {
  type: 'connect'
  credential_id?: number
  host?: string
  port?: number
  username?: string
  password?: string
  cols: number
  rows: number
}

const props = defineProps<{
  visible: boolean
  node: NodeInfo | null
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
}>()

const { t } = useI18n()

const dialogVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
})

// ---- 连接表单 ----
const view = ref<'form' | 'terminal'>('form')
const host = ref('')
const port = ref<number>(22)
const username = ref('root')
const password = ref('')
const saveCredential = ref(false)
const connecting = ref(false)

const credentials = ref<SshCredential[]>([])
const selectedCredentialId = ref<string>('none')
const deleteConfirmOpen = ref(false)
const deleting = ref(false)
let lastPayload: ConnectPayload | null = null

const selectedCredential = computed(() =>
  credentials.value.find((c) => String(c.id) === selectedCredentialId.value) ?? null
)

const canConnect = computed(() => {
  if (selectedCredential.value) return true
  return host.value.trim() !== '' && username.value.trim() !== ''
})

function defaultHost(): string {
  const addr = props.node?.address ?? ''
  const h = addr.split(':')[0] ?? ''
  // Master 地址常是 127.0.0.1（容器内回环），用浏览器访问的主机名更靠谱
  if (h === '127.0.0.1' || h === 'localhost' || h === '') {
    return window.location.hostname
  }
  return h
}

async function loadCredentials() {
  if (!props.node) return
  try {
    credentials.value = await sshApi.listCredentials(props.node.id)
  } catch {
    credentials.value = []
  }
}

function handleCredentialChange(value: unknown) {
  selectedCredentialId.value = String(value)
  const cred = selectedCredential.value
  if (cred) {
    host.value = cred.host
    port.value = cred.port
    username.value = cred.username
    password.value = ''
  }
}

async function handleDeleteCredential() {
  const cred = selectedCredential.value
  if (!cred) return
  deleting.value = true
  try {
    await sshApi.deleteCredential(cred.id)
    credentials.value = credentials.value.filter((c) => c.id !== cred.id)
    selectedCredentialId.value = 'none'
    toast.success(t('ssh.credentialDeleted'))
  } catch {
    // request.ts 已弹错
  } finally {
    deleting.value = false
    deleteConfirmOpen.value = false
  }
}

// ---- 终端 ----
const terminalRef = ref<HTMLElement | null>(null)
let terminal: Terminal | null = null
let fitAddon: FitAddon | null = null
let ws: WebSocket | null = null
let resizeObserver: ResizeObserver | null = null

const connectionStatus = ref<ConnectionStatus>('idle')
const errorMessage = ref('')
const activeTab = ref<'terminal' | 'files'>('terminal')

// SFTP 与终端共用同一套连接参数（凭证ID 或表单明文参数）
const sftpConn = computed<SshConnParams>(() => {
  const cred = selectedCredential.value
  if (cred) return { credential_id: cred.id }
  return {
    host: host.value.trim(),
    port: port.value || 22,
    username: username.value.trim(),
    password: password.value,
  }
})

const statusText = computed(() => {
  switch (connectionStatus.value) {
    case 'connecting':
      return t('ssh.connecting')
    case 'disconnected':
      return t('ssh.disconnected')
    case 'error':
      return errorMessage.value || t('ssh.connectFailed')
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
    if (ws && ws.readyState === WebSocket.OPEN) ws.send(data)
  })
  terminal.onResize(({ cols, rows }) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'resize', cols, rows }))
    }
  })

  resizeObserver = new ResizeObserver(() => fitAddon?.fit())
  resizeObserver.observe(terminalRef.value)
}

function buildPayload(cols: number, rows: number): ConnectPayload {
  const base: ConnectPayload = { type: 'connect', cols, rows }
  const cred = selectedCredential.value
  if (cred) {
    base.credential_id = cred.id
  } else {
    base.host = host.value.trim()
    base.port = port.value || 22
    base.username = username.value.trim()
    base.password = password.value
  }
  return base
}

function openSocket(payload: ConnectPayload) {
  connectionStatus.value = 'connecting'
  errorMessage.value = ''

  ws = new WebSocket(sshApi.getTerminalWsUrl())
  ws.onopen = () => {
    ws?.send(JSON.stringify(payload))
    // SSH 握手期间保持 connecting，等首条输出或一段时间后再认为已连接
    setTimeout(() => {
      if (connectionStatus.value === 'connecting') {
        connectionStatus.value = 'connected'
        terminal?.focus()
      }
    }, 1200)
  }
  ws.onmessage = (event) => {
    if (event.data instanceof Blob) {
      event.data.text().then((text) => terminal?.write(text))
    } else if (typeof event.data === 'string') {
      terminal?.write(event.data)
    }
  }
  ws.onerror = () => {
    errorMessage.value = t('ssh.connectFailed')
    connectionStatus.value = 'error'
  }
  ws.onclose = (event) => {
    if (connectionStatus.value === 'connected' || connectionStatus.value === 'connecting') {
      connectionStatus.value = 'disconnected'
    }
    if (!event.wasClean && connectionStatus.value !== 'error') {
      errorMessage.value = t('ssh.disconnected')
      connectionStatus.value = 'disconnected'
    }
  }
}

async function handleConnect() {
  if (!canConnect.value || connecting.value) return
  connecting.value = true
  try {
    // 需要保存凭证时先落库（不影响本次连接，仍用表单里的密码直连）
    if (saveCredential.value && props.node) {
      try {
        const saved = await sshApi.saveCredential({
          node_id: props.node.id,
          host: host.value.trim(),
          port: port.value || 22,
          username: username.value.trim(),
          password: password.value,
        })
        const idx = credentials.value.findIndex((c) => c.id === saved.id)
        if (idx >= 0) {
          credentials.value[idx] = saved
        } else {
          credentials.value.unshift(saved)
        }
        toast.success(t('ssh.credentialSaved'))
      } catch {
        // 保存失败不阻断连接，request.ts 已提示
      }
    }
    view.value = 'terminal'
    await nextTick()
    if (!terminal) initTerminal()
    fitAddon?.fit()
    const cols = terminal?.cols ?? 80
    const rows = terminal?.rows ?? 24
    lastPayload = buildPayload(cols, rows)
    openSocket(lastPayload)
  } finally {
    connecting.value = false
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
  if (lastPayload) openSocket(lastPayload)
}

function backToForm() {
  disconnect()
  view.value = 'form'
  activeTab.value = 'terminal'
  connectionStatus.value = 'idle'
  errorMessage.value = ''
}

function cleanup() {
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
  view.value = 'form'
  activeTab.value = 'terminal'
  lastPayload = null
}

watch(
  () => props.visible,
  (val) => {
    if (val) {
      view.value = 'form'
      selectedCredentialId.value = 'none'
      host.value = defaultHost()
      port.value = 22
      username.value = 'root'
      password.value = ''
      saveCredential.value = false
      loadCredentials()
    } else {
      cleanup()
    }
  },
  { immediate: true }
)

onUnmounted(cleanup)
</script>

<template>
  <Dialog v-model:open="dialogVisible">
    <DialogContent
      :class="view === 'terminal' ? 'sm:max-w-[85vw] max-w-[1100px] p-0 overflow-hidden gap-0' : 'sm:max-w-[440px]'"
      :show-close-button="view !== 'terminal'"
      @interact-outside.prevent
      @escape-key-down.prevent
    >
      <!-- ===== 连接表单 ===== -->
      <template v-if="view === 'form'">
        <DialogHeader>
          <DialogTitle>{{ t('ssh.title') }} — {{ node?.name }}</DialogTitle>
          <DialogDescription>{{ t('ssh.description') }}</DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-1">
          <!-- 已保存用户 -->
          <div class="space-y-1.5">
            <Label>{{ t('ssh.savedUsers') }}</Label>
            <div class="flex items-center gap-2">
              <Select
                :model-value="selectedCredentialId"
                @update:model-value="handleCredentialChange"
              >
                <SelectTrigger class="flex-1 bg-background">
                  <SelectValue :placeholder="t('ssh.selectCredential')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">{{ t('ssh.noCredential') }}</SelectItem>
                  <SelectItem v-for="c in credentials" :key="c.id" :value="String(c.id)">
                    {{ c.username }}@{{ c.host }}:{{ c.port }}
                  </SelectItem>
                </SelectContent>
              </Select>
              <Button
                v-if="selectedCredential"
                variant="outline"
                size="icon"
                :title="t('ssh.deleteCredential')"
                :disabled="deleting"
                @click="deleteConfirmOpen = true"
              >
                <Trash2 class="size-4 text-destructive" />
              </Button>
            </div>
          </div>

          <div class="grid grid-cols-[1fr_110px] gap-3">
            <div class="space-y-1.5">
              <Label for="ssh-host">{{ t('ssh.host') }}</Label>
              <Input
                id="ssh-host"
                v-model="host"
                :placeholder="t('ssh.hostPlaceholder')"
                :disabled="!!selectedCredential"
                class="bg-background"
              />
            </div>
            <div class="space-y-1.5">
              <Label for="ssh-port">{{ t('ssh.port') }}</Label>
              <Input
                id="ssh-port"
                v-model.number="port"
                type="number"
                min="1"
                max="65535"
                :disabled="!!selectedCredential"
                class="bg-background"
              />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div class="space-y-1.5">
              <Label for="ssh-username">{{ t('ssh.username') }}</Label>
              <Input
                id="ssh-username"
                v-model="username"
                :placeholder="t('ssh.usernamePlaceholder')"
                :disabled="!!selectedCredential"
                class="bg-background"
              />
            </div>
            <div class="space-y-1.5">
              <Label for="ssh-password">{{ t('ssh.password') }}</Label>
              <Input
                id="ssh-password"
                v-model="password"
                type="password"
                :placeholder="selectedCredential ? t('ssh.passwordSaved') : t('ssh.passwordPlaceholder')"
                :disabled="!!selectedCredential"
                class="bg-background"
                @keyup.enter="handleConnect"
              />
            </div>
          </div>

          <div class="flex items-center gap-2">
            <Checkbox id="ssh-save" v-model="saveCredential" :disabled="!!selectedCredential" />
            <Label for="ssh-save" class="text-sm font-normal cursor-pointer">
              {{ t('ssh.saveCredential') }}
            </Label>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="dialogVisible = false">{{ t('common.cancel') }}</Button>
          <Button :disabled="!canConnect || connecting" @click="handleConnect">
            <Loader2 v-if="connecting" class="size-4 animate-spin" />
            {{ t('ssh.connect') }}
          </Button>
        </DialogFooter>
      </template>

      <!-- ===== 终端 ===== -->
      <template v-else>
        <DialogHeader class="flex-row items-center justify-between space-y-0 px-5 py-3.5 border-b">
          <div class="flex items-center gap-2">
            <Button
              variant="ghost"
              size="icon"
              class="size-8 -my-1 shrink-0 text-muted-foreground hover:text-foreground"
              :title="t('ssh.back')"
              @click="backToForm"
            >
              <ArrowLeft class="size-4" />
            </Button>
            <DialogTitle>
              {{ t('ssh.title') }} — {{ username }}@{{ host }}:{{ port }}
            </DialogTitle>
            <!-- 终端 / 文件 页签 -->
            <div class="flex items-center rounded-md border bg-muted/40 p-0.5 ml-2">
              <button
                type="button"
                class="flex items-center gap-1.5 rounded px-2.5 py-1 text-xs transition-colors"
                :class="activeTab === 'terminal' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'"
                @click="activeTab = 'terminal'"
              >
                <TerminalSquare class="size-3.5" />
                {{ t('ssh.tabTerminal') }}
              </button>
              <button
                type="button"
                class="flex items-center gap-1.5 rounded px-2.5 py-1 text-xs transition-colors"
                :class="activeTab === 'files' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'"
                @click="activeTab = 'files'"
              >
                <Folder class="size-3.5" />
                {{ t('ssh.tabFiles') }}
              </button>
            </div>
          </div>
          <DialogClose as-child>
            <Button
              variant="ghost"
              size="icon"
              class="size-8 -my-1 shrink-0 text-muted-foreground hover:text-foreground"
            >
              <X class="size-4" />
              <span class="sr-only">{{ t('common.close') }}</span>
            </Button>
          </DialogClose>
        </DialogHeader>

        <div v-show="activeTab === 'terminal'" class="relative h-[520px] bg-[#1e1e1e]">
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
              <div class="flex items-center gap-2">
                <Button
                  v-if="connectionStatus === 'disconnected' || connectionStatus === 'error'"
                  size="sm"
                  @click="reconnect"
                >
                  {{ t('container.reconnect') }}
                </Button>
                <Button
                  v-if="connectionStatus !== 'connecting'"
                  size="sm"
                  variant="outline"
                  @click="backToForm"
                >
                  {{ t('ssh.back') }}
                </Button>
              </div>
            </div>
          </div>
        </div>

        <!-- 文件管理页签：独立 SFTP 会话，终端断开也能用 -->
        <div v-show="activeTab === 'files'" class="h-[520px] p-4 bg-background">
          <SshFileExplorer :conn="sftpConn" :active="activeTab === 'files'" />
        </div>
      </template>
    </DialogContent>

    <!-- 删除凭证确认 -->
    <AlertDialog v-model:open="deleteConfirmOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ t('ssh.deleteCredential') }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ t('ssh.deleteConfirm').replace('{user}', selectedCredential?.username + '@' + selectedCredential?.host) }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ t('common.cancel') }}</AlertDialogCancel>
          <AlertDialogAction @click="handleDeleteCredential">
            {{ t('common.delete') }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
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
