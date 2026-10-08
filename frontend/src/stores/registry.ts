import { defineStore } from 'pinia'
import { ref } from 'vue'
import { registryApi } from '@/api/registry'
import type { RegistryInfo, CreateRegistryRequest, RegistryTestResult } from '@/types'

/**
 * Registry store
 * Manages registry list and operations
 */
export const useRegistryStore = defineStore('registry', () => {
  // State
  const registries = ref<RegistryInfo[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Actions

  /**
   * Fetch registries from API
   */
  function prepareNodeSwitch(): void {
    registries.value = []
    error.value = null
  }

  async function fetchRegistries(): Promise<void> {
    try {
      loading.value = true
      error.value = null
      registries.value = await registryApi.list()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch registries'
    } finally {
      loading.value = false
    }
  }

  /**
   * Create a new registry
   * @param data Registry creation request
   */
  async function createRegistry(data: CreateRegistryRequest): Promise<void> {
    await registryApi.create(data)
    await fetchRegistries()
  }

  /**
   * Update an existing registry
   * @param id Registry ID
   * @param data Partial registry data to update
   */
  async function updateRegistry(id: string, data: Partial<CreateRegistryRequest>): Promise<void> {
    await registryApi.update(id, data)
    await fetchRegistries()
  }

  /**
   * Remove a registry by ID
   * @param id Registry ID
   */
  async function removeRegistry(id: string): Promise<void> {
    await registryApi.remove(id)
    await fetchRegistries()
  }

  /**
   * Test registry connection
   * @param id Registry ID
   * @returns Test result
   */
  async function testRegistry(id: string, data?: Partial<CreateRegistryRequest>): Promise<RegistryTestResult> {
    return await registryApi.test(id, data)
  }

  return {
    // State
    registries,
    loading,
    error,
    // Actions
    prepareNodeSwitch,
    fetchRegistries,
    createRegistry,
    updateRegistry,
    removeRegistry,
    testRegistry,
  }
})
