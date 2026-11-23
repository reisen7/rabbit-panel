<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useNetworkStore } from '@/stores/networks'
import { useI18n } from '@/composables/useI18n'
import type { NetworkDetail } from '@/types'
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

const props = defineProps<{
  visible: boolean
  networkId: string
}>()

const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void
}>()

const { t } = useI18n()
const networkStore = useNetworkStore()

const dialogVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
})

const loading = ref(false)
const error = ref<string | null>(null)
const networkDetail = ref<NetworkDetail | null>(null)

async function loadNetworkDetails() {
  if (!props.networkId) return

  try {
    loading.value = true
    error.value = null
    networkDetail.value = await networkStore.inspectNetwork(props.networkId)
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load network details'
    networkDetail.value = null
  } finally {
    loading.value = false
  }
}

watch(
  () => props.visible,
  (val) => {
    if (val) {
      loadNetworkDetails()
    }
  },
  { immediate: true }
)

watch(() => props.networkId, () => {
  networkDetail.value = null
  error.value = null
  if (props.visible && props.networkId) {
    loadNetworkDetails()
  }
})
</script>

<template>
  <Dialog v-model:open="dialogVisible">
    <DialogContent class="sm:max-w-[700px] max-h-[88vh] flex flex-col">
      <DialogHeader>
        <DialogTitle>{{ t('network.details') }}</DialogTitle>
      </DialogHeader>

      <div class="relative flex-1 overflow-y-auto min-h-[200px] space-y-5 pr-1">
        <LoadingOverlay :loading="loading" />

        <template v-if="networkDetail">
          <div class="space-y-2.5">
            <h4 class="text-sm font-semibold">{{ t('config.basicInfo') }}</h4>
            <div class="grid grid-cols-1 sm:grid-cols-2 rounded-lg border divide-y sm:divide-y-0 sm:divide-x overflow-hidden text-sm">
              <div class="p-3 border-b space-y-1">
                <div class="text-xs text-muted-foreground">{{ t('network.id') }}</div>
                <div class="font-mono text-xs break-all">{{ networkDetail.id }}</div>
              </div>
              <div class="p-3 border-b space-y-1">
                <div class="text-xs text-muted-foreground">{{ t('common.name') }}</div>
                <div class="font-medium">{{ networkDetail.name }}</div>
              </div>
              <div class="p-3 border-b space-y-1">
                <div class="text-xs text-muted-foreground">{{ t('network.driver') }}</div>
                <div><Badge variant="secondary">{{ networkDetail.driver }}</Badge></div>
              </div>
              <div class="p-3 border-b space-y-1">
                <div class="text-xs text-muted-foreground">{{ t('network.scope') }}</div>
                <div>{{ networkDetail.scope }}</div>
              </div>
              <div class="p-3 border-b sm:border-b-0 space-y-1">
                <div class="text-xs text-muted-foreground">{{ t('network.subnet') }}</div>
                <div class="font-mono text-xs">{{ networkDetail.subnet || '-' }}</div>
              </div>
              <div class="p-3 border-b sm:border-b-0 space-y-1">
                <div class="text-xs text-muted-foreground">{{ t('network.gateway') }}</div>
                <div class="font-mono text-xs">{{ networkDetail.gateway || '-' }}</div>
              </div>
              <div class="p-3 sm:col-span-2 border-t flex items-center justify-between">
                <span class="text-xs text-muted-foreground">{{ t('network.internal') }}</span>
                <Badge :variant="networkDetail.internal ? 'warning' : 'success'">
                  {{ networkDetail.internal ? t('common.yes') : t('common.no') }}
                </Badge>
              </div>
            </div>
          </div>

          <div class="space-y-2.5">
            <h4 class="text-sm font-semibold">{{ t('network.connectedContainers') }}</h4>
            <div v-if="networkDetail.connectedContainers?.length" class="rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{{ t('common.name') }}</TableHead>
                    <TableHead>{{ t('network.id') }}</TableHead>
                    <TableHead>{{ t('config.ipAddress') }}</TableHead>
                    <TableHead>{{ t('config.macAddress') }}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow v-for="row in networkDetail.connectedContainers" :key="row.id">
                    <TableCell class="font-medium">{{ row.name }}</TableCell>
                    <TableCell class="font-mono text-xs">{{ row.id.substring(0, 12) }}</TableCell>
                    <TableCell class="font-mono text-xs">{{ row.ipAddress || '-' }}</TableCell>
                    <TableCell class="font-mono text-xs">{{ row.macAddress || '-' }}</TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </div>
            <div v-else class="rounded-lg border border-dashed py-8 text-center text-sm text-muted-foreground">
              {{ t('network.noContainers') }}
            </div>
          </div>
        </template>

        <div v-else-if="error" class="rounded-lg border border-dashed py-10 text-center text-sm text-destructive">
          {{ error }}
        </div>
      </div>

      <DialogFooter class="pt-2">
        <Button variant="outline" @click="dialogVisible = false">{{ t('common.close') }}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
