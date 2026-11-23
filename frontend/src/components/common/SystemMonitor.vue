<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useSystemStore } from '@/stores/system'
import { useI18n } from '@/composables/useI18n'
import { Progress } from '@/components/ui/progress'

const systemStore = useSystemStore()
const { t } = useI18n()

onMounted(() => {
  systemStore.startPolling()
})

onUnmounted(() => {
  systemStore.stopPolling()
})

function getProgressClass(percent: number): string {
  if (percent >= 90) return 'bg-destructive'
  if (percent >= 70) return 'bg-amber-500'
  return 'bg-emerald-500'
}
</script>

<template>
  <div class="hidden md:flex items-center gap-4 lg:gap-6">
    <!-- CPU -->
    <div class="flex items-center gap-2">
      <span class="text-xs text-muted-foreground whitespace-nowrap">{{ t('system.cpu') }}</span>
      <Progress
        :model-value="systemStore.stats.cpu"
        class="h-1.5 w-14 lg:w-20"
        :indicator-class="getProgressClass(systemStore.stats.cpu)"
      />
      <span class="text-xs font-medium tabular-nums text-foreground min-w-[42px] text-right">
        {{ systemStore.stats.cpu.toFixed(1) }}%
      </span>
    </div>

    <!-- Memory -->
    <div class="flex items-center gap-2">
      <span class="text-xs text-muted-foreground whitespace-nowrap">{{ t('system.memory') }}</span>
      <Progress
        :model-value="systemStore.stats.memory"
        class="h-1.5 w-14 lg:w-20"
        :indicator-class="getProgressClass(systemStore.stats.memory)"
      />
      <span class="text-xs font-medium tabular-nums text-foreground min-w-[42px] text-right">
        {{ systemStore.stats.memory.toFixed(1) }}%
      </span>
    </div>

    <!-- Disk -->
    <div class="flex items-center gap-2">
      <span class="text-xs text-muted-foreground whitespace-nowrap">{{ t('system.disk') }}</span>
      <Progress
        :model-value="systemStore.stats.disk"
        class="h-1.5 w-14 lg:w-20"
        :indicator-class="getProgressClass(systemStore.stats.disk)"
      />
      <span class="text-xs font-medium tabular-nums text-foreground min-w-[42px] text-right">
        {{ systemStore.stats.disk.toFixed(1) }}%
      </span>
    </div>

    <!-- Server Time -->
    <div class="hidden xl:flex items-center gap-2 pl-2 border-l border-border">
      <span class="text-xs text-muted-foreground whitespace-nowrap">{{ t('system.serverTime') }}</span>
      <span class="text-xs font-medium font-mono tabular-nums text-foreground">
        {{ systemStore.stats.time || '--:--:--' }}
      </span>
    </div>
  </div>
</template>
