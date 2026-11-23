<script setup lang="ts">
import { computed } from 'vue'
import { AlertTriangle, Loader2, X } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { useUpdateStore } from '@/stores/update'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

const emit = defineEmits<{ (e: 'open-dialog'): void }>()
const { t } = useI18n()
const toast = useToast()
const updateStore = useUpdateStore()

const description = computed(() => {
  const info = updateStore.info
  if (!info) return ''
  if (info.deploy_mode === 'binary') return t('update.binaryDescription')
  if (info.deploy_mode === 'docker' && info.image_tag === 'latest') return t('update.dockerLatestDescription')
  if (info.deploy_mode === 'docker') return t('update.dockerFixedDescription')
  return t('update.unknownDescription')
})

async function handleIgnore() {
  await updateStore.ignoreVersion()
  toast.success(t('update.ignoreSuccess'))
}
</script>

<template>
  <Alert
    v-if="updateStore.shouldShowBanner && updateStore.info"
    variant="warning"
    class="mb-5 pr-10 relative"
  >
    <AlertTriangle class="size-4" />
    <AlertTitle>{{ updateStore.info.message }}</AlertTitle>
    <AlertDescription>
      <div class="mt-1 flex flex-wrap items-center justify-between gap-3">
        <span>{{ description }}</span>
        <div class="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm" @click="updateStore.fetchUpdateInfo({ force: true })">
            {{ t('update.checkNow') }}
          </Button>
          <Button
            size="sm"
            :disabled="!updateStore.info.has_update || !updateStore.info.can_update || updateStore.running"
            @click="emit('open-dialog')"
          >
            <Loader2 v-if="updateStore.running" class="animate-spin" />
            {{ t('update.updateNow') }}
          </Button>
          <Button variant="outline" size="sm" @click="emit('open-dialog')">
            {{ t('update.details') }}
          </Button>
          <Button variant="ghost" size="sm" @click="handleIgnore">
            {{ t('update.ignoreVersion') }}
          </Button>
        </div>
      </div>
    </AlertDescription>
    <button
      type="button"
      class="absolute top-3 right-3 rounded-xs p-1 opacity-70 hover:opacity-100 transition-opacity"
      :aria-label="t('common.close')"
      @click="updateStore.remindLater"
    >
      <X class="size-4" />
    </button>
  </Alert>
</template>
