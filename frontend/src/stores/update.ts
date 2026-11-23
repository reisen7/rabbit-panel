import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { systemApi } from '@/api/system'
import type { SystemUpdateInfo, UpdateTaskStatus } from '@/types'

export const useUpdateStore = defineStore('update', () => {
  const info = ref<SystemUpdateInfo | null>(null)
  const loading = ref(false)
  const running = ref(false)
  const error = ref<string | null>(null)
  const taskStatus = ref<UpdateTaskStatus | null>(null)
  const minimized = ref(false)
  const bannerDismissedVersion = ref('')
  let pollingTimer: ReturnType<typeof setInterval> | null = null
  let autoHideTimer: ReturnType<typeof setTimeout> | null = null
  let updateInfoInflight: Promise<void> | null = null

  const shouldShowIndicator = computed(() => {
    if (taskStatus.value?.status === 'running' || taskStatus.value?.status === 'downloaded' || taskStatus.value?.status === 'failed' || taskStatus.value?.status === 'applying') {
      return true
    }
    if (!info.value) return false
    if (!info.value.has_update || info.value.ignored) return false
    return true
  })

  const shouldShowBanner = computed(() => {
    if (!shouldShowIndicator.value || !info.value) return false
    return bannerDismissedVersion.value !== info.value.latest_version
  })

  async function loadUpdateInfo(options?: { includeTaskStatus?: boolean; force?: boolean }): Promise<void> {
    try {
      const previousVersion = info.value?.latest_version || ''
      info.value = await systemApi.checkUpdate(options?.force ?? false)
      if (previousVersion !== info.value.latest_version) {
        bannerDismissedVersion.value = ''
      }
      if (options?.includeTaskStatus) {
        await fetchTaskStatus({ refreshInfoOnFinish: false })
        if (taskStatus.value?.status === 'running' || taskStatus.value?.status === 'applying') {
          startTaskPolling()
        }
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to check updates'
    }
  }

  async function fetchUpdateInfo(options?: { force?: boolean }): Promise<void> {
    if (updateInfoInflight) {
      return updateInfoInflight
    }
    loading.value = true
    error.value = null
    updateInfoInflight = loadUpdateInfo({ includeTaskStatus: true, force: options?.force })
      .finally(() => {
        loading.value = false
        updateInfoInflight = null
      })
    return updateInfoInflight
  }

  async function runUpdate(): Promise<string> {
    running.value = true
    try {
      const result = await systemApi.runUpdate()
      if (info.value) {
        info.value.last_update_status = 'running'
      }
      minimized.value = false
      await fetchTaskStatus()
      startTaskPolling()
      return result.message
    } finally {
      running.value = false
    }
  }

  async function applyUpdate(): Promise<string> {
    running.value = true
    try {
      const result = await systemApi.applyUpdate()
      if (info.value) {
        info.value.last_update_status = 'applying'
      }
      minimized.value = false
      await fetchTaskStatus()
      startTaskPolling()
      return result.message
    } finally {
      running.value = false
    }
  }

  async function fetchTaskStatus(options?: { refreshInfoOnFinish?: boolean }): Promise<void> {
    taskStatus.value = await systemApi.getUpdateStatus()
    if (taskStatus.value?.status === 'success' || taskStatus.value?.status === 'failed' || taskStatus.value?.status === 'downloaded') {
      stopTaskPolling()
      if (options?.refreshInfoOnFinish !== false) {
        await loadUpdateInfo()
      }
      if (taskStatus.value.status === 'success') {
        if (autoHideTimer) clearTimeout(autoHideTimer)
        autoHideTimer = setTimeout(() => {
          minimized.value = false
          taskStatus.value = null
        }, 4000)
      }
    }
  }

  function startTaskPolling(): void {
    stopTaskPolling()
    pollingTimer = setInterval(() => {
      fetchTaskStatus().catch(() => undefined)
    }, 2000)
  }

  function stopTaskPolling(): void {
    if (pollingTimer) {
      clearInterval(pollingTimer)
      pollingTimer = null
    }
  }

  function resetTaskState(): void {
    stopTaskPolling()
    if (autoHideTimer) {
      clearTimeout(autoHideTimer)
      autoHideTimer = null
    }
    taskStatus.value = null
    minimized.value = false
  }

  function resetAllState(): void {
    resetTaskState()
    info.value = null
    error.value = null
    loading.value = false
    running.value = false
  }

  function setMinimized(value: boolean): void {
    minimized.value = value
  }

  async function ignoreVersion(): Promise<void> {
    if (!info.value?.latest_version) return
    await systemApi.ignoreVersion(info.value.latest_version)
    bannerDismissedVersion.value = ''
    info.value.ignored = true
    info.value.has_update = false
    info.value.ignored_version = info.value.latest_version
  }

  async function clearIgnoredVersion(): Promise<void> {
    await systemApi.clearIgnoredVersion()
    if (info.value) {
      info.value.ignored = false
      info.value.ignored_version = ''
      if (info.value.latest_version && info.value.current_version !== info.value.latest_version) {
        info.value.has_update = true
      }
    }
    bannerDismissedVersion.value = ''
  }

  async function clearUpdateState(): Promise<void> {
    await systemApi.clearUpdateState()
    resetTaskState()
    if (info.value) {
      info.value.last_update_time = ''
      info.value.last_update_status = ''
      info.value.last_update_error = ''
    }
  }

  function remindLater(): void {
    if (!info.value?.latest_version) return
    bannerDismissedVersion.value = info.value.latest_version
  }

  return {
    info,
    loading,
    running,
    error,
    taskStatus,
    minimized,
    shouldShowIndicator,
    shouldShowBanner,
    fetchUpdateInfo,
    runUpdate,
    applyUpdate,
    fetchTaskStatus,
    startTaskPolling,
    stopTaskPolling,
    resetTaskState,
    resetAllState,
    setMinimized,
    ignoreVersion,
    clearIgnoredVersion,
    clearUpdateState,
    remindLater,
  }
})
