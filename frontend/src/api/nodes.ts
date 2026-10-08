import request from '@/utils/request'
import type { NodeInfo } from '@/types'

/**
 * Schedule request for creating containers on specific nodes
 */
export interface ScheduleRequest {
  image: string
  name: string
  ports?: Record<string, string>
  env?: Record<string, string>
  labels?: Record<string, string>
  node_id?: string // Optional: specify target node, auto-select if empty
  constraints?: Record<string, string>
}

/**
 * Schedule response after container creation
 */
export interface ScheduleResponse {
  status: string
  node_id: string
  node_name: string
  container: {
    status: string
    id: string
    name: string
  }
}

/**
 * Nodes API service
 * Provides methods for managing multi-node Docker infrastructure
 * Only available when running in Master mode
 */
export const nodesApi = {
  /**
   * Get list of all registered nodes
   * @returns Array of node information
   */
  async list(): Promise<NodeInfo[]> {
    const response = await request.get<NodeInfo[]>('/nodes', { suppressErrorMessage: true })
    return response.data
  },

  /**
   * Schedule a container to run on a specific node or auto-select best node
   * @param data Schedule request with container configuration
   * @returns Schedule response with node and container info
   */
  async schedule(data: ScheduleRequest): Promise<ScheduleResponse> {
    const response = await request.post<ScheduleResponse>('/containers/schedule', data)
    return response.data
  },

  /**
   * Get the best node for scheduling based on current load
   * This is a convenience method that calls list and finds the node with lowest load
   * @returns The best available node or null if no nodes are online
   */
  async getBestNode(): Promise<NodeInfo | null> {
    const nodes = await this.list()
    const onlineNodes = nodes.filter(n => n.status === 'online')
    
    if (onlineNodes.length === 0) {
      return null
    }

    // Find node with lowest load (CPU + Memory average)
    const firstNode = onlineNodes[0]!
    let bestNode: NodeInfo = firstNode
    let minLoad = (bestNode.cpu + bestNode.memory) / 2

    for (const node of onlineNodes) {
      const load = (node.cpu + node.memory) / 2
      if (load < minLoad) {
        minLoad = load
        bestNode = node
      }
    }

    return bestNode
  },
}
