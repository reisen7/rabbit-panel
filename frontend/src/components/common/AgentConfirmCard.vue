<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ShieldAlert, Check, X, Loader2 } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { ConfirmRequest } from '@/composables/useAgentChat'

const { t } = useI18n()

defineProps<{
  item: ConfirmRequest
  compact?: boolean
}>()

const emit = defineEmits<{
  resolve: [item: ConfirmRequest, approve: boolean]
}>()
</script>

<template>
  <div
    class="rounded-xl border border-amber-300/60 bg-amber-50 dark:border-amber-800 dark:bg-amber-950/40 shadow-2xs"
    :class="compact ? 'px-3 py-2.5' : 'px-4 py-3'"
  >
    <div class="flex items-center gap-2 text-amber-700 dark:text-amber-400">
      <ShieldAlert class="size-4 shrink-0" />
      <span class="font-semibold" :class="compact ? 'text-xs' : 'text-sm'">
        {{ t('agent.confirmTitle') }}
      </span>
      <Badge v-if="item.state === 'approved' || item.state === 'done'" variant="success">{{ t('agent.confirmApproved') }}</Badge>
      <Badge v-else-if="item.state === 'rejected'" variant="destructive">{{ t('agent.confirmRejected') }}</Badge>
      <Badge v-else-if="item.state === 'expired'" variant="info">{{ t('agent.confirmExpired') }}</Badge>
    </div>

    <template v-if="item.state === 'pending'">
      <p class="mt-1.5 text-muted-foreground" :class="compact ? 'text-xs' : 'text-sm'">
        {{ t('agent.confirmDesc') }}
      </p>
      <p class="mt-1 font-medium" :class="compact ? 'text-xs' : 'text-sm'">{{ item.summary }}</p>
      <code
        class="mt-1.5 block rounded bg-muted px-2 py-1 font-mono break-all"
        :class="compact ? 'text-[11px]' : 'text-xs'"
      >{{ item.command }}</code>
      <div class="mt-2.5 flex gap-2">
        <Button size="sm" variant="destructive" @click="emit('resolve', item, true)">
          <Check class="size-3.5" />
          {{ t('agent.confirmApprove') }}
        </Button>
        <Button size="sm" variant="outline" @click="emit('resolve', item, false)">
          <X class="size-3.5" />
          {{ t('agent.confirmReject') }}
        </Button>
      </div>
    </template>

    <template v-else>
      <p class="mt-1 flex items-center gap-1.5 text-muted-foreground" :class="compact ? 'text-xs' : 'text-sm'">
        <Loader2 v-if="item.state === 'approved'" class="size-3 shrink-0 animate-spin" />
        <Check v-else-if="item.state === 'done'" class="size-3 shrink-0 text-emerald-600 dark:text-emerald-400" />
        <span class="truncate">{{ item.summary }}</span>
      </p>
    </template>
  </div>
</template>
