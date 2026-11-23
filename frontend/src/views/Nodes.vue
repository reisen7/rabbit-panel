<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { AlertTriangle, CheckCircle2, Monitor, RefreshCw, Search, XCircle } from 'lucide-vue-next'
import { useNodesStore } from '@/stores/nodes'
import { useI18n } from '@/composables/useI18n'
import LoadingOverlay from '@/components/common/LoadingOverlay.vue'
import { Alert, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
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
const nodesStore = useNodesStore()

const searchInput = ref('')
const statusFilterValue = ref<'all' | 'online' | 'offline'>('all')

watch(() => nodesStore.searchQuery, (val) => { searchInput.value = val })
watch(() => nodesStore.statusFilter, (val) => { statusFilterValue.value = val })

function handleSearch(value: string) {
  nodesStore.setSearch(value)
}

function handleStatusFilterChange(value: unknown) {
  nodesStore.setStatusFilter(value as 'all' | 'online' | 'offline')
}

async function handleRefresh() {
  await nodesStore.fetchNodes()
}

function getStatusVariant(status: string): 'success' | 'destructive' | 'warning' {
  switch (status) {
    case 'online':
      return 'success'
    case 'offline':
      return 'destructive'
    default:
      return 'warning'
  }
}

function getStatusText(status: string): string {
  switch (status) {
    case 'online':
      return t('node.online')
    case 'offline':
      return t('node.offline')
    case 'error':
      return t('node.error')
    default:
      return status
  }
}

function getProgressIndicatorClass(percentage: number): string {
  if (percentage < 50) return 'bg-emerald-500'
  if (percentage < 80) return 'bg-amber-500'
  return 'bg-destructive'
}

function formatLastSeen(lastSeen: string): string {
  if (!lastSeen) return t('node.neverSeen')
  try {
    const date = new Date(lastSeen)
    return date.toLocaleString()
  } catch {
    return lastSeen
  }
}

onMounted(() => {
  nodesStore.startPolling()
})
</script>

<template>
  <div class="space-y-5">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
      <h2 class="text-xl font-semibold tracking-tight">{{ t('node.title') }}</h2>
      <div class="flex items-center gap-2">
        <Button variant="outline" :disabled="nodesStore.loading" @click="handleRefresh">
          <RefreshCw :class="{ 'animate-spin': nodesStore.loading }" />
          {{ t('common.refresh') }}
        </Button>
      </div>
    </div>

    <!-- Not Master Mode Warning -->
    <Alert v-if="!nodesStore.isMasterMode && !nodesStore.loading" variant="warning">
      <AlertTriangle class="size-4" />
      <AlertTitle>{{ t('node.notMasterMode') }}</AlertTitle>
    </Alert>

    <!-- Stats Cards -->
    <div v-if="nodesStore.isMasterMode" class="grid grid-cols-1 sm:grid-cols-3 gap-4">
      <Card class="py-5">
        <CardContent class="flex items-center gap-4">
          <div class="size-11 rounded-xl bg-blue-500/10 text-blue-600 dark:text-blue-400 flex items-center justify-center shrink-0">
            <Monitor class="size-5" />
          </div>
          <div>
            <div class="text-2xl font-bold tabular-nums leading-tight">{{ nodesStore.totalNodes }}</div>
            <div class="text-xs text-muted-foreground mt-0.5">{{ t('node.totalNodes') }}</div>
          </div>
        </CardContent>
      </Card>

      <Card class="py-5">
        <CardContent class="flex items-center gap-4">
          <div class="size-11 rounded-xl bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 flex items-center justify-center shrink-0">
            <CheckCircle2 class="size-5" />
          </div>
          <div>
            <div class="text-2xl font-bold tabular-nums leading-tight">{{ nodesStore.onlineCount }}</div>
            <div class="text-xs text-muted-foreground mt-0.5">{{ t('node.onlineNodes') }}</div>
          </div>
        </CardContent>
      </Card>

      <Card class="py-5">
        <CardContent class="flex items-center gap-4">
          <div class="size-11 rounded-xl bg-red-500/10 text-destructive flex items-center justify-center shrink-0">
            <XCircle class="size-5" />
          </div>
          <div>
            <div class="text-2xl font-bold tabular-nums leading-tight">{{ nodesStore.offlineCount }}</div>
            <div class="text-xs text-muted-foreground mt-0.5">{{ t('node.offlineNodes') }}</div>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- Filter bar -->
    <div v-if="nodesStore.isMasterMode" class="flex flex-col sm:flex-row gap-3">
      <div class="relative w-full sm:w-[300px]">
        <Search class="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
        <Input
          v-model="searchInput"
          :placeholder="t('node.searchPlaceholder')"
          class="pl-9 bg-background"
          @input="handleSearch(searchInput)"
        />
      </div>

      <Select v-model="statusFilterValue" @update:model-value="handleStatusFilterChange">
        <SelectTrigger class="w-full sm:w-[160px] bg-background">
          <SelectValue :placeholder="t('node.filterByStatus')" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">{{ t('node.allNodes') }}</SelectItem>
          <SelectItem value="online">{{ t('node.onlineOnly') }}</SelectItem>
          <SelectItem value="offline">{{ t('node.offlineOnly') }}</SelectItem>
        </SelectContent>
      </Select>
    </div>

    <!-- Nodes table -->
    <div v-if="nodesStore.isMasterMode" class="relative rounded-lg border bg-card shadow-2xs overflow-hidden">
      <LoadingOverlay :loading="nodesStore.loading" />
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead class="max-[991px]:hidden">{{ t('node.nodeId') }}</TableHead>
            <TableHead>{{ t('node.nodeName') }}</TableHead>
            <TableHead class="max-[767px]:hidden">{{ t('node.address') }}</TableHead>
            <TableHead class="max-[767px]:hidden">{{ t('node.mode') }}</TableHead>
            <TableHead>{{ t('common.status') }}</TableHead>
            <TableHead class="max-[991px]:hidden min-w-[260px]">{{ t('node.resourceUsage') }}</TableHead>
            <TableHead class="text-center">{{ t('node.containers') }}</TableHead>
            <TableHead class="max-[1199px]:hidden">{{ t('node.lastHeartbeat') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-if="nodesStore.filteredNodes.length === 0 && !nodesStore.loading">
            <TableCell colspan="8" class="h-24 text-center text-muted-foreground">
              {{ t('node.noNodes') }}
            </TableCell>
          </TableRow>
          <TableRow v-for="row in nodesStore.filteredNodes" :key="row.id">
            <TableCell class="max-[991px]:hidden">
              <span class="font-mono text-xs text-muted-foreground">{{ row.id.substring(0, 12) }}</span>
            </TableCell>
            <TableCell class="font-medium">{{ row.name }}</TableCell>
            <TableCell class="max-[767px]:hidden font-mono text-xs text-muted-foreground">{{ row.address }}</TableCell>
            <TableCell class="max-[767px]:hidden">
              <Badge :variant="row.mode === 'master' ? 'default' : 'secondary'">
                {{ row.mode === 'master' ? t('node.master') : t('node.worker') }}
              </Badge>
            </TableCell>
            <TableCell>
              <Badge :variant="getStatusVariant(row.status)">
                {{ getStatusText(row.status) }}
              </Badge>
            </TableCell>
            <TableCell class="max-[991px]:hidden">
              <div class="space-y-1.5">
                <div class="flex items-center gap-2 text-xs">
                  <span class="w-11 text-muted-foreground shrink-0">{{ t('node.cpu') }}</span>
                  <Progress :model-value="row.cpu" class="h-1.5 flex-1" :indicator-class="getProgressIndicatorClass(row.cpu)" />
                  <span class="w-12 text-right tabular-nums">{{ row.cpu.toFixed(1) }}%</span>
                </div>
                <div class="flex items-center gap-2 text-xs">
                  <span class="w-11 text-muted-foreground shrink-0">{{ t('node.memory') }}</span>
                  <Progress :model-value="row.memory" class="h-1.5 flex-1" :indicator-class="getProgressIndicatorClass(row.memory)" />
                  <span class="w-12 text-right tabular-nums">{{ row.memory.toFixed(1) }}%</span>
                </div>
                <div class="flex items-center gap-2 text-xs">
                  <span class="w-11 text-muted-foreground shrink-0">{{ t('node.disk') }}</span>
                  <Progress :model-value="row.disk" class="h-1.5 flex-1" :indicator-class="getProgressIndicatorClass(row.disk)" />
                  <span class="w-12 text-right tabular-nums">{{ row.disk.toFixed(1) }}%</span>
                </div>
              </div>
            </TableCell>
            <TableCell class="text-center">
              <Badge variant="success">{{ row.containers }}</Badge>
            </TableCell>
            <TableCell class="max-[1199px]:hidden text-xs text-muted-foreground tabular-nums">
              {{ formatLastSeen(row.last_seen) }}
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>
  </div>
</template>
