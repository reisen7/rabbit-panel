<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  Bell,
  Box,
  Connection,
  Cpu,
  Expand,
  Files,
  Fold,
  FolderOpened,
  Menu,
  Monitor,
  Moon,
  OfficeBuilding,
  Picture,
  Service,
  Setting,
  Sunny,
  SwitchButton,
  User,
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { useNodesStore } from '@/stores/nodes'
import { useContainerStore } from '@/stores/containers'
import { useNetworkStore } from '@/stores/networks'
import { useVolumeStore } from '@/stores/volumes'
import { useComposeStore } from '@/stores/compose'
import { useRegistryStore } from '@/stores/registry'
import { useTheme } from '@/composables/useTheme'
import { useI18n } from '@/composables/useI18n'
import { useUpdateStore } from '@/stores/update'
import SystemMonitor from '@/components/common/SystemMonitor.vue'
import ChangePasswordDialog from '@/components/common/ChangePasswordDialog.vue'
import SettingsDialog from '@/components/common/SettingsDialog.vue'
import AgentChatFloat from '@/components/common/AgentChatFloat.vue'
import UpdateDialog from '@/components/common/UpdateDialog.vue'
import mikuLight from '@/assets/miku-light.svg'
import mikuDark from '@/assets/miku-dark.svg'

const router = useRouter()
const authStore = useAuthStore()
const nodesStore = useNodesStore()
const containerStore = useContainerStore()
const networkStore = useNetworkStore()
const volumeStore = useVolumeStore()
const composeStore = useComposeStore()
const registryStore = useRegistryStore()
const updateStore = useUpdateStore()
const { theme, toggleTheme } = useTheme()
const { language, t, setLanguage } = useI18n()

const logoSrc = computed(() => theme.value === 'dark' ? mikuDark : mikuLight)
const windowWidth = ref(window.innerWidth)
const isMobile = computed(() => windowWidth.value < 768)
const isCollapse = ref(false)
const drawerVisible = ref(false)
const showPasswordDialog = ref(false)
const showSettingsDialog = ref(false)
const showUpdateDialog = ref(false)
const isUpdating = computed(() => updateStore.taskStatus?.status === 'running')
const updateProgressText = computed(() => {
  if (!isUpdating.value) return ''
  if (!updateStore.taskStatus?.progress_known) return '...'
  return `${updateStore.taskStatus?.progress ?? 0}%`
})
const showUpdateDot = computed(() => updateStore.shouldShowIndicator && !isUpdating.value)

const themeIcon = computed(() => theme.value === 'dark' ? Sunny : Moon)
const themeTooltip = computed(() => theme.value === 'dark' ? t('header.lightMode') : t('header.darkMode'))

const languageOptions = [
  { label: '简体中文', value: 'zh-CN' as const },
  { label: 'English', value: 'en-US' as const },
]

const menuItems = computed(() => [
  { index: '/', icon: Monitor, title: t('sideNav.dashboard') },
  { index: '/containers', icon: Box, title: t('sideNav.containers') },
  { index: '/images', icon: Picture, title: t('sideNav.images') },
  { index: '/networks', icon: Connection, title: t('sideNav.networks') },
  { index: '/volumes', icon: FolderOpened, title: t('sideNav.volumes') },
  { index: '/compose', icon: Files, title: t('sideNav.compose') },
  { index: '/nodes', icon: Cpu, title: t('sideNav.nodes') },
  { index: '/registry', icon: OfficeBuilding, title: t('sideNav.registry') },
  { index: '/docker-config', icon: Setting, title: t('sideNav.dockerConfig') },
  { index: '/settings/agent', icon: Service, title: t('sideNav.agentSettings') },
])

function handleResize() {
  windowWidth.value = window.innerWidth
  if (!isMobile.value) {
    drawerVisible.value = false
  }
}

function handleMenuSelect(index: string) {
  router.push(index)
  if (isMobile.value) {
    drawerVisible.value = false
  }
}

async function handleLogout() {
  try {
    await ElMessageBox.confirm(`${t('auth.logout')}?`, t('common.confirm'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
    await authStore.logout()
    ElMessage.success(t('auth.logoutSuccess'))
    router.push('/login')
  } catch {
    // ignore cancel
  }
}

function handleLanguageChange(lang: 'zh-CN' | 'en-US') {
  setLanguage(lang)
}

function handleChangePassword() {
  showPasswordDialog.value = true
  if (isMobile.value) {
    drawerVisible.value = false
  }
}

function handlePasswordChangeSuccess() {
  showPasswordDialog.value = false
  ElMessage.success(t('auth.passwordChanged'))
}

function handleOpenSettings() {
  showSettingsDialog.value = true
  if (isMobile.value) {
    drawerVisible.value = false
  }
}

function openGitHub() {
  window.open('https://github.com/reisen7/rabbit-panel', '_blank')
}

function toggleDrawer() {
  drawerVisible.value = !drawerVisible.value
}

const selectedNodeId = computed({
  get: () => nodesStore.currentNodeId,
  set: (id: string) => nodesStore.setCurrentNode(id),
})

const currentNodeName = computed(() => {
  return nodesStore.nodes.find(node => node.id === nodesStore.currentNodeId)?.name || t('node.master')
})

watch(() => nodesStore.currentNodeId, (id, prev) => {
  if (id === prev) return
  containerStore.prepareNodeSwitch()
  networkStore.prepareNodeSwitch()
  volumeStore.prepareNodeSwitch()
  composeStore.prepareNodeSwitch()
  registryStore.prepareNodeSwitch()
  containerStore.fetchContainers()
  networkStore.fetchNetworks()
  volumeStore.fetchVolumes()
  composeStore.fetchProjects()
  registryStore.fetchRegistries()
})

watch(() => nodesStore.fellBackToMaster, (fellBack) => {
  if (!fellBack) return
  ElMessage.warning(t('node.nodeOfflineSwitched'))
  nodesStore.acknowledgeFallback()
})

onMounted(() => {
  window.addEventListener('resize', handleResize)
  nodesStore.startPolling()
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  nodesStore.stopPolling()
})
</script>

<template>
  <el-container class="main-layout">
    <el-aside v-if="!isMobile" :width="isCollapse ? '64px' : '200px'" class="main-sidebar">
      <div class="sidebar-header">
        <img :src="logoSrc" alt="Logo" class="logo" />
        <span v-show="!isCollapse" class="logo-text">Rabbit Panel</span>
      </div>

      <el-menu :default-active="$route.path" :collapse="isCollapse" :collapse-transition="false" class="sidebar-menu" @select="handleMenuSelect">
        <el-menu-item v-for="item in menuItems" :key="item.index" :index="item.index">
          <el-icon><component :is="item.icon" /></el-icon>
          <template #title>{{ item.title }}</template>
        </el-menu-item>
      </el-menu>

      <div v-if="nodesStore.isMasterMode" class="node-switcher" :class="{ collapsed: isCollapse }">
        <template v-if="!isCollapse">
          <div class="node-switcher-label">{{ t('node.switchNode') }}</div>
          <el-select v-model="selectedNodeId" size="small">
            <el-option
              v-for="node in nodesStore.nodes"
              :key="node.id"
              :label="node.name"
              :value="node.id"
              :disabled="node.status !== 'online'"
            />
          </el-select>
        </template>
        <el-tooltip v-else :content="currentNodeName" placement="right">
          <el-icon :size="20" @click="isCollapse = false"><Cpu /></el-icon>
        </el-tooltip>
      </div>

      <div class="collapse-btn" @click="isCollapse = !isCollapse">
        <el-icon :size="20"><component :is="isCollapse ? Expand : Fold" /></el-icon>
      </div>
    </el-aside>

    <el-drawer v-model="drawerVisible" direction="ltr" :size="280" :with-header="false" class="mobile-drawer">
      <div class="drawer-header">
        <img :src="logoSrc" alt="Logo" class="logo" />
        <span class="logo-text">Rabbit Panel</span>
      </div>

      <el-menu :default-active="$route.path" class="drawer-menu" @select="handleMenuSelect">
        <el-menu-item v-for="item in menuItems" :key="item.index" :index="item.index">
          <el-icon><component :is="item.icon" /></el-icon>
          <span>{{ item.title }}</span>
        </el-menu-item>
      </el-menu>

      <div v-if="nodesStore.isMasterMode" class="drawer-node-switcher">
        <div class="node-switcher-label">{{ t('node.switchNode') }}</div>
        <el-select v-model="selectedNodeId" size="small">
          <el-option
            v-for="node in nodesStore.nodes"
            :key="node.id"
            :label="node.name"
            :value="node.id"
            :disabled="node.status !== 'online'"
          />
        </el-select>
      </div>

      <div class="drawer-footer">
        <div class="drawer-user-info">
          <el-avatar :size="40" :icon="User" />
          <span class="drawer-username">{{ authStore.username }}</span>
        </div>

        <div class="drawer-actions">
          <el-button text @click="showUpdateDialog = true">
            <el-icon><Bell /></el-icon>
            {{ t('update.title') }}
          </el-button>
          <el-button text @click="handleChangePassword">
            <el-icon><SwitchButton /></el-icon>
            {{ t('header.changePassword') }}
          </el-button>
          <el-button text type="danger" @click="handleLogout">
            <el-icon><SwitchButton /></el-icon>
            {{ t('nav.logout') }}
          </el-button>
        </div>
      </div>
    </el-drawer>

    <el-container>
      <el-header class="main-header">
        <div class="header-left">
          <el-button v-if="isMobile" :icon="Menu" class="mobile-menu-btn" @click="toggleDrawer" />
          <SystemMonitor />
          <el-select
            v-if="nodesStore.isMasterMode"
            v-model="selectedNodeId"
            class="header-node-select"
            size="small"
          >
            <el-option
              v-for="node in nodesStore.nodes"
              :key="node.id"
              :label="node.name"
              :value="node.id"
              :disabled="node.status !== 'online'"
            />
          </el-select>
          <el-tag size="small" effect="plain" class="version-tag">
            {{ updateStore.info?.current_version || 'dev' }}
          </el-tag>
        </div>

        <div class="header-right">
          <el-tooltip content="GitHub" placement="bottom">
            <el-button circle @click="openGitHub">
              <svg class="github-icon" viewBox="0 0 16 16" width="18" height="18" fill="currentColor">
                <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
              </svg>
            </el-button>
          </el-tooltip>

          <el-tooltip :content="t('update.title')" placement="bottom">
            <el-badge :is-dot="showUpdateDot" :value="updateProgressText" :hidden="!updateProgressText">
              <el-button :icon="Bell" circle :class="{ 'updating-button': isUpdating }" @click="showUpdateDialog = true" />
            </el-badge>
          </el-tooltip>

          <el-tooltip :content="themeTooltip" placement="bottom">
            <el-button :icon="themeIcon" circle @click="toggleTheme" />
          </el-tooltip>

          <el-dropdown @command="handleLanguageChange">
            <el-button circle>
              <span class="lang-icon">{{ language === 'zh-CN' ? '中' : 'En' }}</span>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item v-for="opt in languageOptions" :key="opt.value" :command="opt.value" :class="{ 'is-active': language === opt.value }">
                  {{ opt.label }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>

          <el-dropdown v-if="!isMobile">
            <div class="user-info">
              <el-avatar :size="32" :icon="User" />
              <span class="username">{{ authStore.username }}</span>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="showUpdateDialog = true">
                  <el-icon><Bell /></el-icon>
                  {{ t('update.title') }}
                </el-dropdown-item>
                <el-dropdown-item @click="handleOpenSettings">
                  <el-icon><Setting /></el-icon>
                  {{ t('settings.pageSettings') }}
                </el-dropdown-item>
                <el-dropdown-item @click="handleChangePassword">
                  <el-icon><SwitchButton /></el-icon>
                  {{ t('header.changePassword') }}
                </el-dropdown-item>
                <el-dropdown-item divided @click="handleLogout">
                  <el-icon><SwitchButton /></el-icon>
                  {{ t('nav.logout') }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <el-main class="main-content">
        <router-view />
      </el-main>
    </el-container>
  </el-container>

  <ChangePasswordDialog v-model="showPasswordDialog" @success="handlePasswordChangeSuccess" />
  <SettingsDialog v-model="showSettingsDialog" />
  <UpdateDialog v-model="showUpdateDialog" />
  <AgentChatFloat />
</template>

<style scoped>
.main-layout {
  height: 100vh;
  overflow: hidden;
}

.main-sidebar {
  background-color: var(--rp-sidebar-bg);
  border-right: 1px solid var(--rp-border-color);
  display: flex;
  flex-direction: column;
  transition: width 0.3s;
}

.sidebar-header {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 16px;
  border-bottom: 1px solid var(--rp-border-color);
}

.logo {
  width: 32px;
  height: 32px;
}

.logo-text {
  margin-left: 12px;
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  white-space: nowrap;
}

.sidebar-menu {
  flex: 1;
  border-right: none;
  overflow-y: auto;
}

.sidebar-menu:not(.el-menu--collapse) {
  width: 100%;
}

.node-switcher {
  padding: 10px 12px 6px;
  border-top: 1px solid var(--rp-border-color);
}

.node-switcher.collapsed {
  display: flex;
  justify-content: center;
  padding: 10px 0;
  cursor: pointer;
  color: var(--el-text-color-secondary);
}

.node-switcher-label {
  margin-bottom: 6px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.node-switcher :deep(.el-select),
.drawer-node-switcher :deep(.el-select) {
  width: 100%;
}

.drawer-node-switcher {
  padding: 12px 16px;
  border-top: 1px solid var(--rp-border-color);
}

.collapse-btn {
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  border-top: 1px solid var(--rp-border-color);
  color: var(--el-text-color-secondary);
  transition: color 0.3s, background-color 0.3s;
}

.collapse-btn:hover {
  color: var(--el-color-primary);
  background-color: var(--rp-hover-bg);
}

.main-header {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  background-color: var(--rp-header-bg);
  border-bottom: 1px solid var(--rp-border-color);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-node-select {
  width: 180px;
}

.version-tag {
  border-radius: 999px;
  font-weight: 600;
}

.updating-button {
  color: var(--el-color-primary);
  box-shadow: 0 0 0 1px rgba(64, 158, 255, 0.25) inset;
}

:deep(.el-badge__content.is-fixed) {
  top: 8px;
  right: 8px;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.header-right > * {
  margin: 0 !important;
}

.lang-icon {
  font-size: 12px;
  font-weight: 600;
}

.github-icon {
  display: block;
}

.user-info {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 4px;
  transition: background-color 0.3s;
}

.user-info:hover {
  background-color: var(--rp-hover-bg);
}

.username {
  font-size: 14px;
  color: var(--el-text-color-primary);
}

.main-content {
  background-color: var(--rp-bg-color);
  overflow-y: auto;
  padding: 20px;
}

:deep(.el-dropdown-menu__item.is-active) {
  color: var(--el-color-primary);
  background-color: var(--el-color-primary-light-9);
}

.mobile-menu-btn {
  margin-right: 8px;
}

.mobile-drawer :deep(.el-drawer__body) {
  padding: 0;
  display: flex;
  flex-direction: column;
}

.drawer-header {
  height: 60px;
  display: flex;
  align-items: center;
  padding: 0 20px;
  border-bottom: 1px solid var(--rp-border-color);
  background-color: var(--rp-header-bg);
}

.drawer-menu {
  flex: 1;
  border-right: none;
  overflow-y: auto;
}

.drawer-menu .el-menu-item {
  height: 50px;
  line-height: 50px;
  font-size: 15px;
}

.drawer-menu .el-menu-item .el-icon {
  font-size: 18px;
  margin-right: 12px;
}

.drawer-footer {
  padding: 16px 20px;
  border-top: 1px solid var(--rp-border-color);
  background-color: var(--rp-header-bg);
}

.drawer-user-info {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.drawer-username {
  font-size: 16px;
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.drawer-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.drawer-actions .el-button {
  justify-content: flex-start;
  width: 100%;
}

@media (max-width: 767px) {
  .main-header {
    padding: 0 12px;
  }

  .main-content {
    padding: 12px;
  }

  .header-right {
    gap: 8px;
  }
}
</style>
