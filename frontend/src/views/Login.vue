<template>
  <div class="login-container">
    <div class="login-card">
      <div class="login-header">
        <h1 class="login-title">Rabbit Panel</h1>
        <p class="login-subtitle">Docker 容器运维面板</p>
      </div>

      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        class="login-form"
        @submit.prevent="handleLogin"
      >
        <el-form-item prop="username">
          <el-input
            v-model="form.username"
            placeholder="用户名"
            size="large"
            :prefix-icon="User"
            autocomplete="username"
          />
        </el-form-item>

        <el-form-item prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="密码"
            size="large"
            :prefix-icon="Lock"
            show-password
            autocomplete="current-password"
            @keyup.enter="handleLogin"
          />
        </el-form-item>

        <el-form-item prop="captcha" class="captcha-item">
          <el-input
            v-model="form.captcha"
            placeholder="验证码"
            size="large"
            class="captcha-input"
            @keyup.enter="handleLogin"
          />
          <img
            v-if="captchaImage"
            :src="captchaImage"
            alt="验证码"
            class="captcha-image"
            @click="loadCaptcha"
          />
          <el-icon v-else class="captcha-loading" :size="36"><Loading /></el-icon>
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            size="large"
            class="login-button"
            :loading="loading"
            @click="handleLogin"
          >
            {{ loading ? '登录中...' : '登录' }}
          </el-button>
        </el-form-item>
      </el-form>

      <p class="login-hint">初始账号密码：admin / admin</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { User, Lock, Loading } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import { useUpdateStore } from '@/stores/update'
import { authApi } from '@/api/auth'

const router = useRouter()
const authStore = useAuthStore()
const updateStore = useUpdateStore()

const formRef = ref<FormInstance>()
const loading = ref(false)
const captchaImage = ref('')
const captchaId = ref('')

const form = reactive({
  username: '',
  password: '',
  captcha: '',
})

const rules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
  ],
  captcha: [
    { required: true, message: '请输入验证码', trigger: 'blur' },
  ],
}

async function loadCaptcha() {
  try {
    const data = await authApi.getCaptcha()
    captchaId.value = data.captcha_id
    captchaImage.value = data.image
  } catch {
    ElMessage.error('加载验证码失败')
  }
}

async function handleLogin() {
  if (!formRef.value) return

  try {
    await formRef.value.validate()
  } catch {
    return
  }

  loading.value = true

  try {
    const success = await authStore.login(form.username, form.password, form.captcha, captchaId.value)

    if (success) {
      await updateStore.fetchUpdateInfo()
      ElMessage.success('登录成功')
      router.push('/')
    } else {
      ElMessage.error('用户名或密码错误')
      // 刷新验证码
      await loadCaptcha()
      form.captcha = ''
    }
  } catch (e: unknown) {
    const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error || '登录失败，请稍后重试'
    ElMessage.error(msg)
    // 刷新验证码
    await loadCaptcha()
    form.captcha = ''
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadCaptcha()
})
</script>

<style scoped>
.login-container {
  min-height: 100vh;
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  box-sizing: border-box;

  background-image:
    radial-gradient(circle at 50% 15%, rgba(230,220,255,0.2) 0%, rgba(170,140,250,0.1) 40%, transparent 65%),
    radial-gradient(circle at 50% 50%, rgba(255,255,255,0.015) 1px, transparent 1.5px),
    radial-gradient(circle at 50% 50%, rgba(220,205,255,0.02) 0.5px, transparent 1px),
    linear-gradient(to bottom, #120d20, #18112c);
  background-attachment: fixed;
  background-size: cover;
  background-repeat: repeat;
  background-blend-mode: normal;
}

.login-card {
  width: 100%;
  max-width: 400px;
  padding: 40px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  backdrop-filter: blur(10px);
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.login-header {
  text-align: center;
  margin-bottom: 32px;
}

.login-title {
  font-size: 28px;
  font-weight: 600;
  color: #fff;
  margin: 0 0 8px 0;
  text-shadow: 0 2px 4px rgba(0, 0, 0, 0.3);
}

.login-subtitle {
  font-size: 14px;
  color: rgba(255, 255, 255, 0.8);
  margin: 0;
}

.login-form {
  width: 100%;
}

.login-form :deep(.el-input__wrapper) {
  background: rgba(255, 255, 255, 0.15);
  box-shadow: none;
  border: 1px solid rgba(255, 255, 255, 0.3);
}

.login-form :deep(.el-input__wrapper:hover) {
  border-color: rgba(255, 255, 255, 0.5);
}

.login-form :deep(.el-input__wrapper.is-focus) {
  border-color: var(--el-color-primary);
}

.login-form :deep(.el-input__inner) {
  color: #fff;
}

.login-form :deep(.el-input__inner::placeholder) {
  color: rgba(255, 255, 255, 0.6);
}

.login-form :deep(.el-input__prefix) {
  color: rgba(255, 255, 255, 0.8);
}

.captcha-item :deep(.el-form-item__content) {
  display: flex;
  gap: 10px;
}

.captcha-input {
  flex: 1;
}

.captcha-image {
  height: 40px;
  border-radius: 6px;
  cursor: pointer;
  border: 1px solid rgba(255, 255, 255, 0.3);
  transition: border-color 0.2s;
  object-fit: contain;
  object-position: center;
  background: rgba(255, 255, 255, 0.95);
}

.captcha-image:hover {
  border-color: rgba(255, 255, 255, 0.6);
}

.captcha-loading {
  display: flex;
  align-items: center;
  color: rgba(255, 255, 255, 0.6);
}

.login-button {
  width: 100%;
  margin-top: 8px;
}

.login-hint {
  text-align: center;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.5);
  margin: 16px 0 0 0;
}
</style>
