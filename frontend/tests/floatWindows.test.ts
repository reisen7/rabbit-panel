import { describe, it, expect, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useFloatWindowsStore } from '@/stores/floatWindows'

describe('floatWindows store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('switching from imageSearch to agent leaves agent active', () => {
    const store = useFloatWindowsStore()
    store.active = 'imageSearch'
    expect(store.active).toBe('imageSearch')
    store.active = 'agent'
    expect(store.active).toBe('agent')
  })

  it('null is a valid value (both windows closed)', () => {
    const store = useFloatWindowsStore()
    store.active = 'imageSearch'
    store.active = null
    expect(store.active).toBeNull()
  })
})
