<template>
  <div class="compose-page">
    <!-- Page Header -->
    <div class="page-header">
      <h2>{{ t('compose.title') }}</h2>
      <div class="header-actions">
        <el-button type="primary" :icon="Plus" @click="showCreateDialog = true">
          {{ t('compose.create') }}
        </el-button>
        <el-button :icon="Refresh" :loading="loading" @click="handleRefresh">
          {{ t('common.refresh') }}
        </el-button>
      </div>
    </div>

    <!-- Main Content -->
    <div class="compose-content">
      <!-- Left Panel: Project List -->
      <div class="project-list-panel">
        <div class="panel-header">
          <span class="panel-title">{{ t('compose.projects') }}</span>
          <el-input
            v-model="searchQuery"
            :placeholder="t('common.search')"
            :prefix-icon="Search"
            clearable
            size="small"
            class="search-input"
          />
        </div>
        <div class="project-list" v-loading="loading">
          <div
            v-for="project in filteredProjects"
            :key="project.name"
            class="project-item"
            :class="{ active: selectedProject === project.name }"
            @click="handleSelectProject(project.name)"
          >
            <div class="project-info">
              <span class="project-name">{{ project.name }}</span>
              <el-tag
                :type="getStatusType(project.status)"
                size="small"
                effect="plain"
              >
                {{ getStatusText(project.status) }}
              </el-tag>
            </div>
            <el-button
              type="danger"
              :icon="Delete"
              size="small"
              text
              @click.stop="handleDeleteProject(project.name)"
            />
          </div>
          <el-empty
            v-if="!loading && filteredProjects.length === 0"
            :description="t('compose.noProjects')"
          />
        </div>
      </div>

      <!-- Right Panel: Editor and Actions -->
      <div class="editor-panel">
        <template v-if="selectedProject">
          <!-- Container Status -->
          <div v-if="currentProject?.containers?.length" class="container-status">
            <div class="status-header">
              <el-icon><Box /></el-icon>
              {{ t('compose.containers') }}
            </div>
            <el-table :data="currentProject.containers" size="small" stripe max-height="200">
              <el-table-column prop="service" :label="t('compose.service')" min-width="80" />
              <el-table-column prop="name" :label="t('common.name')" min-width="120" show-overflow-tooltip />
              <el-table-column prop="state" :label="t('common.status')" width="90">
                <template #default="{ row }">
                  <el-tag
                    :type="row.state === 'running' ? 'success' : 'info'"
                    size="small"
                  >
                    {{ row.state }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="status" :label="t('compose.statusDetail')" min-width="100" show-overflow-tooltip />
              <el-table-column prop="ports" :label="t('container.ports')" min-width="120" show-overflow-tooltip />
              <el-table-column :label="t('common.actions')" width="180" fixed="right">
                <template #default="{ row }">
                  <el-button
                    type="primary"
                    size="small"
                    link
                    :disabled="row.state !== 'running'"
                    @click="handleOpenTerminal(row.id, row.name)"
                  >
                    <el-icon><Monitor /></el-icon>
                    {{ t('container.terminal') }}
                  </el-button>
                  <el-button
                    type="primary"
                    size="small"
                    link
                    :disabled="row.state !== 'running'"
                    @click="handleOpenFiles(row.id, row.name)"
                  >
                    <el-icon><Folder /></el-icon>
                    {{ t('container.files') }}
                  </el-button>
                  <el-button
                    type="primary"
                    size="small"
                    link
                    @click="handleOpenLogs(row.id, row.name)"
                  >
                    <el-icon><Document /></el-icon>
                    {{ t('container.logs') }}
                  </el-button>
                </template>
              </el-table-column>
            </el-table>
          </div>

          <!-- Actions -->
          <ComposeActions
            :project-name="selectedProject"
            :output="actionOutput"
            :loading="actionLoading"
            :status="currentProject?.status"
            @action="handleAction"
            @clear="handleClearOutput"
          />

          <!-- Editor -->
          <div class="editor-section">
            <div class="section-header">
              <el-icon><Document /></el-icon>
              {{ t('compose.editor') }}
            </div>
            <ComposeEditor
              ref="editorRef"
              :project-name="selectedProject"
              :content="fileContent"
              :loading="fileLoading"
              @save="handleSaveFile"
              @change="handleFileChange"
            />
          </div>
        </template>

        <!-- No Project Selected -->
        <el-empty
          v-else
          :description="t('compose.selectProject')"
          class="empty-state"
        />
      </div>
    </div>

    <!-- Create Project Dialog -->
    <el-dialog
      v-model="showCreateDialog"
      :title="t('compose.create')"
      width="400px"
      @close="resetCreateForm"
    >
      <el-form
        ref="createFormRef"
        :model="createForm"
        :rules="createRules"
        label-width="100px"
      >
        <el-form-item :label="t('compose.projectName')" prop="name">
          <el-input
            v-model="createForm.name"
            :placeholder="t('compose.projectNamePlaceholder')"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">
          {{ t('common.cancel') }}
        </el-button>
        <el-button type="primary" :loading="creating" @click="handleCreate">
          {{ t('common.create') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- Container Logs Dialog -->
    <ContainerLogsDialog
      v-model:visible="showLogsDialog"
      :container-id="selectedContainerId"
      :container-name="selectedContainerName"
    />

    <!-- Container Terminal Dialog -->
    <ContainerTerminal
      v-model:visible="showTerminalDialog"
      :container-id="selectedContainerId"
      :container-name="selectedContainerName"
    />

    <!-- Container File Explorer Dialog -->
    <FileExplorer
      v-model:visible="showFilesDialog"
      :container-id="selectedContainerId"
      :container-name="selectedContainerName"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive } from 'vue'
import { storeToRefs } from 'pinia'
import {
  Plus,
  Refresh,
  Search,
  Delete,
  Document,
  Box,
  Monitor,
  Folder,
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { useI18n } from '@/composables/useI18n'
import { useComposeStore } from '@/stores/compose'
import ComposeEditor from '@/components/compose/ComposeEditor.vue'
import ComposeActions from '@/components/compose/ComposeActions.vue'
import ContainerLogsDialog from '@/components/container/ContainerLogsDialog.vue'
import ContainerTerminal from '@/components/container/ContainerTerminal.vue'
import FileExplorer from '@/components/container/FileExplorer.vue'

const { t } = useI18n()
const composeStore = useComposeStore()

// Store state
const {
  filteredProjects,
  loading,
  selectedProject,
  fileContent,
  fileLoading,
  actionLoading,
  actionOutput,
  currentProject,
} = storeToRefs(composeStore)

// Local state
const searchQuery = ref('')
const showCreateDialog = ref(false)
const creating = ref(false)
const editorRef = ref<InstanceType<typeof ComposeEditor> | null>(null)

// 容器对话框状态
const showLogsDialog = ref(false)
const showTerminalDialog = ref(false)
const showFilesDialog = ref(false)
const selectedContainerId = ref('')
const selectedContainerName = ref('')

// Create form
const createFormRef = ref<FormInstance>()
const createForm = reactive({
  name: '',
})
const createRules: FormRules = {
  name: [
    { required: true, message: t('compose.nameRequired'), trigger: 'blur' },
    {
      pattern: /^[a-zA-Z0-9_-]+$/,
      message: t('compose.nameInvalid'),
      trigger: 'blur',
    },
  ],
}

// Computed
const getStatusType = (status: string) => {
  switch (status) {
    case 'running':
      return 'success'
    case 'partial':
      return 'warning'
    case 'stopped':
      return 'info'
    default:
      return 'info'
  }
}

const getStatusText = (status: string) => {
  switch (status) {
    case 'running':
      return t('compose.statusRunning')
    case 'partial':
      return t('compose.statusPartial')
    case 'stopped':
      return t('compose.statusStopped')
    default:
      return t('compose.statusUnknown')
  }
}

// Methods
async function handleRefresh() {
  await composeStore.fetchProjects()
  if (selectedProject.value) {
    await composeStore.fetchProjectStatus(selectedProject.value)
  }
}

async function handleSelectProject(name: string) {
  if (selectedProject.value === name) {
    return
  }
  if (editorRef.value?.hasChanges) {
    try {
      await ElMessageBox.confirm(
        t('compose.unsavedConfirm'),
        t('common.warning'),
        {
          confirmButtonText: t('common.confirm'),
          cancelButtonText: t('common.cancel'),
          type: 'warning',
        }
      )
    } catch {
      return // User cancelled
    }
  }
  await composeStore.selectProject(name)
}

async function handleDeleteProject(name: string) {
  try {
    await ElMessageBox.confirm(
      t('compose.confirmDelete'),
      t('common.warning'),
      {
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
        type: 'warning',
      }
    )
    await composeStore.deleteProject(name)
    ElMessage.success(t('compose.deleteSuccess'))
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(
        error instanceof Error ? error.message : t('compose.deleteFailed')
      )
    }
  }
}

async function handleCreate() {
  if (!createFormRef.value) return

  try {
    await createFormRef.value.validate()
    creating.value = true
    await composeStore.createProject(createForm.name)
    ElMessage.success(t('compose.createSuccess'))
    showCreateDialog.value = false
    resetCreateForm()
  } catch (error) {
    if (error !== false) {
      ElMessage.error(
        error instanceof Error ? error.message : t('compose.createFailed')
      )
    }
  } finally {
    creating.value = false
  }
}

function resetCreateForm() {
  createForm.name = ''
  createFormRef.value?.resetFields()
}

async function handleSaveFile(content: string) {
  if (!selectedProject.value) return

  try {
    await composeStore.saveFileContent(selectedProject.value, content)
    ElMessage.success(t('compose.saveSuccess'))
  } catch (error) {
    ElMessage.error(
      error instanceof Error ? error.message : t('compose.saveFailed')
    )
  }
}

function handleFileChange(_content: string) {
  // Could be used for auto-save or validation
}

async function handleAction(action: 'up' | 'down' | 'restart' | 'pull') {
  if (!selectedProject.value) return
  composeStore.executeAction(selectedProject.value, action)
}

function handleClearOutput() {
  composeStore.clearOutput()
}

// 容器操作函数 - 直接使用容器 ID
function handleOpenTerminal(containerId: string, containerName: string) {
  selectedContainerId.value = containerId
  selectedContainerName.value = containerName
  showTerminalDialog.value = true
}

function handleOpenFiles(containerId: string, containerName: string) {
  selectedContainerId.value = containerId
  selectedContainerName.value = containerName
  showFilesDialog.value = true
}

function handleOpenLogs(containerId: string, containerName: string) {
  selectedContainerId.value = containerId
  selectedContainerName.value = containerName
  showLogsDialog.value = true
}

// Watch search query
function handleSearch() {
  composeStore.setSearch(searchQuery.value)
}

// Lifecycle
onMounted(async () => {
  // 清除之前的选中状态，让用户先选择项目
  composeStore.selectProject(null)
  await composeStore.fetchProjects()
})

// Watch search
import { watch } from 'vue'
watch(searchQuery, handleSearch)
</script>

<style scoped>
.compose-page {
  /* 顶栏 60px + 主区域上下内边距 40px，锁在视口内，避免页面再出一条滚动条 */
  height: calc(100vh - 100px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
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
  gap: 12px;
}

.compose-content {
  flex: 1;
  display: flex;
  gap: 20px;
  min-height: 0;
  overflow: hidden;
}

/* Left Panel */
.project-list-panel {
  width: 280px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--el-border-color-light);
  border-radius: 4px;
  background: var(--el-bg-color);
}

.panel-header {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  border-bottom: 1px solid var(--el-border-color-light);
}

.panel-title {
  font-weight: 500;
  font-size: 14px;
}

.search-input {
  width: 100%;
}

.project-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}

.project-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  margin-bottom: 4px;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.2s;
  border: 1px solid transparent;
}

.project-item:hover {
  background: var(--el-fill-color-light);
}

.project-item.active {
  background: var(--el-color-primary-light-9);
  border-color: var(--el-color-primary);
  box-shadow: 0 0 0 1px var(--el-color-primary-light-5);
}

.project-item.active .project-name {
  color: var(--el-color-primary);
  font-weight: 600;
}

.project-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.project-name {
  font-weight: 500;
  font-size: 14px;
}

/* Right Panel */
.editor-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}

.container-status {
  flex-shrink: 0;
  border: 1px solid var(--el-border-color-light);
  border-radius: 4px;
  overflow: hidden;
}

.status-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 12px;
  background: var(--el-fill-color-light);
  border-bottom: 1px solid var(--el-border-color-light);
  font-weight: 500;
  font-size: 14px;
}

.editor-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--el-border-color-light);
  border-radius: 4px;
  overflow: hidden;
  min-height: 0;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 12px;
  background: var(--el-fill-color-light);
  border-bottom: 1px solid var(--el-border-color-light);
  font-weight: 500;
  font-size: 14px;
}

.empty-state {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* Responsive */
@media (max-width: 768px) {
  .compose-page {
    height: calc(100vh - 84px);
  }

  .compose-content {
    flex-direction: column;
  }

  .project-list-panel {
    width: 100%;
    max-height: 200px;
  }
}
</style>
