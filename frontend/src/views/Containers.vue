<script setup lang="ts">
import { ref, onMounted, watch, computed, onUnmounted, reactive } from 'vue'
import { useRoute } from 'vue-router'
import { Plus, RefreshCw, Search, Monitor } from 'lucide-vue-next'
import { useContainerStore, type ContainerState, type SortField } from '@/stores/containers'
import { useContainerCreateStore } from '@/stores/containerCreate'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { containerApi } from '@/api/containers'
import type { ContainerStats } from '@/types'
import ContainerActions from '@/components/container/ContainerActions.vue'
import CreateContainerDialog from '@/components/container/CreateContainerDialog.vue'
import DockerRunDialog from '@/components/container/DockerRunDialog.vue'
import ContainerLogsDialog from '@/components/container/ContainerLogsDialog.vue'
import ContainerTerminal from '@/components/container/ContainerTerminal.vue'
import FileExplorer from '@/components/container/FileExplorer.vue'
import ContainerConfig from '@/components/container/ContainerConfig.vue'
import DataPagination from '@/components/common/DataPagination.vue'
import LoadingOverlay from '@/components/common/LoadingOverlay.vue'
import SortableTableHead from '@/components/common/SortableTableHead.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

const { t } = useI18n()
const route = useRoute()
const containerStore = useContainerStore()
const toast = useToast()

const containerStats = reactive<Record<string, ContainerStats | null>>({})
const loadingStats = reactive<Record<string, boolean>>({})

function formatMemory(bytes: number): string {
  if (bytes === 0) return '-'
  const mb = bytes / 1024 / 1024
  if (mb >= 1024) {
    return `${(mb / 1024).toFixed(1)} GB`
  }
  return `${mb.toFixed(0)} MB`
}

function hasCpuStats(containerId: string): boolean {
  const stats = containerStats[containerId]
  return !!stats && typeof stats.cpu_percent === 'number' && Number.isFinite(stats.cpu_percent)
}

function hasMemoryStats(containerId: string): boolean {
  const stats = containerStats[containerId]
  return !!stats && typeof stats.memory_usage === 'number' && Number.isFinite(stats.memory_usage)
}

async function fetchContainerStats(containerId: string) {
  if (loadingStats[containerId]) return
  loadingStats[containerId] = true
  try {
    const stats = await containerApi.stats(containerId)
    containerStats[containerId] = stats
  } catch {
    containerStats[containerId] = null
  } finally {
    loadingStats[containerId] = false
  }
}

async function fetchAllRunningStats() {
  const runningContainers = containerStore.containers.filter((c) => c.state === 'running')
  const batchSize = 5
  for (let i = 0; i < runningContainers.length; i += batchSize) {
    const batch = runningContainers.slice(i, i + batchSize)
    await Promise.all(batch.map((c) => fetchContainerStats(c.id)))
  }
}

watch(
  () => containerStore.containers,
  () => {
    fetchAllRunningStats()
  },
  { immediate: true }
)

const windowWidth = ref(window.innerWidth)
const isMobile = computed(() => windowWidth.value < 768)

function handleResize() {
  windowWidth.value = window.innerWidth
}

onMounted(() => {
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})

const searchInput = ref('')
const statusFilterValue = ref<ContainerState>(containerStore.statusFilter)
const currentPageValue = ref(containerStore.currentPage)
const pageSizeValue = ref(containerStore.pageSize)

const containerCreateStore = useContainerCreateStore()

const showCreateDialog = ref(false)
const showDockerRunDialog = ref(false)
const showLogsDialog = ref(false)
const showTerminalDialog = ref(false)
const showFilesDialog = ref(false)
const showConfigDialog = ref(false)

const selectedContainerId = ref('')
const selectedContainerName = ref('')

watch(() => containerStore.searchQuery, (val) => { searchInput.value = val })
watch(() => containerStore.statusFilter, (val) => { statusFilterValue.value = val })
watch(() => containerStore.currentPage, (val) => { currentPageValue.value = val })
watch(() => containerStore.pageSize, (val) => { pageSizeValue.value = val })

function handleSearch(value: string) {
  containerStore.setSearch(value)
}

function handleStatusFilter(value: ContainerState) {
  containerStore.setStatusFilter(value)
}

function handleSortChange(prop: string, order: 'ascending' | 'descending') {
  if (prop && order) {
    const field = prop as SortField
    containerStore.sortField = field
    containerStore.sortOrder = order === 'ascending' ? 'asc' : 'desc'
  }
}

function handlePageChange(page: number) {
  containerStore.setPage(page)
}

function handlePageSizeChange(size: number) {
  containerStore.setPageSize(size)
}

async function handleRefresh() {
  await containerStore.fetchContainers()
}

async function handleContainerAction(id: string, action: 'start' | 'stop' | 'restart' | 'remove') {
  try {
    await containerStore.containerAction(id, action)
    toast.success(t('common.success'))
  } catch {
    // Error is handled by request interceptor
  }
}

function handleViewLogs(id: string, name: string) {
  selectedContainerId.value = id
  selectedContainerName.value = name
  showLogsDialog.value = true
}

function handleOpenTerminal(id: string, name: string) {
  selectedContainerId.value = id
  selectedContainerName.value = name
  showTerminalDialog.value = true
}

function handleOpenFiles(id: string, name: string) {
  selectedContainerId.value = id
  selectedContainerName.value = name
  showFilesDialog.value = true
}

function handleOpenConfig(id: string, name: string) {
  selectedContainerId.value = id
  selectedContainerName.value = name
  showConfigDialog.value = true
}

function handleContainerCreated() {
  containerStore.fetchContainers()
}

function getStateTagType(state: string): 'success' | 'destructive' | 'warning' | 'info' {
  switch (state) {
    case 'running':
      return 'success'
    case 'exited':
      return 'destructive'
    case 'paused':
      return 'warning'
    default:
      return 'info'
  }
}

function getStateLabel(state: string): string {
  switch (state) {
    case 'running':
      return t('container.running')
    case 'exited':
      return t('container.stopped')
    case 'paused':
      return t('container.paused')
    case 'created':
      return t('container.created')
    default:
      return state
  }
}

watch(
  () => containerCreateStore.restorePending,
  (pending) => {
    if (pending) showCreateDialog.value = true
  }
)

onMounted(async () => {
  if (containerCreateStore.restorePending) {
    showCreateDialog.value = true
  }
  await containerStore.fetchContainers()

  const action = route.query.action as string
  const containerName = route.query.container as string

  if (action && containerName) {
    const container = containerStore.containers.find((c) => c.name === containerName)
    if (container) {
      selectedContainerId.value = container.id
      selectedContainerName.value = container.name
      switch (action) {
        case 'terminal':
          showTerminalDialog.value = true
          break
        case 'files':
          showFilesDialog.value = true
          break
        case 'logs':
          showLogsDialog.value = true
          break
      }
    } else {
      toast.warning(`容器 ${containerName} 未找到`)
    }
  }
})
</script>

<template>
  <div class="space-y-5">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
      <h2 class="text-xl font-semibold tracking-tight">{{ t('container.title') }}</h2>
      <div class="flex flex-wrap items-center gap-2">
        <Button @click="showCreateDialog = true">
          <Plus />
          {{ t('container.create') }}
        </Button>
        <Button variant="outline" @click="showDockerRunDialog = true">
          <Monitor />
          {{ t('container.dockerRun') }}
        </Button>
        <Button variant="outline" :disabled="containerStore.loading" @click="handleRefresh">
          <RefreshCw :class="{ 'animate-spin': containerStore.loading }" />
          {{ t('common.refresh') }}
        </Button>
      </div>
    </div>

    <!-- Search and filter bar -->
    <div class="flex flex-col sm:flex-row gap-3">
      <div class="relative w-full sm:w-[300px]">
        <Search class="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
        <Input
          v-model="searchInput"
          :placeholder="t('common.search')"
          class="pl-9 bg-background"
          @input="handleSearch(searchInput)"
        />
      </div>

      <Select v-model="statusFilterValue" @update:model-value="(v) => handleStatusFilter(v as ContainerState)">
        <SelectTrigger class="w-full sm:w-[160px] bg-background">
          <SelectValue :placeholder="t('common.status')" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">{{ t('common.all') }}</SelectItem>
          <SelectItem value="running">{{ t('container.running') }}</SelectItem>
          <SelectItem value="exited">{{ t('container.stopped') }}</SelectItem>
          <SelectItem value="paused">{{ t('container.paused') }}</SelectItem>
          <SelectItem value="created">{{ t('container.created') }}</SelectItem>
        </SelectContent>
      </Select>
    </div>

    <!-- Container table -->
    <div class="relative rounded-lg border bg-card shadow-2xs overflow-hidden">
      <LoadingOverlay :loading="containerStore.loading" />
      <Table>
        <TableHeader>
          <TableRow>
            <SortableTableHead
              prop="id"
              :sort-field="containerStore.sortField"
              :sort-order="containerStore.sortOrder"
              class="max-[991px]:hidden"
              @sort="handleSortChange"
            >
              ID
            </SortableTableHead>
            <SortableTableHead
              prop="name"
              :sort-field="containerStore.sortField"
              :sort-order="containerStore.sortOrder"
              @sort="handleSortChange"
            >
              {{ t('common.name') }}
            </SortableTableHead>
            <SortableTableHead
              prop="image"
              :sort-field="containerStore.sortField"
              :sort-order="containerStore.sortOrder"
              class="max-[767px]:hidden"
              @sort="handleSortChange"
            >
              {{ t('container.image') }}
            </SortableTableHead>
            <SortableTableHead
              prop="state"
              :sort-field="containerStore.sortField"
              :sort-order="containerStore.sortOrder"
              @sort="handleSortChange"
            >
              {{ t('common.status') }}
            </SortableTableHead>
            <TableHead class="max-[991px]:hidden">{{ t('container.ports') }}</TableHead>
            <TableHead class="max-[1199px]:hidden">CPU</TableHead>
            <TableHead class="max-[1199px]:hidden">{{ t('system.memory') }}</TableHead>
            <SortableTableHead
              prop="created"
              :sort-field="containerStore.sortField"
              :sort-order="containerStore.sortOrder"
              class="max-[1199px]:hidden"
              @sort="handleSortChange"
            >
              {{ t('common.time') }}
            </SortableTableHead>
            <TableHead class="text-center">{{ t('common.actions') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-if="containerStore.paginatedContainers.length === 0">
            <TableCell colspan="9" class="h-24 text-center text-muted-foreground">
              {{ t('common.none') }}
            </TableCell>
          </TableRow>
          <TableRow v-for="row in containerStore.paginatedContainers" :key="row.id">
            <TableCell class="max-[991px]:hidden">
              <span class="inline-block truncate align-bottom font-mono text-xs max-w-[110px] text-muted-foreground">
                {{ row.id }}
              </span>
            </TableCell>
            <TableCell>
              <span class="inline-block truncate align-bottom font-medium max-w-[160px]">
                {{ row.name }}
              </span>
            </TableCell>
            <TableCell class="max-[767px]:hidden">
              <span class="inline-block truncate align-bottom font-mono text-xs max-w-[240px]">
                {{ row.image }}
              </span>
            </TableCell>
            <TableCell>
              <Badge :variant="getStateTagType(row.state)">
                {{ getStateLabel(row.state) }}
              </Badge>
            </TableCell>
            <TableCell class="max-[991px]:hidden">
              <span class="inline-block truncate align-bottom font-mono text-xs max-w-[190px]">
                {{ row.ports || '-' }}
              </span>
            </TableCell>
            <TableCell class="max-[1199px]:hidden tabular-nums text-xs">
              <template v-if="row.state === 'running'">
                <span v-if="loadingStats[row.id]" class="text-muted-foreground">...</span>
                <span v-else-if="hasCpuStats(row.id)">{{ containerStats[row.id]!.cpu_percent.toFixed(1) }}%</span>
                <span v-else class="text-muted-foreground">-</span>
              </template>
              <span v-else class="text-muted-foreground">-</span>
            </TableCell>
            <TableCell class="max-[1199px]:hidden tabular-nums text-xs">
              <template v-if="row.state === 'running'">
                <span v-if="loadingStats[row.id]" class="text-muted-foreground">...</span>
                <template v-else-if="hasMemoryStats(row.id)">
                  <span>{{ formatMemory(containerStats[row.id]!.memory_usage) }}</span>
                  <span v-if="containerStats[row.id]!.has_memory_limit" class="text-muted-foreground">
                    / {{ formatMemory(containerStats[row.id]!.memory_limit) }}
                  </span>
                  <span v-else class="text-muted-foreground"> / 无限制</span>
                </template>
                <span v-else class="text-muted-foreground">-</span>
              </template>
              <span v-else class="text-muted-foreground">-</span>
            </TableCell>
            <TableCell class="max-[1199px]:hidden text-xs text-muted-foreground">
              <span class="inline-block truncate align-bottom max-w-[160px]">{{ row.created }}</span>
            </TableCell>
            <TableCell class="text-center">
              <ContainerActions
                :container="row"
                :compact="isMobile"
                @action="handleContainerAction"
                @logs="handleViewLogs"
                @terminal="handleOpenTerminal"
                @files="handleOpenFiles"
                @config="handleOpenConfig"
              />
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <!-- Pagination -->
    <div class="flex justify-center md:justify-end">
      <DataPagination
        v-model:current-page="currentPageValue"
        v-model:page-size="pageSizeValue"
        :total="containerStore.totalContainers"
        @current-change="handlePageChange"
        @size-change="handlePageSizeChange"
      />
    </div>

    <CreateContainerDialog v-model:visible="showCreateDialog" @created="handleContainerCreated" />
    <DockerRunDialog v-model:visible="showDockerRunDialog" @created="handleContainerCreated" />
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
    <ContainerConfig
      v-model:visible="showConfigDialog"
      :container-id="selectedContainerId"
      :container-name="selectedContainerName"
      @updated="handleContainerCreated"
    />
  </div>
</template>
