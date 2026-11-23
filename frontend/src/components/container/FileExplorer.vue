<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import {
  ChevronRight,
  Download,
  Edit3,
  File as FileIcon,
  Folder,
  FolderPlus,
  Loader2,
  RefreshCw,
  Trash2,
  Upload,
} from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { containerApi } from '@/api/containers'
import type { FileInfo } from '@/types'
import FileEditor from './FileEditor.vue'
import LoadingOverlay from '@/components/common/LoadingOverlay.vue'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
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
  visible: boolean
  containerId: string
  containerName: string
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
}>()

const { t } = useI18n()
const toast = useToast()
const { confirm } = useConfirm()

const dialogVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
})

const currentPath = ref('/')
const files = ref<FileInfo[]>([])
const loading = ref(false)
const creating = ref(false)
const showCreateDirDialog = ref(false)
const showEditorDialog = ref(false)
const newDirName = ref('')
const editingFilePath = ref('')
const editingFileName = ref('')
const fileInputRef = ref<HTMLInputElement | null>(null)

const PROTECTED_PATHS = ['/', '/bin', '/sbin', '/usr', '/lib', '/lib64', '/etc', '/var', '/root', '/home']

const pathSegments = computed(() => {
  const segments = currentPath.value.split('/').filter(Boolean)
  return ['', ...segments]
})

async function fetchFiles() {
  if (!props.containerId) return

  loading.value = true
  try {
    files.value = await containerApi.filesList(props.containerId, currentPath.value)
    files.value.sort((a, b) => {
      if (a.is_dir && !b.is_dir) return -1
      if (!a.is_dir && b.is_dir) return 1
      return a.name.localeCompare(b.name)
    })
  } catch {
    toast.error(t('file.loadError'))
  } finally {
    loading.value = false
  }
}

function navigateToSegment(index: number) {
  if (index >= pathSegments.value.length - 1) return

  const newPath = '/' + pathSegments.value.slice(1, index + 1).join('/')
  currentPath.value = newPath || '/'
  fetchFiles()
}

function handleRowDblClick(row: FileInfo) {
  if (row.is_dir) {
    currentPath.value = row.path
    fetchFiles()
  } else if (isTextFile(row.name)) {
    handleEdit(row)
  }
}

function handleFileClick(row: FileInfo) {
  if (row.is_dir) {
    currentPath.value = row.path
    fetchFiles()
  }
}

function handleRefresh() {
  fetchFiles()
}

async function handleCreateDir() {
  if (!newDirName.value.trim()) {
    toast.warning(t('file.dirNameRequired'))
    return
  }

  creating.value = true
  try {
    const path =
      currentPath.value === '/'
        ? '/' + newDirName.value.trim()
        : currentPath.value + '/' + newDirName.value.trim()
    await containerApi.fileMkdir(props.containerId, path)
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

async function handleDelete(file: FileInfo) {
  if (isProtectedPath(file.path)) {
    toast.error(t('file.protectedPath'))
    return
  }

  const ok = await confirm({
    title: t('common.confirm'),
    description: t('file.confirmDelete'),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    destructive: true,
  })
  if (!ok) return

  try {
    await containerApi.fileDelete(props.containerId, file.path)
    toast.success(t('file.deleteSuccess'))
    fetchFiles()
  } catch {
    // Error handled by interceptor
  }
}

function handleDownload(file: FileInfo) {
  const url = containerApi.fileDownload(props.containerId, file.path)
  window.open(url, '_blank')
}

function handleEdit(file: FileInfo) {
  editingFilePath.value = file.path
  editingFileName.value = file.name
  showEditorDialog.value = true
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

  try {
    const reader = new FileReader()
    reader.onload = async () => {
      const result = reader.result as string
      const base64 = result.split(',')[1] || ''
      await containerApi.fileUpload(props.containerId, currentPath.value, file.name, base64)
      toast.success(t('file.uploadSuccess'))
      fetchFiles()
    }
    reader.readAsDataURL(file)
  } catch {
    // Error handled by interceptor
  } finally {
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

function formatTime(time: string): string {
  if (!time) return '-'
  return new Date(time).toLocaleString()
}

watch(
  () => props.visible,
  (val) => {
    if (val) {
      currentPath.value = '/'
      fetchFiles()
    }
  },
  { immediate: true }
)

watch(
  () => props.containerId,
  () => {
    if (props.visible) {
      currentPath.value = '/'
      fetchFiles()
    }
  }
)
</script>

<template>
  <Dialog v-model:open="dialogVisible">
    <DialogContent class="sm:max-w-[80vw] max-w-[1100px] max-h-[88vh] flex flex-col" @interact-outside.prevent>
      <DialogHeader>
        <DialogTitle>{{ t('container.files') + ' - ' + containerName }}</DialogTitle>
      </DialogHeader>

      <div class="flex flex-col gap-3 flex-1 min-h-0">
        <!-- Toolbar -->
        <div class="flex flex-wrap items-center justify-between gap-2 rounded-lg border bg-muted/40 px-3 py-2">
          <nav class="flex flex-wrap items-center gap-1 text-sm">
            <template v-for="(segment, index) in pathSegments" :key="index">
              <ChevronRight v-if="index > 0" class="size-3.5 text-muted-foreground" />
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

          <div class="flex items-center gap-2">
            <Button variant="outline" size="icon-sm" @click="handleRefresh">
              <RefreshCw :class="{ 'animate-spin': loading }" />
            </Button>
            <Button size="sm" @click="showCreateDirDialog = true">
              <FolderPlus />
              {{ t('file.createDir') }}
            </Button>
            <Button variant="outline" size="sm" @click="triggerUpload">
              <Upload />
              {{ t('file.upload') }}
            </Button>
            <input ref="fileInputRef" type="file" class="hidden" @change="handleFileSelect" />
          </div>
        </div>

        <!-- File table -->
        <div class="relative flex-1 min-h-[340px] max-h-[55vh] overflow-auto rounded-md border">
          <LoadingOverlay :loading="loading" />
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead class="w-10" />
                <TableHead>{{ t('common.name') }}</TableHead>
                <TableHead class="w-28">{{ t('common.size') }}</TableHead>
                <TableHead class="w-28 hidden sm:table-cell">{{ t('file.permissions') }}</TableHead>
                <TableHead class="w-44 hidden md:table-cell">{{ t('file.modTime') }}</TableHead>
                <TableHead class="w-36 text-right">{{ t('common.actions') }}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-if="files.length === 0">
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
                  <Folder v-if="row.is_dir" class="size-4 text-amber-500" />
                  <FileIcon v-else class="size-4 text-muted-foreground" />
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
                <TableCell class="hidden sm:table-cell font-mono text-xs text-muted-foreground">
                  {{ row.mode }}
                </TableCell>
                <TableCell class="hidden md:table-cell text-xs text-muted-foreground">
                  {{ formatTime(row.mod_time) }}
                </TableCell>
                <TableCell class="text-right">
                  <div class="inline-flex items-center gap-1">
                    <Button
                      v-if="!row.is_dir"
                      variant="ghost"
                      size="icon-sm"
                      @click.stop="handleDownload(row)"
                    >
                      <Download class="size-4" />
                    </Button>
                    <Button
                      v-if="!row.is_dir && isTextFile(row.name)"
                      variant="ghost"
                      size="icon-sm"
                      @click.stop="handleEdit(row)"
                    >
                      <Edit3 class="size-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      class="text-destructive hover:text-destructive"
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
      </div>

      <!-- Create Directory Dialog -->
      <Dialog v-model:open="showCreateDirDialog">
        <DialogContent class="sm:max-w-[400px]">
          <DialogHeader>
            <DialogTitle>{{ t('file.createDir') }}</DialogTitle>
          </DialogHeader>
          <form class="space-y-4 py-1" @submit.prevent="handleCreateDir">
            <div class="space-y-1.5">
              <Label for="new-dir-name">{{ t('file.dirName') }}</Label>
              <Input
                id="new-dir-name"
                v-model="newDirName"
                :placeholder="t('file.dirNamePlaceholder')"
              />
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

      <!-- File Editor Dialog -->
      <FileEditor
        v-model:visible="showEditorDialog"
        :container-id="containerId"
        :file-path="editingFilePath"
        :file-name="editingFileName"
        @saved="handleRefresh"
      />
    </DialogContent>
  </Dialog>
</template>
