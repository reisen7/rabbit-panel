<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Bell,
  Bot,
  Box,
  ChevronsLeft,
  ChevronsRight,
  Cpu,
  HardDrive,
  Image as ImageIcon,
  KeyRound,
  Layers,
  LogOut,
  Menu,
  MessageSquare,
  Monitor,
  Moon,
  Building2,
  Network,
  PackageSearch,
  Settings,
  Sun,
  User,
} from 'lucide-vue-next'
import { useAuthStore } from '@/stores/auth'
import { useNodesStore } from '@/stores/nodes'
import { useContainerStore } from '@/stores/containers'
import { useNetworkStore } from '@/stores/networks'
import { useVolumeStore } from '@/stores/volumes'
import { useComposeStore } from '@/stores/compose'
import { useRegistryStore } from '@/stores/registry'
import { useTheme } from '@/composables/useTheme'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import { useUpdateStore } from '@/stores/update'
import { useFloatWindowsStore } from '@/stores/floatWindows'
import SystemMonitor from '@/components/common/SystemMonitor.vue'
import ChangePasswordDialog from '@/components/common/ChangePasswordDialog.vue'
import SettingsDialog from '@/components/common/SettingsDialog.vue'
import AgentChatFloat from '@/components/common/AgentChatFloat.vue'
import ImageSearchFloat from '@/components/common/ImageSearchFloat.vue'
import UpdateDialog from '@/components/common/UpdateDialog.vue'
import ContainerCreateTaskFloat from '@/components/container/ContainerCreateTaskFloat.vue'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
} from '@/components/ui/sheet'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import mikuLight from '@/assets/miku-light.svg'
import mikuDark from '@/assets/miku-dark.svg'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const nodesStore = useNodesStore()
const containerStore = useContainerStore()
const networkStore = useNetworkStore()
const volumeStore = useVolumeStore()
const composeStore = useComposeStore()
const registryStore = useRegistryStore()
const updateStore = useUpdateStore()
const floatWindows = useFloatWindowsStore()
const { theme, toggleTheme } = useTheme()
const { language, t, setLanguage } = useI18n()
const toast = useToast()
const { confirm } = useConfirm()

const logoSrc = computed(() => (theme.value === 'dark' ? mikuDark : mikuLight))
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

const themeTooltip = computed(() =>
  theme.value === 'dark' ? t('header.lightMode') : t('header.darkMode')
)

const languageOptions = [
  { label: '简体中文', value: 'zh-CN' as const },
  { label: 'English', value: 'en-US' as const },
]

const menuItems = computed(() => [
  { index: '/', icon: Monitor, title: t('sideNav.dashboard') },
  { index: '/containers', icon: Box, title: t('sideNav.containers') },
  { index: '/images', icon: ImageIcon, title: t('sideNav.images') },
  { index: '/networks', icon: Network, title: t('sideNav.networks') },
  { index: '/volumes', icon: HardDrive, title: t('sideNav.volumes') },
  { index: '/compose', icon: Layers, title: t('sideNav.compose') },
  { index: '/nodes', icon: Cpu, title: t('sideNav.nodes') },
  { index: '/registry', icon: Building2, title: t('sideNav.registry') },
  { index: '/docker-config', icon: Settings, title: t('sideNav.dockerConfig') },
  { index: '/settings/agent', icon: Bot, title: t('sideNav.agentSettings') },
])

function isActiveMenu(index: string) {
  return index === '/' ? route.path === '/' : route.path.startsWith(index)
}

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

function toggleImageSearchFloat() {
  floatWindows.active = floatWindows.active === 'imageSearch' ? null : 'imageSearch'
}

function handleImageSearchFromDrawer() {
  toggleImageSearchFloat()
  drawerVisible.value = false
}

async function handleLogout() {
  const ok = await confirm({
    title: t('common.confirm'),
    description: `${t('auth.logout')}?`,
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
  })
  if (!ok) return
  await authStore.logout()
  toast.success(t('auth.logoutSuccess'))
  router.push('/login')
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
  toast.success(t('auth.passwordChanged'))
}

function handleOpenNotify() {
  router.push('/settings/notify')
  if (isMobile.value) {
    drawerVisible.value = false
  }
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
  return nodesStore.nodes.find((node) => node.id === nodesStore.currentNodeId)?.name || t('node.master')
})

watch(
  () => nodesStore.currentNodeId,
  (id, prev) => {
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
  }
)

watch(
  () => nodesStore.fellBackToMaster,
  (fellBack) => {
    if (!fellBack) return
    toast.warning(t('node.nodeOfflineSwitched'))
    nodesStore.acknowledgeFallback()
  }
)

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
  <TooltipProvider>
    <div class="h-screen flex overflow-hidden bg-background text-foreground">
      <!-- Desktop Sidebar -->
      <aside
        v-if="!isMobile"
        class="shrink-0 bg-sidebar text-sidebar-foreground border-r border-sidebar-border flex flex-col transition-[width] duration-200"
        :class="isCollapse ? 'w-16' : 'w-[var(--rp-sidebar-width,220px)]'"
      >
        <div class="h-14 flex items-center justify-center px-4 border-b border-sidebar-border shrink-0">
          <img :src="logoSrc" alt="Logo" class="size-7 shrink-0" />
          <span v-show="!isCollapse" class="ml-2.5 text-sm font-semibold tracking-tight whitespace-nowrap">
            Rabbit Panel
          </span>
        </div>

        <nav class="flex-1 overflow-y-auto p-2 space-y-1">
          <template v-for="item in menuItems" :key="item.index">
            <Tooltip v-if="isCollapse">
              <TooltipTrigger as-child>
                <button
                  type="button"
                  class="w-full h-9 flex items-center justify-center rounded-md transition-colors cursor-pointer"
                  :class="isActiveMenu(item.index)
                    ? 'bg-sidebar-accent text-sidebar-accent-foreground font-medium shadow-2xs'
                    : 'text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-foreground'"
                  @click="handleMenuSelect(item.index)"
                >
                  <component :is="item.icon" class="size-4 shrink-0" />
                </button>
              </TooltipTrigger>
              <TooltipContent side="right">{{ item.title }}</TooltipContent>
            </Tooltip>
            <button
              v-else
              type="button"
              class="w-full h-9 px-3 flex items-center gap-2.5 rounded-md text-sm whitespace-nowrap transition-colors cursor-pointer"
              :class="isActiveMenu(item.index)
                ? 'bg-sidebar-accent text-sidebar-accent-foreground font-medium shadow-2xs'
                : 'text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-foreground'"
              @click="handleMenuSelect(item.index)"
            >
              <component :is="item.icon" class="size-4 shrink-0" />
              <span class="truncate">{{ item.title }}</span>
            </button>
          </template>
        </nav>

        <div class="border-t border-sidebar-border" :class="isCollapse ? 'flex justify-center py-2' : 'p-2'">
          <Tooltip v-if="isCollapse">
            <TooltipTrigger as-child>
              <button
                type="button"
                class="w-full h-9 flex items-center justify-center rounded-md transition-colors cursor-pointer"
                :class="floatWindows.active === 'imageSearch'
                  ? 'bg-sidebar-accent text-sidebar-accent-foreground font-medium shadow-2xs'
                  : 'text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-foreground'"
                @click="toggleImageSearchFloat"
              >
                <PackageSearch class="size-4 shrink-0" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="right">{{ t('imageSearch.title') }}</TooltipContent>
          </Tooltip>
          <button
            v-else
            type="button"
            class="w-full h-9 px-3 flex items-center gap-2.5 rounded-md text-sm whitespace-nowrap transition-colors cursor-pointer"
            :class="floatWindows.active === 'imageSearch'
              ? 'bg-sidebar-accent text-sidebar-accent-foreground font-medium shadow-2xs'
              : 'text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-foreground'"
            @click="toggleImageSearchFloat"
          >
            <PackageSearch class="size-4 shrink-0" />
            <span class="truncate">{{ t('imageSearch.title') }}</span>
          </button>
        </div>

        <div
          v-if="nodesStore.isMasterMode"
          class="border-t border-sidebar-border"
          :class="isCollapse ? 'flex justify-center py-2.5' : 'p-3 space-y-1.5'"
        >
          <template v-if="!isCollapse">
            <div class="text-[11px] font-medium text-muted-foreground">{{ t('node.switchNode') }}</div>
            <Select v-model="selectedNodeId">
              <SelectTrigger size="sm" class="w-full bg-background">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem
                  v-for="node in nodesStore.nodes"
                  :key="node.id"
                  :value="node.id"
                  :disabled="node.status !== 'online'"
                >
                  {{ node.name }}
                </SelectItem>
              </SelectContent>
            </Select>
          </template>
          <Tooltip v-else>
            <TooltipTrigger as-child>
              <button
                type="button"
                class="p-1.5 rounded-md text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-foreground transition-colors"
                @click="isCollapse = false"
              >
                <Cpu class="size-4" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="right">{{ currentNodeName }}</TooltipContent>
          </Tooltip>
        </div>

        <button
          type="button"
          class="h-11 flex items-center justify-center border-t border-sidebar-border text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-foreground transition-colors cursor-pointer"
          @click="isCollapse = !isCollapse"
        >
          <ChevronsRight v-if="isCollapse" class="size-4" />
          <ChevronsLeft v-else class="size-4" />
        </button>
      </aside>

      <!-- Mobile Drawer -->
      <Sheet v-model:open="drawerVisible">
        <SheetContent side="left" class="w-[280px] p-0 flex flex-col bg-sidebar text-sidebar-foreground">
          <div class="h-14 flex items-center px-5 border-b border-sidebar-border shrink-0">
            <img :src="logoSrc" alt="Logo" class="size-7" />
            <span class="ml-2.5 text-base font-semibold tracking-tight">Rabbit Panel</span>
          </div>

          <nav class="flex-1 overflow-y-auto p-2.5 space-y-1">
            <button
              v-for="item in menuItems"
              :key="item.index"
              type="button"
              class="w-full h-10 px-3 flex items-center gap-3 rounded-md text-sm transition-colors cursor-pointer"
              :class="isActiveMenu(item.index)
                ? 'bg-sidebar-accent text-sidebar-accent-foreground font-medium'
                : 'text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-foreground'"
              @click="handleMenuSelect(item.index)"
            >
              <component :is="item.icon" class="size-4 shrink-0" />
              <span>{{ item.title }}</span>
            </button>
          </nav>

          <div class="px-2.5 py-2 border-t border-sidebar-border">
            <button
              type="button"
              class="w-full h-10 px-3 flex items-center gap-3 rounded-md text-sm transition-colors cursor-pointer"
              :class="floatWindows.active === 'imageSearch'
                ? 'bg-sidebar-accent text-sidebar-accent-foreground font-medium'
                : 'text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-foreground'"
              @click="handleImageSearchFromDrawer"
            >
              <PackageSearch class="size-4 shrink-0" />
              <span>{{ t('imageSearch.title') }}</span>
            </button>
          </div>

          <div v-if="nodesStore.isMasterMode" class="px-4 py-3 border-t border-sidebar-border space-y-1.5">
            <div class="text-xs text-muted-foreground">{{ t('node.switchNode') }}</div>
            <Select v-model="selectedNodeId">
              <SelectTrigger size="sm" class="w-full bg-background">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem
                  v-for="node in nodesStore.nodes"
                  :key="node.id"
                  :value="node.id"
                  :disabled="node.status !== 'online'"
                >
                  {{ node.name }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div class="p-4 border-t border-sidebar-border space-y-3">
            <div class="flex items-center gap-3">
              <Avatar class="size-9">
                <AvatarFallback><User class="size-4" /></AvatarFallback>
              </Avatar>
              <span class="text-sm font-medium">{{ authStore.username }}</span>
            </div>

            <div class="flex flex-col gap-1">
              <Button variant="ghost" size="sm" class="justify-start" @click="handleOpenNotify">
                <MessageSquare />
                {{ t('notify.title') }}
              </Button>
              <Button variant="ghost" size="sm" class="justify-start" @click="showUpdateDialog = true">
                <Bell />
                {{ t('update.title') }}
              </Button>
              <Button variant="ghost" size="sm" class="justify-start" @click="handleChangePassword">
                <KeyRound />
                {{ t('header.changePassword') }}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                class="justify-start text-destructive hover:text-destructive"
                @click="handleLogout"
              >
                <LogOut />
                {{ t('nav.logout') }}
              </Button>
            </div>
          </div>
        </SheetContent>
      </Sheet>

      <!-- Main Body -->
      <div class="flex-1 flex flex-col min-w-0">
        <header class="h-14 shrink-0 flex items-center justify-between px-3 md:px-6 border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
          <div class="flex items-center gap-2.5 md:gap-3 min-w-0">
            <Button v-if="isMobile" variant="outline" size="icon-sm" @click="toggleDrawer">
              <Menu />
            </Button>
            <SystemMonitor />
            <Select v-if="nodesStore.isMasterMode" v-model="selectedNodeId">
              <SelectTrigger size="sm" class="hidden sm:flex w-[160px] md:w-[180px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem
                  v-for="node in nodesStore.nodes"
                  :key="node.id"
                  :value="node.id"
                  :disabled="node.status !== 'online'"
                >
                  {{ node.name }}
                </SelectItem>
              </SelectContent>
            </Select>
            <Badge variant="outline" class="rounded-full font-mono text-xs">
              {{ updateStore.info?.current_version || 'dev' }}
            </Badge>
          </div>

          <div class="flex items-center gap-1.5 md:gap-2 shrink-0">
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="outline" size="icon-sm" class="rounded-full" @click="openGitHub">
                  <svg class="size-4" viewBox="0 0 16 16" fill="currentColor">
                    <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
                  </svg>
                </Button>
              </TooltipTrigger>
              <TooltipContent>GitHub</TooltipContent>
            </Tooltip>

            <Tooltip>
              <TooltipTrigger as-child>
                <span class="relative inline-flex">
                  <Button
                    variant="outline"
                    size="icon-sm"
                    class="rounded-full"
                    :class="{ 'ring-2 ring-primary/40 text-primary': isUpdating }"
                    @click="showUpdateDialog = true"
                  >
                    <Bell />
                  </Button>
                  <span
                    v-if="updateProgressText"
                    class="absolute -top-1.5 -right-2 px-1 text-[10px] leading-3.5 rounded-full bg-primary text-primary-foreground font-mono"
                  >
                    {{ updateProgressText }}
                  </span>
                  <span
                    v-else-if="showUpdateDot"
                    class="absolute top-0 right-0 size-2 rounded-full bg-destructive ring-2 ring-background"
                  />
                </span>
              </TooltipTrigger>
              <TooltipContent>{{ t('update.title') }}</TooltipContent>
            </Tooltip>

            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="outline" size="icon-sm" class="rounded-full" @click="toggleTheme">
                  <Sun v-if="theme === 'dark'" />
                  <Moon v-else />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ themeTooltip }}</TooltipContent>
            </Tooltip>

            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button variant="outline" size="icon-sm" class="rounded-full">
                  <span class="text-xs font-semibold">{{ language === 'zh-CN' ? '中' : 'En' }}</span>
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem
                  v-for="opt in languageOptions"
                  :key="opt.value"
                  :class="{ 'font-semibold text-primary': language === opt.value }"
                  @click="handleLanguageChange(opt.value)"
                >
                  {{ opt.label }}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>

            <DropdownMenu v-if="!isMobile">
              <DropdownMenuTrigger as-child>
                <button
                  type="button"
                  class="flex items-center gap-2 rounded-md px-2 py-1 hover:bg-accent transition-colors cursor-pointer"
                >
                  <Avatar class="size-7">
                    <AvatarFallback><User class="size-3.5" /></AvatarFallback>
                  </Avatar>
                  <span class="text-sm font-medium">{{ authStore.username }}</span>
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" class="w-48">
                <DropdownMenuItem @click="showUpdateDialog = true">
                  <Bell />
                  {{ t('update.title') }}
                </DropdownMenuItem>
                <DropdownMenuItem @click="handleOpenSettings">
                  <Settings />
                  {{ t('settings.pageSettings') }}
                </DropdownMenuItem>
                <DropdownMenuItem @click="handleOpenNotify">
                  <MessageSquare />
                  {{ t('notify.title') }}
                </DropdownMenuItem>
                <DropdownMenuItem @click="handleChangePassword">
                  <KeyRound />
                  {{ t('header.changePassword') }}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem class="text-destructive focus:text-destructive" @click="handleLogout">
                  <LogOut />
                  {{ t('nav.logout') }}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </header>

        <main class="flex-1 overflow-y-auto bg-muted/20 p-4 md:p-6">
          <router-view />
        </main>
      </div>
    </div>
  </TooltipProvider>

  <ChangePasswordDialog v-model="showPasswordDialog" @success="handlePasswordChangeSuccess" />
  <SettingsDialog v-model="showSettingsDialog" />
  <UpdateDialog v-model="showUpdateDialog" />
  <AgentChatFloat />
  <ImageSearchFloat />
  <ContainerCreateTaskFloat />
</template>
