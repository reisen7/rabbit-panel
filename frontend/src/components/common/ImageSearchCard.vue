<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Copy, Download, Loader2, Package, Star } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

const { t } = useI18n()

const props = defineProps<{
  repositoryName: string
  description: string
  logoUrl: string
  pullCount: number
  starCount: number
  isOfficial: boolean
  command: string
  favorited: boolean
  pulling: boolean
}>()

const emit = defineEmits<{
  pull: []
  toggleFavorite: []
  copy: []
}>()

// logo 加载失败时降级为图标；URL 变化时按 URL 记录失败态
const failedLogo = ref('')
const showLogo = () => props.logoUrl !== '' && failedLogo.value !== props.logoUrl

const formatCount = (n: number): string => {
  if (n >= 1e9) return `${(n / 1e9).toFixed(1)}B`
  if (n >= 1e6) return `${(n / 1e6).toFixed(1)}M`
  if (n >= 1e3) return `${(n / 1e3).toFixed(1)}K`
  return String(n)
}
</script>

<template>
  <div class="rounded-lg border bg-card p-3 shadow-2xs">
    <div class="flex items-start gap-2.5">
      <img
        v-if="showLogo()"
        :src="logoUrl"
        loading="lazy"
        referrerpolicy="no-referrer"
        class="size-8 shrink-0 rounded object-contain"
        @error="failedLogo = logoUrl"
      />
      <div v-else class="size-8 shrink-0 rounded bg-muted flex items-center justify-center text-muted-foreground">
        <Package class="size-4" />
      </div>
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-1.5">
          <span class="truncate font-medium text-sm">{{ repositoryName }}</span>
          <Badge v-if="isOfficial" variant="success" class="shrink-0">{{ t('imageSearch.official') }}</Badge>
          <div class="ml-auto flex shrink-0 items-center gap-1">
            <Button
              variant="ghost"
              size="icon-sm"
              :title="favorited ? t('imageSearch.removeFavorite') : t('imageSearch.addFavorite')"
              @click="emit('toggleFavorite')"
            >
              <Star class="size-4" :class="favorited ? 'fill-amber-400 text-amber-400' : ''" />
            </Button>
            <Button
              size="sm"
              variant="outline"
              class="h-7 px-2 text-xs"
              :disabled="pulling"
              :title="t('imageSearch.pullTip')"
              @click="emit('pull')"
            >
              <Loader2 v-if="pulling" class="size-3.5 animate-spin" />
              <Download v-else class="size-3.5" />
              {{ t('imageSearch.pull') }}
            </Button>
          </div>
        </div>
        <p v-if="description" class="mt-0.5 text-xs text-muted-foreground line-clamp-2">{{ description }}</p>
        <div class="mt-1 flex items-center gap-3 text-[11px] text-muted-foreground">
          <span class="flex items-center gap-1">
            <Download class="size-3" />{{ formatCount(pullCount) }}
          </span>
          <span class="flex items-center gap-1">
            <Star class="size-3" />{{ formatCount(starCount) }}
          </span>
        </div>
        <div class="mt-1.5 flex items-center gap-1.5">
          <code class="flex-1 truncate rounded bg-muted px-2 py-1 font-mono text-[11px]">{{ command }}</code>
          <Button
            variant="ghost"
            size="icon-sm"
            class="shrink-0"
            :title="t('imageSearch.copy')"
            @click="emit('copy')"
          >
            <Copy class="size-3.5" />
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
