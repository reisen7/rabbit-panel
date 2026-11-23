<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  ArrowUpRight,
  Crown,
  Maximize2,
  Server,
  TerminalSquare,
} from 'lucide-vue-next'
import { useNodesStore } from '@/stores/nodes'
import { useI18n } from '@/composables/useI18n'
import type { NodeInfo } from '@/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import NodeTopologyCanvas from '@/components/dashboard/NodeTopologyCanvas.vue'
import SshDialog from '@/components/nodes/SshDialog.vue'

const router = useRouter()
const { t } = useI18n()
const nodesStore = useNodesStore()

const expandVisible = ref(false)
const sshDialogVisible = ref(false)
const sshTargetNode = ref<NodeInfo | null>(null)

const totalContainers = computed(() =>
  nodesStore.nodes.reduce((sum, n) => sum + (n.containers || 0), 0)
)

function isNodeSelected(node: NodeInfo): boolean {
  return nodesStore.currentNodeId === node.id
}

function handleSelectNode(node: NodeInfo) {
  if (node.status !== 'online') return
  nodesStore.setCurrentNode(node.id)
}

function openSsh(node: NodeInfo, event: Event) {
  event.stopPropagation()
  if (node.status !== 'online') return
  sshTargetNode.value = node
  sshDialogVisible.value = true
}
</script>

<template>
  <Card v-if="nodesStore.isMasterMode && nodesStore.nodes.length > 0" class="overflow-hidden">
    <CardHeader class="flex flex-row flex-wrap items-center justify-between gap-3 pb-3">
      <div class="flex items-center gap-3">
        <CardTitle class="text-base">{{ t('dashboard.topology') }}</CardTitle>
        <div class="flex items-center gap-1.5">
          <Badge variant="success" class="tabular-nums">
            {{ nodesStore.onlineCount }} / {{ nodesStore.totalNodes }} {{ t('node.online') }}
          </Badge>
          <Badge variant="secondary" class="tabular-nums">
            {{ totalContainers }} {{ t('nav.containers') }}
          </Badge>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <span class="hidden sm:inline text-xs text-muted-foreground">
          {{ t('dashboard.topologyHint') }}
        </span>
        <Button
          variant="ghost"
          size="icon-sm"
          class="size-7 text-muted-foreground hover:text-foreground"
          :title="t('dashboard.topologyExpand')"
          @click="expandVisible = true"
        >
          <Maximize2 class="size-3.5" />
        </Button>
        <Button variant="ghost" size="sm" class="h-7 px-2.5 text-xs" @click="router.push('/nodes')">
          {{ t('nav.nodes') }}
          <ArrowUpRight class="size-3.5" />
        </Button>
      </div>
    </CardHeader>

    <CardContent class="pt-0">
      <!-- 桌面端：紧凑拓扑画布（点击查看大图） -->
      <div class="hidden md:block h-[240px]">
        <NodeTopologyCanvas compact />
      </div>

      <!-- 移动端：紧凑节点列表卡片 -->
      <div class="md:hidden grid grid-cols-1 sm:grid-cols-2 gap-2.5">
        <div
          v-for="node in nodesStore.nodes"
          :key="node.id"
          class="rounded-lg border bg-card p-3 flex items-center justify-between gap-2"
          :class="[
            node.status === 'online' ? 'cursor-pointer' : 'opacity-65',
            isNodeSelected(node) ? 'border-primary ring-1 ring-primary/30' : '',
          ]"
          @click="handleSelectNode(node)"
        >
          <div class="flex items-center gap-2.5 min-w-0">
            <div
              class="size-8 rounded-lg flex items-center justify-center shrink-0"
              :class="node.mode === 'master' ? 'bg-blue-500/15 text-blue-500' : 'bg-emerald-500/15 text-emerald-500'"
            >
              <Crown v-if="node.mode === 'master'" class="size-4" />
              <Server v-else class="size-4" />
            </div>
            <div class="min-w-0">
              <div class="text-sm font-medium truncate">{{ node.name }}</div>
              <div class="text-xs font-mono text-muted-foreground truncate">{{ node.address }}</div>
            </div>
          </div>
          <div class="flex items-center gap-2 shrink-0">
            <Badge :variant="node.status === 'online' ? 'success' : 'destructive'" class="text-[11px]">
              {{ node.status === 'online' ? t('node.online') : t('node.offline') }}
            </Badge>
            <Button
              v-if="node.status === 'online'"
              variant="ghost"
              size="icon-sm"
              class="size-7"
              :title="t('ssh.title')"
              @click="openSsh(node, $event)"
            >
              <TerminalSquare class="size-3.5" />
            </Button>
          </div>
        </div>
      </div>
    </CardContent>

    <SshDialog v-model:visible="sshDialogVisible" :node="sshTargetNode" />

    <!-- 放大视图 -->
    <Dialog v-model:open="expandVisible">
      <DialogContent class="sm:max-w-5xl w-[94vw] gap-3">
        <DialogHeader>
          <DialogTitle class="flex items-center gap-2.5 text-base">
            {{ t('dashboard.topology') }}
            <Badge variant="success" class="tabular-nums font-normal">
              {{ nodesStore.onlineCount }} / {{ nodesStore.totalNodes }} {{ t('node.online') }}
            </Badge>
            <Badge variant="secondary" class="tabular-nums font-normal">
              {{ totalContainers }} {{ t('nav.containers') }}
            </Badge>
          </DialogTitle>
        </DialogHeader>
        <div class="h-[62vh]">
          <NodeTopologyCanvas />
        </div>
        <p class="text-xs text-muted-foreground">{{ t('dashboard.topologyHint') }}</p>
      </DialogContent>
    </Dialog>
  </Card>
</template>
