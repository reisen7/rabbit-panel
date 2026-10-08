<template>
  <div class="docker-config-page">
    <!-- Header -->
    <div class="page-header">
      <h2>{{ t('dockerConfig.title') }}</h2>
      <div class="header-actions">
        <el-button :loading="dockerConfigStore.loading" @click="handleRefresh">
          <el-icon><Refresh /></el-icon>
          {{ t('common.refresh') }}
        </el-button>
      </div>
    </div>

    <!-- Loading state -->
    <div v-if="dockerConfigStore.loading && !dockerConfigStore.info" class="loading-container">
      <el-skeleton :rows="10" animated />
    </div>

    <template v-else>
      <el-alert
        v-if="dockerConfigStore.error"
        :title="dockerConfigStore.error"
        type="warning"
        show-icon
        :closable="false"
        class="page-alert"
      />

      <!-- Docker Info Card -->
      <el-card class="info-card" shadow="hover">
        <template #header>
          <div class="card-header">
            <el-icon><InfoFilled /></el-icon>
            <span>{{ t('dockerConfig.info') }}</span>
          </div>
        </template>
        <el-descriptions :column="responsiveColumns" border>
          <el-descriptions-item :label="t('dockerConfig.version')">
            {{ dockerConfigStore.info?.serverVersion || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('dockerConfig.apiVersion')">
            {{ dockerConfigStore.info?.apiVersion || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('dockerConfig.os')">
            {{ dockerConfigStore.info?.operatingSystem || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('dockerConfig.arch')">
            {{ dockerConfigStore.info?.arch || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('dockerConfig.kernel')">
            {{ dockerConfigStore.info?.kernelVersion || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('dockerConfig.storageDriver')">
            {{ dockerConfigStore.info?.driver || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('dockerConfig.rootDir')">
            {{ dockerConfigStore.info?.dockerRootDir || '-' }}
          </el-descriptions-item>
        </el-descriptions>
      </el-card>

      <!-- Configuration Form -->
      <el-form
        ref="formRef"
        :model="formData"
        label-position="top"
        class="config-form"
      >
        <!-- Registry Mirrors -->
        <el-card class="config-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <el-icon><Link /></el-icon>
              <span>{{ t('dockerConfig.registryMirrors') }}</span>
            </div>
          </template>
          <p class="help-text">{{ t('dockerConfig.registryMirrorsHelp') }}</p>
          <div class="list-items">
            <div
              v-for="(_mirror, index) in formData.registryMirrors"
              :key="index"
              class="list-item"
            >
              <el-input
                v-model="formData.registryMirrors[index]"
                :placeholder="t('dockerConfig.mirrorPlaceholder')"
              />
              <el-button
                type="danger"
                :icon="Delete"
                circle
                @click="removeMirror(index)"
              />
            </div>
          </div>
          <el-button type="primary" link @click="addMirror">
            <el-icon><Plus /></el-icon>
            {{ t('dockerConfig.addMirror') }}
          </el-button>
        </el-card>

        <!-- Insecure Registries -->
        <el-card class="config-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <el-icon><OfficeBuilding /></el-icon>
              <span>{{ t('dockerConfig.insecureRegistries') }}</span>
            </div>
          </template>
          <p class="help-text">{{ t('dockerConfig.insecureRegistriesHelp') }}</p>
          <div class="list-items">
            <div
              v-for="(_registry, index) in formData.insecureRegistries"
              :key="index"
              class="list-item"
            >
              <el-select
                v-model="formData.insecureRegistries[index]"
                filterable
                class="registry-select"
                :placeholder="t('dockerConfig.registrySelectPlaceholder')"
              >
                <el-option
                  v-for="option in registryOptions"
                  :key="option.value"
                  :label="option.label"
                  :value="option.value"
                  :disabled="isRegistrySelected(option.value, index)"
                />
              </el-select>
              <el-button
                type="danger"
                :icon="Delete"
                circle
                @click="removeInsecureRegistry(index)"
              />
            </div>
          </div>
          <el-button type="primary" link @click="addInsecureRegistry">
            <el-icon><Plus /></el-icon>
            {{ t('dockerConfig.addRegistry') }}
          </el-button>
        </el-card>

        <!-- Network Configuration -->
        <el-card class="config-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <el-icon><Connection /></el-icon>
              <span>{{ t('dockerConfig.ipv6') }} & {{ t('dockerConfig.iptables') }}</span>
            </div>
          </template>
          <el-row :gutter="20">
            <el-col :xs="24" :sm="12">
              <el-form-item :label="t('dockerConfig.ipv6')">
                <el-switch v-model="formData.ipv6" />
                <span class="switch-help">{{ t('dockerConfig.ipv6Help') }}</span>
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item :label="t('dockerConfig.iptables')">
                <el-switch v-model="formData.iptables" />
                <span class="switch-help">{{ t('dockerConfig.iptablesHelp') }}</span>
              </el-form-item>
            </el-col>
          </el-row>
        </el-card>

        <!-- Log Configuration -->
        <el-card class="config-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <el-icon><Document /></el-icon>
              <span>{{ t('dockerConfig.logDriver') }}</span>
            </div>
          </template>
          <p class="help-text">{{ t('dockerConfig.logDriverHelp') }}</p>
          <el-row :gutter="20">
            <el-col :xs="24" :sm="8">
              <el-form-item :label="t('dockerConfig.logDriver')">
                <el-select v-model="formData.logDriver" style="width: 100%">
                  <el-option value="json-file" label="json-file" />
                  <el-option value="local" label="local" />
                  <el-option value="journald" label="journald" />
                  <el-option value="syslog" label="syslog" />
                  <el-option value="none" label="none" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="8">
              <el-form-item :label="t('dockerConfig.maxSize')">
                <el-input
                  v-model="formData.logOpts['max-size']"
                  placeholder="10m"
                />
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="8">
              <el-form-item :label="t('dockerConfig.maxFile')">
                <el-input
                  v-model="formData.logOpts['max-file']"
                  placeholder="3"
                />
              </el-form-item>
            </el-col>
          </el-row>
        </el-card>

        <!-- Runtime Configuration -->
        <el-card class="config-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <el-icon><Setting /></el-icon>
              <span>{{ t('dockerConfig.liveRestore') }} & {{ t('dockerConfig.cgroupDriver') }}</span>
            </div>
          </template>
          <el-row :gutter="20">
            <el-col :xs="24" :sm="12">
              <el-form-item :label="t('dockerConfig.liveRestore')">
                <el-switch v-model="formData.liveRestore" />
                <span class="switch-help">{{ t('dockerConfig.liveRestoreHelp') }}</span>
              </el-form-item>
            </el-col>
            <el-col :xs="24" :sm="12">
              <el-form-item :label="t('dockerConfig.cgroupDriver')">
                <el-radio-group v-model="formData.cgroupDriver">
                  <el-radio value="cgroupfs">cgroupfs</el-radio>
                  <el-radio value="systemd">systemd</el-radio>
                </el-radio-group>
                <p class="help-text">{{ t('dockerConfig.cgroupDriverHelp') }}</p>
              </el-form-item>
            </el-col>
          </el-row>
        </el-card>

        <!-- Action Buttons -->
        <div class="action-buttons">
          <el-button
            type="primary"
            size="large"
            :loading="dockerConfigStore.saving"
            @click="handleSave"
          >
            <el-icon><Check /></el-icon>
            {{ t('dockerConfig.save') }}
          </el-button>
          <el-popconfirm
            :title="t('dockerConfig.restartConfirm')"
            :confirm-button-text="t('common.confirm')"
            :cancel-button-text="t('common.cancel')"
            @confirm="handleRestart"
          >
            <template #reference>
              <el-button
                type="warning"
                size="large"
                :loading="restarting"
              >
                <el-icon><RefreshRight /></el-icon>
                {{ restarting ? t('dockerConfig.restarting') : t('dockerConfig.restart') }}
              </el-button>
            </template>
          </el-popconfirm>
        </div>
      </el-form>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed, watch } from 'vue'
import {
  Refresh,
  InfoFilled,
  Link,
  OfficeBuilding,
  Connection,
  Document,
  Setting,
  Delete,
  Plus,
  Check,
  RefreshRight,
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useDockerConfigStore } from '@/stores/dockerConfig'
import { useRegistryStore } from '@/stores/registry'
import { useI18n } from '@/composables/useI18n'
import type { DockerConfig } from '@/types'

const { t } = useI18n()
const dockerConfigStore = useDockerConfigStore()
const registryStore = useRegistryStore()

// Form data
const formData = reactive<DockerConfig>({
  registryMirrors: [],
  insecureRegistries: [],
  ipv6: false,
  logDriver: 'json-file',
  logOpts: {
    'max-size': '10m',
    'max-file': '3',
  },
  iptables: true,
  liveRestore: false,
  cgroupDriver: 'cgroupfs',
})

// Restarting state
const restarting = ref(false)

// Responsive columns for descriptions
const responsiveColumns = computed(() => {
  if (typeof window !== 'undefined') {
    return window.innerWidth < 768 ? 1 : 2
  }
  return 2
})

// Sync form data with store config
watch(
  () => dockerConfigStore.config,
  (config) => {
    if (config) {
      formData.registryMirrors = [...(config.registryMirrors || [])]
      formData.insecureRegistries = (config.insecureRegistries || []).map(registryHost).filter(Boolean)
      formData.ipv6 = config.ipv6 ?? false
      formData.logDriver = config.logDriver || 'json-file'
      formData.logOpts = config.logOpts ? { ...config.logOpts } : { 'max-size': '10m', 'max-file': '3' }
      formData.iptables = config.iptables ?? true
      formData.liveRestore = config.liveRestore ?? false
      formData.cgroupDriver = config.cgroupDriver || 'cgroupfs'
    }
  },
  { immediate: true }
)

// Mirror list handlers
function addMirror() {
  formData.registryMirrors.push('')
}

function removeMirror(index: number) {
  formData.registryMirrors.splice(index, 1)
}

function registryHost(url: string): string {
  let value = url.trim()
  value = value.replace(/^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//, '')
  const slash = value.indexOf('/')
  if (slash >= 0) {
    value = value.slice(0, slash)
  }
  return value
}

const registryOptions = computed(() => {
  const options = new Map<string, string>()
  for (const registry of registryStore.registries) {
    const host = registryHost(registry.url || '')
    if (!host || options.has(host)) continue
    options.set(host, registry.name ? `${registry.name}（${host}）` : host)
  }
  for (const current of formData.insecureRegistries) {
    const host = registryHost(current)
    if (host && !options.has(host)) {
      options.set(host, host)
    }
  }
  return Array.from(options, ([value, label]) => ({ value, label }))
})

function isRegistrySelected(host: string, currentIndex: number): boolean {
  return formData.insecureRegistries.some((item, index) => index !== currentIndex && registryHost(item) === host)
}

// Insecure registry list handlers
function addInsecureRegistry() {
  formData.insecureRegistries.push('')
}

function removeInsecureRegistry(index: number) {
  formData.insecureRegistries.splice(index, 1)
}

// Refresh handler
async function handleRefresh() {
  await dockerConfigStore.fetchConfig()
}

// Save handler
async function handleSave() {
  try {
    // Filter out empty strings
    const configToSave: Partial<DockerConfig> = {
      registryMirrors: formData.registryMirrors.filter((m) => m.trim() !== ''),
      insecureRegistries: formData.insecureRegistries.filter((r) => r.trim() !== ''),
      ipv6: formData.ipv6,
      logDriver: formData.logDriver,
      logOpts: formData.logOpts,
      iptables: formData.iptables,
      liveRestore: formData.liveRestore,
      cgroupDriver: formData.cgroupDriver,
    }
    await dockerConfigStore.updateConfig(configToSave)
    ElMessage.warning(t('dockerConfig.saveWarning'))
  } catch {
    // Error handled by store
  }
}

// Restart handler
async function handleRestart() {
  try {
    restarting.value = true
    await dockerConfigStore.restartDocker()
    ElMessage.success(t('dockerConfig.restartSuccess'))
    // Refresh config after restart
    setTimeout(() => {
      dockerConfigStore.fetchConfig()
    }, 3000)
  } catch {
    // Error handled by store
  } finally {
    restarting.value = false
  }
}

// Fetch config on mount
onMounted(() => {
  dockerConfigStore.fetchConfig()
  registryStore.fetchRegistries()
})
</script>

<style scoped>
.docker-config-page {
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

.loading-container {
  padding: 20px;
}

.page-alert {
  margin-bottom: 20px;
}

.info-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}

.config-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.config-card {
  margin-bottom: 0;
}

.help-text {
  color: var(--el-text-color-secondary);
  font-size: 13px;
  margin: 0 0 15px 0;
}

.list-items {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 10px;
}

.list-item {
  display: flex;
  gap: 10px;
  align-items: center;
}

.list-item .el-input,
.list-item .registry-select {
  flex: 1;
}

.switch-help {
  margin-left: 10px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.action-buttons {
  display: flex;
  gap: 15px;
  justify-content: flex-start;
  padding: 20px 0;
}

/* Responsive styles */
@media (max-width: 767px) {
  .docker-config-page {
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

  .action-buttons {
    flex-direction: column;
  }

  .action-buttons .el-button {
    width: 100%;
  }

  .switch-help {
    display: block;
    margin-left: 0;
    margin-top: 5px;
  }
}
</style>
