<script setup lang="ts">
import { ref, computed, reactive, watch } from 'vue'
import { Check, Edit3, X } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { containerApi } from '@/api/containers'
import type { ContainerConfig } from '@/types'
import RecreateContainerDialog from './RecreateContainerDialog.vue'
import LoadingOverlay from '@/components/common/LoadingOverlay.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/components/ui/tabs'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

const props = defineProps<{
  visible: boolean
  containerId: string
  containerName: string
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
  updated: []
}>()

const { t } = useI18n()
const toast = useToast()

const dialogVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
})

const loading = ref(false)
const activeTab = ref('basic')
const config = ref<ContainerConfig | null>(null)
const editingName = ref(false)
const newName = ref('')
const showRecreateDialog = ref(false)

const resourceForm = reactive({
  memory: 0,
  cpus: 0,
  restart: 'no',
})

async function loadConfig() {
  if (!props.containerId) return

  loading.value = true
  try {
    config.value = await containerApi.inspect(props.containerId)
    resourceForm.memory = config.value.memory || 0
    resourceForm.cpus = config.value.cpus || 0
    resourceForm.restart = config.value.restart || 'no'
  } catch {
    toast.error(t('config.loadError'))
    dialogVisible.value = false
  } finally {
    loading.value = false
  }
}

function startEditName() {
  newName.value = config.value?.name || ''
  editingName.value = true
}

function cancelEditName() {
  editingName.value = false
  newName.value = ''
}

async function handleRename() {
  if (!newName.value.trim()) {
    toast.warning(t('config.nameRequired'))
    return
  }

  try {
    await containerApi.rename(props.containerId, newName.value.trim())
    toast.success(t('config.renameSuccess'))
    if (config.value) {
      config.value.name = newName.value.trim()
    }
    editingName.value = false
    emit('updated')
  } catch {
    // Error handled by interceptor
  }
}

async function updateMemory() {
  try {
    await containerApi.update(props.containerId, { memory: resourceForm.memory })
    toast.success(t('config.updateSuccess'))
    emit('updated')
  } catch {
    // Error handled by interceptor
  }
}

async function updateCpus() {
  try {
    await containerApi.update(props.containerId, { cpus: resourceForm.cpus })
    toast.success(t('config.updateSuccess'))
    emit('updated')
  } catch {
    // Error handled by interceptor
  }
}

async function updateRestart() {
  try {
    await containerApi.update(props.containerId, { restart: resourceForm.restart })
    toast.success(t('config.updateSuccess'))
    emit('updated')
  } catch {
    // Error handled by interceptor
  }
}

function handleRecreated() {
  emit('updated')
  dialogVisible.value = false
}

function formatTime(time?: string): string {
  if (!time) return '-'
  return new Date(time).toLocaleString()
}

function handleOpen() {
  activeTab.value = 'basic'
  editingName.value = false
  loadConfig()
}

watch(
  () => props.visible,
  (val) => {
    if (val) {
      handleOpen()
    }
  },
  { immediate: true }
)
</script>

<template>
  <Dialog v-model:open="dialogVisible">
    <DialogContent class="sm:max-w-[760px] max-h-[88vh] flex flex-col" @interact-outside.prevent>
      <DialogHeader>
        <DialogTitle>{{ t('container.config') + ' - ' + containerName }}</DialogTitle>
      </DialogHeader>

      <TooltipProvider>
        <div class="relative flex-1 overflow-y-auto min-h-[420px] pr-1">
          <LoadingOverlay :loading="loading" />

          <Tabs v-model="activeTab" class="w-full">
            <TabsList class="w-full flex flex-wrap h-auto justify-start gap-1">
              <TabsTrigger value="basic">{{ t('config.basicInfo') }}</TabsTrigger>
              <TabsTrigger value="network">{{ t('config.network') }}</TabsTrigger>
              <TabsTrigger value="storage">{{ t('config.storage') }}</TabsTrigger>
              <TabsTrigger value="env">{{ t('container.envVars') }}</TabsTrigger>
              <TabsTrigger value="resources">{{ t('config.resources') }}</TabsTrigger>
              <TabsTrigger value="security">{{ t('config.security') }}</TabsTrigger>
              <TabsTrigger value="command">{{ t('config.command') }}</TabsTrigger>
            </TabsList>

            <!-- Basic Info Tab -->
            <TabsContent value="basic" class="pt-3">
              <div class="grid grid-cols-1 sm:grid-cols-2 rounded-lg border divide-y sm:divide-y-0 sm:divide-x text-sm overflow-hidden">
                <div class="p-3 border-b space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('config.containerId') }}</div>
                  <div class="font-mono text-xs">{{ config?.id || '-' }}</div>
                </div>
                <div class="p-3 border-b space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('config.fullId') }}</div>
                  <Tooltip>
                    <TooltipTrigger as-child>
                      <div class="font-mono text-xs truncate max-w-[280px]">{{ config?.fullId || '-' }}</div>
                    </TooltipTrigger>
                    <TooltipContent class="max-w-md break-all font-mono text-xs">
                      {{ config?.fullId }}
                    </TooltipContent>
                  </Tooltip>
                </div>
                <div class="p-3 border-b space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('common.name') }}</div>
                  <div class="flex items-center gap-2">
                    <span v-if="!editingName" class="font-medium">{{ config?.name || '-' }}</span>
                    <Input v-else v-model="newName" class="h-7 w-44 text-sm" />
                    <Button v-if="!editingName" variant="ghost" size="icon-sm" @click="startEditName">
                      <Edit3 class="size-3.5" />
                    </Button>
                    <template v-else>
                      <Button variant="ghost" size="icon-sm" class="text-emerald-600" @click="handleRename">
                        <Check class="size-3.5" />
                      </Button>
                      <Button variant="ghost" size="icon-sm" @click="cancelEditName">
                        <X class="size-3.5" />
                      </Button>
                    </template>
                  </div>
                </div>
                <div class="p-3 border-b space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('container.image') }}</div>
                  <div class="font-mono text-xs break-all">{{ config?.image || '-' }}</div>
                </div>
                <div class="p-3 border-b space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('common.status') }}</div>
                  <div>
                    <Badge :variant="config?.running ? 'success' : 'destructive'">
                      {{ config?.state || '-' }}
                    </Badge>
                  </div>
                </div>
                <div class="p-3 border-b space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('config.platform') }}</div>
                  <div>{{ config?.platform || '-' }}</div>
                </div>
                <div class="p-3 border-b sm:border-b-0 space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('config.created') }}</div>
                  <div class="text-xs tabular-nums">{{ formatTime(config?.created) }}</div>
                </div>
                <div class="p-3 space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('config.started') }}</div>
                  <div class="text-xs tabular-nums">{{ formatTime(config?.started) }}</div>
                </div>
              </div>
            </TabsContent>

            <!-- Network Tab -->
            <TabsContent value="network" class="pt-3 space-y-4">
              <div class="grid grid-cols-1 sm:grid-cols-2 rounded-lg border divide-y sm:divide-y-0 sm:divide-x text-sm overflow-hidden">
                <div class="p-3 border-b space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('config.networkMode') }}</div>
                  <div class="font-mono text-xs">{{ config?.networkMode || '-' }}</div>
                </div>
                <div class="p-3 border-b space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('config.hostname') }}</div>
                  <div class="font-mono text-xs">{{ config?.hostname || '-' }}</div>
                </div>
                <div class="p-3 border-b space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('config.ipAddress') }}</div>
                  <div class="font-mono text-xs">{{ config?.ipAddress || '-' }}</div>
                </div>
                <div class="p-3 border-b space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('config.gateway') }}</div>
                  <div class="font-mono text-xs">{{ config?.gateway || '-' }}</div>
                </div>
                <div class="p-3 sm:col-span-2 space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('config.macAddress') }}</div>
                  <div class="font-mono text-xs">{{ config?.macAddress || '-' }}</div>
                </div>
              </div>

              <div class="space-y-2">
                <h4 class="text-sm font-semibold">{{ t('container.ports') }}</h4>
                <div class="rounded-md border">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{{ t('config.hostIP') }}</TableHead>
                        <TableHead>{{ t('container.hostPort') }}</TableHead>
                        <TableHead>{{ t('container.containerPort') }}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      <TableRow v-if="!config?.ports?.length">
                        <TableCell colspan="3" class="text-center text-muted-foreground py-6">
                          {{ t('common.none') }}
                        </TableCell>
                      </TableRow>
                      <TableRow v-for="(port, idx) in config?.ports || []" :key="idx">
                        <TableCell class="font-mono text-xs">{{ port.hostIP || '-' }}</TableCell>
                        <TableCell class="font-mono text-xs">{{ port.host }}</TableCell>
                        <TableCell class="font-mono text-xs">{{ port.container }}</TableCell>
                      </TableRow>
                    </TableBody>
                  </Table>
                </div>
              </div>
            </TabsContent>

            <!-- Storage Tab -->
            <TabsContent value="storage" class="pt-3 space-y-4">
              <div class="space-y-2">
                <h4 class="text-sm font-semibold">{{ t('container.volumes') }}</h4>
                <div class="rounded-md border">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{{ t('container.hostPath') }}</TableHead>
                        <TableHead>{{ t('container.containerPath') }}</TableHead>
                        <TableHead class="w-24">{{ t('config.mode') }}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      <TableRow v-if="!config?.volumes?.length">
                        <TableCell colspan="3" class="text-center text-muted-foreground py-6">
                          {{ t('common.none') }}
                        </TableCell>
                      </TableRow>
                      <TableRow v-for="(vol, idx) in config?.volumes || []" :key="idx">
                        <TableCell class="font-mono text-xs break-all">{{ vol.host }}</TableCell>
                        <TableCell class="font-mono text-xs break-all">{{ vol.container }}</TableCell>
                        <TableCell class="font-mono text-xs">{{ vol.mode }}</TableCell>
                      </TableRow>
                    </TableBody>
                  </Table>
                </div>
              </div>

              <div class="grid grid-cols-1 sm:grid-cols-2 rounded-lg border divide-y sm:divide-y-0 sm:divide-x text-sm overflow-hidden">
                <div class="p-3 space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('config.workingDir') }}</div>
                  <div class="font-mono text-xs break-all">{{ config?.workingDir || '-' }}</div>
                </div>
                <div class="p-3 space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('config.readOnly') }}</div>
                  <div>{{ config?.readOnly ? t('common.yes') : t('common.no') }}</div>
                </div>
              </div>
            </TabsContent>

            <!-- Environment Tab -->
            <TabsContent value="env" class="pt-3">
              <div class="rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{{ t('container.key') }}</TableHead>
                      <TableHead>{{ t('container.value') }}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <TableRow v-if="!config?.env?.length">
                      <TableCell colspan="2" class="text-center text-muted-foreground py-6">
                        {{ t('common.none') }}
                      </TableCell>
                    </TableRow>
                    <TableRow v-for="(env, idx) in config?.env || []" :key="idx">
                      <TableCell class="font-mono text-xs font-medium">{{ env.key }}</TableCell>
                      <TableCell class="font-mono text-xs break-all">{{ env.value }}</TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </div>
            </TabsContent>

            <!-- Resources Tab -->
            <TabsContent value="resources" class="pt-3 space-y-4">
              <div class="space-y-3 rounded-lg border p-4">
                <div class="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4">
                  <Label class="sm:w-36 shrink-0">{{ t('config.memory') }}</Label>
                  <div class="flex flex-wrap items-center gap-2.5 flex-1">
                    <Input
                      type="number"
                      min="0"
                      step="64"
                      :model-value="resourceForm.memory"
                      class="w-32 h-8 font-mono tabular-nums"
                      @update:model-value="(v) => resourceForm.memory = Number(v) || 0"
                    />
                    <span class="text-xs text-muted-foreground">
                      MB (0 = {{ t('config.unlimited') }})
                    </span>
                    <Button size="sm" @click="updateMemory">{{ t('config.apply') }}</Button>
                  </div>
                </div>

                <div class="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4">
                  <Label class="sm:w-36 shrink-0">{{ t('config.cpus') }}</Label>
                  <div class="flex flex-wrap items-center gap-2.5 flex-1">
                    <Input
                      type="number"
                      min="0"
                      max="64"
                      step="0.1"
                      :model-value="resourceForm.cpus"
                      class="w-32 h-8 font-mono tabular-nums"
                      @update:model-value="(v) => resourceForm.cpus = Number(v) || 0"
                    />
                    <span class="text-xs text-muted-foreground">
                      {{ t('config.cores') }} (0 = {{ t('config.unlimited') }})
                    </span>
                    <Button size="sm" @click="updateCpus">{{ t('config.apply') }}</Button>
                  </div>
                </div>

                <div class="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4">
                  <Label class="sm:w-36 shrink-0">{{ t('container.restartPolicy') }}</Label>
                  <div class="flex flex-wrap items-center gap-2.5 flex-1">
                    <Select v-model="resourceForm.restart">
                      <SelectTrigger class="w-48 h-8">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="no">{{ t('container.restartPolicies.no') }}</SelectItem>
                        <SelectItem value="always">{{ t('container.restartPolicies.always') }}</SelectItem>
                        <SelectItem value="on-failure">{{ t('container.restartPolicies.onFailure') }}</SelectItem>
                        <SelectItem value="unless-stopped">{{ t('container.restartPolicies.unlessStopped') }}</SelectItem>
                      </SelectContent>
                    </Select>
                    <Button size="sm" @click="updateRestart">{{ t('config.apply') }}</Button>
                  </div>
                </div>
              </div>

              <div class="grid grid-cols-1 sm:grid-cols-3 rounded-lg border divide-y sm:divide-y-0 sm:divide-x text-sm overflow-hidden">
                <div class="p-3 space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('config.cpuShares') }}</div>
                  <div class="font-mono text-xs">{{ config?.cpuShares ?? '-' }}</div>
                </div>
                <div class="p-3 space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('config.pidsLimit') }}</div>
                  <div class="font-mono text-xs">
                    {{ config?.pidsLimit === 0 ? t('config.unlimited') : config?.pidsLimit }}
                  </div>
                </div>
                <div class="p-3 space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('config.oomKillDisable') }}</div>
                  <div>{{ config?.oomKillDisable ? t('common.yes') : t('common.no') }}</div>
                </div>
              </div>
            </TabsContent>

            <!-- Security Tab -->
            <TabsContent value="security" class="pt-3 space-y-4">
              <div class="grid grid-cols-1 sm:grid-cols-2 rounded-lg border divide-y sm:divide-y-0 sm:divide-x text-sm overflow-hidden">
                <div class="p-3 space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('config.privileged') }}</div>
                  <div>
                    <Badge :variant="config?.privileged ? 'destructive' : 'success'">
                      {{ config?.privileged ? t('common.yes') : t('common.no') }}
                    </Badge>
                  </div>
                </div>
                <div class="p-3 space-y-1">
                  <div class="text-xs text-muted-foreground">{{ t('config.user') }}</div>
                  <div class="font-mono text-xs">{{ config?.user || 'root' }}</div>
                </div>
              </div>

              <div class="space-y-2">
                <h4 class="text-sm font-semibold">{{ t('config.capAdd') }}</h4>
                <div class="flex flex-wrap gap-2">
                  <Badge v-for="cap in config?.capAdd" :key="cap" variant="success">{{ cap }}</Badge>
                  <span v-if="!config?.capAdd?.length" class="text-sm text-muted-foreground">-</span>
                </div>
              </div>

              <div class="space-y-2">
                <h4 class="text-sm font-semibold">{{ t('config.capDrop') }}</h4>
                <div class="flex flex-wrap gap-2">
                  <Badge v-for="cap in config?.capDrop" :key="cap" variant="destructive">{{ cap }}</Badge>
                  <span v-if="!config?.capDrop?.length" class="text-sm text-muted-foreground">-</span>
                </div>
              </div>
            </TabsContent>

            <!-- Command Tab -->
            <TabsContent value="command" class="pt-3">
              <div class="rounded-lg border divide-y text-sm overflow-hidden">
                <div class="p-3.5 space-y-1.5">
                  <div class="text-xs text-muted-foreground">{{ t('config.entrypoint') }}</div>
                  <code class="block rounded bg-muted px-2.5 py-1.5 font-mono text-xs break-all">
                    {{ config?.entrypoint?.join(' ') || '-' }}
                  </code>
                </div>
                <div class="p-3.5 space-y-1.5">
                  <div class="text-xs text-muted-foreground">{{ t('config.cmd') }}</div>
                  <code class="block rounded bg-muted px-2.5 py-1.5 font-mono text-xs break-all">
                    {{ config?.cmd?.join(' ') || '-' }}
                  </code>
                </div>
              </div>
            </TabsContent>
          </Tabs>
        </div>
      </TooltipProvider>

      <DialogFooter class="gap-2 pt-2">
        <Button variant="outline" class="text-amber-600 dark:text-amber-400" @click="showRecreateDialog = true">
          {{ t('config.recreate') }}
        </Button>
        <Button variant="outline" @click="dialogVisible = false">{{ t('common.close') }}</Button>
      </DialogFooter>

      <RecreateContainerDialog
        v-model:visible="showRecreateDialog"
        :container-id="containerId"
        :container-config="config"
        @recreated="handleRecreated"
      />
    </DialogContent>
  </Dialog>
</template>
