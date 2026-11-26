import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import { toast } from '@/composables/useToast'
import i18n from '@/locales'
import { imageApi } from '@/api/images'
import { imageFavoriteApi, searchRegistry, type ImageFavoriteRecord } from '@/api/imageSearch'
import {
  buildImageRef,
  isValidTag,
  resolveTag,
  sanitizeSearchItem,
  type ImageRefSource,
  type PullSource,
  type SearchItem,
} from '@/utils/imageRef'

const STORAGE_KEY = 'rabbit-image-search'

interface PersistedState {
  query: string
  page: number
  total: number
  items: SearchItem[]
  tag: string
  source: PullSource
  activeTab: 'search' | 'favorites'
}

// 收藏列表条目本身也带仓库/域名/标签字段，直接复用 ImageRefSource 即可
export type FavoriteItem = ImageFavoriteRecord

// 镜像搜索悬浮窗状态。搜索结果持久化到 sessionStorage，
// 路由切换与页内刷新都不会丢失当前结果页。
export const useImageSearchStore = defineStore('imageSearch', () => {
  const { t } = i18n.global

  const query = ref('')
  const page = ref(1)
  const pageSize = ref(12)
  const total = ref(0)
  const items = ref<SearchItem[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const tag = ref('latest')
  const source = ref<PullSource>('mirror')
  const activeTab = ref<'search' | 'favorites'>('search')
  const favorites = ref<ImageFavoriteRecord[]>([])
  const favoritesLoading = ref(false)
  const pullingKeys = ref<Set<string>>(new Set())

  let requestSeq = 0
  let abortController: AbortController | null = null

  // 恢复上次的搜索状态（store 初始化时执行一次）
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    if (raw) {
      const saved = JSON.parse(raw) as Partial<PersistedState>
      if (typeof saved.query === 'string') query.value = saved.query
      if (Number.isInteger(saved.page) && (saved.page as number) > 0) page.value = saved.page as number
      if (Number.isInteger(saved.total) && (saved.total as number) >= 0) total.value = saved.total as number
      if (Array.isArray(saved.items)) {
        items.value = saved.items
          .map(sanitizeSearchItem)
          .filter((item): item is SearchItem => item !== null)
      }
      if (typeof saved.tag === 'string') tag.value = saved.tag
      if (saved.source === 'mirror' || saved.source === 'origin') source.value = saved.source
      if (saved.activeTab === 'search' || saved.activeTab === 'favorites') activeTab.value = saved.activeTab
    }
  } catch {
    // sessionStorage 不可用或数据损坏时忽略
  }

  function persist(): void {
    try {
      const state: PersistedState = {
        query: query.value,
        page: page.value,
        total: total.value,
        items: items.value,
        tag: tag.value,
        source: source.value,
        activeTab: activeTab.value,
      }
      sessionStorage.setItem(STORAGE_KEY, JSON.stringify(state))
    } catch {
      // 存储配额/隐私模式下静默失败
    }
  }

  watch([tag, source, activeTab], persist)

  const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

  function effectiveTag(): string {
    return resolveTag(tag.value)
  }

  async function search(q?: string, p?: number): Promise<void> {
    if (q !== undefined) query.value = q
    if (p !== undefined) page.value = p

    const trimmed = query.value.trim()
    const seq = ++requestSeq
    abortController?.abort()
    if (!trimmed) {
      abortController = null
      items.value = []
      total.value = 0
      page.value = 1
      error.value = null
      loading.value = false
      persist()
      return
    }

    const controller = new AbortController()
    abortController = controller
    loading.value = true
    error.value = null

    try {
      const result = await searchRegistry(trimmed, page.value, pageSize.value, controller.signal)
      if (seq !== requestSeq) return // 更新的请求已发出，丢弃过期响应
      items.value = result.items
      total.value = result.total
      page.value = result.page
      persist()
    } catch (e) {
      if (seq !== requestSeq || (e as Error)?.name === 'AbortError') return
      error.value = e instanceof Error ? e.message : 'search failed'
    } finally {
      if (seq === requestSeq) loading.value = false
    }
  }

  async function loadFavorites(): Promise<void> {
    favoritesLoading.value = true
    try {
      favorites.value = await imageFavoriteApi.list()
    } catch {
      favorites.value = []
    } finally {
      favoritesLoading.value = false
    }
  }

  function favoriteTagOf(item: Partial<Pick<ImageFavoriteRecord, 'tag' | 'repository_name'>>): string {
    return item.tag && isValidTag(item.tag) ? item.tag : effectiveTag()
  }

  function isFavorited(item: ImageRefSource & { tag?: string }): boolean {
    const itemTag = favoriteTagOf(item)
    return favorites.value.some(
      (f) => f.repository_name === item.repository_name && f.tag === itemTag
    )
  }

  async function toggleFavorite(item: SearchItem | ImageFavoriteRecord): Promise<void> {
    const itemTag = favoriteTagOf(item)
    const existing = favorites.value.find(
      (f) => f.repository_name === item.repository_name && f.tag === itemTag
    )
    if (existing) {
      await imageFavoriteApi.remove(existing.id)
      favorites.value = favorites.value.filter((f) => f.id !== existing.id)
      return
    }
    const record = await imageFavoriteApi.add({
      repository_name: item.repository_name,
      registry: item.registry,
      pull_domain: item.pull_domain,
      tag: itemTag,
      description: item.description,
      logo_url: item.logo_url,
      pull_count: item.pull_count,
      star_count: item.star_count,
      is_official: item.is_official,
    })
    favorites.value = [
      record,
      ...favorites.value.filter(
        (f) => !(f.repository_name === record.repository_name && f.tag === record.tag)
      ),
    ]
  }

  // pull 拉取到当前选中节点；命令与卡片上展示的 docker pull 完全一致
  async function pull(item: ImageRefSource & { tag?: string }): Promise<void> {
    const ref = buildImageRef(item, favoriteTagOf(item), source.value)
    if (pullingKeys.value.has(ref)) return
    pullingKeys.value = new Set(pullingKeys.value).add(ref)
    try {
      await imageApi.pull(ref)
      toast.success(t('imageSearch.pullSuccess', { name: ref }))
    } catch (e) {
      const serverMsg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error
      const msg = serverMsg || (e instanceof Error ? e.message : String(e))
      toast.error(t('imageSearch.pullFailed', { name: ref }) + (msg ? `：${msg}` : ''))
    } finally {
      const next = new Set(pullingKeys.value)
      next.delete(ref)
      pullingKeys.value = next
    }
  }

  function isPulling(ref: string): boolean {
    return pullingKeys.value.has(ref)
  }

  return {
    query,
    page,
    pageSize,
    total,
    items,
    loading,
    error,
    tag,
    source,
    activeTab,
    favorites,
    favoritesLoading,
    pullingKeys,
    totalPages,
    search,
    loadFavorites,
    isFavorited,
    toggleFavorite,
    pull,
    isPulling,
  }
})
