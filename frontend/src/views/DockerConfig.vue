<script setup lang="ts">
import { ref, reactive, onMounted, computed, watch } from 'vue'
import {
  AlertTriangle,
  Building2,
  Check,
  FileText,
  Info,
  Link2,
  Loader2,
  Network,
  Plus,
  RefreshCw,
  RotateCw,
  Settings,
  Trash2,
} from 'lucide-vue-next'
import { useDockerConfigStore } from '@/stores/dockerConfig'
import { useRegistryStore } from '@/stores/registry'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import type { DockerConfig } from '@/types'
import { Alert, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'

const { t } = useI18n()
const toast = useToast()
const { confirm } = useConfirm()
const dockerConfigStore = useDockerConfigStore()
const registryStore = useRegistryStore()

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

const restarting = ref(false)

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
  return formData.insecureRegistries.some(
    (item, index) => index !== currentIndex && registryHost(item) === host
  )
}

function addInsecureRegistry() {
  formData.insecureRegistries.push('')
}

function removeInsecureRegistry(index: number) {
  formData.insecureRegistries.splice(index, 1)
}

async function handleRefresh() {
  await dockerConfigStore.fetchConfig()
}

async function handleSave() {
  try {
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
    toast.warning(t('dockerConfig.saveWarning'))
  } catch {
    // Error handled by store
  }
}

async function confirmRestart() {
  const ok = await confirm({
    title: t('common.warning'),
    description: t('dockerConfig.restartConfirm'),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    destructive: true,
  })
  if (!ok) return
  await handleRestart()
}

async function handleRestart() {
  try {
    restarting.value = true
    await dockerConfigStore.restartDocker()
    toast.success(t('dockerConfig.restartSuccess'))
    setTimeout(() => {
      dockerConfigStore.fetchConfig()
    }, 3000)
  } catch {
    // Error handled by store
  } finally {
    restarting.value = false
  }
}

onMounted(() => {
  dockerConfigStore.fetchConfig()
  registryStore.fetchRegistries()
})
</script>

<template>
  <div class="space-y-5">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
      <h2 class="text-xl font-semibold tracking-tight">{{ t('dockerConfig.title') }}</h2>
      <div class="flex items-center gap-2">
        <Button variant="outline" :disabled="dockerConfigStore.loading" @click="handleRefresh">
          <RefreshCw :class="{ 'animate-spin': dockerConfigStore.loading }" />
          {{ t('common.refresh') }}
        </Button>
      </div>
    </div>

    <!-- Loading skeleton -->
    <div v-if="dockerConfigStore.loading && !dockerConfigStore.info" class="space-y-4">
      <Skeleton class="h-44 w-full rounded-xl" />
      <Skeleton class="h-36 w-full rounded-xl" />
      <Skeleton class="h-36 w-full rounded-xl" />
    </div>

    <template v-else>
      <Alert v-if="dockerConfigStore.error" variant="warning">
        <AlertTriangle class="size-4" />
        <AlertTitle>{{ dockerConfigStore.error }}</AlertTitle>
      </Alert>

      <!-- Docker Info Card -->
      <Card>
        <CardHeader>
          <CardTitle class="flex items-center gap-2 text-base">
            <Info class="size-4 text-muted-foreground" />
            <span>{{ t('dockerConfig.info') }}</span>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div class="grid grid-cols-1 md:grid-cols-2 rounded-lg border divide-y md:divide-y-0 md:divide-x text-sm overflow-hidden">
            <div class="p-3 border-b space-y-1">
              <div class="text-xs text-muted-foreground">{{ t('dockerConfig.version') }}</div>
              <div class="font-mono text-xs">{{ dockerConfigStore.info?.serverVersion || '-' }}</div>
            </div>
            <div class="p-3 border-b space-y-1">
              <div class="text-xs text-muted-foreground">{{ t('dockerConfig.apiVersion') }}</div>
              <div class="font-mono text-xs">{{ dockerConfigStore.info?.apiVersion || '-' }}</div>
            </div>
            <div class="p-3 border-b space-y-1">
              <div class="text-xs text-muted-foreground">{{ t('dockerConfig.os') }}</div>
              <div>{{ dockerConfigStore.info?.operatingSystem || '-' }}</div>
            </div>
            <div class="p-3 border-b space-y-1">
              <div class="text-xs text-muted-foreground">{{ t('dockerConfig.arch') }}</div>
              <div class="font-mono text-xs">{{ dockerConfigStore.info?.arch || '-' }}</div>
            </div>
            <div class="p-3 border-b space-y-1">
              <div class="text-xs text-muted-foreground">{{ t('dockerConfig.kernel') }}</div>
              <div class="font-mono text-xs">{{ dockerConfigStore.info?.kernelVersion || '-' }}</div>
            </div>
            <div class="p-3 border-b space-y-1">
              <div class="text-xs text-muted-foreground">{{ t('dockerConfig.storageDriver') }}</div>
              <div class="font-mono text-xs">{{ dockerConfigStore.info?.driver || '-' }}</div>
            </div>
            <div class="p-3 md:col-span-2 space-y-1">
              <div class="text-xs text-muted-foreground">{{ t('dockerConfig.rootDir') }}</div>
              <div class="font-mono text-xs break-all">{{ dockerConfigStore.info?.dockerRootDir || '-' }}</div>
            </div>
          </div>
        </CardContent>
      </Card>

      <!-- Registry Mirrors -->
      <Card>
        <CardHeader>
          <CardTitle class="flex items-center gap-2 text-base">
            <Link2 class="size-4 text-muted-foreground" />
            <span>{{ t('dockerConfig.registryMirrors') }}</span>
          </CardTitle>
        </CardHeader>
        <CardContent class="space-y-3">
          <p class="text-xs text-muted-foreground">{{ t('dockerConfig.registryMirrorsHelp') }}</p>
          <div class="space-y-2.5">
            <div
              v-for="(_mirror, index) in formData.registryMirrors"
              :key="index"
              class="flex items-center gap-2"
            >
              <Input
                v-model="formData.registryMirrors[index]"
                :placeholder="t('dockerConfig.mirrorPlaceholder')"
                class="flex-1 font-mono text-xs"
              />
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                class="text-destructive shrink-0"
                @click="removeMirror(index)"
              >
                <Trash2 class="size-4" />
              </Button>
            </div>
          </div>
          <Button type="button" variant="outline" size="sm" @click="addMirror">
            <Plus />
            {{ t('dockerConfig.addMirror') }}
          </Button>
        </CardContent>
      </Card>

      <!-- Insecure Registries -->
      <Card>
        <CardHeader>
          <CardTitle class="flex items-center gap-2 text-base">
            <Building2 class="size-4 text-muted-foreground" />
            <span>{{ t('dockerConfig.insecureRegistries') }}</span>
          </CardTitle>
        </CardHeader>
        <CardContent class="space-y-3">
          <p class="text-xs text-muted-foreground">{{ t('dockerConfig.insecureRegistriesHelp') }}</p>
          <div class="space-y-2.5">
            <div
              v-for="(_registry, index) in formData.insecureRegistries"
              :key="index"
              class="flex items-center gap-2"
            >
              <Select v-model="formData.insecureRegistries[index]">
                <SelectTrigger class="flex-1 font-mono text-xs">
                  <SelectValue :placeholder="t('dockerConfig.registrySelectPlaceholder')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem
                    v-for="option in registryOptions"
                    :key="option.value"
                    :value="option.value"
                    :disabled="isRegistrySelected(option.value, index)"
                  >
                    {{ option.label }}
                  </SelectItem>
                </SelectContent>
              </Select>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                class="text-destructive shrink-0"
                @click="removeInsecureRegistry(index)"
              >
                <Trash2 class="size-4" />
              </Button>
            </div>
          </div>
          <Button type="button" variant="outline" size="sm" @click="addInsecureRegistry">
            <Plus />
            {{ t('dockerConfig.addRegistry') }}
          </Button>
        </CardContent>
      </Card>

      <!-- Network Configuration -->
      <Card>
        <CardHeader>
          <CardTitle class="flex items-center gap-2 text-base">
            <Network class="size-4 text-muted-foreground" />
            <span>{{ t('dockerConfig.ipv6') }} & {{ t('dockerConfig.iptables') }}</span>
          </CardTitle>
        </CardHeader>
        <CardContent class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div class="flex items-center justify-between gap-4 rounded-lg border p-3.5">
            <div class="space-y-0.5">
              <Label>{{ t('dockerConfig.ipv6') }}</Label>
              <p class="text-xs text-muted-foreground">{{ t('dockerConfig.ipv6Help') }}</p>
            </div>
            <Switch v-model="formData.ipv6" />
          </div>

          <div class="flex items-center justify-between gap-4 rounded-lg border p-3.5">
            <div class="space-y-0.5">
              <Label>{{ t('dockerConfig.iptables') }}</Label>
              <p class="text-xs text-muted-foreground">{{ t('dockerConfig.iptablesHelp') }}</p>
            </div>
            <Switch v-model="formData.iptables" />
          </div>
        </CardContent>
      </Card>

      <!-- Log Configuration -->
      <Card>
        <CardHeader>
          <CardTitle class="flex items-center gap-2 text-base">
            <FileText class="size-4 text-muted-foreground" />
            <span>{{ t('dockerConfig.logDriver') }}</span>
          </CardTitle>
        </CardHeader>
        <CardContent class="space-y-4">
          <p class="text-xs text-muted-foreground">{{ t('dockerConfig.logDriverHelp') }}</p>
          <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div class="space-y-1.5">
              <Label>{{ t('dockerConfig.logDriver') }}</Label>
              <Select v-model="formData.logDriver">
                <SelectTrigger class="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="json-file">json-file</SelectItem>
                  <SelectItem value="local">local</SelectItem>
                  <SelectItem value="journald">journald</SelectItem>
                  <SelectItem value="syslog">syslog</SelectItem>
                  <SelectItem value="none">none</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div class="space-y-1.5">
              <Label>{{ t('dockerConfig.maxSize') }}</Label>
              <Input v-model="formData.logOpts['max-size']" placeholder="10m" class="font-mono" />
            </div>

            <div class="space-y-1.5">
              <Label>{{ t('dockerConfig.maxFile') }}</Label>
              <Input v-model="formData.logOpts['max-file']" placeholder="3" class="font-mono" />
            </div>
          </div>
        </CardContent>
      </Card>

      <!-- Runtime Configuration -->
      <Card>
        <CardHeader>
          <CardTitle class="flex items-center gap-2 text-base">
            <Settings class="size-4 text-muted-foreground" />
            <span>{{ t('dockerConfig.liveRestore') }} & {{ t('dockerConfig.cgroupDriver') }}</span>
          </CardTitle>
        </CardHeader>
        <CardContent class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div class="flex items-center justify-between gap-4 rounded-lg border p-3.5">
            <div class="space-y-0.5">
              <Label>{{ t('dockerConfig.liveRestore') }}</Label>
              <p class="text-xs text-muted-foreground">{{ t('dockerConfig.liveRestoreHelp') }}</p>
            </div>
            <Switch v-model="formData.liveRestore" />
          </div>

          <div class="rounded-lg border p-3.5 space-y-2">
            <Label>{{ t('dockerConfig.cgroupDriver') }}</Label>
            <div class="inline-flex rounded-md border p-0.5 bg-muted/40">
              <button
                type="button"
                class="px-3 py-1 rounded text-xs font-medium transition-colors cursor-pointer"
                :class="formData.cgroupDriver === 'cgroupfs'
                  ? 'bg-background text-foreground shadow-2xs'
                  : 'text-muted-foreground hover:text-foreground'"
                @click="formData.cgroupDriver = 'cgroupfs'"
              >
                cgroupfs
              </button>
              <button
                type="button"
                class="px-3 py-1 rounded text-xs font-medium transition-colors cursor-pointer"
                :class="formData.cgroupDriver === 'systemd'
                  ? 'bg-background text-foreground shadow-2xs'
                  : 'text-muted-foreground hover:text-foreground'"
                @click="formData.cgroupDriver = 'systemd'"
              >
                systemd
              </button>
            </div>
            <p class="text-xs text-muted-foreground">{{ t('dockerConfig.cgroupDriverHelp') }}</p>
          </div>
        </CardContent>
      </Card>

      <!-- Action Buttons -->
      <div class="flex flex-wrap justify-end gap-3 pt-1">
        <Button size="lg" :disabled="dockerConfigStore.saving" @click="handleSave">
          <Loader2 v-if="dockerConfigStore.saving" class="animate-spin" />
          <Check v-else />
          {{ t('dockerConfig.save') }}
        </Button>
        <Button
          size="lg"
          variant="outline"
          class="text-amber-600 dark:text-amber-400"
          :disabled="restarting"
          @click="confirmRestart"
        >
          <Loader2 v-if="restarting" class="animate-spin" />
          <RotateCw v-else />
          {{ restarting ? t('dockerConfig.restarting') : t('dockerConfig.restart') }}
        </Button>
      </div>
    </template>
  </div>
</template>
