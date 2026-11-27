<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ChevronLeft,
  ChevronRight,
  Loader2,
  Maximize2,
  Minimize2,
  PackageSearch,
  Search,
  X,
} from 'lucide-vue-next'
import { useImageSearchStore } from '@/stores/imageSearch'
import { useFloatWindowsStore } from '@/stores/floatWindows'
import { useToast } from '@/composables/useToast'
import { buildImageRef, buildPullCommand, isValidTag, type ImageRefSource } from '@/utils/imageRef'
import { copyText } from '@/utils/clipboard'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import ImageSearchCard from '@/components/common/ImageSearchCard.vue'

const { t } = useI18n()
const toast = useToast()
const store = useImageSearchStore()
const floatWindows = useFloatWindowsStore()

// 展开状态由 floatWindows store 派生，侧边栏入口可直接打开本窗口
const isOpen = computed({
  get: () => floatWindows.active === 'imageSearch',
  set: (v) => {
    if (v) floatWindows.active = 'imageSearch'
    else if (floatWindows.active === 'imageSearch') floatWindows.active = null
  },
})
const isExpanded = ref(false)

// Reka Tabs 的 modelValue 是普通 string，收窄到 store 的联合类型
const activeTabModel = computed<'search' | 'favorites'>({
  get: () => store.activeTab,
  set: (v) => {
    if (v === 'search' || v === 'favorites') store.activeTab = v
  },
})

watch(isOpen, (val) => {
  if (val) store.loadFavorites()
})

watch(
  () => store.activeTab,
  (val) => {
    if (val === 'favorites') store.loadFavorites()
  }
)

const toggleExpand = () => {
  isExpanded.value = !isExpanded.value
}

const onSearch = () => {
  store.search(store.query, 1)
}

const goPage = (p: number) => {
  if (p >= 1 && p <= store.totalPages) store.search(undefined, p)
}

// 搜索条目用当前输入的 tag，收藏条目用其收藏时存的 tag
const tagOf = (item: ImageRefSource & { tag?: string }): string => {
  return item.tag && isValidTag(item.tag) ? item.tag : store.tag
}

const commandOf = (item: ImageRefSource & { tag?: string }): string => {
  return buildPullCommand(item, tagOf(item), store.source)
}

const isPulling = (item: ImageRefSource & { tag?: string }): boolean => {
  return store.pullingKeys.has(buildImageRef(item, tagOf(item), store.source))
}

const copyCommand = async (command: string) => {
  const ok = await copyText(command)
  if (ok) {
    toast.success(t('imageSearch.copied'))
  } else {
    toast.error(t('imageSearch.copyFailed'))
  }
}
</script>

<template>
  <div class="fixed left-3 bottom-3 z-40">
    <!-- Search Window -->
    <transition name="chat-popup">
      <div
        v-show="isOpen"
        class="absolute left-0 bottom-0 flex flex-col rounded-xl border bg-card text-card-foreground shadow-2xl overflow-hidden origin-bottom-left transition-all duration-200"
        :class="isExpanded ? 'w-[min(52vw,760px)] h-[72vh]' : 'w-[420px] max-w-[calc(100vw-1.5rem)] h-[560px]'"
      >
        <div class="flex items-center justify-between px-4 py-3 border-b bg-muted/40">
          <div class="flex items-center gap-2 font-semibold text-sm">
            <PackageSearch class="size-4 text-primary" />
            <span>{{ t('imageSearch.title') }}</span>
          </div>
          <div class="flex items-center gap-1">
            <Button
              variant="ghost"
              size="icon-sm"
              :title="isExpanded ? t('imageSearch.shrink') : t('imageSearch.expand')"
              @click="toggleExpand"
            >
              <Minimize2 v-if="isExpanded" class="size-4" />
              <Maximize2 v-else class="size-4" />
            </Button>
            <Button variant="ghost" size="icon-sm" @click="isOpen = false">
              <X class="size-4" />
            </Button>
          </div>
        </div>

        <Tabs v-model="activeTabModel" class="flex min-h-0 flex-1 flex-col gap-0">
          <div class="px-3 pt-2.5">
            <TabsList class="w-full">
              <TabsTrigger value="search" class="flex-1">{{ t('imageSearch.searchTab') }}</TabsTrigger>
              <TabsTrigger value="favorites" class="flex-1">
                {{ t('imageSearch.favoritesTab', { n: store.favorites.length }) }}
              </TabsTrigger>
            </TabsList>
          </div>

          <TabsContent value="search" class="flex min-h-0 flex-1 flex-col">
            <div class="flex items-center gap-2 px-3 pt-2">
              <Input
                v-model="store.query"
                :placeholder="t('imageSearch.searchPlaceholder')"
                class="h-8 flex-1 text-sm"
                @keydown.enter.prevent="onSearch"
              />
              <Button size="sm" class="h-8 shrink-0" :disabled="store.loading" @click="onSearch">
                <Loader2 v-if="store.loading" class="size-3.5 animate-spin" />
                <Search v-else class="size-3.5" />
                {{ t('imageSearch.search') }}
              </Button>
            </div>
            <div class="flex items-center gap-2 px-3 pt-2">
              <Input
                v-model="store.tag"
                :placeholder="t('imageSearch.tagPlaceholder')"
                class="h-7 w-28 text-xs"
              />
              <div class="flex flex-1 rounded-md border p-0.5 text-xs">
                <button
                  type="button"
                  class="flex-1 rounded px-2 py-1 transition-colors"
                  :class="store.source === 'mirror' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'"
                  @click="store.source = 'mirror'"
                >
                  {{ t('imageSearch.sourceMirror') }}
                </button>
                <button
                  type="button"
                  class="flex-1 rounded px-2 py-1 transition-colors"
                  :class="store.source === 'origin' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'"
                  @click="store.source = 'origin'"
                >
                  {{ t('imageSearch.sourceOrigin') }}
                </button>
              </div>
            </div>

            <div class="mt-2 min-h-0 flex-1 overflow-y-auto px-3 pb-2 space-y-2">
              <div v-if="store.loading" class="space-y-2 pt-1">
                <Skeleton v-for="i in 4" :key="i" class="h-[84px] w-full rounded-lg" />
              </div>

              <div
                v-else-if="store.error"
                class="flex h-full flex-col items-center justify-center gap-3 text-center"
              >
                <p class="text-sm text-muted-foreground">{{ t('imageSearch.error') }}</p>
                <Button variant="outline" size="sm" @click="store.search()">
                  {{ t('imageSearch.retry') }}
                </Button>
              </div>

              <div
                v-else-if="store.items.length === 0 && store.query.trim()"
                class="flex h-full items-center justify-center"
              >
                <p class="text-sm text-muted-foreground">{{ t('imageSearch.empty') }}</p>
              </div>

              <div
                v-else-if="store.items.length === 0"
                class="flex h-full items-center justify-center px-4 text-center"
              >
                <p class="text-sm text-muted-foreground">{{ t('imageSearch.initialHint') }}</p>
              </div>

              <template v-else>
                <ImageSearchCard
                  v-for="item in store.items"
                  :key="item.repository_name"
                  :repository-name="item.repository_name"
                  :description="item.description"
                  :logo-url="item.logo_url"
                  :pull-count="item.pull_count"
                  :star-count="item.star_count"
                  :is-official="item.is_official"
                  :command="commandOf(item)"
                  :favorited="store.isFavorited(item)"
                  :pulling="isPulling(item)"
                  @pull="store.pull(item)"
                  @toggle-favorite="store.toggleFavorite(item)"
                  @copy="copyCommand(commandOf(item))"
                />
              </template>
            </div>

            <div
              v-if="store.total > 0"
              class="flex items-center justify-between border-t px-3 py-2 text-xs text-muted-foreground"
            >
              <Button
                variant="ghost"
                size="sm"
                class="h-7 px-2 text-xs"
                :disabled="store.page <= 1 || store.loading"
                @click="goPage(store.page - 1)"
              >
                <ChevronLeft class="size-3.5" />
                {{ t('imageSearch.prev') }}
              </Button>
              <span>
                {{ t('imageSearch.pageInfo', { page: store.page, pages: store.totalPages, total: store.total }) }}
              </span>
              <Button
                variant="ghost"
                size="sm"
                class="h-7 px-2 text-xs"
                :disabled="store.page >= store.totalPages || store.loading"
                @click="goPage(store.page + 1)"
              >
                {{ t('imageSearch.next') }}
                <ChevronRight class="size-3.5" />
              </Button>
            </div>
          </TabsContent>

          <TabsContent value="favorites" class="flex min-h-0 flex-1 flex-col">
            <div class="min-h-0 flex-1 overflow-y-auto px-3 py-2 space-y-2">
              <div v-if="store.favoritesLoading" class="space-y-2 pt-1">
                <Skeleton v-for="i in 3" :key="i" class="h-[84px] w-full rounded-lg" />
              </div>
              <div
                v-else-if="store.favorites.length === 0"
                class="flex h-full items-center justify-center px-4 text-center"
              >
                <p class="text-sm text-muted-foreground">{{ t('imageSearch.favoritesEmpty') }}</p>
              </div>
              <template v-else>
                <ImageSearchCard
                  v-for="fav in store.favorites"
                  :key="fav.id"
                  :repository-name="fav.repository_name"
                  :description="fav.description"
                  :logo-url="fav.logo_url"
                  :pull-count="fav.pull_count"
                  :star-count="fav.star_count"
                  :is-official="fav.is_official"
                  :command="commandOf(fav)"
                  :favorited="true"
                  :pulling="isPulling(fav)"
                  @pull="store.pull(fav)"
                  @toggle-favorite="store.toggleFavorite(fav)"
                  @copy="copyCommand(commandOf(fav))"
                />
              </template>
            </div>
          </TabsContent>
        </Tabs>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.chat-popup-enter-active,
.chat-popup-leave-active {
  transition: all 0.2s ease;
}
.chat-popup-enter-from,
.chat-popup-leave-to {
  opacity: 0;
  transform: scale(0.92) translateY(12px);
}
</style>
