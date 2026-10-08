import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { networkApi } from '@/api/networks'
import type { NetworkInfo, NetworkDetail } from '@/types'
import { getSavedPageSize, savePageSize } from '@/utils/pagination'

export type SortField = 'id' | 'name' | 'driver' | 'scope' | 'containers'
export type SortOrder = 'asc' | 'desc'
const PAGE_SIZE_STORAGE_KEY = 'rabbit-page-size-networks'

// System networks that cannot be deleted
const PROTECTED_NETWORKS = ['bridge', 'host', 'none']

/**
 * Network store
 * Manages network list with search, sort, and pagination
 */
export const useNetworkStore = defineStore('networks', () => {
  // State
  const networks = ref<NetworkInfo[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  
  // Search state
  const searchQuery = ref('')
  
  // Sort state
  const sortField = ref<SortField>('name')
  const sortOrder = ref<SortOrder>('asc')
  
  // Pagination state
  const currentPage = ref(1)
  const pageSize = ref(getSavedPageSize(PAGE_SIZE_STORAGE_KEY))

  // Getters

  /**
   * Filter networks by search query (name or driver)
   */
  const filteredNetworks = computed(() => {
    let result = [...networks.value]

    // Filter by search query
    if (searchQuery.value) {
      const query = searchQuery.value.toLowerCase()
      result = result.filter(
        (n) =>
          n.name.toLowerCase().includes(query) ||
          n.driver.toLowerCase().includes(query)
      )
    }

    // Sort
    result.sort((a, b) => {
      let aVal: string | number
      let bVal: string | number

      switch (sortField.value) {
        case 'id':
          aVal = a.id
          bVal = b.id
          break
        case 'name':
          aVal = a.name.toLowerCase()
          bVal = b.name.toLowerCase()
          break
        case 'driver':
          aVal = a.driver.toLowerCase()
          bVal = b.driver.toLowerCase()
          break
        case 'scope':
          aVal = a.scope.toLowerCase()
          bVal = b.scope.toLowerCase()
          break
        case 'containers':
          aVal = a.container_count
          bVal = b.container_count
          break
        default:
          aVal = a.name.toLowerCase()
          bVal = b.name.toLowerCase()
      }

      if (aVal < bVal) return sortOrder.value === 'asc' ? -1 : 1
      if (aVal > bVal) return sortOrder.value === 'asc' ? 1 : -1
      return 0
    })

    return result
  })

  /**
   * Get paginated networks from filtered list
   */
  const paginatedNetworks = computed(() => {
    const start = (currentPage.value - 1) * pageSize.value
    const end = start + pageSize.value
    return filteredNetworks.value.slice(start, end)
  })

  /**
   * Total number of pages
   */
  const totalPages = computed(() => {
    return Math.ceil(filteredNetworks.value.length / pageSize.value) || 1
  })

  /**
   * Total number of filtered networks
   */
  const totalNetworks = computed(() => filteredNetworks.value.length)

  // Actions

  /**
   * Fetch networks from API
   */
  function prepareNodeSwitch(): void {
    networks.value = []
    currentPage.value = 1
    error.value = null
  }

  async function fetchNetworks(): Promise<void> {
    try {
      loading.value = true
      error.value = null
      networks.value = await networkApi.list()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch networks'
    } finally {
      loading.value = false
    }
  }

  /**
   * Create a new network
   * @param name Network name
   * @param driver Network driver
   */
  async function createNetwork(name: string, driver: string): Promise<void> {
    await networkApi.create(name, driver)
    // Refresh list after creation
    await fetchNetworks()
  }

  /**
   * Remove a network
   * @param id Network ID
   */
  async function removeNetwork(id: string): Promise<void> {
    // Find network by ID to check if it's protected
    const network = networks.value.find(n => n.id === id)
    if (network && isProtectedNetwork(network.name)) {
      throw new Error('Cannot delete system network')
    }
    
    await networkApi.remove(id)
    // Refresh list after removal
    await fetchNetworks()
  }

  /**
   * Get network details
   * @param id Network ID
   * @returns Network details
   */
  async function inspectNetwork(id: string): Promise<NetworkDetail> {
    return await networkApi.inspect(id)
  }

  /**
   * Check if a network is a protected system network
   * @param name Network name
   * @returns true if protected
   */
  function isProtectedNetwork(name: string): boolean {
    return PROTECTED_NETWORKS.includes(name.toLowerCase())
  }

  /**
   * Set search query
   * @param query Search string
   */
  function setSearch(query: string): void {
    searchQuery.value = query
    // Reset to first page when search changes
    currentPage.value = 1
  }

  /**
   * Set sort field and toggle order if same field
   * @param field Field to sort by
   */
  function setSort(field: SortField): void {
    if (sortField.value === field) {
      // Toggle order if same field
      sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
    } else {
      sortField.value = field
      sortOrder.value = 'asc'
    }
  }

  /**
   * Set current page
   * @param page Page number
   */
  function setPage(page: number): void {
    if (page >= 1 && page <= totalPages.value) {
      currentPage.value = page
    }
  }

  /**
   * Set page size
   * @param size Number of items per page
   */
  function setPageSize(size: number): void {
    pageSize.value = size
    savePageSize(PAGE_SIZE_STORAGE_KEY, size)
    // Reset to first page when page size changes
    currentPage.value = 1
  }

  /**
   * Reset all filters and pagination
   */
  function resetFilters(): void {
    searchQuery.value = ''
    sortField.value = 'name'
    sortOrder.value = 'asc'
    currentPage.value = 1
  }

  return {
    // State
    networks,
    loading,
    error,
    searchQuery,
    sortField,
    sortOrder,
    currentPage,
    pageSize,
    // Getters
    filteredNetworks,
    paginatedNetworks,
    totalPages,
    totalNetworks,
    // Actions
    prepareNodeSwitch,
    fetchNetworks,
    createNetwork,
    removeNetwork,
    inspectNetwork,
    isProtectedNetwork,
    setSearch,
    setSort,
    setPage,
    setPageSize,
    resetFilters,
  }
})
