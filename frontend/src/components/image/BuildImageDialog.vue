<script setup lang="ts">
import { ref, reactive, computed, watch, nextTick } from 'vue'
import { Loader2 } from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { imageApi } from '@/api/images'
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
import { Textarea } from '@/components/ui/textarea'

const props = defineProps<{
  visible: boolean
}>()

const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void
  (e: 'built'): void
}>()

const { t } = useI18n()
const toast = useToast()

const dialogVisible = computed({
  get: () => props.visible,
  set: (value) => {
    if (!value && building.value) return
    emit('update:visible', value)
  },
})

const form = reactive({
  name: '',
  tag: 'latest',
  dockerfile: '',
})

const errors = reactive({
  name: '',
  tag: '',
  dockerfile: '',
})

const building = ref(false)
const buildSuccess = ref(false)
const buildError = ref(false)
const buildLogs = ref<Array<{ type: string; message: string }>>([])
const logsContainer = ref<HTMLElement>()

const dockerfilePlaceholder = `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
EXPOSE 3000
CMD ["npm", "start"]`

watch(
  buildLogs,
  () => {
    nextTick(() => {
      if (logsContainer.value) {
        logsContainer.value.scrollTop = logsContainer.value.scrollHeight
      }
    })
  },
  { deep: true }
)

function handleClose() {
  if (building.value) return
  resetForm()
  emit('update:visible', false)
}

function resetForm() {
  form.name = ''
  form.tag = 'latest'
  form.dockerfile = ''
  errors.name = ''
  errors.tag = ''
  errors.dockerfile = ''
  buildLogs.value = []
  buildSuccess.value = false
  buildError.value = false
}

function validate(): boolean {
  errors.name = form.name.trim() ? '' : t('image.nameRequired')
  errors.tag = form.tag.trim() ? '' : t('image.tagRequired')
  errors.dockerfile = form.dockerfile.trim() ? '' : t('image.dockerfileRequired')
  return !errors.name && !errors.tag && !errors.dockerfile
}

async function handleBuild() {
  if (!validate()) return

  building.value = true
  buildSuccess.value = false
  buildError.value = false
  buildLogs.value = []

  try {
    await imageApi.build(
      form.name.trim(),
      form.tag.trim(),
      form.dockerfile,
      (data) => {
        if (data.message) {
          buildLogs.value.push({
            type: data.type || 'info',
            message: data.message,
          })
        }
        if (data.error) {
          buildLogs.value.push({
            type: 'error',
            message: data.error,
          })
          buildError.value = true
        }
      },
      (error) => {
        buildLogs.value.push({
          type: 'error',
          message: error,
        })
        buildError.value = true
        building.value = false
        toast.error(t('image.buildFailed'))
      },
      () => {
        building.value = false
        if (!buildError.value) {
          buildSuccess.value = true
          toast.success(t('image.buildSuccess'))
          emit('built')
        }
      }
    )
  } catch {
    building.value = false
    buildError.value = true
    toast.error(t('image.buildFailed'))
  }
}
</script>

<template>
  <Dialog v-model:open="dialogVisible">
    <DialogContent
      class="sm:max-w-[700px] max-h-[90vh] flex flex-col"
      :show-close-button="!building"
      @interact-outside.prevent
      @escape-key-down="(e) => { if (building) e.preventDefault() }"
    >
      <DialogHeader>
        <DialogTitle>{{ t('image.build') }}</DialogTitle>
      </DialogHeader>

      <div class="flex-1 overflow-y-auto space-y-4 pr-1 py-1">
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <div class="sm:col-span-2 space-y-1.5">
            <Label for="build-img-name">
              {{ t('image.imageName') }} <span class="text-destructive">*</span>
            </Label>
            <Input
              id="build-img-name"
              v-model="form.name"
              :placeholder="t('image.nameRequired')"
              :disabled="building"
              @input="errors.name = ''"
            />
            <p v-if="errors.name" class="text-xs text-destructive">{{ errors.name }}</p>
          </div>

          <div class="space-y-1.5">
            <Label for="build-img-tag">
              {{ t('image.imageTag') }} <span class="text-destructive">*</span>
            </Label>
            <Input
              id="build-img-tag"
              v-model="form.tag"
              placeholder="latest"
              :disabled="building"
              @input="errors.tag = ''"
            />
            <p v-if="errors.tag" class="text-xs text-destructive">{{ errors.tag }}</p>
          </div>
        </div>

        <div class="space-y-1.5">
          <Label for="build-dockerfile">
            {{ t('image.dockerfile') }} <span class="text-destructive">*</span>
          </Label>
          <Textarea
            id="build-dockerfile"
            v-model="form.dockerfile"
            rows="11"
            :placeholder="dockerfilePlaceholder"
            :disabled="building"
            class="font-mono text-xs leading-relaxed min-h-[220px]"
            @input="errors.dockerfile = ''"
          />
          <p v-if="errors.dockerfile" class="text-xs text-destructive">{{ errors.dockerfile }}</p>
        </div>

        <div v-if="buildLogs.length > 0" class="rounded-lg border overflow-hidden">
          <div class="flex items-center justify-between px-3.5 py-2 bg-muted/50 border-b text-xs font-medium">
            <span>{{ t('image.buildLogs') }}</span>
            <Badge v-if="building" variant="warning">{{ t('image.building') }}</Badge>
            <Badge v-else-if="buildSuccess" variant="success">{{ t('image.buildSuccess') }}</Badge>
            <Badge v-else-if="buildError" variant="destructive">{{ t('image.buildFailed') }}</Badge>
          </div>
          <div
            ref="logsContainer"
            class="max-h-[240px] overflow-y-auto p-3 bg-zinc-950 text-zinc-100 font-mono text-xs leading-relaxed space-y-0.5"
          >
            <div
              v-for="(log, index) in buildLogs"
              :key="index"
              class="whitespace-pre-wrap break-all"
              :class="{
                'text-red-400': log.type === 'error',
                'text-amber-400': log.type === 'warning',
                'text-emerald-400': log.type === 'success',
              }"
            >
              {{ log.message }}
            </div>
          </div>
        </div>
      </div>

      <DialogFooter class="gap-2 pt-2">
        <Button type="button" variant="outline" :disabled="building" @click="handleClose">
          {{ t('common.cancel') }}
        </Button>
        <Button :disabled="building" @click="handleBuild">
          <Loader2 v-if="building" class="animate-spin" />
          {{ building ? t('image.building') : t('image.build') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
