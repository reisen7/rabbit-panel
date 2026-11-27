import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useImageSearchStore } from '@/stores/imageSearch'

const { favoriteApiMocks, pullMock } = vi.hoisted(() => ({
  favoriteApiMocks: {
    list: vi.fn(),
    add: vi.fn(),
    remove: vi.fn(),
  },
  pullMock: vi.fn(),
}))

vi.mock('@/utils/request', () => ({
  default: { get: vi.fn(), post: vi.fn(), delete: vi.fn() },
  handleUnauthorized: vi.fn(),
}))

vi.mock('@/api/imageSearch', async (importOriginal) => {
  const mod = await importOriginal<typeof import('@/api/imageSearch')>()
  return { ...mod, imageFavoriteApi: favoriteApiMocks }
})

vi.mock('@/api/images', () => ({
  imageApi: { pull: (...args: unknown[]) => pullMock(...args) },
}))

vi.mock('@/composables/useToast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
    info: vi.fn(),
    loading: vi.fn(),
    dismiss: vi.fn(),
  },
  useToast: () => ({
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
    info: vi.fn(),
    loading: vi.fn(),
    dismiss: vi.fn(),
  }),
}))

const fetchMock = vi.fn()
vi.stubGlobal('fetch', fetchMock)

const rawItem = (over: Record<string, unknown> = {}) => ({
  id: 532,
  name: 'php',
  namespace: 'library',
  registry: 'docker.io',
  repository_name: 'library/php',
  pull_domain: 'docker.1ms.run',
  logo_url: 'https://static.1ms.run/image/avatar/abc',
  description: 'PHP image',
  pull_count: 100,
  star_count: 5,
  is_official: true,
  ...over,
})

const okResponse = (list: unknown[], total = list.length, page = 1) => ({
  ok: true,
  status: 200,
  json: async () => ({ code: 0, msg: 'ok', data: { total, page, page_size: 12, list } }),
})

describe('imageSearch store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    sessionStorage.clear()
    vi.clearAllMocks()
    fetchMock.mockReset()
    pullMock.mockReset()
    favoriteApiMocks.list.mockResolvedValue([])
  })

  it('search populates items and persists to sessionStorage', async () => {
    fetchMock.mockResolvedValue(okResponse([rawItem()], 42, 1))
    const store = useImageSearchStore()
    await store.search('php')

    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(String(fetchMock.mock.calls[0][0])).toContain('query=php')
    expect(store.items).toHaveLength(1)
    expect(store.items[0].repository_name).toBe('library/php')
    expect(store.total).toBe(42)

    const saved = JSON.parse(sessionStorage.getItem('rabbit-image-search') || '{}')
    expect(saved.query).toBe('php')
    expect(saved.items).toHaveLength(1)
  })

  it('a fresh store instance restores persisted state', async () => {
    fetchMock.mockResolvedValue(okResponse([rawItem()], 7, 2))
    const store = useImageSearchStore()
    store.tag = '8.4'
    await store.search('php', 2)

    setActivePinia(createPinia())
    const restored = useImageSearchStore()
    expect(restored.query).toBe('php')
    expect(restored.page).toBe(2)
    expect(restored.total).toBe(7)
    expect(restored.items).toHaveLength(1)
    expect(restored.items[0].repository_name).toBe('library/php')
    expect(restored.tag).toBe('8.4')
  })

  it('ignores a stale response when a newer search starts', async () => {
    const resolvers: Array<(v: unknown) => void> = []
    fetchMock.mockImplementation(() => new Promise((resolve) => resolvers.push(resolve)))

    const store = useImageSearchStore()
    const p1 = store.search('first')
    const p2 = store.search('second')

    // 慢的旧请求后返回，绝不能覆盖新结果
    resolvers[1](okResponse([rawItem({ repository_name: 'library/second' })]))
    await p2
    resolvers[0](okResponse([rawItem({ repository_name: 'library/first' })]))
    await p1

    expect(store.query).toBe('second')
    expect(store.items).toHaveLength(1)
    expect(store.items[0].repository_name).toBe('library/second')
    expect(store.loading).toBe(false)
  })

  it('drops malformed results from display', async () => {
    fetchMock.mockResolvedValue(
      okResponse([
        rawItem(),
        rawItem({ repository_name: 'php; rm -rf /' }),
        rawItem({ repository_name: 'a b' }),
        rawItem({ pull_domain: 'evil;id' }),
      ])
    )
    const store = useImageSearchStore()
    await store.search('php')
    expect(store.items).toHaveLength(1)
    expect(store.items[0].repository_name).toBe('library/php')
  })

  it('toggleFavorite adds and removes via the API', async () => {
    const store = useImageSearchStore()
    favoriteApiMocks.add.mockResolvedValue({
      id: 9,
      repository_name: 'library/php',
      registry: 'docker.io',
      pull_domain: 'docker.1ms.run',
      tag: 'latest',
      description: '',
      logo_url: '',
      pull_count: 100,
      star_count: 5,
      is_official: true,
      created_at: '2026-01-01',
    })
    favoriteApiMocks.remove.mockResolvedValue(undefined)

    const item = store.items[0] ?? (await (async () => {
      fetchMock.mockResolvedValue(okResponse([rawItem()]))
      await store.search('php')
      return store.items[0]
    })())

    expect(store.isFavorited(item)).toBe(false)
    await store.toggleFavorite(item)
    expect(favoriteApiMocks.add).toHaveBeenCalledTimes(1)
    expect(favoriteApiMocks.add.mock.calls[0][0].tag).toBe('latest')
    expect(store.favorites).toHaveLength(1)
    expect(store.isFavorited(item)).toBe(true)

    await store.toggleFavorite(item)
    expect(favoriteApiMocks.remove).toHaveBeenCalledWith(9)
    expect(store.favorites).toHaveLength(0)
    expect(store.isFavorited(item)).toBe(false)
  })

  it('pull calls the images API with the displayed ref', async () => {
    fetchMock.mockResolvedValue(okResponse([rawItem()]))
    pullMock.mockResolvedValue(undefined)
    const store = useImageSearchStore()
    await store.search('php')
    store.source = 'mirror'
    store.tag = '8.4'

    await store.pull(store.items[0])
    expect(pullMock).toHaveBeenCalledWith('docker.1ms.run/library/php:8.4')
  })

  it('search failure surfaces an error state', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 500, json: async () => ({}) })
    const store = useImageSearchStore()
    await store.search('php')
    expect(store.error).toBeTruthy()
    expect(store.loading).toBe(false)
  })

  it('restores a persisted catalog activeTab', () => {
    sessionStorage.setItem(
      'rabbit-image-search',
      JSON.stringify({ activeTab: 'catalog', query: '', page: 1, total: 0, items: [], tag: 'latest', source: 'mirror' })
    )
    setActivePinia(createPinia())
    const store = useImageSearchStore()
    expect(store.activeTab).toBe('catalog')
  })

  it('falls back to the default tab for garbage persisted values', () => {
    sessionStorage.setItem(
      'rabbit-image-search',
      JSON.stringify({ activeTab: 'nonsense', query: '', page: 1, total: 0, items: [], tag: 'latest', source: 'mirror' })
    )
    setActivePinia(createPinia())
    const store = useImageSearchStore()
    expect(store.activeTab).toBe('search')
  })
})
