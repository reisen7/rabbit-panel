<script setup lang="ts">
import type { AlertDialogActionProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import { AlertDialogAction } from 'reka-ui'
import { buttonVariants } from '@/components/ui/button'
import type { ButtonVariants } from '@/components/ui/button'
import { cn } from '@/lib/utils'

const props = defineProps<AlertDialogActionProps & {
  class?: HTMLAttributes['class']
  variant?: ButtonVariants['variant']
}>()

const delegatedProps = reactiveOmit(props, 'class', 'variant')
</script>

<template>
  <AlertDialogAction
    data-slot="alert-dialog-action"
    v-bind="delegatedProps"
    :class="cn(buttonVariants({ variant }), props.class)"
  >
    <slot />
  </AlertDialogAction>
</template>
