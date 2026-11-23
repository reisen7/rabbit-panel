<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { RefreshCw, Search, Trash2, Plus, Eye } from 'lucide-vue-next'
import { useNetworkStore, type SortField } from '@/stores/networks'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import type { NetworkInfo } from '@/types'
import CreateNetworkDialog from '@/components/network/CreateNetworkDialog.vue'
import NetworkDetailDialog from '@/components/network/NetworkDetailDialog.vue'
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
const networkStore = useNetworkStore()
const toast = useToast()
const { confirm } = useConfirm()

const searchInput = ref('')
const currentPageValue = ref(networkStore.currentPage)
const pageSizeValue = ref(networkStore.pageSize)
const showCreateDialog = ref(false)
const showDetailDialog = ref(false)
const selectedNetworkId = ref('')

watch(() => networkStore.searchQuery, (val) => { searchInput.value = val })
watch(() => networkStore.currentPage, (val) => { currentPageValue.value = val })
watch(() => networkStore.pageSize, (val) => { pageSizeValue.value = val })

function handleSearch(value: string) {
  networkStore.setSearch(value)
}

function handleSortChange(prop: string, order: 'ascending' | 'descending') {
  if (prop && order) {
    const field = prop as SortField
    networkStore.sortField = field
    networkStore.sortOrder = order === 'ascending' ? 'asc' : 'desc'
  }
}

function handlePageChange(page: number) {
  networkStore.setPage(page)
}

function handlePageSizeChange(size: number) {
  networkStore.setPageSize(size)
}

async function handleRefresh() {
  await networkStore.fetchNetworks()
}

async function confirmRemoveNetwork(id: string) {
  const ok = await confirm({
    title: t('common.confirm'),
    description: t('network.confirmRemove'),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    destructive: true,
  })
  if (!ok) return
  await handleRemoveNetwork(id)
}

async function handleRemoveNetwork(id: string) {
  try {
    await networkStore.removeNetwork(id)
    toast.success(t('network.removeSuccess'))
  } catch {
    // Error is handled by request interceptor
  }
}

function handleViewDetails(network: NetworkInfo) {
  selectedNetworkId.value = network.id
  showDetailDialog.value = true
}

function handleNetworkCreated() {
  networkStore.fetchNetworks()
}

onMounted(() => {
  networkStore.fetchNetworks()
})
</script>

<template>
  <div class="space-y-5">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
      <h2 class="text-xl font-semibold tracking-tight">{{ t('network.title') }}</h2>
      <div class="flex flex-wrap items-center gap-2">
        <Button @click="showCreateDialog = true">
          <Plus />
          {{ t('network.create') }}
        </Button>
        <Button variant="outline" :disabled="networkStore.loading" @click="handleRefresh">
          <RefreshCw :class="{ 'animate-spin': networkStore.loading }" />
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

    <!-- Network table -->
    <div class="relative rounded-lg border bg-card shadow-2xs overflow-hidden">
      <LoadingOverlay :loading="networkStore.loading" />
      <Table>
        <TableHeader>
          <TableRow>
            <SortableTableHead
              prop="id"
              :sort-field="networkStore.sortField"
              :sort-order="networkStore.sortOrder"
              class="max-[991px]:hidden"
              @sort="handleSortChange"
            >
              {{ t('network.id') }}
            </SortableTableHead>
            <SortableTableHead
              prop="name"
              :sort-field="networkStore.sortField"
              :sort-order="networkStore.sortOrder"
              @sort="handleSortChange"
            >
              {{ t('common.name') }}
            </SortableTableHead>
            <SortableTableHead
              prop="driver"
              :sort-field="networkStore.sortField"
              :sort-order="networkStore.sortOrder"
              @sort="handleSortChange"
            >
              {{ t('network.driver') }}
            </SortableTableHead>
            <SortableTableHead
              prop="scope"
              :sort-field="networkStore.sortField"
              :sort-order="networkStore.sortOrder"
              class="max-[767px]:hidden"
              @sort="handleSortChange"
            >
              {{ t('network.scope') }}
            </SortableTableHead>
            <TableHead class="max-[1199px]:hidden">{{ t('network.ipam') }}</TableHead>
            <SortableTableHead
              prop="containers"
              :sort-field="networkStore.sortField"
              :sort-order="networkStore.sortOrder"
              class="max-[767px]:hidden text-center"
              @sort="handleSortChange"
            >
              {{ t('network.containers') }}
            </SortableTableHead>
            <TableHead class="text-center">Usage</TableHead>
            <TableHead class="text-right">{{ t('common.actions') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-if="networkStore.paginatedNetworks.length === 0">
            <TableCell colspan="8" class="h-24 text-center text-muted-foreground">
              {{ t('common.none') }}
            </TableCell>
          </TableRow>
          <TableRow v-for="row in networkStore.paginatedNetworks" :key="row.id">
            <TableCell class="max-[991px]:hidden">
              <span class="font-mono text-xs text-muted-foreground">{{ row.id.substring(0, 12) }}</span>
            </TableCell>
            <TableCell>
              <span class="font-medium">{{ row.name }}</span>
              <Badge v-if="networkStore.isProtectedNetwork(row.name)" variant="info" class="ml-2">
                System
              </Badge>
            </TableCell>
            <TableCell>
              <Badge variant="secondary">{{ row.driver }}</Badge>
            </TableCell>
            <TableCell class="max-[767px]:hidden text-xs text-muted-foreground">{{ row.scope }}</TableCell>
            <TableCell class="max-[1199px]:hidden font-mono text-xs text-muted-foreground">{{ row.ipam || '-' }}</TableCell>
            <TableCell class="max-[767px]:hidden text-center">
              <Badge :variant="row.container_count > 0 ? 'success' : 'info'">
                {{ row.container_count }}
              </Badge>
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
              <div class="inline-flex items-center gap-1">
                <Button variant="ghost" size="sm" @click="handleViewDetails(row)">
                  <Eye />
                  {{ t('network.details') }}
                </Button>
                <Button
                  v-if="!networkStore.isProtectedNetwork(row.name)"
                  variant="ghost"
                  size="sm"
                  class="text-destructive hover:text-destructive"
                  @click="confirmRemoveNetwork(row.id)"
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
                  <TooltipContent>{{ t('network.protectedNetwork') }}</TooltipContent>
                </Tooltip>
              </div>
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
        :total="networkStore.totalNetworks"
        @current-change="handlePageChange"
        @size-change="handlePageSizeChange"
      />
    </div>

    <CreateNetworkDialog v-model:visible="showCreateDialog" @created="handleNetworkCreated" />
    <NetworkDetailDialog v-model:visible="showDetailDialog" :network-id="selectedNetworkId" />
  </div>
</template>
