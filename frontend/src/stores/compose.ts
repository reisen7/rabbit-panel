import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { composeApi } from '@/api/compose'
import type { ComposeProject } from '@/types'

/**
 * Compose store
 * Manages Docker Compose projects state
 */
export const useComposeStore = defineStore('compose', () => {
  // State
  const projects = ref<ComposeProject[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const selectedProject = ref<string | null>(null)
  const fileContent = ref<string>('')
  const fileLoading = ref(false)
  const actionLoading = ref(false)
  const actionOutput = ref<string>('')

  // Search state
  const searchQuery = ref('')

  // Getters

  /**
   * Filter projects by search query
   */
  const filteredProjects = computed(() => {
    if (!searchQuery.value) {
      return projects.value
    }
    const query = searchQuery.value.toLowerCase()
    return projects.value.filter((p) => p.name.toLowerCase().includes(query))
  })

  /**
   * Get currently selected project details
   */
  const currentProject = computed(() => {
    if (!selectedProject.value) return null
    return projects.value.find((p) => p.name === selectedProject.value) || null
  })

  // Actions

  /**
   * Fetch all Compose projects
   */
  function prepareNodeSwitch(): void {
    projects.value = []
    selectedProject.value = null
    fileContent.value = ''
    actionOutput.value = ''
    error.value = null
  }

  async function fetchProjects(): Promise<void> {
    try {
      loading.value = true
      error.value = null
      projects.value = await composeApi.list()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch projects'
    } finally {
      loading.value = false
    }
  }

  /**
   * Create a new Compose project
   * @param name Project name
   */
  async function createProject(name: string): Promise<void> {
    await composeApi.create(name)
    await fetchProjects()
    // Select the newly created project
    selectedProject.value = name
    await loadFileContent(name)
  }

  /**
   * Delete a Compose project
   * @param project Project name
   */
  async function deleteProject(project: string): Promise<void> {
    await composeApi.delete(project)
    // Clear selection if deleted project was selected
    if (selectedProject.value === project) {
      selectedProject.value = null
      fileContent.value = ''
    }
    await fetchProjects()
  }

  /**
   * Load docker-compose.yml file content for a project
   * @param project Project name
   */
  async function loadFileContent(project: string): Promise<void> {
    try {
      fileLoading.value = true
      fileContent.value = await composeApi.getFile(project)
    } catch (e) {
      fileContent.value = ''
      throw e
    } finally {
      fileLoading.value = false
    }
  }

  /**
   * Save docker-compose.yml file content
   * @param project Project name
   * @param content File content
   */
  async function saveFileContent(project: string, content: string): Promise<void> {
    await composeApi.saveFile(project, content)
    fileContent.value = content
  }

  /**
   * Get project status with container details
   * @param project Project name
   */
  async function fetchProjectStatus(project: string): Promise<ComposeProject> {
    const status = await composeApi.status(project)
    // Update project in list
    const index = projects.value.findIndex((p) => p.name === project)
    if (index !== -1) {
      projects.value[index] = status
    }
    return status
  }

  /**
   * Execute Compose action with streaming output
   * @param project Project name
   * @param action Action to perform
   */
  function executeAction(
    project: string,
    action: 'up' | 'down' | 'restart' | 'pull'
  ): AbortController {
    actionLoading.value = true
    actionOutput.value = ''
    
    const controller = composeApi.actionStream(project, action, {
      onLog: (message) => {
        // 每条日志后加换行符
        actionOutput.value += message + '\n'
      },
      onError: (message) => {
        actionOutput.value += `❌ 错误: ${message}\n`
      },
      onDone: async () => {
        actionLoading.value = false
        // Refresh project status after action
        await fetchProjectStatus(project)
      }
    })
    
    return controller
  }

  /**
   * Select a project and load its file content
   * @param project Project name
   */
  async function selectProject(project: string | null): Promise<void> {
    selectedProject.value = project
    actionOutput.value = ''
    if (project) {
      await loadFileContent(project)
      await fetchProjectStatus(project)
    } else {
      fileContent.value = ''
    }
  }

  /**
   * Set search query
   * @param query Search string
   */
  function setSearch(query: string): void {
    searchQuery.value = query
  }

  /**
   * Clear action output
   */
  function clearOutput(): void {
    actionOutput.value = ''
  }

  return {
    // State
    projects,
    loading,
    error,
    selectedProject,
    fileContent,
    fileLoading,
    actionLoading,
    actionOutput,
    searchQuery,
    // Getters
    filteredProjects,
    currentProject,
    // Actions
    prepareNodeSwitch,
    fetchProjects,
    createProject,
    deleteProject,
    loadFileContent,
    saveFileContent,
    fetchProjectStatus,
    executeAction,
    selectProject,
    setSearch,
    clearOutput,
  }
})
