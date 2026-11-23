<script setup lang="ts">
import { Loader2 } from 'lucide-vue-next'

defineProps<{
  loading?: boolean
}>()
</script>

<template>
  <div
    v-if="loading"
    class="absolute inset-0 z-10 flex items-center justify-center bg-background/60"
  >
    <Loader2 class="size-8 animate-spin text-primary" />
  </div>
</template>
