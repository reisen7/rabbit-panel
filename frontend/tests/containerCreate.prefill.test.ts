import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useContainerCreateStore } from '@/stores/containerCreate'
import type { ContainerCreateForm } from '@/stores/containerCreate'

vi.mock('@/composables/useToast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
    info: vi.fn(),
    loading: vi.fn(),
    dismiss: vi.fn(),
  },
}))

vi.mock('@/composables/useI18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('@/api/containers', () => ({
  containerApi: {
    createStream: vi.fn(),
  },
}))

const sampleForm = (): ContainerCreateForm => ({
  image: 'docker.1ms.run/library/mysql:8',
  name: 'mysql',
  restart: 'unless-stopped',
  network: 'bridge',
  ports: [{ host: '3306', container: '3306' }],
  volumes: [{ host: 'mysql-data', container: '/var/lib/mysql' }],
  envs: [{ key: 'MYSQL_ROOT_PASSWORD', value: 'abc123' }],
})

describe('containerCreate prefill', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('consumePrefill returns the form exactly once', () => {
    const store = useContainerCreateStore()
    expect(store.consumePrefill()).toBeNull()
    store.requestPrefill(sampleForm())
    expect(store.pendingPrefill).not.toBeNull()
    const consumed = store.consumePrefill()
    expect(consumed?.image).toBe('docker.1ms.run/library/mysql:8')
    expect(consumed?.name).toBe('mysql')
    expect(store.consumePrefill()).toBeNull()
    expect(store.pendingPrefill).toBeNull()
  })

  it('prefillSeq increments on each request', () => {
    const store = useContainerCreateStore()
    const before = store.prefillSeq
    store.requestPrefill(sampleForm())
    expect(store.prefillSeq).toBe(before + 1)
    store.requestPrefill(sampleForm())
    expect(store.prefillSeq).toBe(before + 2)
  })

  it('requestPrefill deep-copies so later mutations do not leak in', () => {
    const store = useContainerCreateStore()
    const form = sampleForm()
    store.requestPrefill(form)
    form.image = 'changed'
    form.ports[0]!.host = '9999'
    const consumed = store.consumePrefill()
    expect(consumed?.image).toBe('docker.1ms.run/library/mysql:8')
    expect(consumed?.ports[0]?.host).toBe('3306')
  })
})
