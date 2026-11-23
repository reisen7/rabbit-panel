<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { RefreshCw, Search, Trash2, Plus } from 'lucide-vue-next'
import { useVolumeStore, type SortField } from '@/stores/volumes'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import CreateVolumeDialog from '@/components/volumes/CreateVolumeDialog.vue'
import DataPagination from '@/components/common/DataPagination.vue'
import LoadingOverlay from '@/components/common/LoadingOverlay.vue'
import SortableTableHead from '@/components/common/SortableTableHead.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

const { t } = useI18n()
const volumeStore = useVolumeStore()
const toast = useToast()
const { confirm } = useConfirm()

const searchInput = ref('')
const currentPageValue = ref(volumeStore.currentPage)
const pageSizeValue = ref(volumeStore.pageSize)
const showCreateDialog = ref(false)

watch(() => volumeStore.searchQuery, (val) => { searchInput.value = val })
watch(() => volumeStore.currentPage, (val) => { currentPageValue.value = val })
watch(() => volumeStore.pageSize, (val) => { pageSizeValue.value = val })

function formatDate(dateStr: string): string {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleString()
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function handleSearch(value: string) {
  volumeStore.setSearch(value)
}

function handleSortChange(prop: string, order: 'ascending' | 'descending') {
  if (prop && order) {
    const field = prop as SortField
    volumeStore.sortField = field
    volumeStore.sortOrder = order === 'ascending' ? 'asc' : 'desc'
  }
}

function handlePageChange(page: number) {
  volumeStore.setPage(page)
}

function handlePageSizeChange(size: number) {
  volumeStore.setPageSize(size)
}

async function handleRefresh() {
  await volumeStore.fetchVolumes()
}

async function confirmRemove(name: string) {
  const ok = await confirm({
    title: t('common.confirm'),
    description: t('volumes.confirmRemove'),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    destructive: true,
  })
  if (!ok) return
  await handleRemove(name)
}

async function handleRemove(name: string) {
  try {
    await volumeStore.removeVolume(name)
    toast.success(t('volumes.removeSuccess'))
  } catch {
    // Error is handled by request interceptor
  }
}

async function confirmPrune() {
  const ok = await confirm({
    title: t('common.confirm'),
    description: t('volumes.pruneConfirm'),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    destructive: true,
  })
  if (!ok) return
  await handlePrune()
}

async function handlePrune() {
  try {
    const result = await volumeStore.pruneVolumes()
    const count = result.volumesDeleted?.length || 0
    const size = formatBytes(result.spaceReclaimed || 0)
    toast.success(t('volumes.pruneSuccess').replace('{count}', String(count)).replace('{size}', size))
  } catch {
    // Error is handled by request interceptor
  }
}

function handleVolumeCreated() {
  volumeStore.fetchVolumes()
}

onMounted(() => {
  volumeStore.fetchVolumes()
})
</script>

<template>
  <div class="space-y-5">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
      <h2 class="text-xl font-semibold tracking-tight">{{ t('volumes.title') }}</h2>
      <div class="flex flex-wrap items-center gap-2">
        <Button @click="showCreateDialog = true">
          <Plus />
          {{ t('volumes.create') }}
        </Button>
        <Button
          variant="outline"
          class="text-amber-600 dark:text-amber-500"
          :disabled="volumeStore.unusedCount === 0"
          @click="confirmPrune"
        >
          <Trash2 />
          {{ t('volumes.prune') }} ({{ volumeStore.unusedCount }})
        </Button>
        <Button variant="outline" :disabled="volumeStore.loading" @click="handleRefresh">
          <RefreshCw :class="{ 'animate-spin': volumeStore.loading }" />
          {{ t('common.refresh') }}
        </Button>
      </div>
    </div>

    <!-- Search bar -->
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
    </div>

    <!-- Volume table -->
    <div class="relative rounded-lg border bg-card shadow-2xs overflow-hidden">
      <LoadingOverlay :loading="volumeStore.loading" />
      <Table>
        <TableHeader>
          <TableRow>
            <SortableTableHead
              prop="name"
              :sort-field="volumeStore.sortField"
              :sort-order="volumeStore.sortOrder"
              @sort="handleSortChange"
            >
              {{ t('volumes.name') }}
            </SortableTableHead>
            <SortableTableHead
              prop="driver"
              :sort-field="volumeStore.sortField"
              :sort-order="volumeStore.sortOrder"
              @sort="handleSortChange"
            >
              {{ t('volumes.driver') }}
            </SortableTableHead>
            <TableHead class="max-[1199px]:hidden">{{ t('volumes.mountpoint') }}</TableHead>
            <TableHead class="max-[991px]:hidden">{{ t('volumes.usedBy') }}</TableHead>
            <SortableTableHead
              prop="created"
              :sort-field="volumeStore.sortField"
              :sort-order="volumeStore.sortOrder"
              class="max-[767px]:hidden"
              @sort="handleSortChange"
            >
              {{ t('common.createdAt') }}
            </SortableTableHead>
            <TableHead class="text-center">Usage</TableHead>
            <TableHead class="text-right">{{ t('common.actions') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-if="volumeStore.paginatedVolumes.length === 0">
            <TableCell colspan="7" class="h-24 text-center text-muted-foreground">
              {{ t('common.none') }}
            </TableCell>
          </TableRow>
          <TableRow v-for="row in volumeStore.paginatedVolumes" :key="row.name">
            <TableCell>
              <span class="font-mono text-xs font-medium">{{ row.name }}</span>
            </TableCell>
            <TableCell>
              <Badge variant="secondary">{{ row.driver }}</Badge>
            </TableCell>
            <TableCell class="max-[1199px]:hidden">
              <span class="font-mono text-xs text-muted-foreground break-all">{{ row.mountpoint }}</span>
            </TableCell>
            <TableCell class="max-[991px]:hidden">
              <Tooltip v-if="row.containers && row.containers.length > 0">
                <TooltipTrigger as-child>
                  <span>
                    <Badge variant="success">
                      {{ row.containers.length }} {{ t('volumes.containers') }}
                    </Badge>
                  </span>
                </TooltipTrigger>
                <TooltipContent>{{ row.containers.join(', ') }}</TooltipContent>
              </Tooltip>
              <Badge v-else variant="info">
                {{ t('volumes.unused') }}
              </Badge>
            </TableCell>
            <TableCell class="max-[767px]:hidden text-xs text-muted-foreground tabular-nums">
              {{ formatDate(row.created) }}
            </TableCell>
            <TableCell class="text-center">
              <Tooltip v-if="row.containers && row.containers.length > 0">
                <TooltipTrigger as-child>
                  <span>
                    <Badge variant="success">In Use</Badge>
                  </span>
                </TooltipTrigger>
                <TooltipContent>{{ row.containers.join(', ') }}</TooltipContent>
              </Tooltip>
              <Badge v-else variant="info">Unused</Badge>
            </TableCell>
            <TableCell class="text-right">
              <Button
                v-if="!row.containers || row.containers.length === 0"
                variant="ghost"
                size="sm"
                class="text-destructive hover:text-destructive"
                @click="confirmRemove(row.name)"
              >
                <Trash2 />
                {{ t('common.delete') }}
              </Button>
              <Tooltip v-else>
                <TooltipTrigger as-child>
                  <span>
                    <Button variant="ghost" size="sm" disabled>
                      <Trash2 />
                      {{ t('common.delete') }}
                    </Button>
                  </span>
                </TooltipTrigger>
                <TooltipContent>{{ t('volumes.inUse') }}</TooltipContent>
              </Tooltip>
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
        :total="volumeStore.totalVolumes"
        @current-change="handlePageChange"
        @size-change="handlePageSizeChange"
      />
    </div>

    <CreateVolumeDialog v-model:visible="showCreateDialog" @created="handleVolumeCreated" />
  </div>
</template>
