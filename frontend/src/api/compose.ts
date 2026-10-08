import request from '@/utils/request'
import type { ComposeProject } from '@/types'
import { getToken } from '@/utils/request'
import { targetNodeHeaders } from '@/utils/targetNode'

/**
 * Compose action SSE callback
 */
export interface ComposeActionCallbacks {
  onLog?: (message: string) => void
  onError?: (message: string) => void
  onDone?: (success: boolean) => void
}

/**
 * Compose API service
 * Handles Docker Compose project management operations
 */
export const composeApi = {
  /**
   * Get list of all Compose projects
   * @returns Array of compose projects
   */
  async list(): Promise<ComposeProject[]> {
    const response = await request.get<ComposeProject[]>('/compose/list')
    return response.data
  },

  /**
   * Create a new Compose project
   * @param name Project name
   */
  async create(name: string): Promise<void> {
    await request.post('/compose/create', { name })
  },

  /**
   * Delete a Compose project (stops containers first)
   * @param project Project name
   */
  async delete(project: string): Promise<void> {
    await request.post('/compose/delete', { project })
  },

  /**
   * Get docker-compose.yml file content
   * @param project Project name
   * @returns File content as string
   */
  async getFile(project: string): Promise<string> {
    const response = await request.get<string>('/compose/file', {
      params: { project },
      transformResponse: [(data) => data], // Return raw string
    })
    return response.data as unknown as string
  },

  /**
   * Save docker-compose.yml file content
   * @param project Project name
   * @param content File content
   */
  async saveFile(project: string, content: string): Promise<void> {
    await request.post('/compose/save', { project, content })
  },

  /**
   * Get Compose project status with container details
   * @param project Project name
   * @returns Project status with containers
   */
  async status(project: string): Promise<ComposeProject> {
    const response = await request.get<ComposeProject>('/compose/status', {
      params: { project },
    })
    return response.data
  },

  /**
   * Execute Compose action with SSE streaming (up, down, restart, pull, logs)
   * @param project Project name
   * @param action Action to perform
   * @param callbacks SSE event callbacks
   * @returns AbortController to cancel the request
   */
  actionStream(
    project: string,
    action: 'up' | 'down' | 'restart' | 'pull' | 'logs',
    callbacks: ComposeActionCallbacks
  ): AbortController {
    const controller = new AbortController()
    const token = getToken()
    
    fetch(`/api/compose/action`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
        ...targetNodeHeaders(),
      },
      body: JSON.stringify({ project, action }),
      signal: controller.signal
    }).then(async response => {
      if (!response.ok) {
        const text = await response.text()
        callbacks.onError?.(text || `HTTP ${response.status}`)
        callbacks.onDone?.(false)
        return
      }

      const reader = response.body?.getReader()
      if (!reader) {
        callbacks.onError?.('无法读取响应流')
        callbacks.onDone?.(false)
        return
      }

      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        
        // 解析 SSE 事件
        const lines = buffer.split('\n')
        buffer = lines.pop() || '' // 保留不完整的行

        let eventType = ''
        for (const line of lines) {
          if (line.startsWith('event: ')) {
            eventType = line.slice(7)
          } else if (line.startsWith('data: ')) {
            const data = line.slice(6)
            if (eventType === 'log') {
              callbacks.onLog?.(data)
            } else if (eventType === 'error') {
              callbacks.onError?.(data)
            } else if (eventType === 'done') {
              callbacks.onDone?.(data === 'success')
            }
          }
        }
      }
    }).catch(err => {
      if (err.name !== 'AbortError') {
        callbacks.onError?.(err.message || '请求失败')
        callbacks.onDone?.(false)
      }
    })

    return controller
  },
}

export default composeApi
