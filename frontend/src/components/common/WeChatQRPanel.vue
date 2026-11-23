<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { Loader2, QrCode } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import type { WeChatQR } from '@/api/wechat'
import { useWeChatQR } from '@/composables/useWeChatQR'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const props = defineProps<{
  initial?: WeChatQR | null
}>()

const emit = defineEmits<{
  linked: []
}>()

const { t } = useI18n()
const { qr, creating, code, create, resume, submit } = useWeChatQR(() => emit('linked'))

const statusText = computed(() => {
  switch (qr.value?.status) {
    case 'scanned':
      return t('agent.wechat.scanned')
    case 'linked':
      return t('agent.wechat.linked')
    case 'expired':
      return t('agent.wechat.expired')
    case 'blocked':
      return t('agent.wechat.blocked')
    case 'verify':
      return t('agent.wechat.verify')
    case 'error':
      return qr.value.message || t('agent.wechat.failed')
    default:
      return t('agent.wechat.pending')
  }
})

onMounted(() => {
  resume(props.initial)
})
</script>

<template>
  <div class="flex flex-col items-start gap-3">
    <Button :disabled="creating" @click="create">
      <Loader2 v-if="creating" class="animate-spin" />
      <QrCode v-else />
      {{ t('agent.wechat.createQR') }}
    </Button>

    <div v-if="qr?.image" class="flex flex-col items-start gap-2.5">
      <img
        class="size-[220px] rounded-lg border bg-white p-2 object-contain shadow-xs"
        :src="qr.image"
        :alt="t('agent.wechat.qrAlt')"
      />
      <div class="text-sm text-muted-foreground">{{ statusText }}</div>
      <div v-if="qr.status === 'verify'" class="flex flex-col sm:flex-row gap-2 w-full max-w-[320px]">
        <Input
          v-model="code"
          :placeholder="t('agent.wechat.verifyPlaceholder')"
          @keyup.enter="submit"
        />
        <Button @click="submit">{{ t('agent.wechat.verifySubmit') }}</Button>
      </div>
    </div>
  </div>
</template>
