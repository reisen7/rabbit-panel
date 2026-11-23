<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { RefreshCw, Search, Trash2, Box } from 'lucide-vue-next'
import { useImageStore, type SortField } from '@/stores/images'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { useNodeRefresh } from '@/composables/useNodeRefresh'
import BuildImageDialog from '@/components/image/BuildImageDialog.vue'
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
const imageStore = useImageStore()
const toast = useToast()
const { confirm } = useConfirm()

const searchInput = ref('')
const currentPageValue = ref(imageStore.currentPage)
const pageSizeValue = ref(imageStore.pageSize)
const showBuildDialog = ref(false)

watch(() => imageStore.searchQuery, (val) => { searchInput.value = val })
watch(() => imageStore.currentPage, (val) => { currentPageValue.value = val })
watch(() => imageStore.pageSize, (val) => { pageSizeValue.value = val })

function handleSearch(value: string) {
  imageStore.setSearch(value)
}

function handleSortChange(prop: string, order: 'ascending' | 'descending') {
  if (prop && order) {
    const field = prop as SortField
    imageStore.sortField = field
    imageStore.sortOrder = order === 'ascending' ? 'asc' : 'desc'
  }
}

function handlePageChange(page: number) {
  imageStore.setPage(page)
}

function handlePageSizeChange(size: number) {
  imageStore.setPageSize(size)
}

async function handleRefresh() {
  await imageStore.fetchImages(true)
}

async function confirmRemoveImage(id: string) {
  const ok = await confirm({
    title: t('common.confirm'),
    description: t('image.confirmRemove'),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    destructive: true,
  })
  if (!ok) return
  await handleRemoveImage(id)
}

async function handleRemoveImage(id: string) {
  try {
    await imageStore.removeImage(id)
    toast.success(t('image.removeSuccess'))
  } catch {
    // Error is handled by request interceptor
  }
}

function handleImageBuilt() {
  imageStore.fetchImages(true)
}

useNodeRefresh(() => imageStore.fetchImages())

onMounted(() => {
  imageStore.fetchImages()
})
</script>

<template>
  <div class="space-y-5">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
      <h2 class="text-xl font-semibold tracking-tight">{{ t('image.title') }}</h2>
      <div class="flex flex-wrap items-center gap-2">
        <Button @click="showBuildDialog = true">
          <Box />
          {{ t('image.build') }}
        </Button>
        <Button variant="outline" :disabled="imageStore.loading" @click="handleRefresh">
          <RefreshCw :class="{ 'animate-spin': imageStore.loading }" />
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

    <!-- Image table -->
    <div class="relative rounded-lg border bg-card shadow-2xs overflow-hidden">
      <LoadingOverlay :loading="imageStore.loading && imageStore.paginatedImages.length === 0" />
      <Table>
        <TableHeader>
          <TableRow>
            <SortableTableHead
              prop="id"
              :sort-field="imageStore.sortField"
              :sort-order="imageStore.sortOrder"
              class="max-[991px]:hidden"
              @sort="handleSortChange"
            >
              {{ t('image.id') }}
            </SortableTableHead>
            <SortableTableHead
              prop="name"
              :sort-field="imageStore.sortField"
              :sort-order="imageStore.sortOrder"
              @sort="handleSortChange"
            >
              {{ t('common.name') }}
            </SortableTableHead>
            <SortableTableHead
              prop="tag"
              :sort-field="imageStore.sortField"
              :sort-order="imageStore.sortOrder"
              @sort="handleSortChange"
            >
              {{ t('image.tag') }}
            </SortableTableHead>
            <SortableTableHead
              prop="size"
              :sort-field="imageStore.sortField"
              :sort-order="imageStore.sortOrder"
              class="max-[767px]:hidden"
              @sort="handleSortChange"
            >
              {{ t('common.size') }}
            </SortableTableHead>
            <SortableTableHead
              prop="created"
              :sort-field="imageStore.sortField"
              :sort-order="imageStore.sortOrder"
              class="max-[1199px]:hidden"
              @sort="handleSortChange"
            >
              {{ t('image.created') }}
            </SortableTableHead>
            <TableHead class="text-center">Usage</TableHead>
            <TableHead class="text-right">{{ t('common.actions') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-if="imageStore.paginatedImages.length === 0">
            <TableCell colspan="7" class="h-24 text-center text-muted-foreground">
              {{ t('common.none') }}
            </TableCell>
          </TableRow>
          <TableRow v-for="row in imageStore.paginatedImages" :key="row.id">
            <TableCell class="max-[991px]:hidden">
              <span class="font-mono text-xs text-muted-foreground">{{ row.id.substring(0, 12) }}</span>
            </TableCell>
            <TableCell class="font-medium">{{ row.name }}</TableCell>
            <TableCell>
              <Badge variant="secondary">{{ row.tag }}</Badge>
            </TableCell>
            <TableCell class="max-[767px]:hidden tabular-nums text-xs">{{ row.size }}</TableCell>
            <TableCell class="max-[1199px]:hidden text-xs text-muted-foreground">{{ row.created }}</TableCell>
            <TableCell class="text-center">
              <Tooltip v-if="row.used_by && row.used_by.length > 0">
                <TooltipTrigger as-child>
                  <span>
                    <Badge variant="success">In Use ({{ row.used_by_count }})</Badge>
                  </span>
                </TooltipTrigger>
                <TooltipContent>{{ row.used_by.join(', ') }}</TooltipContent>
              </Tooltip>
              <Badge v-else variant="info">Unused</Badge>
            </TableCell>
            <TableCell class="text-right">
              <Button
                variant="ghost"
                size="sm"
                class="text-destructive hover:text-destructive"
                @click="confirmRemoveImage(row.id)"
              >
                <Trash2 />
                {{ t('common.delete') }}
              </Button>
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
        :total="imageStore.totalImages"
        @current-change="handlePageChange"
        @size-change="handlePageSizeChange"
      />
    </div>

    <BuildImageDialog v-model:visible="showBuildDialog" @built="handleImageBuilt" />
  </div>
</template>
