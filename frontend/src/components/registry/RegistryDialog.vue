<template>
  <el-dialog
    v-model="dialogVisible"
    :title="isEdit ? t('registry.edit') : t('registry.add')"
    width="500px"
    :close-on-click-modal="false"
    @close="handleClose"
  >
    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      label-width="120px"
      label-position="right"
    >
      <el-form-item :label="t('registry.name')" prop="name">
        <el-input
          v-model="form.name"
          :placeholder="t('registry.namePlaceholder')"
          clearable
        />
      </el-form-item>

      <el-form-item :label="t('registry.url')" prop="url">
        <el-input
          v-model="form.url"
          :placeholder="t('registry.urlPlaceholder')"
          clearable
        />
      </el-form-item>

      <el-form-item :label="t('registry.username')" prop="username">
        <el-input
          v-model="form.username"
          placeholder="Optional"
          clearable
        />
      </el-form-item>

      <el-form-item :label="t('registry.password')" prop="password">
        <el-input
          v-model="form.password"
          type="password"
          :placeholder="isEdit ? t('registry.passwordKeep') : 'Optional'"
          show-password
          clearable
        />
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="dialog-footer">
        <el-button @click="handleClose">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="loading" @click="handleSubmit">
          {{ isEdit ? t('common.save') : t('common.create') }}
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage } from 'element-plus'
import { useRegistryStore } from '@/stores/registry'
import { useI18n } from '@/composables/useI18n'
import type { RegistryInfo } from '@/types'

const props = defineProps<{
  visible: boolean
  registry?: RegistryInfo | null
}>()

const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void
  (e: 'saved'): void
}>()

const { t } = useI18n()
const registryStore = useRegistryStore()

// Dialog visibility
const dialogVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
})

// Check if editing
const isEdit = computed(() => !!props.registry)

// Form state
const formRef = ref<FormInstance>()
const loading = ref(false)

const form = ref({
  name: '',
  url: '',
  username: '',
  password: '',
})

// Form validation rules
const rules = computed<FormRules>(() => ({
  name: [
    { required: true, message: t('registry.nameRequired'), trigger: 'blur' },
  ],
  url: [
    { required: true, message: t('registry.urlRequired'), trigger: 'blur' },
    {
      pattern: /^https?:\/\/.+|^[a-zA-Z0-9][a-zA-Z0-9.-]*:[0-9]+$/,
      message: 'Please enter a valid URL or host:port',
      trigger: 'blur',
    },
  ],
}))

// Reset form when dialog opens or registry changes
watch(
  () => [props.visible, props.registry],
  ([newVisible]) => {
    if (newVisible) {
      if (props.registry) {
        form.value = {
          name: props.registry.name,
          url: props.registry.url,
          username: props.registry.username || '',
          password: '',
        }
      } else {
        form.value = {
          name: '',
          url: '',
          username: '',
          password: '',
        }
      }
      formRef.value?.clearValidate()
    }
  },
  { immediate: true }
)

// Handlers
function handleClose() {
  dialogVisible.value = false
}

async function handleSubmit() {
  if (!formRef.value) return

  try {
    await formRef.value.validate()
    loading.value = true

    const data = {
      name: form.value.name,
      url: form.value.url,
      username: form.value.username || undefined,
      password: form.value.password || undefined,
    }

    if (isEdit.value && props.registry) {
      await registryStore.updateRegistry(props.registry.url, data)
      ElMessage.success(t('registry.updateSuccess'))
    } else {
      await registryStore.createRegistry(data)
      ElMessage.success(t('registry.createSuccess'))
    }
    emit('saved')
    handleClose()
  } catch {
    // Validation error or API error (handled by interceptor)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
