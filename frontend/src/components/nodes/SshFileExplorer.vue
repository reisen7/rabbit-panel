<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import {
  ArrowLeft,
  ChevronRight,
  Download,
  Edit3,
  File as FileIcon,
  Folder,
  FolderPlus,
  Link2,
  Loader2,
  Pencil,
  RefreshCw,
  Save,
  Trash2,
  Upload,
} from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { sshApi, type SshConnParams } from '@/api/ssh'
import type { FileInfo } from '@/types'
import LoadingOverlay from '@/components/common/LoadingOverlay.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

const MAX_UPLOAD_SIZE = 10 * 1024 * 1024 // 10MB

const props = defineProps<{
  /** SSH 连接参数（credential_id 或明文 host/port/username/password），由父组件注入 */
  conn: SshConnParams
  /** 页签激活时才拉取数据，切换回来保持状态 */
  active: boolean
}>()

const { t } = useI18n()
const toast = useToast()
const { confirm } = useConfirm()

const currentPath = ref('/')
const files = ref<FileInfo[]>([])
const loading = ref(false)
const loaded = ref(false)

const showCreateDirDialog = ref(false)
const showRenameDialog = ref(false)
const newDirName = ref('')
const creating = ref(false)
const renameTarget = ref<FileInfo | null>(null)
const renameValue = ref('')
const renaming = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)
const uploading = ref(false)
const downloadingPath = ref<string | null>(null)

// 在线编辑
const editingFile = ref<{ path: string; name: string; content: string } | null>(null)
const editLoading = ref(false)
const saving = ref(false)

const PROTECTED_PATHS = ['/', '/bin', '/sbin', '/usr', '/lib', '/lib64', '/etc', '/var', '/root', '/home', '/boot', '/dev', '/proc', '/sys']

const pathSegments = computed(() => {
  const segments = currentPath.value.split('/').filter(Boolean)
  return ['', ...segments]
})

async function fetchFiles() {
  loading.value = true
  try {
    files.value = await sshApi.sftpList(props.conn, currentPath.value)
    files.value.sort((a, b) => {
      if (a.is_dir && !b.is_dir) return -1
      if (!a.is_dir && b.is_dir) return 1
      return a.name.localeCompare(b.name)
    })
    loaded.value = true
  } catch {
    toast.error(t('file.loadError'))
  } finally {
    loading.value = false
  }
}

function navigateTo(path: string) {
  currentPath.value = path || '/'
  editingFile.value = null
  fetchFiles()
}

function navigateToSegment(index: number) {
  if (index >= pathSegments.value.length - 1) return
  navigateTo('/' + pathSegments.value.slice(1, index + 1).join('/'))
}

function handleRowDblClick(row: FileInfo) {
  if (row.is_dir) {
    navigateTo(row.path)
  } else if (isTextFile(row.name)) {
    handleEdit(row)
  }
}

function handleFileClick(row: FileInfo) {
  if (row.is_dir) navigateTo(row.path)
}

async function handleCreateDir() {
  const name = newDirName.value.trim()
  if (!name) {
    toast.warning(t('file.dirNameRequired'))
    return
  }
  creating.value = true
  try {
    const p = currentPath.value === '/' ? '/' + name : currentPath.value + '/' + name
    await sshApi.sftpMkdir(props.conn, p)
    toast.success(t('file.createSuccess'))
    showCreateDirDialog.value = false
    newDirName.value = ''
    fetchFiles()
  } catch {
    // Error handled by interceptor
  } finally {
    creating.value = false
  }
}

function openRename(row: FileInfo) {
  renameTarget.value = row
  renameValue.value = row.name
  showRenameDialog.value = true
}

async function handleRename() {
  const row = renameTarget.value
  const name = renameValue.value.trim()
  if (!row || !name || name === row.name) {
    showRenameDialog.value = false
    return
  }
  renaming.value = true
  try {
    const dir = row.path.slice(0, row.path.length - row.name.length) || '/'
    await sshApi.sftpRename(props.conn, row.path, dir + name)
    toast.success(t('file.renameSuccess'))
    showRenameDialog.value = false
    fetchFiles()
  } catch {
    // Error handled by interceptor
  } finally {
    renaming.value = false
  }
}

async function handleDelete(file: FileInfo) {
  if (isProtectedPath(file.path)) {
    toast.error(t('file.protectedPath'))
    return
  }
  const ok = await confirm({
    title: t('common.confirm'),
    description: file.is_dir
      ? t('file.confirmDeleteDir')
      : t('file.confirmDelete'),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    destructive: true,
  })
  if (!ok) return
  try {
    await sshApi.sftpDelete(props.conn, file.path)
    toast.success(t('file.deleteSuccess'))
    fetchFiles()
  } catch {
    // Error handled by interceptor
  }
}

async function handleDownload(file: FileInfo) {
  downloadingPath.value = file.path
  try {
    const blob = await sshApi.sftpDownload(props.conn, file.path)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = file.name
    a.click()
    URL.revokeObjectURL(url)
  } catch {
    // Error handled by interceptor
  } finally {
    downloadingPath.value = null
  }
}

async function handleEdit(file: FileInfo) {
  editLoading.value = true
  try {
    const content = await sshApi.sftpRead(props.conn, file.path)
    editingFile.value = { path: file.path, name: file.name, content }
  } catch {
    // Error handled by interceptor
  } finally {
    editLoading.value = false
  }
}

async function handleSaveEdit() {
  const file = editingFile.value
  if (!file) return
  saving.value = true
  try {
    await sshApi.sftpWrite(props.conn, file.path, file.content)
    toast.success(t('file.saveSuccess'))
  } catch {
    // Error handled by interceptor
  } finally {
    saving.value = false
  }
}

function triggerUpload() {
  fileInputRef.value?.click()
}

async function handleFileSelect(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return

  if (file.size > MAX_UPLOAD_SIZE) {
    toast.error(t('file.fileTooLarge'))
    input.value = ''
    return
  }

  uploading.value = true
  try {
    const base64 = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = () => resolve((reader.result as string).split(',')[1] || '')
      reader.onerror = reject
      reader.readAsDataURL(file)
    })
    await sshApi.sftpUpload(props.conn, currentPath.value, file.name, base64)
    toast.success(t('file.uploadSuccess'))
    fetchFiles()
  } catch {
    // Error handled by interceptor
  } finally {
    uploading.value = false
    input.value = ''
  }
}

function isProtectedPath(path: string): boolean {
  return PROTECTED_PATHS.includes(path)
}

function isTextFile(filename: string): boolean {
  const textExtensions = [
    '.txt', '.md', '.json', '.xml', '.yml', '.yaml', '.toml', '.ini', '.cfg', '.conf',
    '.sh', '.bash', '.zsh', '.fish', '.ps1', '.bat', '.cmd',
    '.js', '.ts', '.jsx', '.tsx', '.vue', '.svelte',
    '.py', '.rb', '.php', '.go', '.rs', '.java', '.kt', '.scala', '.c', '.cpp', '.h', '.hpp',
    '.html', '.htm', '.css', '.scss', '.sass', '.less',
    '.sql', '.graphql', '.gql',
    '.env', '.gitignore', '.dockerignore', '.editorconfig',
    '.log', '.csv',
  ]
  const lowerName = filename.toLowerCase()
  return textExtensions.some((ext) => lowerName.endsWith(ext)) || !filename.includes('.')
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// 页签首次激活时加载；连接参数变化时回到根目录重载
watch(
  () => props.active,
  (val) => {
    if (val && !loaded.value) fetchFiles()
  },
  { immediate: true }
)

watch(
  () => props.conn,
  () => {
    loaded.value = false
    currentPath.value = '/'
    editingFile.value = null
    if (props.active) fetchFiles()
  }
)
</script>

<template>
  <div class="relative flex flex-col gap-3 size-full min-h-0">
    <LoadingOverlay :loading="loading || uploading || editLoading" />

    <!-- 文件编辑视图 -->
    <template v-if="editingFile">
      <div class="flex items-center justify-between gap-2 shrink-0">
        <div class="flex items-center gap-2 min-w-0">
          <Button variant="ghost" size="icon-sm" class="shrink-0" @click="editingFile = null">
            <ArrowLeft class="size-4" />
          </Button>
          <span class="font-mono text-xs text-muted-foreground truncate">{{ editingFile.path }}</span>
        </div>
        <div class="flex items-center gap-2 shrink-0">
          <Button variant="outline" size="sm" @click="editingFile = null">{{ t('common.cancel') }}</Button>
          <Button size="sm" :disabled="saving" @click="handleSaveEdit">
            <Loader2 v-if="saving" class="animate-spin" />
            <Save v-else />
            {{ t('common.save') }}
          </Button>
        </div>
      </div>
      <Textarea
        v-model="editingFile.content"
        class="flex-1 min-h-0 resize-none font-mono text-xs leading-relaxed bg-zinc-950 text-zinc-100"
        spellcheck="false"
      />
    </template>

    <!-- 文件浏览视图 -->
    <template v-else>
      <!-- Toolbar -->
      <div class="flex flex-wrap items-center justify-between gap-2 rounded-lg border bg-muted/40 px-3 py-2 shrink-0">
        <nav class="flex flex-wrap items-center gap-1 text-sm min-w-0">
          <template v-for="(segment, index) in pathSegments" :key="index">
            <ChevronRight v-if="index > 0" class="size-3.5 text-muted-foreground shrink-0" />
            <button
              type="button"
              class="rounded px-1.5 py-0.5 text-xs font-mono transition-colors"
              :class="index < pathSegments.length - 1
                ? 'text-primary hover:bg-accent cursor-pointer'
                : 'text-foreground font-semibold cursor-default'"
              @click="navigateToSegment(index)"
            >
              {{ segment || '/' }}
            </button>
          </template>
        </nav>

        <div class="flex items-center gap-2 shrink-0">
          <Button variant="outline" size="icon-sm" @click="fetchFiles">
            <RefreshCw :class="{ 'animate-spin': loading }" />
          </Button>
          <Button size="sm" variant="outline" @click="showCreateDirDialog = true">
            <FolderPlus />
            {{ t('file.createDir') }}
          </Button>
          <Button variant="outline" size="sm" :disabled="uploading" @click="triggerUpload">
            <Loader2 v-if="uploading" class="animate-spin" />
            <Upload v-else />
            {{ t('file.upload') }}
          </Button>
          <input ref="fileInputRef" type="file" class="hidden" @change="handleFileSelect" />
        </div>
      </div>

      <!-- File table -->
      <div class="relative flex-1 min-h-0 overflow-auto rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead class="w-10" />
              <TableHead>{{ t('common.name') }}</TableHead>
              <TableHead class="w-24">{{ t('common.size') }}</TableHead>
              <TableHead class="w-28 hidden lg:table-cell">{{ t('file.permissions') }}</TableHead>
              <TableHead class="w-40 hidden xl:table-cell">{{ t('file.modTime') }}</TableHead>
              <TableHead class="w-40 text-right">{{ t('common.actions') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-if="files.length === 0 && !loading">
              <TableCell colspan="6" class="h-24 text-center text-muted-foreground">
                {{ t('common.none') }}
              </TableCell>
            </TableRow>
            <TableRow
              v-for="row in files"
              :key="row.path"
              class="cursor-pointer"
              @dblclick="handleRowDblClick(row)"
            >
              <TableCell>
                <span class="relative inline-flex">
                  <Folder v-if="row.is_dir" class="size-4 text-amber-500" />
                  <FileIcon v-else class="size-4 text-muted-foreground" />
                  <Link2 v-if="row.is_link" class="absolute -right-1.5 -bottom-1 size-2.5 text-blue-400" />
                </span>
              </TableCell>
              <TableCell>
                <button
                  type="button"
                  class="text-left font-medium hover:underline"
                  :class="row.is_dir ? 'text-primary' : 'text-foreground'"
                  @click="handleFileClick(row)"
                >
                  {{ row.name }}
                </button>
              </TableCell>
              <TableCell class="text-xs tabular-nums text-muted-foreground">
                {{ row.is_dir ? '-' : formatSize(row.size) }}
              </TableCell>
              <TableCell class="hidden lg:table-cell font-mono text-xs text-muted-foreground">
                {{ row.mode }}
              </TableCell>
              <TableCell class="hidden xl:table-cell text-xs text-muted-foreground">
                {{ row.mod_time }}
              </TableCell>
              <TableCell class="text-right">
                <div class="inline-flex items-center gap-0.5">
                  <Button
                    v-if="!row.is_dir"
                    variant="ghost"
                    size="icon-sm"
                    :title="t('file.download')"
                    :disabled="downloadingPath === row.path"
                    @click.stop="handleDownload(row)"
                  >
                    <Loader2 v-if="downloadingPath === row.path" class="size-4 animate-spin" />
                    <Download v-else class="size-4" />
                  </Button>
                  <Button
                    v-if="!row.is_dir && isTextFile(row.name)"
                    variant="ghost"
                    size="icon-sm"
                    :title="t('file.edit')"
                    @click.stop="handleEdit(row)"
                  >
                    <Edit3 class="size-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    :title="t('file.rename')"
                    @click.stop="openRename(row)"
                  >
                    <Pencil class="size-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    class="text-destructive hover:text-destructive"
                    :title="t('file.delete')"
                    :disabled="isProtectedPath(row.path)"
                    @click.stop="handleDelete(row)"
                  >
                    <Trash2 class="size-4" />
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
    </template>

    <!-- 新建目录 -->
    <Dialog v-model:open="showCreateDirDialog">
      <DialogContent class="sm:max-w-[400px]">
        <DialogHeader>
          <DialogTitle>{{ t('file.createDir') }}</DialogTitle>
        </DialogHeader>
        <form class="space-y-4 py-1" @submit.prevent="handleCreateDir">
          <div class="space-y-1.5">
            <Label for="sftp-new-dir">{{ t('file.dirName') }}</Label>
            <Input id="sftp-new-dir" v-model="newDirName" :placeholder="t('file.dirNamePlaceholder')" />
          </div>
          <DialogFooter class="gap-2">
            <Button type="button" variant="outline" @click="showCreateDirDialog = false">
              {{ t('common.cancel') }}
            </Button>
            <Button type="submit" :disabled="creating">
              <Loader2 v-if="creating" class="animate-spin" />
              {{ t('common.create') }}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>

    <!-- 重命名 -->
    <Dialog v-model:open="showRenameDialog">
      <DialogContent class="sm:max-w-[400px]">
        <DialogHeader>
          <DialogTitle>{{ t('file.rename') }}</DialogTitle>
        </DialogHeader>
        <form class="space-y-4 py-1" @submit.prevent="handleRename">
          <div class="space-y-1.5">
            <Label for="sftp-rename">{{ t('file.newName') }}</Label>
            <Input id="sftp-rename" v-model="renameValue" />
          </div>
          <DialogFooter class="gap-2">
            <Button type="button" variant="outline" @click="showRenameDialog = false">
              {{ t('common.cancel') }}
            </Button>
            <Button type="submit" :disabled="renaming">
              <Loader2 v-if="renaming" class="animate-spin" />
              {{ t('common.confirm') }}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  </div>
</template>
