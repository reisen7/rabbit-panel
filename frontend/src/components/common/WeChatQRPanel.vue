<template>
  <div class="wechat-qr">
    <el-button type="primary" :loading="creating" @click="create">
      {{ t('agent.wechat.createQR') }}
    </el-button>
    <div v-if="qr?.image" class="qr-box">
      <img class="qr-image" :src="qr.image" :alt="t('agent.wechat.qrAlt')" />
      <div class="qr-status">{{ statusText }}</div>
      <div v-if="qr.status === 'verify'" class="verify-row">
        <el-input v-model="code" :placeholder="t('agent.wechat.verifyPlaceholder')" @keyup.enter="submit" />
        <el-button type="primary" @click="submit">{{ t('agent.wechat.verifySubmit') }}</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import type { WeChatQR } from '@/api/wechat'
import { useWeChatQR } from '@/composables/useWeChatQR'

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

<style scoped>
.wechat-qr {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 12px;
}
.qr-box {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
}
.qr-image {
  width: 220px;
  height: 220px;
  padding: 8px;
  background: #fff;
  border-radius: 8px;
  object-fit: contain;
}
.qr-status {
  font-size: 14px;
}
.verify-row {
  display: flex;
  gap: 8px;
  width: min(100%, 320px);
}
@media (max-width: 768px) {
  .verify-row {
    flex-direction: column;
    width: 100%;
  }
}
</style>
