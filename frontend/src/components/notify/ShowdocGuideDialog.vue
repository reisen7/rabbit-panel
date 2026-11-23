<script setup lang="ts">
import { computed } from 'vue'
import { AlertTriangle } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import guideImage from '@/assets/showdoc-push-guide.png'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

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

<template>
  <Dialog v-model:open="visible">
    <DialogContent class="sm:max-w-[760px] max-h-[90vh] flex flex-col">
      <DialogHeader>
        <DialogTitle>{{ t('notify.guideTitle') }}</DialogTitle>
      </DialogHeader>

      <div class="flex-1 overflow-y-auto space-y-4 pr-1 text-sm">
        <p class="leading-relaxed text-muted-foreground">{{ t('notify.guideLead') }}</p>

        <figure class="space-y-2">
          <img
            :src="guideImage"
            :alt="t('notify.figureCaption')"
            class="block w-full rounded-lg border"
          />
          <figcaption class="text-xs leading-relaxed text-muted-foreground">
            {{ t('notify.figureCaption') }}
          </figcaption>
        </figure>

        <ol class="space-y-3 list-decimal pl-5">
          <li v-for="step in steps" :key="step.title" class="space-y-1">
            <strong class="block font-semibold text-foreground">{{ step.title }}</strong>
            <p class="leading-relaxed text-muted-foreground">{{ step.body }}</p>
          </li>
        </ol>

        <Alert variant="warning">
          <AlertTriangle class="size-4" />
          <AlertTitle>{{ t('notify.notesTitle') }}</AlertTitle>
          <AlertDescription>
            <ul class="mt-1 list-disc pl-4 space-y-1">
              <li>{{ t('notify.note1') }}</li>
              <li>{{ t('notify.note2') }}</li>
              <li>{{ t('notify.note3') }}</li>
            </ul>
          </AlertDescription>
        </Alert>
      </div>

      <DialogFooter class="pt-2">
        <Button @click="visible = false">{{ t('notify.gotIt') }}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
