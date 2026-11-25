<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { AlertTriangle, Eye, EyeOff, Loader2 } from 'lucide-vue-next'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import {
  validatePassword,
  getStrengthColor,
  getStrengthPercentage,
  getStrengthLabel,
} from '@/utils/password'
import { Alert, AlertDescription } from '@/components/ui/alert'
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
import { Progress } from '@/components/ui/progress'

interface Props {
  modelValue: boolean
  forced?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  forced: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'success': []
}>()

const authStore = useAuthStore()
const toast = useToast()

const visible = computed({
  get: () => props.modelValue,
  set: (value) => {
    if (!value && isForced.value) return
    emit('update:modelValue', value)
  },
})

const isForced = computed(() => props.forced)
const loading = ref(false)
const showOldPassword = ref(false)
const showNewPassword = ref(false)
const showConfirmPassword = ref(false)

const form = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
})

const errors = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
})

const passwordErrors = ref<string[]>([])
const passwordStrength = ref<'weak' | 'medium' | 'strong'>('weak')

const strengthColor = computed(() => getStrengthColor(passwordStrength.value))
const strengthPercentage = computed(() => getStrengthPercentage(passwordStrength.value))
const strengthLabel = computed(() => getStrengthLabel(passwordStrength.value))

function handlePasswordInput() {
  errors.newPassword = ''
  if (form.newPassword) {
    const result = validatePassword(form.newPassword)
    passwordErrors.value = result.errors
    passwordStrength.value = result.strength
  } else {
    passwordErrors.value = []
    passwordStrength.value = 'weak'
  }
}

function validate(): boolean {
  errors.oldPassword = ''
  errors.newPassword = ''
  errors.confirmPassword = ''

  if (!form.oldPassword) {
    errors.oldPassword = isForced.value ? '请输入当前密码（默认为 admin）' : '请输入旧密码'
  }

  if (!form.newPassword) {
    errors.newPassword = '请输入新密码'
  } else {
    const result = validatePassword(form.newPassword)
    if (!result.valid) {
      errors.newPassword = result.errors[0] || '密码格式不符合要求'
    }
  }

  if (!form.confirmPassword) {
    errors.confirmPassword = '请再次输入新密码'
  } else if (form.confirmPassword !== form.newPassword) {
    errors.confirmPassword = '两次输入的密码不一致'
  }

  return !errors.oldPassword && !errors.newPassword && !errors.confirmPassword
}

async function handleSubmit() {
  if (!validate()) return

  loading.value = true
  try {
    await authStore.changePassword(
      form.oldPassword,
      form.newPassword
    )
    toast.success('密码修改成功')
    emit('success')
    handleClose()
  } catch (error: unknown) {
    const err = error as { response?: { data?: { error?: string } } }
    toast.error(err.response?.data?.error || '密码修改失败')
  } finally {
    loading.value = false
  }
}

function handleClose() {
  if (!isForced.value) {
    emit('update:modelValue', false)
    resetForm()
  }
}

function resetForm() {
  form.oldPassword = ''
  form.newPassword = ''
  form.confirmPassword = ''
  errors.oldPassword = ''
  errors.newPassword = ''
  errors.confirmPassword = ''
  passwordErrors.value = []
  passwordStrength.value = 'weak'
  showOldPassword.value = false
  showNewPassword.value = false
  showConfirmPassword.value = false
}

watch(visible, (newVal) => {
  if (newVal) {
    resetForm()
  }
})
</script>

<template>
  <Dialog v-model:open="visible">
    <DialogContent
      class="sm:max-w-[440px]"
      :show-close-button="!isForced"
      @interact-outside="(e) => { if (isForced) e.preventDefault() }"
      @escape-key-down="(e) => { if (isForced) e.preventDefault() }"
    >
      <DialogHeader>
        <DialogTitle>{{ isForced ? '首次登录 - 请修改密码' : '修改密码' }}</DialogTitle>
      </DialogHeader>

      <Alert v-if="isForced" variant="warning">
        <AlertTriangle class="size-4" />
        <AlertDescription>首次登录需要修改默认密码以确保账户安全，请先输入当前密码（默认为 admin）</AlertDescription>
      </Alert>

      <form class="space-y-4" @submit.prevent="handleSubmit">
        <div class="space-y-1.5">
          <Label for="old-password">{{ isForced ? '当前密码' : '旧密码' }}</Label>
          <div class="relative">
            <Input
              id="old-password"
              v-model="form.oldPassword"
              :type="showOldPassword ? 'text' : 'password'"
              :placeholder="isForced ? '请输入当前密码（默认为 admin）' : '请输入旧密码'"
              autocomplete="current-password"
              class="pr-9"
              @input="errors.oldPassword = ''"
            />
            <button
              type="button"
              class="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              @click="showOldPassword = !showOldPassword"
            >
              <EyeOff v-if="showOldPassword" class="size-4" />
              <Eye v-else class="size-4" />
            </button>
          </div>
          <p v-if="errors.oldPassword" class="text-xs text-destructive">{{ errors.oldPassword }}</p>
        </div>

        <div class="space-y-1.5">
          <Label for="new-password">新密码</Label>
          <div class="relative">
            <Input
              id="new-password"
              v-model="form.newPassword"
              :type="showNewPassword ? 'text' : 'password'"
              placeholder="请输入新密码"
              autocomplete="new-password"
              class="pr-9"
              @input="handlePasswordInput"
            />
            <button
              type="button"
              class="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              @click="showNewPassword = !showNewPassword"
            >
              <EyeOff v-if="showNewPassword" class="size-4" />
              <Eye v-else class="size-4" />
            </button>
          </div>
          <p v-if="errors.newPassword" class="text-xs text-destructive">{{ errors.newPassword }}</p>

          <div v-if="form.newPassword" class="pt-1 space-y-1.5">
            <div class="flex items-center gap-2">
              <span class="text-xs text-muted-foreground whitespace-nowrap">密码强度：</span>
              <Progress
                :model-value="strengthPercentage"
                class="h-1.5 max-w-[140px]"
                :indicator-style="{ backgroundColor: strengthColor }"
              />
              <span class="text-xs font-medium" :style="{ color: strengthColor }">
                {{ strengthLabel }}
              </span>
            </div>
            <div v-if="passwordErrors.length > 0" class="space-y-1">
              <div
                v-for="error in passwordErrors"
                :key="error"
                class="flex items-center gap-1.5 text-xs text-amber-600 dark:text-amber-400"
              >
                <AlertTriangle class="size-3.5 shrink-0" />
                <span>{{ error }}</span>
              </div>
            </div>
          </div>
        </div>

        <div class="space-y-1.5">
          <Label for="confirm-password">确认密码</Label>
          <div class="relative">
            <Input
              id="confirm-password"
              v-model="form.confirmPassword"
              :type="showConfirmPassword ? 'text' : 'password'"
              placeholder="请再次输入新密码"
              autocomplete="new-password"
              class="pr-9"
              @input="errors.confirmPassword = ''"
              @keyup.enter="handleSubmit"
            />
            <button
              type="button"
              class="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              @click="showConfirmPassword = !showConfirmPassword"
            >
              <EyeOff v-if="showConfirmPassword" class="size-4" />
              <Eye v-else class="size-4" />
            </button>
          </div>
          <p v-if="errors.confirmPassword" class="text-xs text-destructive">{{ errors.confirmPassword }}</p>
        </div>

        <DialogFooter class="gap-2 pt-2">
          <Button v-if="!isForced" type="button" variant="outline" @click="handleClose">取消</Button>
          <Button type="submit" :disabled="loading">
            <Loader2 v-if="loading" class="animate-spin" />
            {{ loading ? '提交中...' : '确认修改' }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>
