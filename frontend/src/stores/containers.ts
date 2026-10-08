import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { containerApi } from '@/api/containers'
import type { ContainerInfo } from '@/types'
import { getSavedPageSize, savePageSize } from '@/utils/pagination'

export type ContainerState = 'all' | 'running' | 'exited' | 'paused' | 'created'
export type SortField = 'id' | 'name' | 'image' | 'state' | 'created'
export type SortOrder = 'asc' | 'desc'
const PAGE_SIZE_STORAGE_KEY = 'rabbit-page-size-containers'

/**
 * Container store
 * Manages container list with search, filter, sort, and pagination
 */
export const useContainerStore = defineStore('containers', () => {
  // State
  const containers = ref<ContainerInfo[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const actionInProgress = ref(false) // 防止并发操作
  
  // Search and filter state
  const searchQuery = ref('')
  const statusFilter = ref<ContainerState>('all')
  
  // Sort state
  const sortField = ref<SortField>('name')
  const sortOrder = ref<SortOrder>('asc')
  
  // Pagination state
  const currentPage = ref(1)
  const pageSize = ref(getSavedPageSize(PAGE_SIZE_STORAGE_KEY))

  // Getters

  /**
   * Filter containers by search query and status
   */
  const filteredContainers = computed(() => {
    let result = [...containers.value]

    // Filter by search query (name or image)
    if (searchQuery.value) {
      const query = searchQuery.value.toLowerCase()
      result = result.filter(
        (c) =>
          c.name.toLowerCase().includes(query) ||
          c.image.toLowerCase().includes(query)
      )
    }

    // Filter by status
    if (statusFilter.value !== 'all') {
      result = result.filter((c) => c.state === statusFilter.value)
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
        case 'image':
          aVal = a.image.toLowerCase()
          bVal = b.image.toLowerCase()
          break
        case 'state':
          aVal = a.state
          bVal = b.state
          break
        case 'created':
          aVal = a.created
          bVal = b.created
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
   * Get paginated containers from filtered list
   */
  const paginatedContainers = computed(() => {
    const start = (currentPage.value - 1) * pageSize.value
    const end = start + pageSize.value
    return filteredContainers.value.slice(start, end)
  })

  /**
   * Total number of pages
   */
  const totalPages = computed(() => {
    return Math.ceil(filteredContainers.value.length / pageSize.value) || 1
  })

  /**
   * Total number of filtered containers
   */
  const totalContainers = computed(() => filteredContainers.value.length)

  // Actions

  /**
   * Fetch containers from API
   */
  function prepareNodeSwitch(): void {
    containers.value = []
    currentPage.value = 1
    error.value = null
  }

  async function fetchContainers(): Promise<void> {
    try {
      loading.value = true
      error.value = null
      containers.value = await containerApi.list()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch containers'
    } finally {
      loading.value = false
    }
  }

  /**
   * Perform action on a container
   * @param id Container ID
   * @param action Action to perform
   */
  async function containerAction(
    id: string,
    action: 'start' | 'stop' | 'restart' | 'remove'
  ): Promise<void> {
    // 防止并发操作
    if (actionInProgress.value) {
      return
    }
    
    try {
      actionInProgress.value = true
      await containerApi.action(id, action)
      // Refresh list after action
      await fetchContainers()
    } finally {
      actionInProgress.value = false
    }
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
   * Set status filter
   * @param status Status to filter by
   */
  function setStatusFilter(status: ContainerState): void {
    statusFilter.value = status
    // Reset to first page when filter changes
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
    statusFilter.value = 'all'
    sortField.value = 'name'
    sortOrder.value = 'asc'
    currentPage.value = 1
  }

  return {
    // State
    containers,
    loading,
    error,
    actionInProgress,
    searchQuery,
    statusFilter,
    sortField,
    sortOrder,
    currentPage,
    pageSize,
    // Getters
    filteredContainers,
    paginatedContainers,
    totalPages,
    totalContainers,
    // Actions
    prepareNodeSwitch,
    fetchContainers,
    containerAction,
    setSearch,
    setStatusFilter,
    setSort,
    setPage,
    setPageSize,
    resetFilters,
  }
})
