<script setup lang="ts">
import { ref, reactive, computed, watch, nextTick } from 'vue'
import { Loader2, Plus, Trash2 } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { useContainerCreateStore } from '@/stores/containerCreate'
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

const { t } = useI18n()
const store = useContainerCreateStore()

const props = defineProps<{
  visible: boolean
  initialImage?: string
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
  created: []
}>()

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => {
    if (!val && (store.loading || store.isStreaming)) {
      handleMinimize()
      return
    }
    emit('update:visible', val)
  },
})

const logPanelRef = ref<HTMLElement | null>(null)
const imageError = ref('')

interface PortMapping {
  host: string
  container: string
}

interface VolumeMapping {
  host: string
  container: string
}

interface EnvVar {
  key: string
  value: string
}

interface FormData {
  image: string
  name: string
  restart: string
  network: string
  ports: PortMapping[]
  volumes: VolumeMapping[]
  envs: EnvVar[]
}

const initialForm = (): FormData => ({
  image: '',
  name: '',
  restart: 'no',
  network: 'bridge',
  ports: [],
  volumes: [],
  envs: [],
})

const form = reactive<FormData>(initialForm())

watch(
  () => props.visible,
  (val) => {
    if (val) {
      if (store.consumeRestore()) {
        if (store.lastForm) {
          Object.assign(form, store.lastForm)
        }
        imageError.value = ''
        return
      }
      if (!store.loading && !store.isStreaming) {
        store.reset()
        Object.assign(form, initialForm())
        const prefill = store.consumePrefill()
        if (prefill) {
          Object.assign(form, prefill)
        } else {
          form.image = props.initialImage ?? ''
        }
        imageError.value = ''
      } else {
        // 任务进行中打开的对话框不能覆盖表单，丢弃预填避免下次误用
        store.consumePrefill()
      }
    }
  }
)

watch(
  () => store.createdSignal,
  () => {
    emit('created')
  }
)

watch(
  () => store.streamLogs,
  async () => {
    await nextTick()
    if (logPanelRef.value) {
      logPanelRef.value.scrollTop = logPanelRef.value.scrollHeight
    }
  },
  { deep: true }
)

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

function handleMinimize() {
  store.minimize()
  emit('update:visible', false)
}

function handleCancelTask() {
  store.cancel()
}

function handleClose() {
  if (store.loading || store.isStreaming) {
    handleMinimize()
    return
  }
  emit('update:visible', false)
}

function handleSubmit() {
  if (!form.image.trim()) {
    imageError.value = `${t('container.image')} is required`
    return
  }
  imageError.value = ''

  const ports = form.ports.filter((p) => p.host && p.container)
  const volumes = form.volumes.filter((v) => v.host && v.container)
  const envs = form.envs.filter((e) => e.key)

  store.start(form, {
    image: form.image.trim(),
    name: form.name.trim(),
    restart: form.restart,
    network: form.network.trim(),
    ports,
    volumes,
    env: envs,
  })
}
</script>

<template>
  <Dialog v-model:open="dialogVisible">
    <DialogContent
      class="sm:max-w-[1080px] max-h-[90vh] flex flex-col"
      @interact-outside.prevent
    >
      <DialogHeader>
        <DialogTitle>{{ t('container.create') }}</DialogTitle>
      </DialogHeader>

      <div class="grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_400px] gap-5 flex-1 min-h-0 overflow-y-auto pr-1">
        <div class="space-y-4">
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div class="space-y-1.5">
              <Label for="create-img">
                {{ t('container.image') }} <span class="text-destructive">*</span>
              </Label>
              <Input
                id="create-img"
                v-model="form.image"
                placeholder="nginx:latest"
                @input="imageError = ''"
              />
              <p v-if="imageError" class="text-xs text-destructive">{{ imageError }}</p>
            </div>

            <div class="space-y-1.5">
              <Label for="create-name">{{ t('container.containerName') }}</Label>
              <Input id="create-name" v-model="form.name" placeholder="my-container" />
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
              <Label for="create-network">{{ t('container.network') }}</Label>
              <Input id="create-network" v-model="form.network" placeholder="bridge" />
            </div>
          </div>

          <!-- Ports -->
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

          <!-- Volumes -->
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

          <!-- Env Vars -->
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

        <!-- Output Panel -->
        <div class="flex flex-col rounded-lg border overflow-hidden bg-zinc-950 min-h-[260px] lg:min-h-[480px]">
          <div class="flex items-center justify-between px-3.5 py-2.5 bg-muted/50 border-b text-xs font-medium text-foreground">
            <span>{{ t('container.output') }}</span>
            <span class="text-muted-foreground">{{ store.isStreaming ? t('container.building') : '-' }}</span>
          </div>
          <div
            ref="logPanelRef"
            class="flex-1 max-h-[500px] overflow-y-auto p-3 font-mono text-xs leading-relaxed text-zinc-200 space-y-1"
          >
            <div v-if="store.streamLogs.length === 0" class="text-zinc-500">
              {{ t('container.output') }}
            </div>
            <div
              v-for="(line, index) in store.streamLogs"
              :key="index"
              class="whitespace-pre-wrap break-all"
              :class="{
                'text-red-400': line.type === 'error',
                'text-emerald-400': line.type === 'success',
              }"
            >
              {{ line.message }}
            </div>
          </div>
        </div>
      </div>

      <DialogFooter class="gap-2 pt-2">
        <template v-if="store.loading || store.isStreaming">
          <Button variant="outline" @click="handleMinimize">{{ t('update.minimize') }}</Button>
          <Button variant="destructive" @click="handleCancelTask">{{ t('container.cancelTask') }}</Button>
        </template>
        <Button v-else variant="outline" @click="handleClose">{{ t('common.close') }}</Button>
        <Button :disabled="store.loading" @click="handleSubmit">
          <Loader2 v-if="store.loading" class="animate-spin" />
          {{ t('common.confirm') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
