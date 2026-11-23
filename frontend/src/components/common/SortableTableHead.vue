<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { ArrowDown, ArrowUp, ArrowUpDown } from 'lucide-vue-next'
import { TableHead } from '@/components/ui/table'
import { cn } from '@/lib/utils'

const props = defineProps<{
  prop: string
  sortField?: string
  sortOrder?: 'asc' | 'desc' | null
  class?: HTMLAttributes['class']
}>()

const emit = defineEmits<{
  sort: [prop: string, order: 'ascending' | 'descending']
}>()

function handleClick() {
  const order = props.sortField === props.prop && props.sortOrder === 'asc' ? 'descending' : 'ascending'
  emit('sort', props.prop, order)
}
</script>

<template>
  <TableHead :class="cn('cursor-pointer select-none', props.class)" @click="handleClick">
    <span class="inline-flex items-center gap-1">
      <slot />
      <ArrowUp v-if="sortField === prop && sortOrder === 'asc'" class="size-3.5" />
      <ArrowDown v-else-if="sortField === prop && sortOrder === 'desc'" class="size-3.5" />
      <ArrowUpDown v-else class="size-3.5 text-muted-foreground/50" />
    </span>
  </TableHead>
</template>
