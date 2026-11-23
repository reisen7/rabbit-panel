<script setup lang="ts">
import { ref, computed, reactive, watch } from 'vue'
import { AlertTriangle, Loader2, Plus, Trash2 } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { containerApi } from '@/api/containers'
import type { ContainerConfig } from '@/types'
import { Alert, AlertTitle } from '@/components/ui/alert'
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

const props = defineProps<{
  visible: boolean
  containerId: string
  containerConfig: ContainerConfig | null
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
  recreated: []
}>()

const { t } = useI18n()
const toast = useToast()

const dialogVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
})

const form = reactive({
  image: '',
  name: '',
  restart: 'no',
  network: 'bridge',
  ports: [] as Array<{ host: string; container: string }>,
  volumes: [] as Array<{ host: string; container: string }>,
  envs: [] as Array<{ key: string; value: string }>,
})

const recreating = ref(false)

function initForm() {
  if (!props.containerConfig) return

  const config = props.containerConfig
  form.image = config.image || ''
  form.name = config.name || ''
  form.restart = config.restart || 'no'
  form.network = config.networkMode || 'bridge'

  form.ports = (config.ports || []).map((p) => ({
    host: p.host,
    container: p.container,
  }))

  form.volumes = (config.volumes || []).map((v) => ({
    host: v.host,
    container: v.container,
  }))

  form.envs = (config.env || []).map((e) => ({
    key: e.key,
    value: e.value,
  }))
}

function addPort() {
  form.ports.push({ host: '', container: '' })
}

function removePort(index: number) {
  form.ports.splice(index, 1)
}

function addVolume() {
  form.volumes.push({ host: '', container: '' })
}

function removeVolume(index: number) {
  form.volumes.splice(index, 1)
}

function addEnv() {
  form.envs.push({ key: '', value: '' })
}

function removeEnv(index: number) {
  form.envs.splice(index, 1)
}

async function handleRecreate() {
  if (!form.image.trim()) {
    toast.warning(`${t('container.image')} ${t('common.required')}`)
    return
  }

  if (!form.name.trim()) {
    toast.warning(`${t('common.name')} ${t('common.required')}`)
    return
  }

  recreating.value = true
  try {
    const ports = form.ports.filter((p) => p.host && p.container)
    const volumes = form.volumes.filter((v) => v.host && v.container)
    const envs = form.envs.filter((e) => e.key)

    await containerApi.recreate({
      container_id: props.containerId,
      image: form.image.trim(),
      name: form.name.trim(),
      restart: form.restart,
      network: form.network.trim(),
      ports,
      volumes,
      env: envs,
    })

    toast.success(t('config.recreateSuccess'))
    emit('recreated')
    dialogVisible.value = false
  } catch {
    // Error handled by interceptor
  } finally {
    recreating.value = false
  }
}

watch(
  () => props.visible,
  (val) => {
    if (val) {
      initForm()
    }
  },
  { immediate: true }
)
</script>

<template>
  <Dialog v-model:open="dialogVisible">
    <DialogContent class="sm:max-w-[700px] max-h-[88vh] flex flex-col" @interact-outside.prevent>
      <DialogHeader>
        <DialogTitle>{{ t('config.recreate') }}</DialogTitle>
      </DialogHeader>

      <div class="flex-1 overflow-y-auto space-y-4 pr-1 py-1">
        <Alert variant="warning">
          <AlertTriangle class="size-4" />
          <AlertTitle>{{ t('config.recreateConfirm') }}</AlertTitle>
        </Alert>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div class="space-y-1.5">
            <Label for="recreate-img">
              {{ t('container.image') }} <span class="text-destructive">*</span>
            </Label>
            <Input id="recreate-img" v-model="form.image" :placeholder="t('container.image')" />
          </div>

          <div class="space-y-1.5">
            <Label for="recreate-name">
              {{ t('common.name') }} <span class="text-destructive">*</span>
            </Label>
            <Input id="recreate-name" v-model="form.name" :placeholder="t('container.containerName')" />
          </div>

          <div class="space-y-1.5">
            <Label>{{ t('container.restartPolicy') }}</Label>
            <Select v-model="form.restart">
              <SelectTrigger class="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="no">{{ t('container.restartPolicies.no') }}</SelectItem>
                <SelectItem value="always">{{ t('container.restartPolicies.always') }}</SelectItem>
                <SelectItem value="on-failure">{{ t('container.restartPolicies.onFailure') }}</SelectItem>
                <SelectItem value="unless-stopped">{{ t('container.restartPolicies.unlessStopped') }}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div class="space-y-1.5">
            <Label for="recreate-network">{{ t('container.network') }}</Label>
            <Input id="recreate-network" v-model="form.network" placeholder="bridge" />
          </div>
        </div>

        <div class="space-y-2">
          <Label>{{ t('container.ports') }}</Label>
          <div class="space-y-2">
            <div v-for="(port, index) in form.ports" :key="index" class="flex items-center gap-2">
              <Input v-model="port.host" :placeholder="t('container.hostPort')" class="flex-1 font-mono" />
              <span class="text-muted-foreground font-mono">:</span>
              <Input v-model="port.container" :placeholder="t('container.containerPort')" class="flex-1 font-mono" />
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                class="text-destructive shrink-0"
                @click="removePort(index)"
              >
                <Trash2 class="size-4" />
              </Button>
            </div>
            <Button type="button" variant="outline" size="sm" @click="addPort">
              <Plus />
              {{ t('container.addPort') }}
            </Button>
          </div>
        </div>

        <div class="space-y-2">
          <Label>{{ t('container.volumes') }}</Label>
          <div class="space-y-2">
            <div v-for="(volume, index) in form.volumes" :key="index" class="flex items-center gap-2">
              <Input v-model="volume.host" :placeholder="t('container.hostPath')" class="flex-1 font-mono" />
              <span class="text-muted-foreground font-mono">:</span>
              <Input v-model="volume.container" :placeholder="t('container.containerPath')" class="flex-1 font-mono" />
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                class="text-destructive shrink-0"
                @click="removeVolume(index)"
              >
                <Trash2 class="size-4" />
              </Button>
            </div>
            <Button type="button" variant="outline" size="sm" @click="addVolume">
              <Plus />
              {{ t('container.addVolume') }}
            </Button>
          </div>
        </div>

        <div class="space-y-2">
          <Label>{{ t('container.envVars') }}</Label>
          <div class="space-y-2">
            <div v-for="(env, index) in form.envs" :key="index" class="flex items-center gap-2">
              <Input v-model="env.key" :placeholder="t('container.key')" class="flex-1 font-mono" />
              <span class="text-muted-foreground font-mono">=</span>
              <Input v-model="env.value" :placeholder="t('container.value')" class="flex-[1.3] font-mono" />
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                class="text-destructive shrink-0"
                @click="removeEnv(index)"
              >
                <Trash2 class="size-4" />
              </Button>
            </div>
            <Button type="button" variant="outline" size="sm" @click="addEnv">
              <Plus />
              {{ t('container.addEnv') }}
            </Button>
          </div>
        </div>
      </div>

      <DialogFooter class="gap-2 pt-2">
        <Button type="button" variant="outline" @click="dialogVisible = false">
          {{ t('common.cancel') }}
        </Button>
        <Button :disabled="recreating" @click="handleRecreate">
          <Loader2 v-if="recreating" class="animate-spin" />
          {{ t('config.recreate') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
