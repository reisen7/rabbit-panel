<script setup lang="ts">
import type { ToasterProps } from 'vue-sonner'
import { Toaster } from 'vue-sonner'
import { useTheme } from '@/composables/useTheme'
import 'vue-sonner/style.css'

const props = defineProps<ToasterProps>()
const { theme } = useTheme()
</script>

<template>
  <Toaster v-bind="props" :theme="theme" />
</template>
