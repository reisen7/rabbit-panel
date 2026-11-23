<script setup lang="ts">
import { ref, watch, onMounted, computed } from 'vue'
import { Check } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Slider } from '@/components/ui/slider'

const { t } = useI18n()

const props = defineProps<{
  modelValue: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val),
})

const SETTINGS_KEY = 'rabbit_panel_settings'
const DEFAULT_COLOR = 'neutral'

const settings = ref({
  fontSize: 14,
  sidebarWidth: 220,
  primaryColor: DEFAULT_COLOR,
})

const colorOptions = [
  { label: 'Neutral', value: 'neutral', swatch: '#18181b' },
  { label: 'Blue', value: '#2563eb', swatch: '#2563eb' },
  { label: 'Emerald', value: '#16a34a', swatch: '#16a34a' },
  { label: 'Amber', value: '#d97706', swatch: '#d97706' },
  { label: 'Rose', value: '#e11d48', swatch: '#e11d48' },
  { label: 'Violet', value: '#7c3aed', swatch: '#7c3aed' },
]

function applySettings() {
  const root = document.documentElement
  root.style.setProperty('--rp-font-size', `${settings.value.fontSize}px`)
  root.style.setProperty('--rp-sidebar-width', `${settings.value.sidebarWidth}px`)
  if (!settings.value.primaryColor || settings.value.primaryColor === 'neutral') {
    root.style.removeProperty('--primary')
    root.style.removeProperty('--primary-foreground')
  } else {
    root.style.setProperty('--primary', settings.value.primaryColor)
    root.style.setProperty('--primary-foreground', '#ffffff')
  }
  localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings.value))
}

function resetSettings() {
  settings.value = {
    fontSize: 14,
    sidebarWidth: 220,
    primaryColor: DEFAULT_COLOR,
  }
  applySettings()
}

function confirmAndRefresh() {
  applySettings()
  window.location.reload()
}

watch(
  settings,
  () => {
    applySettings()
  },
  { deep: true }
)

onMounted(() => {
  const saved = localStorage.getItem(SETTINGS_KEY)
  if (saved) {
    try {
      const parsed = JSON.parse(saved)
      settings.value = { ...settings.value, ...parsed }
    } catch (e) {
      console.error('Failed to parse settings', e)
    }
  }
  applySettings()
})
</script>

<template>
  <Dialog v-model:open="visible">
    <DialogContent class="sm:max-w-[420px]">
      <DialogHeader>
        <DialogTitle>{{ t('settings.pageSettings') }}</DialogTitle>
      </DialogHeader>

      <div class="space-y-5 py-2">
        <!-- Font Size -->
        <div class="space-y-2.5">
          <div class="flex items-center justify-between">
            <Label>{{ t('settings.fontSize') }}</Label>
            <span class="text-xs font-mono tabular-nums text-muted-foreground">{{ settings.fontSize }}px</span>
          </div>
          <Slider
            :model-value="[settings.fontSize]"
            :min="14"
            :max="20"
            :step="1"
            @update:model-value="(v) => { if (v?.length) settings.fontSize = v[0]! }"
          />
        </div>

        <!-- Sidebar Width -->
        <div class="space-y-2.5">
          <div class="flex items-center justify-between">
            <Label>{{ t('settings.sidebarWidth') }}</Label>
            <span class="text-xs font-mono tabular-nums text-muted-foreground">{{ settings.sidebarWidth }}px</span>
          </div>
          <Slider
            :model-value="[settings.sidebarWidth]"
            :min="180"
            :max="280"
            :step="10"
            @update:model-value="(v) => { if (v?.length) settings.sidebarWidth = v[0]! }"
          />
        </div>

        <!-- Theme Color -->
        <div class="space-y-2.5">
          <Label>{{ t('settings.themeColor') }}</Label>
          <div class="flex flex-wrap gap-2.5">
            <button
              v-for="color in colorOptions"
              :key="color.value"
              type="button"
              :title="color.label"
              class="size-8 rounded-md border flex items-center justify-center transition-transform hover:scale-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              :class="settings.primaryColor === color.value ? 'ring-2 ring-foreground ring-offset-2 ring-offset-background' : 'border-border'"
              :style="{ backgroundColor: color.swatch }"
              @click="settings.primaryColor = color.value"
            >
              <Check v-if="settings.primaryColor === color.value" class="size-4 text-white" />
            </button>
          </div>
        </div>
      </div>

      <DialogFooter class="gap-2">
        <Button variant="outline" @click="resetSettings">{{ t('settings.reset') }}</Button>
        <Button @click="confirmAndRefresh">{{ t('settings.confirmRefresh') }}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
