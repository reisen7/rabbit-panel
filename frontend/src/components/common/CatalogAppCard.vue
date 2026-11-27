<script setup lang="ts">
import { computed, type FunctionalComponent } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Copy,
  Database,
  GitBranch,
  Globe,
  MessageSquare,
  Package,
  Rocket,
} from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { AppCategory, CatalogApp } from '@/data/appCatalog'

const { t, locale } = useI18n()

const props = defineProps<{
  app: CatalogApp
  imageRef: string
}>()

const emit = defineEmits<{
  deploy: []
  copy: []
}>()

const CATEGORY_ICONS: Record<AppCategory, FunctionalComponent> = {
  database: Database,
  mq: MessageSquare,
  web: Globe,
  devops: GitBranch,
  app: Package,
}

const icon = computed(() => CATEGORY_ICONS[props.app.category] ?? Package)
const isZh = computed(() => locale.value.startsWith('zh'))
const desc = computed(() => (isZh.value ? props.app.desc.zh : props.app.desc.en))
const note = computed(() => (props.app.note ? (isZh.value ? props.app.note.zh : props.app.note.en) : ''))
</script>

<template>
  <div class="rounded-lg border bg-card p-3 shadow-2xs">
    <div class="flex items-start gap-2.5">
      <div class="size-8 shrink-0 rounded bg-muted flex items-center justify-center text-muted-foreground">
        <component :is="icon" class="size-4" />
      </div>
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-1.5">
          <span class="truncate font-medium text-sm">{{ app.name }}</span>
          <Badge
            v-for="port in app.ports"
            :key="port.host"
            variant="outline"
            class="shrink-0 font-mono text-[10px] px-1 py-0"
          >
            {{ port.host }}
          </Badge>
          <div class="ml-auto flex shrink-0 items-center gap-1">
            <Button size="sm" class="h-7 px-2 text-xs" :title="t('catalog.deployTip')" @click="emit('deploy')">
              <Rocket class="size-3.5" />
              {{ t('catalog.deploy') }}
            </Button>
          </div>
        </div>
        <p class="mt-0.5 text-xs text-muted-foreground line-clamp-2">{{ desc }}</p>
        <div class="mt-1.5 flex items-center gap-1.5">
          <code class="flex-1 truncate rounded bg-muted px-2 py-1 font-mono text-[11px]">{{ imageRef }}</code>
          <Button
            variant="ghost"
            size="icon-sm"
            class="shrink-0"
            :title="t('catalog.copyCommand')"
            @click="emit('copy')"
          >
            <Copy class="size-3.5" />
          </Button>
        </div>
        <p v-if="note" class="mt-1 text-[11px] text-muted-foreground/80">{{ note }}</p>
      </div>
    </div>
  </div>
</template>
