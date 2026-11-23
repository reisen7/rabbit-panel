<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { Check, Settings, X } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Slider } from '@/components/ui/slider'

const { t } = useI18n()
const SETTINGS_KEY = 'rabbit_panel_settings'
const DEFAULT_COLOR = 'neutral'

const showPanel = ref(false)

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
  <div class="fixed right-5 bottom-5 z-50">
    <Button
      size="icon"
      class="size-12 rounded-full shadow-lg"
      @click="showPanel = !showPanel"
    >
      <Settings class="size-5" />
    </Button>

    <transition name="slide">
      <div
        v-show="showPanel"
        class="absolute right-0 bottom-15 w-72 rounded-xl border bg-popover text-popover-foreground shadow-xl overflow-hidden"
      >
        <div class="flex items-center justify-between px-4 py-3 border-b">
          <span class="text-sm font-semibold">{{ t('settings.pageSettings') }}</span>
          <Button variant="ghost" size="icon-sm" @click="showPanel = false">
            <X class="size-4" />
          </Button>
        </div>

        <div class="p-4 space-y-4">
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <Label class="text-xs">{{ t('settings.fontSize') }}</Label>
              <span class="text-xs font-mono tabular-nums text-muted-foreground">{{ settings.fontSize }}px</span>
            </div>
            <Slider
              :model-value="[settings.fontSize]"
              :min="12"
              :max="18"
              :step="1"
              @update:model-value="(v) => { if (v?.length) settings.fontSize = v[0]! }"
            />
          </div>

          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <Label class="text-xs">{{ t('settings.sidebarWidth') }}</Label>
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

          <div class="space-y-2">
            <Label class="text-xs">{{ t('settings.themeColor') }}</Label>
            <div class="flex flex-wrap gap-2">
              <button
                v-for="color in colorOptions"
                :key="color.value"
                type="button"
                :title="color.label"
                class="size-7 rounded-md border flex items-center justify-center transition-transform hover:scale-105"
                :class="settings.primaryColor === color.value ? 'ring-2 ring-foreground ring-offset-2 ring-offset-background' : 'border-border'"
                :style="{ backgroundColor: color.swatch }"
                @click="settings.primaryColor = color.value"
              >
                <Check v-if="settings.primaryColor === color.value" class="size-3.5 text-white" />
              </button>
            </div>
          </div>

          <div class="flex justify-end gap-2 pt-1">
            <Button variant="outline" size="sm" @click="resetSettings">{{ t('settings.reset') }}</Button>
            <Button size="sm" @click="confirmAndRefresh">{{ t('settings.confirmRefresh') }}</Button>
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.slide-enter-active,
.slide-leave-active {
  transition: all 0.2s ease;
}

.slide-enter-from,
.slide-leave-to {
  opacity: 0;
  transform: translateY(8px);
}
</style>
