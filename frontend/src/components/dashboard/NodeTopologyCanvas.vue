<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  Container,
  Cpu,
  Crown,
  MemoryStick,
  Server,
  TerminalSquare,
  Wifi,
  WifiOff,
} from 'lucide-vue-next'
import { useNodesStore } from '@/stores/nodes'
import { useI18n } from '@/composables/useI18n'
import type { NodeInfo } from '@/types'
import { Button } from '@/components/ui/button'
import SshDialog from '@/components/nodes/SshDialog.vue'

const props = withDefaults(defineProps<{ compact?: boolean }>(), { compact: false })

const { t } = useI18n()
const nodesStore = useNodesStore()

const hoveredNodeId = ref<string | null>(null)
const sshDialogVisible = ref(false)
const sshTargetNode = ref<NodeInfo | null>(null)

const masterNode = computed(
  () => nodesStore.nodes.find((n) => n.mode === 'master') ?? null
)

const workerNodes = computed(() =>
  nodesStore.nodes.filter((n) => n.mode !== 'master')
)

// 画布逻辑坐标系 1000 x 380（SVG viewBox 与百分比定位对齐）
const VB_W = 1000
const VB_H = 380
const CENTER_X = 500
const CENTER_Y = 190

interface PositionedWorker {
  node: NodeInfo
  x: number
  y: number
  xPct: number
  yPct: number
  pathD: string
  isHovered: boolean
  isSelected: boolean
}

const positionedWorkers = computed<PositionedWorker[]>(() => {
  const list = workerNodes.value
  const count = list.length
  if (count === 0) return []

  // 根据数量分布左右或椭圆环绕，避免卡片重叠；compact 模式收缩纵向半径
  const ryScale = props.compact ? 0.72 : 1
  const rx = count <= 2 ? 320 : count <= 4 ? 340 : 370
  const ry = (count <= 2 ? 95 : count <= 4 ? 120 : 132) * ryScale

  return list.map((node, idx) => {
    let angle: number
    if (count === 1) {
      angle = 0 // 正右
    } else if (count === 2) {
      angle = idx === 0 ? Math.PI : 0 // 左、右对称
    } else if (count === 3) {
      const angles = [-Math.PI * 0.78, -Math.PI * 0.22, Math.PI * 0.5]
      angle = angles[idx]!
    } else if (count === 4) {
      const angles = [-Math.PI * 0.78, -Math.PI * 0.22, Math.PI * 0.78, Math.PI * 0.22]
      angle = angles[idx]!
    } else {
      angle = -Math.PI / 2 + (2 * Math.PI * idx) / count
    }

    const x = CENTER_X + rx * Math.cos(angle)
    const y = CENTER_Y + ry * Math.sin(angle)

    // 柔和的 S 形贝塞尔连线：从 Master 边缘连到 Worker
    const midX = (CENTER_X + x) / 2
    const pathD = `M ${CENTER_X} ${CENTER_Y} C ${midX} ${CENTER_Y}, ${midX} ${y}, ${x} ${y}`

    return {
      node,
      x,
      y,
      xPct: (x / VB_W) * 100,
      yPct: (y / VB_H) * 100,
      pathD,
      isHovered: hoveredNodeId.value === node.id,
      isSelected: nodesStore.currentNodeId === node.id,
    }
  })
})

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

function metricColor(val: number): string {
  if (val >= 90) return 'bg-destructive'
  if (val >= 70) return 'bg-amber-500'
  return 'bg-emerald-500'
}
</script>

<template>
  <div class="relative w-full h-full rounded-xl border bg-muted/20 overflow-hidden topology-grid select-none">
    <!-- 背景柔和中心光晕 -->
    <div
      class="pointer-events-none absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 w-[420px] h-[260px] rounded-full bg-primary/10 blur-3xl"
    />

    <!-- SVG 连线层 -->
    <svg
      class="pointer-events-none absolute inset-0 size-full"
      :viewBox="`0 0 ${VB_W} ${VB_H}`"
      preserveAspectRatio="none"
    >
      <defs>
        <linearGradient id="topo-link-online" x1="0%" y1="0%" x2="100%" y2="0%">
          <stop offset="0%" stop-color="#3b82f6" stop-opacity="0.7" />
          <stop offset="100%" stop-color="#10b981" stop-opacity="0.7" />
        </linearGradient>
        <linearGradient id="topo-link-active" x1="0%" y1="0%" x2="100%" y2="0%">
          <stop offset="0%" stop-color="#60a5fa" stop-opacity="0.95" />
          <stop offset="100%" stop-color="#34d399" stop-opacity="0.95" />
        </linearGradient>
      </defs>

      <g v-for="w in positionedWorkers" :key="w.node.id">
        <!-- 底线 -->
        <path
          :d="w.pathD"
          fill="none"
          :stroke="
            w.node.status === 'online'
              ? w.isHovered || w.isSelected
                ? 'url(#topo-link-active)'
                : 'url(#topo-link-online)'
              : 'currentColor'
          "
          :class="w.node.status !== 'online' ? 'text-muted-foreground/30' : ''"
          :stroke-width="w.isHovered || w.isSelected ? 2.5 : 1.75"
          :stroke-dasharray="w.node.status === 'online' ? 'none' : '6 6'"
          class="transition-all duration-300"
        />
        <!-- 在线节点的流动脉冲 -->
        <path
          v-if="w.node.status === 'online'"
          :d="w.pathD"
          fill="none"
          stroke="#34d399"
          stroke-width="2.5"
          stroke-linecap="round"
          stroke-dasharray="6 22"
          class="topo-flow"
        />
        <!-- 端点头尾小圆点 -->
        <circle
          :cx="w.x"
          :cy="w.y"
          r="4"
          :fill="w.node.status === 'online' ? '#10b981' : '#71717a'"
        />
      </g>
    </svg>

    <!-- 中心 Master 节点 -->
    <div
      v-if="masterNode"
      class="absolute -translate-x-1/2 -translate-y-1/2 z-10 w-[228px]"
      style="left: 50%; top: 50%"
      @mouseenter="hoveredNodeId = masterNode.id"
      @mouseleave="hoveredNodeId = null"
      @click="handleSelectNode(masterNode)"
    >
      <div
        class="group relative rounded-xl border bg-card/95 backdrop-blur-sm p-3.5 shadow-md transition-all duration-200 cursor-pointer hover:shadow-lg"
        :class="
          isNodeSelected(masterNode)
            ? 'border-blue-500 ring-2 ring-blue-500/25'
            : 'border-border hover:border-blue-500/60'
        "
      >
        <!-- 在线脉冲环 -->
        <span
          class="pointer-events-none absolute -inset-px rounded-xl border border-blue-500/20 topo-master-glow"
        />

        <div class="flex items-start justify-between gap-2">
          <div class="flex items-center gap-2 min-w-0">
            <div class="size-8 rounded-lg bg-blue-500/15 text-blue-500 flex items-center justify-center shrink-0">
              <Crown class="size-4" />
            </div>
            <div class="min-w-0">
              <div class="flex items-center gap-1.5">
                <span class="font-semibold text-sm truncate">{{ masterNode.name }}</span>
                <span class="relative flex size-2 shrink-0">
                  <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
                  <span class="relative inline-flex rounded-full size-2 bg-emerald-500" />
                </span>
              </div>
              <div class="text-[11px] font-mono text-muted-foreground truncate">
                {{ masterNode.address }}
              </div>
            </div>
          </div>
          <Button
            variant="ghost"
            size="icon-sm"
            class="size-7 shrink-0 text-muted-foreground hover:text-foreground"
            :title="t('ssh.title')"
            @click="openSsh(masterNode, $event)"
          >
            <TerminalSquare class="size-3.5" />
          </Button>
        </div>

        <div class="mt-3 grid grid-cols-3 gap-2 text-[11px]">
          <div class="space-y-1">
            <div class="flex items-center justify-between text-muted-foreground">
              <Cpu class="size-3" />
              <span class="tabular-nums font-medium text-foreground">{{ masterNode.cpu.toFixed(0) }}%</span>
            </div>
            <div class="h-1 rounded-full bg-muted overflow-hidden">
              <div class="h-full rounded-full transition-all" :class="metricColor(masterNode.cpu)" :style="{ width: `${Math.min(100, masterNode.cpu)}%` }" />
            </div>
          </div>
          <div class="space-y-1">
            <div class="flex items-center justify-between text-muted-foreground">
              <MemoryStick class="size-3" />
              <span class="tabular-nums font-medium text-foreground">{{ masterNode.memory.toFixed(0) }}%</span>
            </div>
            <div class="h-1 rounded-full bg-muted overflow-hidden">
              <div class="h-full rounded-full transition-all" :class="metricColor(masterNode.memory)" :style="{ width: `${Math.min(100, masterNode.memory)}%` }" />
            </div>
          </div>
          <div class="flex items-center justify-end gap-1 text-muted-foreground">
            <Container class="size-3" />
            <span class="tabular-nums font-semibold text-foreground">{{ masterNode.containers }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 环绕 Worker 节点 -->
    <div
      v-for="w in positionedWorkers"
      :key="w.node.id"
      class="absolute -translate-x-1/2 -translate-y-1/2 z-10 w-[212px]"
      :style="{ left: `${w.xPct}%`, top: `${w.yPct}%` }"
      @mouseenter="hoveredNodeId = w.node.id"
      @mouseleave="hoveredNodeId = null"
      @click="handleSelectNode(w.node)"
    >
      <div
        class="rounded-xl border bg-card/95 backdrop-blur-sm p-3 shadow-sm transition-all duration-200"
        :class="[
          w.node.status === 'online' ? 'cursor-pointer hover:shadow-md' : 'opacity-65',
          w.isSelected
            ? 'border-emerald-500 ring-2 ring-emerald-500/25'
            : w.node.status === 'online'
              ? 'border-border hover:border-emerald-500/60'
              : 'border-dashed border-border',
        ]"
      >
        <div class="flex items-start justify-between gap-2">
          <div class="flex items-center gap-2 min-w-0">
            <div
              class="size-7 rounded-lg flex items-center justify-center shrink-0"
              :class="
                w.node.status === 'online'
                  ? 'bg-emerald-500/15 text-emerald-500'
                  : 'bg-muted text-muted-foreground'
              "
            >
              <Server class="size-3.5" />
            </div>
            <div class="min-w-0">
              <div class="flex items-center gap-1.5">
                <span class="font-medium text-xs truncate">{{ w.node.name }}</span>
                <Wifi v-if="w.node.status === 'online'" class="size-3 text-emerald-500 shrink-0" />
                <WifiOff v-else class="size-3 text-destructive shrink-0" />
              </div>
              <div class="text-[11px] font-mono text-muted-foreground truncate">
                {{ w.node.address }}
              </div>
            </div>
          </div>
          <Button
            v-if="w.node.status === 'online'"
            variant="ghost"
            size="icon-sm"
            class="size-6 shrink-0 text-muted-foreground hover:text-foreground"
            :title="t('ssh.title')"
            @click="openSsh(w.node, $event)"
          >
            <TerminalSquare class="size-3.5" />
          </Button>
        </div>

        <div class="mt-2.5 grid grid-cols-3 gap-2 text-[11px]">
          <div class="space-y-1">
            <div class="flex items-center justify-between text-muted-foreground">
              <Cpu class="size-3" />
              <span class="tabular-nums font-medium text-foreground">{{ w.node.cpu.toFixed(0) }}%</span>
            </div>
            <div class="h-1 rounded-full bg-muted overflow-hidden">
              <div class="h-full rounded-full transition-all" :class="metricColor(w.node.cpu)" :style="{ width: `${Math.min(100, w.node.cpu)}%` }" />
            </div>
          </div>
          <div class="space-y-1">
            <div class="flex items-center justify-between text-muted-foreground">
              <MemoryStick class="size-3" />
              <span class="tabular-nums font-medium text-foreground">{{ w.node.memory.toFixed(0) }}%</span>
            </div>
            <div class="h-1 rounded-full bg-muted overflow-hidden">
              <div class="h-full rounded-full transition-all" :class="metricColor(w.node.memory)" :style="{ width: `${Math.min(100, w.node.memory)}%` }" />
            </div>
          </div>
          <div class="flex items-center justify-end gap-1 text-muted-foreground">
            <Container class="size-3" />
            <span class="tabular-nums font-semibold text-foreground">{{ w.node.containers }}</span>
          </div>
        </div>
      </div>
    </div>

    <SshDialog v-model:visible="sshDialogVisible" :node="sshTargetNode" />
  </div>
</template>

<style scoped>
.topology-grid {
  background-image: radial-gradient(circle, color-mix(in oklab, var(--muted-foreground) 22%, transparent) 1px, transparent 1px);
  background-size: 20px 20px;
}

@keyframes topoFlow {
  to {
    stroke-dashoffset: -56;
  }
}

@keyframes topoGlow {
  0%,
  100% {
    opacity: 0.45;
  }
  50% {
    opacity: 1;
  }
}

.topo-flow {
  animation: topoFlow 1.8s linear infinite;
}

.topo-master-glow {
  animation: topoGlow 2.8s ease-in-out infinite;
}

@media (prefers-reduced-motion: reduce) {
  .topo-flow,
  .topo-master-glow {
    animation: none;
  }
}
</style>
