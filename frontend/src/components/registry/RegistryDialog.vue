<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { Eye, EyeOff, Loader2 } from 'lucide-vue-next'
import { useRegistryStore } from '@/stores/registry'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import type { RegistryInfo } from '@/types'
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

const props = defineProps<{
  visible: boolean
  registry?: RegistryInfo | null
}>()

const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void
  (e: 'saved'): void
}>()

const { t } = useI18n()
const toast = useToast()
const registryStore = useRegistryStore()

const dialogVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
})

const isEdit = computed(() => !!props.registry)
const loading = ref(false)
const showPassword = ref(false)

const form = reactive({
  name: '',
  url: '',
  username: '',
  password: '',
})

const errors = reactive({
  name: '',
  url: '',
})

const urlPattern = /^https?:\/\/.+|^[a-zA-Z0-9][a-zA-Z0-9.-]*:[0-9]+$/

function validate(): boolean {
  errors.name = form.name.trim() ? '' : t('registry.nameRequired')
  if (!form.url.trim()) {
    errors.url = t('registry.urlRequired')
  } else if (!urlPattern.test(form.url.trim())) {
    errors.url = 'Please enter a valid URL or host:port'
  } else {
    errors.url = ''
  }
  return !errors.name && !errors.url
}

watch(
  () => [props.visible, props.registry],
  ([newVisible]) => {
    if (newVisible) {
      if (props.registry) {
        form.name = props.registry.name
        form.url = props.registry.url
        form.username = props.registry.username || ''
        form.password = ''
      } else {
        form.name = ''
        form.url = ''
        form.username = ''
        form.password = ''
      }
      errors.name = ''
      errors.url = ''
      showPassword.value = false
    }
  },
  { immediate: true }
)

function handleClose() {
  dialogVisible.value = false
}

async function handleSubmit() {
  if (!validate()) return

  loading.value = true
  try {
    const data = {
      name: form.name.trim(),
      url: form.url.trim(),
      username: form.username.trim() || undefined,
      password: form.password || undefined,
    }

    if (isEdit.value && props.registry) {
      await registryStore.updateRegistry(props.registry.url, data)
      toast.success(t('registry.updateSuccess'))
    } else {
      await registryStore.createRegistry(data)
      toast.success(t('registry.createSuccess'))
    }
    emit('saved')
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
    <DialogContent class="sm:max-w-[480px]" @interact-outside.prevent>
      <DialogHeader>
        <DialogTitle>{{ isEdit ? t('registry.edit') : t('registry.add') }}</DialogTitle>
      </DialogHeader>

      <form class="space-y-4 py-1" @submit.prevent="handleSubmit">
        <div class="space-y-1.5">
          <Label for="reg-name">
            {{ t('registry.name') }} <span class="text-destructive">*</span>
          </Label>
          <Input
            id="reg-name"
            v-model="form.name"
            :placeholder="t('registry.namePlaceholder')"
            @input="errors.name = ''"
          />
          <p v-if="errors.name" class="text-xs text-destructive">{{ errors.name }}</p>
        </div>

        <div class="space-y-1.5">
          <Label for="reg-url">
            {{ t('registry.url') }} <span class="text-destructive">*</span>
          </Label>
          <Input
            id="reg-url"
            v-model="form.url"
            :placeholder="t('registry.urlPlaceholder')"
            class="font-mono"
            @input="errors.url = ''"
          />
          <p v-if="errors.url" class="text-xs text-destructive">{{ errors.url }}</p>
        </div>

        <div class="space-y-1.5">
          <Label for="reg-username">{{ t('registry.username') }}</Label>
          <Input
            id="reg-username"
            v-model="form.username"
            placeholder="Optional"
          />
        </div>

        <div class="space-y-1.5">
          <Label for="reg-password">{{ t('registry.password') }}</Label>
          <div class="relative">
            <Input
              id="reg-password"
              v-model="form.password"
              :type="showPassword ? 'text' : 'password'"
              :placeholder="isEdit ? t('registry.passwordKeep') : 'Optional'"
              class="pr-9"
            />
            <button
              type="button"
              class="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              @click="showPassword = !showPassword"
            >
              <EyeOff v-if="showPassword" class="size-4" />
              <Eye v-else class="size-4" />
            </button>
          </div>
        </div>

        <DialogFooter class="gap-2 pt-2">
          <Button type="button" variant="outline" @click="handleClose">{{ t('common.cancel') }}</Button>
          <Button type="submit" :disabled="loading">
            <Loader2 v-if="loading" class="animate-spin" />
            {{ isEdit ? t('common.save') : t('common.create') }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>
