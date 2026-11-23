<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { Loader2 } from 'lucide-vue-next'
import { networkApi } from '@/api/networks'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
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
import { Switch } from '@/components/ui/switch'

const props = defineProps<{
  visible: boolean
}>()

const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void
  (e: 'created'): void
}>()

const { t } = useI18n()
const toast = useToast()

const dialogVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
})

const loading = ref(false)

const form = reactive({
  name: '',
  driver: 'bridge',
  subnet: '',
  gateway: '',
  internal: false,
})

const errors = reactive({
  name: '',
  subnet: '',
  gateway: '',
})

const driverOptions = computed(() => [
  { value: 'bridge', label: t('network.drivers.bridge') },
  { value: 'overlay', label: t('network.drivers.overlay') },
  { value: 'macvlan', label: t('network.drivers.macvlan') },
  { value: 'host', label: t('network.drivers.host') },
  { value: 'none', label: t('network.drivers.none') },
])

const cidrRegex = /^(\d{1,3}\.){3}\d{1,3}\/\d{1,2}$/
const ipRegex = /^(\d{1,3}\.){3}\d{1,3}$/
const nameRegex = /^[a-zA-Z0-9][a-zA-Z0-9_.-]*$/

function validate(): boolean {
  const name = form.name.trim()
  if (!name) {
    errors.name = t('network.nameRequired')
  } else if (!nameRegex.test(name)) {
    errors.name = '网络名称必须以字母或数字开头，只能包含字母、数字、下划线、点或连字符'
  } else {
    errors.name = ''
  }

  errors.subnet = form.subnet && !cidrRegex.test(form.subnet.trim())
    ? '子网格式不正确，例如: 172.20.0.0/16'
    : ''

  errors.gateway = form.gateway && !ipRegex.test(form.gateway.trim())
    ? '网关格式不正确，例如: 172.20.0.1'
    : ''

  return !errors.name && !errors.subnet && !errors.gateway
}

watch(() => props.visible, (newVal) => {
  if (newVal) {
    form.name = ''
    form.driver = 'bridge'
    form.subnet = ''
    form.gateway = ''
    form.internal = false
    errors.name = ''
    errors.subnet = ''
    errors.gateway = ''
  }
})

watch(() => form.subnet, (newVal) => {
  if (!newVal) {
    form.gateway = ''
    errors.gateway = ''
  }
})

function handleClose() {
  dialogVisible.value = false
}

async function handleSubmit() {
  if (!validate()) return

  loading.value = true
  try {
    await networkApi.createFull({
      name: form.name.trim(),
      driver: form.driver,
      subnet: form.subnet.trim() || undefined,
      gateway: form.gateway.trim() || undefined,
      internal: form.internal,
    })
    toast.success(t('network.createSuccess'))
    emit('created')
    handleClose()
  } catch {
    // Error handled by interceptor
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <Dialog v-model:open="dialogVisible">
    <DialogContent class="sm:max-w-[520px]" @interact-outside.prevent>
      <DialogHeader>
        <DialogTitle>{{ t('network.create') }}</DialogTitle>
      </DialogHeader>

      <form class="space-y-4 py-1" @submit.prevent="handleSubmit">
        <div class="space-y-1.5">
          <Label for="net-name">
            {{ t('network.networkName') }} <span class="text-destructive">*</span>
          </Label>
          <Input
            id="net-name"
            v-model="form.name"
            :placeholder="t('network.nameRequired')"
            @input="errors.name = ''"
          />
          <p v-if="errors.name" class="text-xs text-destructive">{{ errors.name }}</p>
        </div>

        <div class="space-y-1.5">
          <Label>{{ t('network.networkDriver') }}</Label>
          <Select v-model="form.driver">
            <SelectTrigger class="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="driver in driverOptions" :key="driver.value" :value="driver.value">
                {{ driver.label }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div class="space-y-1.5">
          <Label for="net-subnet">{{ t('network.subnet') }}</Label>
          <Input
            id="net-subnet"
            v-model="form.subnet"
            placeholder="例如: 172.20.0.0/16"
            class="font-mono"
            @input="errors.subnet = ''"
          />
          <p v-if="errors.subnet" class="text-xs text-destructive">{{ errors.subnet }}</p>
          <p class="text-xs text-muted-foreground">可选，留空则由 Docker 自动分配</p>
        </div>

        <div class="space-y-1.5">
          <Label for="net-gateway">{{ t('network.gateway') }}</Label>
          <Input
            id="net-gateway"
            v-model="form.gateway"
            placeholder="例如: 172.20.0.1"
            class="font-mono"
            :disabled="!form.subnet"
            @input="errors.gateway = ''"
          />
          <p v-if="errors.gateway" class="text-xs text-destructive">{{ errors.gateway }}</p>
          <p class="text-xs text-muted-foreground">可选，需要先设置子网</p>
        </div>

        <div class="flex items-center justify-between rounded-lg border p-3">
          <div class="space-y-0.5">
            <Label for="net-internal">{{ t('network.internal') }}</Label>
            <p class="text-xs text-muted-foreground">内部网络不允许外部连接</p>
          </div>
          <div class="flex items-center gap-2">
            <Switch id="net-internal" v-model="form.internal" />
            <span class="text-xs text-muted-foreground w-4">{{ form.internal ? '是' : '否' }}</span>
          </div>
        </div>

        <DialogFooter class="gap-2 pt-2">
          <Button type="button" variant="outline" @click="handleClose">{{ t('common.cancel') }}</Button>
          <Button type="submit" :disabled="loading">
            <Loader2 v-if="loading" class="animate-spin" />
            {{ t('common.create') }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>
