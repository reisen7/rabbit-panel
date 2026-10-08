import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { volumeApi } from '@/api/volumes'
import type { VolumeInfo, CreateVolumeRequest, VolumePruneResult } from '@/types'
import { getSavedPageSize, savePageSize } from '@/utils/pagination'

export type SortField = 'name' | 'driver' | 'created'
export type SortOrder = 'asc' | 'desc'
const PAGE_SIZE_STORAGE_KEY = 'rabbit-page-size-volumes'

/**
 * Volume store
 * Manages volume list with search, sort, and pagination
 */
export const useVolumeStore = defineStore('volumes', () => {
  // State
  const volumes = ref<VolumeInfo[]>([])
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
   * Filter and sort volumes
   */
  const filteredVolumes = computed(() => {
    let result = [...volumes.value]

    // Filter by search query (name or driver)
    if (searchQuery.value) {
      const query = searchQuery.value.toLowerCase()
      result = result.filter(
        (v) =>
          v.name.toLowerCase().includes(query) ||
          v.driver.toLowerCase().includes(query)
      )
    }

    // Sort
    result.sort((a, b) => {
      let aVal: string
      let bVal: string

      switch (sortField.value) {
        case 'name':
          aVal = a.name.toLowerCase()
          bVal = b.name.toLowerCase()
          break
        case 'driver':
          aVal = a.driver.toLowerCase()
          bVal = b.driver.toLowerCase()
          break
        case 'created':
          aVal = a.created || ''
          bVal = b.created || ''
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
   * Get paginated volumes from filtered list
   */
  const paginatedVolumes = computed(() => {
    const start = (currentPage.value - 1) * pageSize.value
    const end = start + pageSize.value
    return filteredVolumes.value.slice(start, end)
  })

  /**
   * Total number of pages
   */
  const totalPages = computed(() => {
    return Math.ceil(filteredVolumes.value.length / pageSize.value) || 1
  })

  /**
   * Total number of filtered volumes
   */
  const totalVolumes = computed(() => filteredVolumes.value.length)

  /**
   * Count of unused volumes (not attached to any container)
   */
  const unusedCount = computed(() =>
    volumes.value.filter((v) => v.containers.length === 0).length
  )

  // Actions

  /**
   * Fetch volumes from API
   */
  function prepareNodeSwitch(): void {
    volumes.value = []
    currentPage.value = 1
    error.value = null
  }

  async function fetchVolumes(): Promise<void> {
    try {
      loading.value = true
      error.value = null
      volumes.value = await volumeApi.list()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch volumes'
    } finally {
      loading.value = false
    }
  }

  /**
   * Create a new volume
   * @param data Volume creation request
   */
  async function createVolume(data: CreateVolumeRequest): Promise<void> {
    await volumeApi.create(data)
    await fetchVolumes()
  }

  /**
   * Remove a volume by name
   * @param name Volume name
   */
  async function removeVolume(name: string): Promise<void> {
    await volumeApi.remove(name)
    await fetchVolumes()
  }

  /**
   * Prune unused volumes
   * @returns Prune result
   */
  async function pruneVolumes(): Promise<VolumePruneResult> {
    const result = await volumeApi.prune()
    await fetchVolumes()
    return result
  }

  /**
   * Set search query
   * @param query Search string
   */
  function setSearch(query: string): void {
    searchQuery.value = query
    currentPage.value = 1
  }

  /**
   * Set sort field and toggle order if same field
   * @param field Field to sort by
   */
  function setSort(field: SortField): void {
    if (sortField.value === field) {
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
    currentPage.value = 1
  }

  return {
    // State
    volumes,
    loading,
    error,
    searchQuery,
    sortField,
    sortOrder,
    currentPage,
    pageSize,
    // Getters
    filteredVolumes,
    paginatedVolumes,
    totalPages,
    totalVolumes,
    unusedCount,
    // Actions
    prepareNodeSwitch,
    fetchVolumes,
    createVolume,
    removeVolume,
    pruneVolumes,
    setSearch,
    setSort,
    setPage,
    setPageSize,
  }
})
