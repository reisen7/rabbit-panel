<template>
  <div class="nodes-page">
    <!-- Header with title and actions -->
    <div class="page-header">
      <h2>{{ t('node.title') }}</h2>
      <div class="header-actions">
        <el-button :loading="nodesStore.loading" @click="handleRefresh">
          <el-icon><Refresh /></el-icon>
          {{ t('common.refresh') }}
        </el-button>
      </div>
    </div>

    <!-- Not Master Mode Warning -->
    <el-alert
      v-if="!nodesStore.isMasterMode && !nodesStore.loading"
      :title="t('node.notMasterMode')"
      type="warning"
      show-icon
      :closable="false"
      style="margin-bottom: 20px"
    />

    <!-- Stats Cards -->
    <div v-if="nodesStore.isMasterMode" class="stats-cards">
      <el-card class="stat-card">
        <div class="stat-content">
          <div class="stat-icon total">
            <el-icon><Monitor /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">{{ nodesStore.totalNodes }}</div>
            <div class="stat-label">{{ t('node.totalNodes') }}</div>
          </div>
        </div>
      </el-card>
      <el-card class="stat-card">
        <div class="stat-content">
          <div class="stat-icon online">
            <el-icon><CircleCheck /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">{{ nodesStore.onlineCount }}</div>
            <div class="stat-label">{{ t('node.onlineNodes') }}</div>
          </div>
        </div>
      </el-card>
      <el-card class="stat-card">
        <div class="stat-content">
          <div class="stat-icon offline">
            <el-icon><CircleClose /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value">{{ nodesStore.offlineCount }}</div>
            <div class="stat-label">{{ t('node.offlineNodes') }}</div>
          </div>
        </div>
      </el-card>
    </div>

    <!-- Filter bar -->
    <div v-if="nodesStore.isMasterMode" class="filter-bar">
      <el-input
        v-model="searchInput"
        :placeholder="t('node.searchPlaceholder')"
        clearable
        style="width: 300px"
        @input="handleSearch"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>
      <el-select
        v-model="statusFilterValue"
        :placeholder="t('node.filterByStatus')"
        style="width: 150px"
        @change="handleStatusFilterChange"
      >
        <el-option :label="t('node.allNodes')" value="all" />
        <el-option :label="t('node.onlineOnly')" value="online" />
        <el-option :label="t('node.offlineOnly')" value="offline" />
      </el-select>
    </div>

    <!-- Nodes table -->
    <el-table
      v-if="nodesStore.isMasterMode"
      v-loading="nodesStore.loading"
      :data="nodesStore.filteredNodes"
      stripe
      style="width: 100%"
    >
      <el-table-column
        prop="id"
        :label="t('node.nodeId')"
        width="140"
        class-name="hidden-sm-and-down"
      >
        <template #default="{ row }">
          <span class="node-id">{{ row.id.substring(0, 12) }}</span>
        </template>
      </el-table-column>
      <el-table-column
        prop="name"
        :label="t('node.nodeName')"
        min-width="150"
      >
        <template #default="{ row }">
          <span>{{ row.name }}</span>
        </template>
      </el-table-column>
      <el-table-column
        prop="address"
        :label="t('node.address')"
        min-width="150"
        class-name="hidden-xs-only"
      />
      <el-table-column
        prop="mode"
        :label="t('node.mode')"
        width="100"
        class-name="hidden-xs-only"
      >
        <template #default="{ row }">
          <el-tag :type="row.mode === 'master' ? 'primary' : 'info'" size="small">
            {{ row.mode === 'master' ? t('node.master') : t('node.worker') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column
        prop="status"
        :label="t('common.status')"
        width="100"
      >
        <template #default="{ row }">
          <el-tag :type="getStatusType(row.status)" size="small">
            {{ getStatusText(row.status) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column
        :label="t('node.resourceUsage')"
        min-width="280"
        class-name="hidden-sm-and-down"
      >
        <template #default="{ row }">
          <div class="resource-bars">
            <div class="resource-item">
              <span class="resource-label">{{ t('node.cpu') }}</span>
              <el-progress
                :percentage="row.cpu"
                :stroke-width="8"
                :color="getProgressColor(row.cpu)"
                :show-text="true"
                :format="(p: number) => `${p.toFixed(1)}%`"
              />
            </div>
            <div class="resource-item">
              <span class="resource-label">{{ t('node.memory') }}</span>
              <el-progress
                :percentage="row.memory"
                :stroke-width="8"
                :color="getProgressColor(row.memory)"
                :show-text="true"
                :format="(p: number) => `${p.toFixed(1)}%`"
              />
            </div>
            <div class="resource-item">
              <span class="resource-label">{{ t('node.disk') }}</span>
              <el-progress
                :percentage="row.disk"
                :stroke-width="8"
                :color="getProgressColor(row.disk)"
                :show-text="true"
                :format="(p: number) => `${p.toFixed(1)}%`"
              />
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column
        prop="containers"
        :label="t('node.containers')"
        width="100"
        align="center"
      >
        <template #default="{ row }">
          <el-tag type="success" size="small">{{ row.containers }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column
        prop="last_seen"
        :label="t('node.lastHeartbeat')"
        width="180"
        class-name="hidden-md-and-down"
      >
        <template #default="{ row }">
          <span>{{ formatLastSeen(row.last_seen) }}</span>
        </template>
      </el-table-column>
    </el-table>

    <!-- Empty state when no nodes -->
    <el-empty
      v-if="nodesStore.isMasterMode && nodesStore.filteredNodes.length === 0 && !nodesStore.loading"
      :description="t('node.noNodes')"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { Refresh, Search, Monitor, CircleCheck, CircleClose } from '@element-plus/icons-vue'
import { useNodesStore } from '@/stores/nodes'
import { useI18n } from '@/composables/useI18n'

const { t } = useI18n()
const nodesStore = useNodesStore()

// Local state for v-model bindings
const searchInput = ref('')
const statusFilterValue = ref<'all' | 'online' | 'offline'>('all')

// Sync local state with store
watch(() => nodesStore.searchQuery, (val) => { searchInput.value = val })
watch(() => nodesStore.statusFilter, (val) => { statusFilterValue.value = val })

// Handlers
function handleSearch(value: string) {
  nodesStore.setSearch(value)
}

function handleStatusFilterChange(value: 'all' | 'online' | 'offline') {
  nodesStore.setStatusFilter(value)
}

async function handleRefresh() {
  await nodesStore.fetchNodes()
}

// Helper functions
function getStatusType(status: string): 'success' | 'danger' | 'warning' {
  switch (status) {
    case 'online':
      return 'success'
    case 'offline':
      return 'danger'
    case 'error':
      return 'warning'
    default:
      return 'warning'
  }
}

function getStatusText(status: string): string {
  switch (status) {
    case 'online':
      return t('node.online')
    case 'offline':
      return t('node.offline')
    case 'error':
      return t('node.error')
    default:
      return status
  }
}

function getProgressColor(percentage: number): string {
  if (percentage < 50) return '#67c23a'
  if (percentage < 80) return '#e6a23c'
  return '#f56c6c'
}

function formatLastSeen(lastSeen: string): string {
  if (!lastSeen) return t('node.neverSeen')
  
  try {
    const date = new Date(lastSeen)
    return date.toLocaleString()
  } catch {
    return lastSeen
  }
}

onMounted(() => {
  nodesStore.startPolling()
})
</script>

<style scoped>
.nodes-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.stats-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 20px;
  margin-bottom: 20px;
}

.stat-card {
  border-radius: 8px;
}

.stat-content {
  display: flex;
  align-items: center;
  gap: 15px;
}

.stat-icon {
  width: 50px;
  height: 50px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
}

.stat-icon.total {
  background-color: rgba(64, 158, 255, 0.1);
  color: #409eff;
}

.stat-icon.online {
  background-color: rgba(103, 194, 58, 0.1);
  color: #67c23a;
}

.stat-icon.offline {
  background-color: rgba(245, 108, 108, 0.1);
  color: #f56c6c;
}

.stat-info {
  flex: 1;
}

.stat-value {
  font-size: 28px;
  font-weight: 600;
  line-height: 1.2;
}

.stat-label {
  font-size: 14px;
  color: var(--el-text-color-secondary);
}

.filter-bar {
  display: flex;
  gap: 15px;
  margin-bottom: 20px;
}

.node-id {
  font-family: monospace;
  font-size: 12px;
}

.resource-bars {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.resource-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.resource-label {
  width: 50px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.resource-item :deep(.el-progress) {
  flex: 1;
}

.resource-item :deep(.el-progress__text) {
  min-width: 50px;
  font-size: 12px;
}

/* Responsive styles */
@media (max-width: 767px) {
  .nodes-page {
    padding: 12px;
  }
  
  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }
  
  .header-actions {
    width: 100%;
  }
  
  .header-actions .el-button {
    flex: 1;
  }
  
  .stats-cards {
    grid-template-columns: 1fr;
    gap: 12px;
  }
  
  .stat-value {
    font-size: 24px;
  }
  
  .filter-bar {
    flex-direction: column;
  }
  
  .filter-bar .el-input,
  .filter-bar .el-select {
    width: 100% !important;
  }
}

/* Hide columns on smaller screens */
:deep(.hidden-xs-only) {
  @media (max-width: 767px) {
    display: none !important;
  }
}

:deep(.hidden-sm-and-down) {
  @media (max-width: 991px) {
    display: none !important;
  }
}

:deep(.hidden-md-and-down) {
  @media (max-width: 1199px) {
    display: none !important;
  }
}
</style>
