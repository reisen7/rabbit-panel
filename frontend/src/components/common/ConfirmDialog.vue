<script setup lang="ts">
import { useConfirm } from '@/composables/useConfirm'
import { useI18n } from '@/composables/useI18n'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

const { t } = useI18n()
const { pending, handleResult } = useConfirm()
</script>

<template>
  <AlertDialog :open="!!pending">
    <AlertDialogContent @escape-key-down="handleResult(false)">
      <AlertDialogHeader>
        <AlertDialogTitle>{{ pending?.title || t('common.confirm') }}</AlertDialogTitle>
        <AlertDialogDescription v-if="pending?.description">
          {{ pending.description }}
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel @click="handleResult(false)">
          {{ pending?.cancelText || t('common.cancel') }}
        </AlertDialogCancel>
        <AlertDialogAction
          :variant="pending?.destructive ? 'destructive' : 'default'"
          @click="handleResult(true)"
        >
          {{ pending?.confirmText || t('common.confirm') }}
        </AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>
