import request from '@/utils/request'
import { getToken } from '@/utils/request'
import { targetNodeHeaders, withTargetNode } from '@/utils/targetNode'
import type {
  ContainerInfo,
  ContainerConfig,
  ContainerStats,
  CreateContainerRequest,
  RecreateContainerRequest,
  FileInfo,
} from '@/types'

/**
 * Container API service
 * Handles container CRUD operations, logs, terminal, and file management
 * 
 * 字段命名规范：
 * - 所有容器 ID 字段统一使用 container_id
 * - 所有请求体使用 snake_case
 */
export const containerApi = {
  /**
   * Get list of all containers
   * @returns Array of container info
   */
  async list(): Promise<ContainerInfo[]> {
    // 添加时间戳避免浏览器缓存
    const response = await request.get<ContainerInfo[]>('/containers', {
      params: { _t: Date.now() }
    })
    return response.data
  },

  /**
   * Perform action on a container (start, stop, restart, remove)
   * @param containerId Container ID
   * @param action Action to perform
   */
  async action(containerId: string, action: 'start' | 'stop' | 'restart' | 'remove'): Promise<void> {
    await request.post('/containers/action', { container_id: containerId, action })
  },

  /**
   * Create and start a new container
   * @param data Container configuration
   * @returns Created container ID
   */
  async create(data: CreateContainerRequest): Promise<{ container_id: string }> {
    const response = await request.post<{ container_id: string; status: string }>('/containers/run', data)
    return { container_id: response.data.container_id }
  },

  async createStream(
    data: CreateContainerRequest,
    onMessage: (entry: { type: string; message: string }) => void,
    signal?: AbortSignal
  ): Promise<{ container_id?: string }> {
    const token = getToken()
    const response = await fetch('/api/containers/run/stream', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...targetNodeHeaders(),
      },
      body: JSON.stringify(data),
      signal,
    })

    if (!response.ok || !response.body) {
      throw new Error(await response.text() || 'Request failed')
    }

    const reader = response.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let containerId = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const events = buffer.split('\n\n')
      buffer = events.pop() || ''

      for (const event of events) {
        const line = event.split('\n').find((item) => item.startsWith('data: '))
        if (!line) continue
        try {
          const payload = JSON.parse(line.slice(6))
          const type = payload.type || 'log'
          if (type === 'success' && payload.id) {
            containerId = payload.id
          }
          onMessage({
            type,
            message: payload.message || payload.id || '',
          })
        } catch {
          // ignore malformed event
        }
      }
    }

    return containerId ? { container_id: containerId } : {}
  },

  /**
   * Create container using raw docker run command (SSE stream)
   * Note: This method is deprecated, use createRawStream instead for POST requests
   * @param containerId Container ID for logs (kept for API compatibility)
   * @returns EventSource for streaming output
   */
  createRawEventSource(containerId: string): EventSource {
    const token = getToken()
    const url = `/api/containers/logs?id=${encodeURIComponent(containerId)}&token=${encodeURIComponent(token || '')}`
    return new EventSource(url)
  },

  /**
   * Create container with streaming output using fetch
   * @param command Docker run command string
   * @param onMessage Callback for each message
   * @param onError Callback for errors
   * @param onComplete Callback when complete
   */
  async createRawStream(
    command: string,
    onMessage: (data: { type: string; message?: string; container_id?: string }) => void,
    onError: (error: string) => void,
    onComplete: () => void
  ): Promise<void> {
    const token = getToken()
    
    try {
      const response = await fetch('/api/containers/run/raw', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
          ...targetNodeHeaders(),
        },
        body: JSON.stringify({ command }),
      })

      if (!response.ok) {
        const text = await response.text()
        onError(text || 'Request failed')
        return
      }

      const reader = response.body?.getReader()
      if (!reader) {
        onError('No response body')
        return
      }

      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            try {
              const data = JSON.parse(line.slice(6))
              onMessage(data)
            } catch {
              // Ignore parse errors
            }
          }
        }
      }

      onComplete()
    } catch (error) {
      onError(error instanceof Error ? error.message : 'Unknown error')
    }
  },

  /**
   * Get container logs as SSE stream
   * @param containerId Container ID
   * @param tail Number of lines or 'all' for all logs
   * @param follow Whether to follow new logs (default: true)
   * @returns EventSource for streaming logs
   */
  logs(containerId: string, tail: number | string = 100, follow: boolean = true): EventSource {
    const token = getToken()
    return new EventSource(withTargetNode(`/api/containers/logs?id=${encodeURIComponent(containerId)}&tail=${tail}&follow=${follow}&token=${encodeURIComponent(token || '')}`))
  },

  async logsOnce(containerId: string, tail: number | string = 'all'): Promise<string[]> {
    const token = getToken()
    const response = await fetch(withTargetNode(`/api/containers/logs?id=${encodeURIComponent(containerId)}&tail=${tail}&follow=false&token=${encodeURIComponent(token || '')}`), {
      headers: token ? { Authorization: `Bearer ${token}`, ...targetNodeHeaders() } : targetNodeHeaders(),
    })
    if (!response.ok) {
      throw new Error(await response.text() || `HTTP ${response.status}`)
    }
    const text = await response.text()
    return text
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line.startsWith('data: '))
      .map((line) => line.slice(6))
      .filter((line) => line.trim() !== '')
  },

  /**
   * Get detailed container configuration
   * @param containerId Container ID
   * @returns Container configuration details
   */
  async inspect(containerId: string): Promise<ContainerConfig> {
    const response = await request.get<ContainerConfig>('/containers/inspect', {
      params: { id: containerId },
    })
    return response.data
  },

  /**
   * Update container configuration (memory, CPU, restart policy)
   * @param containerId Container ID
   * @param config Partial configuration to update
   */
  async update(containerId: string, config: { memory?: number; cpus?: number; restart?: string }): Promise<void> {
    await request.post('/containers/update', { container_id: containerId, ...config })
  },

  /**
   * Rename a container
   * @param containerId Container ID
   * @param newName New container name
   */
  async rename(containerId: string, newName: string): Promise<void> {
    await request.post('/containers/rename', { container_id: containerId, new_name: newName })
  },

  /**
   * Recreate container with new configuration
   * @param data Recreate configuration
   * @returns New container ID
   */
  async recreate(data: RecreateContainerRequest): Promise<{ container_id: string }> {
    const response = await request.post<{ container_id: string }>('/containers/recreate', data)
    return response.data
  },

  /**
   * Get container resource statistics
   * @param containerId Container ID
   * @returns Container stats (CPU, memory, network, etc.)
   */
  async stats(containerId: string): Promise<ContainerStats> {
    const response = await request.get<ContainerStats>('/containers/stats', {
      params: { id: containerId },
    })
    return response.data
  },

  /**
   * Execute command in container
   * @param containerId Container ID
   * @param command Command array to execute
   * @returns Execution result
   */
  async exec(containerId: string, command: string[]): Promise<{ output: string; exit_code: number }> {
    const response = await request.post<{ output: string; exit_code: number }>('/containers/exec', {
      container_id: containerId,
      command,
    })
    return response.data
  },

  // File management methods

  /**
   * List files in container directory
   * @param containerId Container ID
   * @param path Directory path
   * @returns Array of file info
   */
  async filesList(containerId: string, path: string): Promise<FileInfo[]> {
    const response = await request.get<FileInfo[]>('/containers/files', {
      params: { id: containerId, path },
    })
    return response.data
  },

  /**
   * Read file content from container
   * @param containerId Container ID
   * @param path File path
   * @returns File content
   */
  async fileRead(containerId: string, path: string): Promise<{ content: string }> {
    const response = await request.get<{ content: string }>('/containers/files/read', {
      params: { id: containerId, path },
    })
    return response.data
  },

  /**
   * Write content to file in container
   * @param containerId Container ID
   * @param path File path
   * @param content File content
   */
  async fileWrite(containerId: string, path: string, content: string): Promise<void> {
    await request.post('/containers/files/write', { container_id: containerId, path, content })
  },

  /**
   * Create directory in container
   * @param containerId Container ID
   * @param path Directory path
   */
  async fileMkdir(containerId: string, path: string): Promise<void> {
    await request.post('/containers/files/mkdir', { container_id: containerId, path })
  },

  /**
   * Delete file or directory in container
   * @param containerId Container ID
   * @param path Path to delete
   */
  async fileDelete(containerId: string, path: string): Promise<void> {
    await request.post('/containers/files/delete', { container_id: containerId, path })
  },

  /**
   * Upload file to container
   * @param containerId Container ID
   * @param path Target directory path
   * @param filename File name
   * @param content Base64 encoded file content
   */
  async fileUpload(containerId: string, path: string, filename: string, content: string): Promise<void> {
    await request.post('/containers/files/upload', { container_id: containerId, path, filename, content })
  },

  /**
   * Get file download URL
   * @param containerId Container ID
   * @param path File path
   * @returns Download URL
   */
  fileDownload(containerId: string, path: string): string {
    const token = getToken()
    return withTargetNode(`/api/containers/files/download?id=${encodeURIComponent(containerId)}&path=${encodeURIComponent(path)}&token=${encodeURIComponent(token || '')}`)
  },

  /**
   * Get WebSocket URL for terminal connection
   * @param containerId Container ID
   * @returns WebSocket URL
   */
  getTerminalWsUrl(containerId: string): string {
    const token = getToken()
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return withTargetNode(`${protocol}//${window.location.host}/api/containers/terminal?id=${encodeURIComponent(containerId)}&token=${encodeURIComponent(token || '')}`)
  },
}

export default containerApi
