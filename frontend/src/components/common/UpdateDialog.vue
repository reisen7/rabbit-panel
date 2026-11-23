<script setup lang="ts">
import { computed } from 'vue'
import { AlertCircle, AlertTriangle, CheckCircle2, Info, Loader2 } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { useUpdateStore } from '@/stores/update'
import LoadingOverlay from '@/components/common/LoadingOverlay.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Progress } from '@/components/ui/progress'

const { t } = useI18n()
const toast = useToast()
const updateStore = useUpdateStore()

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: boolean): void }>()

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

const deployModeLabel = computed(() => {
  const mode = updateStore.info?.deploy_mode
  if (mode === 'docker') return 'Docker'
  if (mode === 'binary') return t('update.binary')
  return t('update.unknown')
})

const deployDescription = computed(() => {
  const info = updateStore.info
  if (!info) return ''
  if (info.deploy_mode === 'binary') return t('update.binaryDescription')
  if (info.deploy_mode === 'docker' && info.image_tag === 'latest') return t('update.dockerLatestDescription')
  if (info.deploy_mode === 'docker') return t('update.dockerFixedDescription')
  return t('update.unknownDescription')
})

const progressIndicatorClass = computed(() => {
  if (updateStore.taskStatus?.status === 'failed') return 'bg-destructive'
  if (updateStore.taskStatus?.status === 'success') return 'bg-emerald-500'
  if (updateStore.taskStatus?.status === 'running' && !updateStore.taskStatus.progress_known) {
    return 'animate-pulse'
  }
  return ''
})

const displayProgress = computed(() => {
  if (!updateStore.taskStatus) return 0
  if (updateStore.taskStatus.progress_known) return updateStore.taskStatus.progress
  return 100
})

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

async function handleApplyUpdate() {
  try {
    const message = await updateStore.applyUpdate()
    toast.success(message)
  } catch {
    // handled by interceptor
  }
}

async function handleIgnore() {
  await updateStore.ignoreVersion()
  toast.success(t('update.ignoreSuccess'))
}

async function handleClearIgnore() {
  await updateStore.clearIgnoredVersion()
  toast.success(t('update.clearIgnoreSuccess'))
}

async function handleClearState() {
  await updateStore.clearUpdateState()
  toast.success(t('update.clearStateSuccess'))
}

function handleRemindLater() {
  updateStore.remindLater()
  visible.value = false
}

function openRelease() {
  if (updateStore.info?.release_url) {
    window.open(updateStore.info.release_url, '_blank')
  }
}

function minimize() {
  updateStore.setMinimized(true)
  visible.value = false
}

function restore() {
  updateStore.setMinimized(false)
  visible.value = true
}
</script>

<template>
  <div>
    <Dialog v-model:open="visible">
      <DialogContent class="sm:max-w-[760px] max-h-[88vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>{{ t('update.title') }}</DialogTitle>
        </DialogHeader>

        <div class="relative flex-1 overflow-y-auto space-y-4 pr-1">
          <LoadingOverlay :loading="updateStore.loading" />
          <template v-if="updateStore.info">
            <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
              <div class="rounded-lg border bg-muted/40 p-3.5 flex flex-col gap-1">
                <span class="text-xs text-muted-foreground">{{ t('update.currentVersion') }}</span>
                <strong class="text-sm font-semibold tabular-nums">{{ updateStore.info.current_version || 'dev' }}</strong>
              </div>
              <div class="rounded-lg border bg-muted/40 p-3.5 flex flex-col gap-1">
                <span class="text-xs text-muted-foreground">{{ t('update.currentCommit') }}</span>
                <strong class="text-sm font-semibold font-mono">{{ updateStore.info.current_commit || '-' }}</strong>
              </div>
              <div class="rounded-lg border bg-muted/40 p-3.5 flex flex-col gap-1">
                <span class="text-xs text-muted-foreground">{{ t('update.currentBuildTime') }}</span>
                <strong class="text-sm font-semibold tabular-nums">{{ formatTime(updateStore.info.current_build_time) }}</strong>
              </div>
              <div class="rounded-lg border bg-muted/40 p-3.5 flex flex-col gap-1">
                <span class="text-xs text-muted-foreground">{{ t('update.latestVersion') }}</span>
                <strong class="text-sm font-semibold tabular-nums">{{ updateStore.info.latest_version || '-' }}</strong>
              </div>
              <div class="rounded-lg border bg-muted/40 p-3.5 flex flex-col gap-1">
                <span class="text-xs text-muted-foreground">{{ t('update.deployMode') }}</span>
                <strong class="text-sm font-semibold">{{ deployModeLabel }}</strong>
              </div>
              <div v-if="updateStore.info.deploy_mode === 'docker'" class="rounded-lg border bg-muted/40 p-3.5 flex flex-col gap-1">
                <span class="text-xs text-muted-foreground">{{ t('update.imageInfo') }}</span>
                <strong class="text-sm font-semibold font-mono break-all">{{ updateStore.info.image }}:{{ updateStore.info.image_tag }}</strong>
              </div>
            </div>

            <Alert :variant="updateStore.info.has_update ? 'warning' : 'info'">
              <AlertTriangle v-if="updateStore.info.has_update" class="size-4" />
              <Info v-else class="size-4" />
              <AlertTitle>{{ updateStore.info.message }}</AlertTitle>
              <AlertDescription>{{ deployDescription }}</AlertDescription>
            </Alert>

            <Alert v-if="updateStore.info.ignored" variant="info">
              <Info class="size-4" />
              <AlertTitle>{{ t('update.ignoredNotice') }}</AlertTitle>
            </Alert>

            <div
              v-if="updateStore.taskStatus && (updateStore.taskStatus.status || updateStore.taskStatus.log_lines.length)"
              class="rounded-xl border bg-muted/30 p-4 space-y-3"
            >
              <div class="flex flex-wrap items-center justify-between gap-2">
                <strong class="text-sm font-semibold">{{ t('update.updateProgress') }}</strong>
                <div class="flex flex-wrap items-center gap-1">
                  <Button variant="ghost" size="sm" :disabled="!updateStore.info?.ignored" @click="handleClearIgnore">
                    {{ t('update.clearIgnore') }}
                  </Button>
                  <Button variant="ghost" size="sm" :disabled="!updateStore.info?.last_update_status" @click="handleClearState">
                    {{ t('update.clearState') }}
                  </Button>
                  <Button variant="ghost" size="sm" @click="minimize">
                    {{ t('update.minimize') }}
                  </Button>
                  <Button variant="ghost" size="sm" @click="updateStore.fetchTaskStatus()">
                    {{ t('common.refresh') }}
                  </Button>
                </div>
              </div>

              <Progress :model-value="displayProgress" class="h-2" :indicator-class="progressIndicatorClass" />

              <div class="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                <span>{{ t('update.currentStage') }}: {{ updateStore.taskStatus.stage || '-' }}</span>
                <span>{{ t('update.lastUpdateStatus') }}: {{ updateStore.taskStatus.status || '-' }}</span>
                <span>{{ t('update.lastUpdateTime') }}: {{ formatTime(updateStore.taskStatus.last_update_time) }}</span>
              </div>

              <Alert v-if="updateStore.taskStatus.status === 'downloaded'" variant="success">
                <CheckCircle2 class="size-4" />
                <AlertTitle>{{ t('update.downloadReady') }}</AlertTitle>
              </Alert>
              <Alert v-if="updateStore.taskStatus.last_error" variant="destructive">
                <AlertCircle class="size-4" />
                <AlertTitle>{{ updateStore.taskStatus.last_error }}</AlertTitle>
              </Alert>
            </div>

            <div class="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
              <span>{{ t('update.lastCheckTime') }}: {{ formatTime(updateStore.info.last_check_time) }}</span>
              <span>{{ t('update.lastUpdateTime') }}: {{ formatTime(updateStore.info.last_update_time) }}</span>
              <span>{{ t('update.lastUpdateStatus') }}: {{ updateStore.info.last_update_status || '-' }}</span>
            </div>

            <Alert v-if="updateStore.info.last_update_error" variant="destructive">
              <AlertCircle class="size-4" />
              <AlertTitle>{{ updateStore.info.last_update_error }}</AlertTitle>
            </Alert>
          </template>
        </div>

        <DialogFooter class="sm:justify-start flex-wrap gap-2 pt-2">
          <Button variant="outline" @click="updateStore.fetchUpdateInfo({ force: true })">
            {{ t('update.checkNow') }}
          </Button>
          <Button
            :disabled="!updateStore.info?.has_update || !updateStore.info?.can_update || updateStore.taskStatus?.status === 'running' || updateStore.taskStatus?.status === 'downloaded' || updateStore.taskStatus?.status === 'applying' || updateStore.running"
            @click="handleRunUpdate"
          >
            <Loader2 v-if="updateStore.running" class="animate-spin" />
            {{ t('update.downloadNow') }}
          </Button>
          <Button
            variant="secondary"
            :disabled="updateStore.taskStatus?.status !== 'downloaded' || updateStore.running"
            @click="handleApplyUpdate"
          >
            <Loader2 v-if="updateStore.running" class="animate-spin" />
            {{ t('update.applyNow') }}
          </Button>
          <Button variant="outline" @click="handleRemindLater">{{ t('update.remindLater') }}</Button>
          <Button variant="outline" :disabled="!updateStore.info?.latest_version" @click="handleIgnore">
            {{ t('update.ignoreVersion') }}
          </Button>
          <Button v-if="updateStore.info?.release_url" variant="outline" @click="openRelease">
            {{ t('update.viewRelease') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <transition name="fade">
      <div
        v-if="updateStore.minimized && updateStore.taskStatus && updateStore.taskStatus.status === 'running' && updateStore.taskStatus.stage === 'downloading'"
        class="fixed right-6 bottom-6 z-50 w-80 rounded-xl border bg-popover text-popover-foreground p-4 shadow-lg cursor-pointer"
        @click="restore"
      >
        <div class="flex items-center justify-between text-sm mb-2">
          <strong>{{ t('update.updatingNow') }}</strong>
          <span class="tabular-nums text-muted-foreground">
            {{ updateStore.taskStatus.progress_known ? `${updateStore.taskStatus.progress}%` : '...' }}
          </span>
        </div>
        <Progress :model-value="displayProgress" class="h-1.5" :indicator-class="progressIndicatorClass" />
        <div class="mt-2 text-xs text-muted-foreground">{{ updateStore.taskStatus.stage }}</div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.fade-enter-active, .fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from, .fade-leave-to {
  opacity: 0;
}
</style>
