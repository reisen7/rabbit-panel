import request from '@/utils/request'
import { handleUnauthorized } from '@/utils/request'
import { targetNodeHeaders, withTargetNode } from '@/utils/targetNode'
import type { ImageInfo } from '@/types'

/**
 * Image API service
 * Handles image CRUD operations and build functionality
 */
export const imageApi = {
  /**
   * Get list of all images
   * @param refresh Force refresh from Docker daemon
   * @returns Array of image info
   */
  async list(refresh?: boolean): Promise<ImageInfo[]> {
    const response = await request.get<ImageInfo[]>('/images', {
      params: refresh ? { refresh: 'true' } : undefined,
    })
    return response.data
  },

  /**
   * Remove an image
   * @param id Image ID
   */
  async remove(id: string): Promise<void> {
    await request.post('/images/remove', { id })
  },

  /**
   * Build an image from Dockerfile content using SSE stream
   * @param imageName Image name
   * @param tag Image tag
   * @param dockerfile Dockerfile content
   * @param onMessage Callback for each build message
   * @param onError Callback for errors
   * @param onComplete Callback when build completes
   */
  async build(
    imageName: string,
    tag: string,
    dockerfile: string,
    onMessage: (data: { type: string; message?: string; error?: string }) => void,
    onError: (error: string) => void,
    onComplete: () => void
  ): Promise<void> {
    try {
      const response = await fetch('/api/images/build', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...targetNodeHeaders(),
        },
        body: JSON.stringify({
          image_name: imageName,
          tag,
          dockerfile,
        }),
      })

      if (response.status === 401) handleUnauthorized()
      if (!response.ok) {
        const text = await response.text()
        onError(text || 'Build request failed')
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
   * Build an image using EventSource (alternative SSE method)
   * @param imageName Image name
   * @param tag Image tag
   * @param dockerfile Dockerfile content
   * @returns EventSource for streaming build logs
   */
  buildEventSource(imageName: string, tag: string, dockerfile: string): EventSource {
    const params = new URLSearchParams({
      image_name: imageName,
      tag,
      dockerfile,
    })
    return new EventSource(withTargetNode(`/api/images/build?${params.toString()}`))
  },
}

export default imageApi
