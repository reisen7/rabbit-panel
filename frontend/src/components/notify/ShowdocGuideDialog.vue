<template>
  <el-dialog
    v-model="visible"
    :title="t('notify.guideTitle')"
    width="760px"
    class="showdoc-guide"
    align-center
    append-to-body
  >
    <p class="lead">{{ t('notify.guideLead') }}</p>

    <figure class="figure">
      <img :src="guideImage" :alt="t('notify.figureCaption')" />
      <figcaption>{{ t('notify.figureCaption') }}</figcaption>
    </figure>

    <ol class="steps">
      <li v-for="step in steps" :key="step.title">
        <strong>{{ step.title }}</strong>
        <p>{{ step.body }}</p>
      </li>
    </ol>

    <el-alert type="warning" :closable="false" show-icon>
      <template #title>{{ t('notify.notesTitle') }}</template>
      <ul class="notes">
        <li>{{ t('notify.note1') }}</li>
        <li>{{ t('notify.note2') }}</li>
        <li>{{ t('notify.note3') }}</li>
      </ul>
    </el-alert>

    <template #footer>
      <el-button type="primary" @click="visible = false">{{ t('notify.gotIt') }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/composables/useI18n'
import guideImage from '@/assets/showdoc-push-guide.png'

const { t } = useI18n()

const props = defineProps<{
  modelValue: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value),
})

const steps = computed(() => [
  { title: t('notify.step1Title'), body: t('notify.step1') },
  { title: t('notify.step2Title'), body: t('notify.step2') },
  { title: t('notify.step3Title'), body: t('notify.step3') },
  { title: t('notify.step4Title'), body: t('notify.step4') },
  { title: t('notify.step5Title'), body: t('notify.step5') },
])
</script>

<style scoped>
.lead {
  margin: 0 0 16px;
  line-height: 1.7;
  color: var(--el-text-color-regular);
}

.figure {
  margin: 0 0 18px;
}

.figure img {
  display: block;
  width: 100%;
  border-radius: 8px;
  border: 1px solid var(--el-border-color);
}

.figure figcaption {
  margin-top: 8px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--el-text-color-secondary);
}

.steps {
  margin: 0 0 16px;
  padding-left: 20px;
}

.steps li + li {
  margin-top: 12px;
}

.steps strong {
  display: block;
  margin-bottom: 4px;
}

.steps p {
  margin: 0;
  line-height: 1.7;
  color: var(--el-text-color-regular);
}

.notes {
  margin: 8px 0 0;
  padding-left: 18px;
  line-height: 1.7;
}
</style>

<style>
.showdoc-guide.el-dialog {
  max-height: 90vh;
  display: flex;
  flex-direction: column;
}

.showdoc-guide .el-dialog__body {
  overflow-y: auto;
}
</style>
