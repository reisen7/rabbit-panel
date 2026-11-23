<script setup lang="ts">
import { X } from 'lucide-vue-next'
import { useRouter } from 'vue-router'
import { useI18n } from '@/composables/useI18n'
import { useContainerCreateStore } from '@/stores/containerCreate'

const { t } = useI18n()
const router = useRouter()
const store = useContainerCreateStore()

function handleCardClick() {
  router.push({ name: 'containers' })
  store.restore()
}

function handleClose() {
  if (store.loading || store.isStreaming) {
    store.cancel()
    return
  }
  store.dismiss()
}
</script>

<template>
  <transition name="fade">
    <div
      v-if="store.minimized && store.hasTaskState"
      class="fixed right-6 bottom-24 z-50 w-90 rounded-xl border bg-popover text-popover-foreground p-4 pr-10 shadow-xl cursor-pointer"
      @click="handleCardClick"
    >
      <button
        type="button"
        class="absolute top-2.5 right-2.5 size-6 rounded-full bg-muted flex items-center justify-center hover:bg-accent"
        @click.stop="handleClose"
      >
        <X class="size-3.5" />
      </button>
      <div class="flex items-center justify-between gap-3 mb-1.5 text-sm">
        <strong>{{ t('container.create') }}</strong>
        <span class="text-xs text-muted-foreground">
          {{ store.isStreaming ? t('container.building') : t('common.refresh') }}
        </span>
      </div>
      <div class="text-xs text-muted-foreground truncate font-mono">{{ store.latestLogLine }}</div>
    </div>
  </transition>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
