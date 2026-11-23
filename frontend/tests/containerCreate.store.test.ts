import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useContainerCreateStore } from '@/stores/containerCreate'

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

const createStreamMock = vi.fn()
vi.mock('@/api/containers', () => ({
  containerApi: {
    createStream: (...args: unknown[]) => createStreamMock(...args),
  },
}))

import { toast } from '@/composables/useToast'

const form = {
  image: 'nginx:latest',
  name: 'web',
  restart: 'no',
  network: 'bridge',
  ports: [],
  volumes: [],
  envs: [],
}
const payload = {
  image: 'nginx:latest',
  name: 'web',
  restart: 'no',
  network: 'bridge',
  ports: [],
  volumes: [],
  env: [],
}

describe('containerCreate store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    createStreamMock.mockReset()
    vi.clearAllMocks()
  })

  it('start sets streaming state and appends logs', async () => {
    createStreamMock.mockImplementation(async (_p, onMessage, _signal) => {
      expect(useContainerCreateStore().isStreaming).toBe(true)
      onMessage({ type: 'log', message: 'pulling image' })
      return { container_id: 'abc' }
    })
    const store = useContainerCreateStore()
    await store.start(form, payload)
    expect(store.streamLogs).toHaveLength(1)
    expect(store.streamLogs[0].message).toBe('pulling image')
    expect(store.lastForm?.image).toBe('nginx:latest')
  })

  it('success resets flags, bumps createdSignal, toasts once, keeps minimized', async () => {
    createStreamMock.mockResolvedValue({ container_id: 'abc' })
    const store = useContainerCreateStore()
    store.minimize()
    const before = store.createdSignal
    await store.start(form, payload)
    expect(store.loading).toBe(false)
    expect(store.isStreaming).toBe(false)
    expect(store.createdSignal).toBe(before + 1)
    expect(store.minimized).toBe(true)
    expect(toast.success).toHaveBeenCalledTimes(1)

    store.dismiss()
    await store.start(form, payload)
    expect(store.minimized).toBe(false)
  })

  it('minimize/restore/consumeRestore keep state without components', async () => {
    let resolveStream: (v: unknown) => void = () => {}
    createStreamMock.mockImplementation((_p, _onMessage, _signal) => new Promise((r) => { resolveStream = r }))
    const store = useContainerCreateStore()
    const task = store.start(form, payload)
    store.minimize()
    expect(store.minimized).toBe(true)
    expect(store.isStreaming).toBe(true)
    store.restore()
    expect(store.minimized).toBe(false)
    expect(store.restorePending).toBe(true)
    expect(store.consumeRestore()).toBe(true)
    expect(store.consumeRestore()).toBe(false)
    resolveStream({})
    await task
  })

  it('cancel aborts, appends cancel log, clears flags', async () => {
    let capturedSignal: AbortSignal | undefined
    createStreamMock.mockImplementation((_p, _onMessage, signal: AbortSignal) => {
      capturedSignal = signal
      return new Promise((_, reject) => {
        signal.addEventListener('abort', () => reject(Object.assign(new Error('aborted'), { name: 'AbortError' })))
      })
    })
    const store = useContainerCreateStore()
    const task = store.start(form, payload)
    store.cancel()
    expect(capturedSignal?.aborted).toBe(true)
    expect(store.loading).toBe(false)
    expect(store.isStreaming).toBe(false)
    expect(store.streamLogs[store.streamLogs.length - 1].message).toBe('已取消创建任务')
    await task
  })

  it('reset is a no-op while running, clears when idle', async () => {
    createStreamMock.mockImplementation(async (_p, onMessage) => {
      onMessage({ type: 'log', message: 'pulling image' })
      return {}
    })
    const store = useContainerCreateStore()
    const task = store.start(form, payload)
    store.reset()
    expect(store.isStreaming).toBe(true)
    await task
    expect(store.streamLogs).toHaveLength(1)
    store.reset()
    expect(store.streamLogs).toHaveLength(0)
    expect(store.minimized).toBe(false)
  })
})
