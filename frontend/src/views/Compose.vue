<script setup lang="ts">
import { ref, onMounted, reactive, watch } from 'vue'
import { storeToRefs } from 'pinia'
import {
  Box,
  FileCode2,
  FolderOpen,
  Layers,
  Loader2,
  Plus,
  RefreshCw,
  Search,
  Terminal,
  FileText,
  Trash2,
} from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { useComposeStore } from '@/stores/compose'
import ComposeEditor from '@/components/compose/ComposeEditor.vue'
import ComposeActions from '@/components/compose/ComposeActions.vue'
import ContainerLogsDialog from '@/components/container/ContainerLogsDialog.vue'
import ContainerTerminal from '@/components/container/ContainerTerminal.vue'
import FileExplorer from '@/components/container/FileExplorer.vue'
import LoadingOverlay from '@/components/common/LoadingOverlay.vue'
import { Badge } from '@/components/ui/badge'
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

const { t } = useI18n()
const toast = useToast()
const { confirm } = useConfirm()
const composeStore = useComposeStore()

const {
  filteredProjects,
  loading,
  selectedProject,
  fileContent,
  fileLoading,
  actionLoading,
  actionOutput,
  currentProject,
} = storeToRefs(composeStore)

const searchQuery = ref('')
const showCreateDialog = ref(false)
const creating = ref(false)
const editorRef = ref<InstanceType<typeof ComposeEditor> | null>(null)

const showLogsDialog = ref(false)
const showTerminalDialog = ref(false)
const showFilesDialog = ref(false)
const selectedContainerId = ref('')
const selectedContainerName = ref('')

const createForm = reactive({
  name: '',
})
const createError = ref('')
const namePattern = /^[a-zA-Z0-9_-]+$/

function getStatusVariant(status: string): 'success' | 'warning' | 'info' {
  switch (status) {
    case 'running':
      return 'success'
    case 'partial':
      return 'warning'
    default:
      return 'info'
  }
}

function getStatusText(status: string) {
  switch (status) {
    case 'running':
      return t('compose.statusRunning')
    case 'partial':
      return t('compose.statusPartial')
    case 'stopped':
      return t('compose.statusStopped')
    default:
      return t('compose.statusUnknown')
  }
}

async function handleRefresh() {
  await composeStore.fetchProjects()
  if (selectedProject.value) {
    await composeStore.fetchProjectStatus(selectedProject.value)
  }
}

async function handleSelectProject(name: string) {
  if (selectedProject.value === name) return
  if (editorRef.value?.hasChanges) {
    const ok = await confirm({
      title: t('common.warning'),
      description: t('compose.unsavedConfirm'),
      confirmText: t('common.confirm'),
      cancelText: t('common.cancel'),
    })
    if (!ok) return
  }
  await composeStore.selectProject(name)
}

async function handleDeleteProject(name: string) {
  const ok = await confirm({
    title: t('common.warning'),
    description: t('compose.confirmDelete'),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    destructive: true,
  })
  if (!ok) return

  try {
    await composeStore.deleteProject(name)
    toast.success(t('compose.deleteSuccess'))
  } catch (error) {
    toast.error(error instanceof Error ? error.message : t('compose.deleteFailed'))
  }
}

function validateCreate(): boolean {
  const val = createForm.name.trim()
  if (!val) {
    createError.value = t('compose.nameRequired')
    return false
  }
  if (!namePattern.test(val)) {
    createError.value = t('compose.nameInvalid')
    return false
  }
  createError.value = ''
  return true
}

async function handleCreate() {
  if (!validateCreate()) return

  try {
    creating.value = true
    await composeStore.createProject(createForm.name.trim())
    toast.success(t('compose.createSuccess'))
    showCreateDialog.value = false
    resetCreateForm()
  } catch (error) {
    toast.error(error instanceof Error ? error.message : t('compose.createFailed'))
  } finally {
    creating.value = false
  }
}

function resetCreateForm() {
  createForm.name = ''
  createError.value = ''
}

watch(showCreateDialog, (open) => {
  if (!open) resetCreateForm()
})

async function handleSaveFile(content: string) {
  if (!selectedProject.value) return

  try {
    await composeStore.saveFileContent(selectedProject.value, content)
    toast.success(t('compose.saveSuccess'))
  } catch (error) {
    toast.error(error instanceof Error ? error.message : t('compose.saveFailed'))
  }
}

function handleFileChange(_content: string) {
  // Handled by editor state
}

async function handleAction(action: 'up' | 'down' | 'restart' | 'pull') {
  if (!selectedProject.value) return
  composeStore.executeAction(selectedProject.value, action)
}

function handleClearOutput() {
  composeStore.clearOutput()
}

function handleOpenTerminal(containerId: string, containerName: string) {
  selectedContainerId.value = containerId
  selectedContainerName.value = containerName
  showTerminalDialog.value = true
}

function handleOpenFiles(containerId: string, containerName: string) {
  selectedContainerId.value = containerId
  selectedContainerName.value = containerName
  showFilesDialog.value = true
}

function handleOpenLogs(containerId: string, containerName: string) {
  selectedContainerId.value = containerId
  selectedContainerName.value = containerName
  showLogsDialog.value = true
}

watch(searchQuery, () => {
  composeStore.setSearch(searchQuery.value)
})

onMounted(async () => {
  composeStore.selectProject(null)
  await composeStore.fetchProjects()
})
</script>

<template>
  <div class="h-[calc(100vh-5.25rem)] md:h-[calc(100vh-6.5rem)] flex flex-col gap-4 overflow-hidden">
    <!-- Page Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 shrink-0">
      <h2 class="text-xl font-semibold tracking-tight">{{ t('compose.title') }}</h2>
      <div class="flex flex-wrap items-center gap-2">
        <Button @click="showCreateDialog = true">
          <Plus />
          {{ t('compose.create') }}
        </Button>
        <Button variant="outline" :disabled="loading" @click="handleRefresh">
          <RefreshCw :class="{ 'animate-spin': loading }" />
          {{ t('common.refresh') }}
        </Button>
      </div>
    </div>

    <!-- Main Content -->
    <div class="flex-1 flex flex-col md:flex-row gap-4 min-h-0 overflow-hidden">
      <!-- Left Panel: Project List -->
      <div class="w-full md:w-[270px] max-h-[220px] md:max-h-none shrink-0 flex flex-col min-h-0 rounded-lg border bg-card shadow-2xs overflow-hidden">
        <div class="p-3 border-b space-y-2">
          <div class="text-sm font-medium">{{ t('compose.projects') }}</div>
          <div class="relative">
            <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 size-3.5 text-muted-foreground pointer-events-none" />
            <Input
              v-model="searchQuery"
              :placeholder="t('common.search')"
              class="h-8 pl-8 text-xs"
            />
          </div>
        </div>

        <div class="relative flex-1 overflow-y-auto p-2 space-y-1">
          <LoadingOverlay :loading="loading" />
          <div
            v-for="project in filteredProjects"
            :key="project.name"
            class="group flex items-center justify-between gap-2 px-3 py-2 rounded-md border cursor-pointer transition-colors"
            :class="selectedProject === project.name
              ? 'bg-accent border-ring/40 text-accent-foreground'
              : 'border-transparent hover:bg-muted/60'"
            @click="handleSelectProject(project.name)"
          >
            <div class="min-w-0 space-y-1">
              <div class="text-sm font-medium truncate">{{ project.name }}</div>
              <Badge :variant="getStatusVariant(project.status)">
                {{ getStatusText(project.status) }}
              </Badge>
            </div>
            <Button
              variant="ghost"
              size="icon-sm"
              class="text-destructive hover:text-destructive opacity-80 group-hover:opacity-100 shrink-0"
              @click.stop="handleDeleteProject(project.name)"
            >
              <Trash2 class="size-3.5" />
            </Button>
          </div>

          <div
            v-if="!loading && filteredProjects.length === 0"
            class="py-10 text-center text-xs text-muted-foreground"
          >
            {{ t('compose.noProjects') }}
          </div>
        </div>
      </div>

      <!-- Right Panel: Editor and Actions -->
      <div class="flex-1 flex flex-col gap-4 min-w-0 min-h-0 overflow-hidden">
        <template v-if="selectedProject">
          <!-- Container Status -->
          <div
            v-if="currentProject?.containers?.length"
            class="shrink-0 rounded-lg border bg-card shadow-2xs overflow-hidden"
          >
            <div class="flex items-center gap-2 px-3.5 py-2.5 bg-muted/40 border-b text-sm font-medium">
              <Box class="size-4 text-muted-foreground" />
              <span>{{ t('compose.containers') }}</span>
            </div>
            <div class="max-h-[200px] overflow-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{{ t('compose.service') }}</TableHead>
                    <TableHead>{{ t('common.name') }}</TableHead>
                    <TableHead>{{ t('common.status') }}</TableHead>
                    <TableHead class="hidden lg:table-cell">{{ t('compose.statusDetail') }}</TableHead>
                    <TableHead class="hidden xl:table-cell">{{ t('container.ports') }}</TableHead>
                    <TableHead class="text-right">{{ t('common.actions') }}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow v-for="row in currentProject.containers" :key="row.id">
                    <TableCell class="font-medium text-xs">{{ row.service }}</TableCell>
                    <TableCell class="font-mono text-xs">{{ row.name }}</TableCell>
                    <TableCell>
                      <Badge :variant="row.state === 'running' ? 'success' : 'info'">
                        {{ row.state }}
                      </Badge>
                    </TableCell>
                    <TableCell class="hidden lg:table-cell text-xs text-muted-foreground">
                      {{ row.status }}
                    </TableCell>
                    <TableCell class="hidden xl:table-cell font-mono text-xs text-muted-foreground">
                      {{ row.ports || '-' }}
                    </TableCell>
                    <TableCell class="text-right">
                      <div class="inline-flex items-center gap-1">
                        <Button
                          variant="ghost"
                          size="sm"
                          :disabled="row.state !== 'running'"
                          @click="handleOpenTerminal(row.id, row.name)"
                        >
                          <Terminal class="size-3.5" />
                          {{ t('container.terminal') }}
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          :disabled="row.state !== 'running'"
                          @click="handleOpenFiles(row.id, row.name)"
                        >
                          <FolderOpen class="size-3.5" />
                          {{ t('container.files') }}
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          @click="handleOpenLogs(row.id, row.name)"
                        >
                          <FileText class="size-3.5" />
                          {{ t('container.logs') }}
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </div>
          </div>

          <!-- Actions -->
          <ComposeActions
            :project-name="selectedProject"
            :output="actionOutput"
            :loading="actionLoading"
            :status="currentProject?.status"
            @action="handleAction"
            @clear="handleClearOutput"
          />

          <!-- Editor -->
          <div class="flex-1 flex flex-col rounded-lg border bg-card shadow-2xs overflow-hidden min-h-0">
            <div class="flex items-center gap-2 px-3.5 py-2.5 bg-muted/40 border-b text-sm font-medium">
              <FileCode2 class="size-4 text-muted-foreground" />
              <span>{{ t('compose.editor') }}</span>
            </div>
            <ComposeEditor
              ref="editorRef"
              :project-name="selectedProject"
              :content="fileContent"
              :loading="fileLoading"
              @save="handleSaveFile"
              @change="handleFileChange"
            />
          </div>
        </template>

        <div
          v-else
          class="flex-1 rounded-lg border border-dashed bg-card flex flex-col items-center justify-center gap-3 text-center p-8"
        >
          <div class="size-11 rounded-full bg-muted flex items-center justify-center text-muted-foreground">
            <Layers class="size-5" />
          </div>
          <p class="text-sm text-muted-foreground">{{ t('compose.selectProject') }}</p>
        </div>
      </div>
    </div>

    <!-- Create Project Dialog -->
    <Dialog v-model:open="showCreateDialog">
      <DialogContent class="sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle>{{ t('compose.create') }}</DialogTitle>
        </DialogHeader>
        <form class="space-y-4 py-1" @submit.prevent="handleCreate">
          <div class="space-y-1.5">
            <Label for="compose-project-name">
              {{ t('compose.projectName') }} <span class="text-destructive">*</span>
            </Label>
            <Input
              id="compose-project-name"
              v-model="createForm.name"
              :placeholder="t('compose.projectNamePlaceholder')"
              @input="createError = ''"
            />
            <p v-if="createError" class="text-xs text-destructive">{{ createError }}</p>
          </div>
          <DialogFooter class="gap-2">
            <Button type="button" variant="outline" @click="showCreateDialog = false">
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

    <ContainerLogsDialog
      v-model:visible="showLogsDialog"
      :container-id="selectedContainerId"
      :container-name="selectedContainerName"
    />
    <ContainerTerminal
      v-model:visible="showTerminalDialog"
      :container-id="selectedContainerId"
      :container-name="selectedContainerName"
    />
    <FileExplorer
      v-model:visible="showFilesDialog"
      :container-id="selectedContainerId"
      :container-name="selectedContainerName"
    />
  </div>
</template>
