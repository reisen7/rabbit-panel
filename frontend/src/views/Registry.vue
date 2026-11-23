<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { RefreshCw, Plus, Pencil, Trash2, Building2, Cable, Loader2 } from 'lucide-vue-next'
import { useRegistryStore } from '@/stores/registry'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import RegistryDialog from '@/components/registry/RegistryDialog.vue'
import LoadingOverlay from '@/components/common/LoadingOverlay.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardFooter, CardHeader } from '@/components/ui/card'
import type { RegistryInfo } from '@/types'

const { t } = useI18n()
const registryStore = useRegistryStore()
const toast = useToast()
const { confirm } = useConfirm()

const showDialog = ref(false)
const editingRegistry = ref<RegistryInfo | null>(null)
const testingId = ref<string | null>(null)

function formatDate(dateStr: string): string {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleString()
}

function handleAdd() {
  editingRegistry.value = null
  showDialog.value = true
}

function handleEdit(registry: RegistryInfo) {
  editingRegistry.value = registry
  showDialog.value = true
}

async function confirmRemove(id: string) {
  const ok = await confirm({
    title: t('common.confirm'),
    description: t('registry.confirmRemove'),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    destructive: true,
  })
  if (!ok) return
  await handleRemove(id)
}

async function handleRemove(id: string) {
  try {
    await registryStore.removeRegistry(id)
    toast.success(t('registry.removeSuccess'))
  } catch {
    // Error is handled by request interceptor
  }
}

async function handleTest(registry: RegistryInfo) {
  try {
    testingId.value = registry.url
    const result = await registryStore.testRegistry(registry.url, {
      url: registry.url,
      username: registry.username,
    })
    if (result.success) {
      toast.success(t('registry.testSuccess'))
    } else {
      toast.error(`${t('registry.testFailed')}: ${result.message}`)
    }
  } catch {
    toast.error(t('registry.testFailed'))
  } finally {
    testingId.value = null
  }
}

async function handleRefresh() {
  await registryStore.fetchRegistries()
}

function handleSaved() {
  registryStore.fetchRegistries()
}

onMounted(() => {
  registryStore.fetchRegistries()
})
</script>

<template>
  <div class="space-y-5">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
      <div>
        <h2 class="text-xl font-semibold tracking-tight">{{ t('registry.title') }}</h2>
        <p class="mt-1 text-xs text-muted-foreground leading-relaxed">{{ t('registry.usage') }}</p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <Button @click="handleAdd">
          <Plus />
          {{ t('registry.add') }}
        </Button>
        <Button variant="outline" :disabled="registryStore.loading" @click="handleRefresh">
          <RefreshCw :class="{ 'animate-spin': registryStore.loading }" />
          {{ t('common.refresh') }}
        </Button>
      </div>
    </div>

    <!-- Registry cards -->
    <div class="relative grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
      <LoadingOverlay :loading="registryStore.loading" />
      <template v-if="registryStore.registries.length > 0">
        <Card
          v-for="registry in registryStore.registries"
          :key="registry.id"
          class="transition-shadow hover:shadow-md"
        >
          <CardHeader class="pb-2">
            <div class="flex items-center justify-between gap-2">
              <div class="flex items-center gap-2 font-semibold text-base min-w-0">
                <Building2 class="size-4 text-primary shrink-0" />
                <span class="truncate">{{ registry.name }}</span>
                <Badge v-if="registry.isDefault" variant="success">
                  {{ t('registry.default') }}
                </Badge>
              </div>
              <Button
                variant="ghost"
                size="sm"
                class="shrink-0"
                :disabled="testingId === registry.url"
                @click="handleTest(registry)"
              >
                <Loader2 v-if="testingId === registry.url" class="animate-spin" />
                <Cable v-else />
                {{ testingId === registry.url ? t('registry.testing') : t('registry.testConnection') }}
              </Button>
            </div>
          </CardHeader>

          <CardContent class="space-y-2 text-sm">
            <div class="flex gap-2">
              <span class="text-muted-foreground min-w-[80px] shrink-0">{{ t('registry.url') }}:</span>
              <span class="font-mono text-xs break-all text-foreground">{{ registry.url }}</span>
            </div>
            <div v-if="registry.username" class="flex gap-2">
              <span class="text-muted-foreground min-w-[80px] shrink-0">{{ t('registry.username') }}:</span>
              <span class="break-all">{{ registry.username }}</span>
            </div>
            <div class="flex gap-2">
              <span class="text-muted-foreground min-w-[80px] shrink-0">{{ t('common.createdAt') }}:</span>
              <span class="text-xs tabular-nums text-muted-foreground">{{ formatDate(registry.createdAt) }}</span>
            </div>
          </CardContent>

          <CardFooter class="border-t pt-4 gap-2">
            <Button size="sm" variant="outline" @click="handleEdit(registry)">
              <Pencil />
              {{ t('common.edit') }}
            </Button>
            <Button
              variant="destructive"
              size="sm"
              @click="confirmRemove(registry.url)"
            >
              <Trash2 />
              {{ t('common.delete') }}
            </Button>
          </CardFooter>
        </Card>
      </template>

      <div
        v-else-if="!registryStore.loading"
        class="col-span-full rounded-xl border border-dashed bg-card py-16 flex flex-col items-center justify-center gap-4 text-center"
      >
        <p class="text-sm text-muted-foreground">{{ t('registry.noRegistries') }}</p>
        <Button @click="handleAdd">
          <Plus />
          {{ t('registry.add') }}
        </Button>
      </div>
    </div>

    <RegistryDialog
      v-model:visible="showDialog"
      :registry="editingRegistry"
      @saved="handleSaved"
    />
  </div>
</template>
