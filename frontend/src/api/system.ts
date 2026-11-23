import request from '@/utils/request'
import type { SystemStats, SystemUpdateInfo, UpdateTaskStatus } from '@/types'

/**
 * System API service
 * Provides methods for fetching system statistics
 */
export const systemApi = {
  /**
   * Get current system statistics
   * @returns System stats including CPU, memory, disk usage and server time
   */
  async stats(): Promise<SystemStats> {
    const response = await request.get<SystemStats>('/system/stats')
    return response.data
  },

  async checkUpdate(force = false): Promise<SystemUpdateInfo> {
    const response = await request.get<SystemUpdateInfo>('/system/update/check', { params: force ? { force: 1 } : undefined, suppressErrorMessage: !force })
    return response.data
  },

  async runUpdate(): Promise<{ message: string }> {
    const response = await request.post<{ message: string }>('/system/update/run')
    return response.data
  },

  async applyUpdate(): Promise<{ message: string }> {
    const response = await request.post<{ message: string }>('/system/update/apply')
    return response.data
  },

  async getUpdateStatus(): Promise<UpdateTaskStatus> {
    const response = await request.get<UpdateTaskStatus>('/system/update/status')
    return response.data
  },

  async ignoreVersion(version: string): Promise<{ message: string }> {
    const response = await request.post<{ message: string }>('/system/update/ignore', { version })
    return response.data
  },

  async clearIgnoredVersion(): Promise<{ message: string }> {
    const response = await request.post<{ message: string }>('/system/update/clear-ignore')
    return response.data
  },

  async clearUpdateState(): Promise<{ message: string }> {
    const response = await request.post<{ message: string }>('/system/update/clear-state')
    return response.data
  },
}
