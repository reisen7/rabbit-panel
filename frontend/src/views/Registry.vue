<template>
  <div class="registry-page">
    <!-- Header with title and actions -->
    <div class="page-header">
      <div>
        <h2>{{ t('registry.title') }}</h2>
        <p class="page-desc">{{ t('registry.usage') }}</p>
      </div>
      <div class="header-actions">
        <el-button type="primary" @click="handleAdd">
          <el-icon><Plus /></el-icon>
          {{ t('registry.add') }}
        </el-button>
        <el-button :loading="registryStore.loading" @click="handleRefresh">
          <el-icon><Refresh /></el-icon>
          {{ t('common.refresh') }}
        </el-button>
      </div>
    </div>

    <!-- Registry cards -->
    <div v-loading="registryStore.loading" class="registry-list">
      <template v-if="registryStore.registries.length > 0">
        <el-card
          v-for="registry in registryStore.registries"
          :key="registry.id"
          class="registry-card"
          shadow="hover"
        >
          <template #header>
            <div class="card-header">
              <div class="registry-name">
                <el-icon><OfficeBuilding /></el-icon>
                <span>{{ registry.name }}</span>
                <el-tag v-if="registry.isDefault" size="small" type="success">
                  {{ t('registry.default') }}
                </el-tag>
              </div>
              <div class="card-actions">
                <el-button
                  type="primary"
                  size="small"
                  link
                  :loading="testingId === registry.url"
                  @click="handleTest(registry)"
                >
                  <el-icon><Connection /></el-icon>
                  {{ testingId === registry.url ? t('registry.testing') : t('registry.testConnection') }}
                </el-button>
              </div>
            </div>
          </template>

          <div class="registry-info">
            <div class="info-row">
              <span class="label">{{ t('registry.url') }}:</span>
              <span class="value url">{{ registry.url }}</span>
            </div>
            <div v-if="registry.username" class="info-row">
              <span class="label">{{ t('registry.username') }}:</span>
              <span class="value">{{ registry.username }}</span>
            </div>
            <div class="info-row">
              <span class="label">{{ t('common.createdAt') }}:</span>
              <span class="value">{{ formatDate(registry.createdAt) }}</span>
            </div>
          </div>

          <div class="card-footer">
            <el-button type="primary" size="small" @click="handleEdit(registry)">
              <el-icon><Edit /></el-icon>
              {{ t('common.edit') }}
            </el-button>
            <el-popconfirm
              :title="t('registry.confirmRemove')"
              :confirm-button-text="t('common.confirm')"
              :cancel-button-text="t('common.cancel')"
              @confirm="handleRemove(registry.url)"
            >
              <template #reference>
                <el-button type="danger" size="small">
                  <el-icon><Delete /></el-icon>
                  {{ t('common.delete') }}
                </el-button>
              </template>
            </el-popconfirm>
          </div>
        </el-card>
      </template>

      <!-- Empty state -->
      <el-empty
        v-else-if="!registryStore.loading"
        :description="t('registry.noRegistries')"
      >
        <el-button type="primary" @click="handleAdd">
          <el-icon><Plus /></el-icon>
          {{ t('registry.add') }}
        </el-button>
      </el-empty>
    </div>

    <!-- Registry Dialog -->
    <RegistryDialog
      v-model:visible="showDialog"
      :registry="editingRegistry"
      @saved="handleSaved"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Refresh, Plus, Edit, Delete, OfficeBuilding, Connection } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useRegistryStore } from '@/stores/registry'
import { useI18n } from '@/composables/useI18n'
import RegistryDialog from '@/components/registry/RegistryDialog.vue'
import type { RegistryInfo } from '@/types'

const { t } = useI18n()
const registryStore = useRegistryStore()

// Dialog state
const showDialog = ref(false)
const editingRegistry = ref<RegistryInfo | null>(null)

// Testing state
const testingId = ref<string | null>(null)

// Format date helper
function formatDate(dateStr: string): string {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleString()
}

// Handlers
function handleAdd() {
  editingRegistry.value = null
  showDialog.value = true
}

function handleEdit(registry: RegistryInfo) {
  editingRegistry.value = registry
  showDialog.value = true
}

async function handleRemove(id: string) {
  try {
    await registryStore.removeRegistry(id)
    ElMessage.success(t('registry.removeSuccess'))
  } catch {
    // Error is handled by request interceptor
  }
}

async function handleTest(registry: RegistryInfo) {
  try {
    testingId.value = registry.url
    const result = await registryStore.testRegistry(registry.url, {
      url: registry.url,
      username: registry.username,
    })
    if (result.success) {
      ElMessage.success(t('registry.testSuccess'))
    } else {
      ElMessage.error(`${t('registry.testFailed')}: ${result.message}`)
    }
  } catch {
    ElMessage.error(t('registry.testFailed'))
  } finally {
    testingId.value = null
  }
}

async function handleRefresh() {
  await registryStore.fetchRegistries()
}

function handleSaved() {
  registryStore.fetchRegistries()
}

// Fetch registries on mount
onMounted(() => {
  registryStore.fetchRegistries()
})
</script>

<style scoped>
.registry-page {
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

.page-desc {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
  line-height: 1.5;
}

.header-actions {
  display: flex;
  gap: 10px;
}

.registry-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
  gap: 20px;
}

.registry-card {
  transition: transform 0.2s;
}

.registry-card:hover {
  transform: translateY(-2px);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.registry-name {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  font-size: 16px;
}

.registry-name .el-icon {
  color: var(--el-color-primary);
}

.card-actions {
  display: flex;
  gap: 8px;
}

.registry-info {
  margin-bottom: 16px;
}

.info-row {
  display: flex;
  margin-bottom: 8px;
  font-size: 14px;
}

.info-row .label {
  color: var(--el-text-color-secondary);
  min-width: 80px;
}

.info-row .value {
  color: var(--el-text-color-primary);
  word-break: break-all;
}

.info-row .value.url {
  font-family: monospace;
  font-size: 13px;
}

.card-footer {
  display: flex;
  gap: 10px;
  padding-top: 12px;
  border-top: 1px solid var(--el-border-color-lighter);
}

/* Responsive styles */
@media (max-width: 767px) {
  .registry-page {
    padding: 12px;
  }

  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .header-actions {
    width: 100%;
    flex-wrap: wrap;
  }

  .header-actions .el-button {
    flex: 1;
    min-width: 80px;
  }

  .registry-list {
    grid-template-columns: 1fr;
  }

  .card-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .card-footer {
    flex-wrap: wrap;
  }
}
</style>
