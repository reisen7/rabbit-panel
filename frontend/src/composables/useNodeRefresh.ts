import { watch } from 'vue'
import { useNodesStore } from '@/stores/nodes'

/**
 * 切换目标节点后自动重新拉取节点级数据。
 * 节点相关页面（容器/镜像/网络/存储卷/Compose/镜像仓库）挂载时调用一次。
 */
export function useNodeRefresh(refetch: () => void | Promise<void>) {
  const nodesStore = useNodesStore()
  watch(
    () => nodesStore.currentNodeId,
    () => {
      void refetch()
    }
  )
}
