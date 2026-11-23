<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useI18n } from '@/composables/useI18n'

const props = withDefaults(defineProps<{
  currentPage: number
  pageSize: number
  total: number
  pageSizes?: number[]
}>(), {
  pageSizes: () => [10, 20, 50, 100],
})

const emit = defineEmits<{
  'update:currentPage': [value: number]
  'update:pageSize': [value: number]
  'current-change': [page: number]
  'size-change': [size: number]
}>()

const { t } = useI18n()

const jumperValue = ref('')

const pageCount = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))

const pages = computed<Array<number | 'ellipsis'>>(() => {
  const count = pageCount.value
  const current = props.currentPage
  if (count <= 7) {
    return Array.from({ length: count }, (_, i) => i + 1)
  }
  const list: Array<number | 'ellipsis'> = [1]
  if (current > 4) {
    list.push('ellipsis')
  }
  for (let i = Math.max(2, current - 1); i <= Math.min(count - 1, current + 1); i++) {
    list.push(i)
  }
  if (current < count - 3) {
    list.push('ellipsis')
  }
  list.push(count)
  return list
})

const totalText = computed(() => t('pagination.total').replace('{total}', String(props.total)))

function goToPage(page: number) {
  const target = Math.min(Math.max(1, page), pageCount.value)
  if (target === props.currentPage) return
  emit('update:currentPage', target)
  emit('current-change', target)
}

function handleSizeChange(value: unknown) {
  const size = Number(value)
  if (!size || size === props.pageSize) return
  emit('update:pageSize', size)
  emit('size-change', size)
}

function handleJumper() {
  const page = Number(jumperValue.value)
  jumperValue.value = ''
  if (!Number.isInteger(page) || page < 1) return
  goToPage(page)
}

watch(() => props.currentPage, () => {
  jumperValue.value = ''
})
</script>

<template>
  <div class="flex flex-wrap items-center gap-3 text-sm text-foreground">
    <span class="text-muted-foreground whitespace-nowrap">{{ totalText }}</span>

    <Select :model-value="String(pageSize)" @update:model-value="handleSizeChange">
      <SelectTrigger class="w-[110px]" size="sm">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem v-for="size in pageSizes" :key="size" :value="String(size)">
          {{ size }} {{ t('pagination.itemsPerPage') }}
        </SelectItem>
      </SelectContent>
    </Select>

    <div class="flex items-center gap-1">
      <Button
        variant="outline"
        size="icon-sm"
        :disabled="currentPage <= 1"
        :aria-label="t('pagination.prev')"
        @click="goToPage(currentPage - 1)"
      >
        <ChevronLeft />
      </Button>

      <template v-for="(page, index) in pages" :key="index">
        <span v-if="page === 'ellipsis'" class="px-1 text-muted-foreground">…</span>
        <Button
          v-else
          :variant="page === currentPage ? 'default' : 'outline'"
          size="icon-sm"
          @click="goToPage(page)"
        >
          {{ page }}
        </Button>
      </template>

      <Button
        variant="outline"
        size="icon-sm"
        :disabled="currentPage >= pageCount"
        :aria-label="t('pagination.next')"
        @click="goToPage(currentPage + 1)"
      >
        <ChevronRight />
      </Button>
    </div>

    <div class="flex items-center gap-2">
      <span class="text-muted-foreground whitespace-nowrap">{{ t('pagination.goto') }}</span>
      <Input
        v-model="jumperValue"
        class="h-8 w-16"
        @keydown.enter="handleJumper"
      />
    </div>
  </div>
</template>
