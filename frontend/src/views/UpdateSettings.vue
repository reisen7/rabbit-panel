<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ExternalLink, Loader2, RefreshCw } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { useUpdateStore } from '@/stores/update'
import UpdateBanner from '@/components/common/UpdateBanner.vue'
import UpdateDialog from '@/components/common/UpdateDialog.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

const { t } = useI18n()
const toast = useToast()
const updateStore = useUpdateStore()
const showDialog = ref(false)

function formatTime(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

async function handleRunUpdate() {
  try {
    const message = await updateStore.runUpdate()
    toast.success(message)
  } catch {
    // handled by interceptor
  }
}

async function handleIgnore() {
  await updateStore.ignoreVersion()
  toast.success(t('update.ignoreSuccess'))
}

function openRelease() {
  if (updateStore.info?.release_url) {
    window.open(updateStore.info.release_url, '_blank')
  }
}

onMounted(() => {
  if (!updateStore.info) {
    updateStore.fetchUpdateInfo()
  }
})
</script>

<template>
  <div class="space-y-5">
    <UpdateBanner @open-dialog="showDialog = true" />

    <Card>
      <CardHeader class="flex flex-row items-center justify-between space-y-0">
        <CardTitle class="text-lg">{{ t('update.title') }}</CardTitle>
        <Button variant="outline" size="sm" :disabled="updateStore.loading" @click="updateStore.fetchUpdateInfo({ force: true })">
          <RefreshCw :class="{ 'animate-spin': updateStore.loading }" />
          {{ t('update.checkNow') }}
        </Button>
      </CardHeader>

      <CardContent class="space-y-5">
        <div v-if="updateStore.info" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
          <div class="rounded-lg border bg-muted/30 p-3.5 space-y-1">
            <span class="text-xs text-muted-foreground block">{{ t('update.currentVersion') }}</span>
            <strong class="text-sm font-semibold tabular-nums">{{ updateStore.info.current_version || 'dev' }}</strong>
          </div>
          <div class="rounded-lg border bg-muted/30 p-3.5 space-y-1">
            <span class="text-xs text-muted-foreground block">{{ t('update.currentCommit') }}</span>
            <strong class="text-sm font-semibold font-mono">{{ updateStore.info.current_commit || '-' }}</strong>
          </div>
          <div class="rounded-lg border bg-muted/30 p-3.5 space-y-1">
            <span class="text-xs text-muted-foreground block">{{ t('update.currentBuildTime') }}</span>
            <strong class="text-sm font-semibold tabular-nums">{{ formatTime(updateStore.info.current_build_time) }}</strong>
          </div>
          <div class="rounded-lg border bg-muted/30 p-3.5 space-y-1">
            <span class="text-xs text-muted-foreground block">{{ t('update.latestVersion') }}</span>
            <strong class="text-sm font-semibold tabular-nums">{{ updateStore.info.latest_version || '-' }}</strong>
          </div>
          <div class="rounded-lg border bg-muted/30 p-3.5 space-y-1">
            <span class="text-xs text-muted-foreground block">{{ t('update.deployMode') }}</span>
            <strong class="text-sm font-semibold">{{ updateStore.info.deploy_mode }}</strong>
          </div>
          <div class="rounded-lg border bg-muted/30 p-3.5 space-y-1">
            <span class="text-xs text-muted-foreground block">{{ t('update.imageInfo') }}</span>
            <strong class="text-sm font-semibold font-mono break-all">{{ updateStore.info.image }}:{{ updateStore.info.image_tag }}</strong>
          </div>
          <div class="rounded-lg border bg-muted/30 p-3.5 space-y-1">
            <span class="text-xs text-muted-foreground block">{{ t('update.lastCheckTime') }}</span>
            <strong class="text-sm font-semibold tabular-nums">{{ formatTime(updateStore.info.last_check_time) }}</strong>
          </div>
          <div class="rounded-lg border bg-muted/30 p-3.5 space-y-1">
            <span class="text-xs text-muted-foreground block">{{ t('update.lastUpdateStatus') }}</span>
            <strong class="text-sm font-semibold">{{ updateStore.info.last_update_status || '-' }}</strong>
          </div>
        </div>

        <div class="flex flex-wrap gap-2.5">
          <Button
            :disabled="!updateStore.info?.can_update || updateStore.running"
            @click="handleRunUpdate"
          >
            <Loader2 v-if="updateStore.running" class="animate-spin" />
            {{ t('update.updateNow') }}
          </Button>
          <Button variant="outline" @click="updateStore.remindLater()">{{ t('update.remindLater') }}</Button>
          <Button variant="outline" @click="handleIgnore">{{ t('update.ignoreVersion') }}</Button>
          <Button v-if="updateStore.info?.release_url" variant="outline" @click="openRelease">
            <ExternalLink />
            {{ t('update.viewRelease') }}
          </Button>
        </div>

        <Card class="bg-muted/20">
          <CardHeader class="pb-2">
            <CardTitle class="text-sm">{{ t('update.releaseNotes') }}</CardTitle>
          </CardHeader>
          <CardContent>
            <pre class="m-0 whitespace-pre-wrap break-words max-h-[320px] overflow-auto text-xs font-mono leading-relaxed text-muted-foreground">{{ updateStore.info?.release_notes || '-' }}</pre>
          </CardContent>
        </Card>
      </CardContent>
    </Card>

    <UpdateDialog v-model="showDialog" />
  </div>
</template>
