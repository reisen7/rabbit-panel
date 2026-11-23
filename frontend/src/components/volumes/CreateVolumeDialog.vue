<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { Loader2, Plus, Trash2 } from 'lucide-vue-next'
import { useVolumeStore } from '@/stores/volumes'
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

const props = defineProps<{
  visible: boolean
}>()

const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void
  (e: 'created'): void
}>()

const { t } = useI18n()
const toast = useToast()
const volumeStore = useVolumeStore()

const dialogVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
})

const loading = ref(false)

interface KeyValue {
  key: string
  value: string
}

const form = reactive({
  name: '',
  driver: 'local',
  driverOpts: [] as KeyValue[],
  labels: [] as KeyValue[],
})

const nameError = ref('')
const namePattern = /^[a-zA-Z0-9][a-zA-Z0-9_.-]*$/

const driverOptions = [
  { value: 'local', label: 'local' },
]

function addDriverOpt() {
  form.driverOpts.push({ key: '', value: '' })
}

function removeDriverOpt(index: number) {
  form.driverOpts.splice(index, 1)
}

function addLabel() {
  form.labels.push({ key: '', value: '' })
}

function removeLabel(index: number) {
  form.labels.splice(index, 1)
}

watch(
  () => props.visible,
  (newVal) => {
    if (newVal) {
      form.name = ''
      form.driver = 'local'
      form.driverOpts = []
      form.labels = []
      nameError.value = ''
    }
  }
)

function validate(): boolean {
  const val = form.name.trim()
  if (!val) {
    nameError.value = t('volumes.nameRequired')
    return false
  }
  if (!namePattern.test(val)) {
    nameError.value = 'Volume name must start with alphanumeric and contain only alphanumeric, underscore, period, or hyphen'
    return false
  }
  nameError.value = ''
  return true
}

function handleClose() {
  dialogVisible.value = false
}

async function handleSubmit() {
  if (!validate()) return

  loading.value = true
  try {
    const driverOpts: Record<string, string> = {}
    form.driverOpts.forEach((opt) => {
      if (opt.key) driverOpts[opt.key] = opt.value
    })

    const labels: Record<string, string> = {}
    form.labels.forEach((label) => {
      if (label.key) labels[label.key] = label.value
    })

    await volumeStore.createVolume({
      name: form.name.trim(),
      driver: form.driver,
      driverOpts: Object.keys(driverOpts).length > 0 ? driverOpts : undefined,
      labels: Object.keys(labels).length > 0 ? labels : undefined,
    })
    toast.success(t('volumes.createSuccess'))
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
    <DialogContent class="sm:max-w-[580px] max-h-[88vh] flex flex-col" @interact-outside.prevent>
      <DialogHeader>
        <DialogTitle>{{ t('volumes.create') }}</DialogTitle>
      </DialogHeader>

      <form class="flex-1 overflow-y-auto space-y-4 pr-1 py-1" @submit.prevent="handleSubmit">
        <div class="space-y-1.5">
          <Label for="volume-name">
            {{ t('volumes.name') }} <span class="text-destructive">*</span>
          </Label>
          <Input
            id="volume-name"
            v-model="form.name"
            :placeholder="t('volumes.namePlaceholder')"
            @input="nameError = ''"
          />
          <p v-if="nameError" class="text-xs text-destructive">{{ nameError }}</p>
        </div>

        <div class="space-y-1.5">
          <Label>{{ t('volumes.driver') }}</Label>
          <Select v-model="form.driver">
            <SelectTrigger class="w-full">
              <SelectValue :placeholder="t('volumes.driverPlaceholder')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="driver in driverOptions" :key="driver.value" :value="driver.value">
                {{ driver.label }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div class="space-y-2">
          <Label>{{ t('volumes.driverOpts') }}</Label>
          <div class="space-y-2">
            <div v-for="(opt, index) in form.driverOpts" :key="index" class="flex items-center gap-2">
              <Input v-model="opt.key" :placeholder="t('volumes.optKey')" class="flex-1" />
              <span class="text-muted-foreground font-mono">=</span>
              <Input v-model="opt.value" :placeholder="t('volumes.optValue')" class="flex-[1.3]" />
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                class="text-destructive shrink-0"
                @click="removeDriverOpt(index)"
              >
                <Trash2 class="size-4" />
              </Button>
            </div>
            <Button type="button" variant="outline" size="sm" @click="addDriverOpt">
              <Plus />
              {{ t('volumes.addDriverOpt') }}
            </Button>
          </div>
        </div>

        <div class="space-y-2">
          <Label>{{ t('volumes.labels') }}</Label>
          <div class="space-y-2">
            <div v-for="(label, index) in form.labels" :key="index" class="flex items-center gap-2">
              <Input v-model="label.key" :placeholder="t('volumes.labelKey')" class="flex-1" />
              <span class="text-muted-foreground font-mono">=</span>
              <Input v-model="label.value" :placeholder="t('volumes.labelValue')" class="flex-[1.3]" />
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                class="text-destructive shrink-0"
                @click="removeLabel(index)"
              >
                <Trash2 class="size-4" />
              </Button>
            </div>
            <Button type="button" variant="outline" size="sm" @click="addLabel">
              <Plus />
              {{ t('volumes.addLabel') }}
            </Button>
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
