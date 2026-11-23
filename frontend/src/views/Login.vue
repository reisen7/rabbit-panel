<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Box, Eye, EyeOff, Loader2, Lock, User } from 'lucide-vue-next'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/composables/useToast'
import { authApi } from '@/api/auth'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import mikuDark from '@/assets/miku-dark.svg'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const toast = useToast()

const loading = ref(false)
const showPassword = ref(false)
const captchaImage = ref('')
const captchaId = ref('')

const form = reactive({
  username: '',
  password: '',
  captcha: '',
})

const errors = reactive({
  username: '',
  password: '',
  captcha: '',
})

const floatingContainers = [
  { name: 'nginx:alpine', port: '80:80', pos: 'top-[14%] left-[8%]', delay: '0s', duration: '7s' },
  { name: 'redis:7-alpine', port: '6379', pos: 'top-[22%] right-[9%]', delay: '1.2s', duration: '8.5s' },
  { name: 'postgres:16', port: '5432', pos: 'bottom-[20%] left-[10%]', delay: '2.1s', duration: '9s' },
  { name: 'rabbit-worker', port: '3958', pos: 'bottom-[18%] right-[10%]', delay: '0.7s', duration: '7.8s' },
]

function validate(): boolean {
  errors.username = form.username ? '' : '请输入用户名'
  errors.password = form.password ? '' : '请输入密码'
  errors.captcha = form.captcha ? '' : '请输入验证码'
  return !errors.username && !errors.password && !errors.captcha
}

async function loadCaptcha() {
  try {
    const data = await authApi.getCaptcha()
    captchaId.value = data.captcha_id
    captchaImage.value = data.image
  } catch {
    toast.error('加载验证码失败')
  }
}

async function handleLogin() {
  if (loading.value || !validate()) return

  loading.value = true
  try {
    const success = await authStore.login(form.username, form.password, form.captcha, captchaId.value)

    if (success) {
      toast.success('登录成功')
      const redirect = route.query.redirect
      router.push(typeof redirect === 'string' && redirect.startsWith('/') ? redirect : '/')
    } else {
      toast.error('用户名或密码错误')
      await loadCaptcha()
      form.captcha = ''
    }
  } catch (e: unknown) {
    const msg =
      (e as { response?: { data?: { error?: string } } })?.response?.data?.error ||
      '登录失败，请稍后重试'
    toast.error(msg)
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

<template>
  <div class="dark relative min-h-screen w-full flex items-center justify-center p-4 bg-[#09090b] text-zinc-50 overflow-hidden">
    <!-- Subtle architectural grid -->
    <div
      class="pointer-events-none absolute inset-0 opacity-[0.18]"
      style="background-image: linear-gradient(to right, rgba(255,255,255,0.08) 1px, transparent 1px), linear-gradient(to bottom, rgba(255,255,255,0.08) 1px, transparent 1px); background-size: 48px 48px;"
    />

    <!-- Radial mask + subtle Docker sky glow -->
    <div
      class="pointer-events-none absolute inset-0"
      style="background: radial-gradient(circle at 50% 42%, rgba(36, 150, 237, 0.14), rgba(9, 9, 11, 0.25) 45%, #09090b 80%);"
    />

    <!-- Animated network mesh connecting containers (desktop) -->
    <svg
      class="pointer-events-none hidden md:block absolute inset-0 size-full opacity-35"
      viewBox="0 0 1200 800"
      preserveAspectRatio="none"
      fill="none"
    >
      <path
        d="M150 150 C 320 200, 420 340, 600 400"
        stroke="url(#docker-line-grad)"
        stroke-width="1.25"
        stroke-dasharray="6 6"
        class="network-stream"
      />
      <path
        d="M1040 190 C 860 240, 740 340, 600 400"
        stroke="url(#docker-line-grad)"
        stroke-width="1.25"
        stroke-dasharray="6 6"
        class="network-stream network-stream-reverse"
      />
      <path
        d="M170 630 C 340 560, 460 470, 600 400"
        stroke="url(#docker-line-grad)"
        stroke-width="1.25"
        stroke-dasharray="6 6"
        class="network-stream"
      />
      <path
        d="M1030 640 C 850 560, 730 470, 600 400"
        stroke="url(#docker-line-grad)"
        stroke-width="1.25"
        stroke-dasharray="6 6"
        class="network-stream network-stream-reverse"
      />
      <defs>
        <linearGradient id="docker-line-grad" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stop-color="#38bdf8" stop-opacity="0.15" />
          <stop offset="50%" stop-color="#38bdf8" stop-opacity="0.7" />
          <stop offset="100%" stop-color="#38bdf8" stop-opacity="0.15" />
        </linearGradient>
      </defs>
    </svg>

    <!-- Floating Docker container nodes -->
    <div
      v-for="node in floatingContainers"
      :key="node.name"
      class="pointer-events-none hidden md:flex items-center gap-2.5 absolute rounded-lg border border-zinc-800/90 bg-zinc-900/75 backdrop-blur-md px-3 py-2 shadow-lg container-float"
      :class="node.pos"
      :style="{ animationDelay: node.delay, animationDuration: node.duration }"
    >
      <span class="relative flex size-2">
        <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
        <span class="relative inline-flex rounded-full size-2 bg-emerald-500" />
      </span>
      <Box class="size-3.5 text-sky-400" />
      <div class="leading-tight">
        <div class="text-[11px] font-mono font-medium text-zinc-200">{{ node.name }}</div>
        <div class="text-[10px] font-mono text-zinc-500">:{{ node.port }}</div>
      </div>
    </div>

    <!-- Subtle animated Docker container-ship / whale wave silhouette at bottom -->
    <div class="pointer-events-none fixed inset-x-0 bottom-0 h-28 overflow-hidden opacity-30">
      <div class="absolute bottom-6 left-1/2 -translate-x-1/2 whale-bob">
        <svg width="160" height="64" viewBox="0 0 160 64" fill="none" class="text-sky-400/50">
          <!-- Stacked containers -->
          <rect x="44" y="14" width="16" height="12" rx="2" stroke="currentColor" stroke-width="1.4" class="cube-pulse" />
          <rect x="63" y="14" width="16" height="12" rx="2" stroke="currentColor" stroke-width="1.4" class="cube-pulse cube-delay-1" />
          <rect x="82" y="14" width="16" height="12" rx="2" stroke="currentColor" stroke-width="1.4" class="cube-pulse cube-delay-2" />
          <rect x="53" y="1" width="16" height="11" rx="2" stroke="currentColor" stroke-width="1.4" class="cube-pulse cube-delay-2" />
          <rect x="72" y="1" width="16" height="11" rx="2" stroke="currentColor" stroke-width="1.4" class="cube-pulse cube-delay-1" />
          <!-- Stylized hull -->
          <path
            d="M28 30 H118 C122 30 128 24 134 21 C134 28 138 33 144 35 C135 45 122 52 96 52 H56 C38 52 28 43 28 30 Z"
            stroke="currentColor"
            stroke-width="1.5"
            fill="rgba(56, 189, 248, 0.06)"
          />
          <circle cx="114" cy="39" r="2" fill="currentColor" />
        </svg>
      </div>
      <svg class="absolute bottom-0 w-[200%] h-10 wave-slide" viewBox="0 0 1200 40" preserveAspectRatio="none">
        <path
          d="M0 20 Q 150 5 300 20 T 600 20 T 900 20 T 1200 20 L 1200 40 L 0 40 Z"
          fill="rgba(56, 189, 248, 0.08)"
        />
      </svg>
    </div>

    <!-- Login Card -->
    <Card class="relative z-10 w-full max-w-[400px] border-zinc-800/90 bg-zinc-900/85 backdrop-blur-xl shadow-2xl card-enter transition-colors focus-within:border-zinc-700">
      <CardHeader class="text-center space-y-2.5">
        <div class="mx-auto relative">
          <div class="size-12 rounded-xl border border-zinc-700/80 bg-zinc-800/80 flex items-center justify-center shadow-inner">
            <img :src="mikuDark" alt="Rabbit Panel" class="size-7" />
          </div>
          <span
            class="absolute -bottom-1 -right-1 size-3.5 rounded-full bg-zinc-900 flex items-center justify-center border border-zinc-800"
          >
            <span class="size-2 rounded-full bg-emerald-500 animate-pulse" />
          </span>
        </div>

        <div class="space-y-1">
          <CardTitle class="text-2xl font-bold tracking-tight text-zinc-50">Rabbit Panel</CardTitle>
          <CardDescription class="text-zinc-400">Docker 容器运维面板</CardDescription>
        </div>

        <!-- Live container-stack indicator pill -->
        <div class="mx-auto inline-flex items-center gap-2 rounded-full border border-zinc-800 bg-zinc-950/70 px-3 py-1 text-[11px] font-mono text-zinc-400">
          <span class="inline-flex items-end gap-0.5 h-3">
            <span class="w-1 bg-sky-400/80 rounded-xs stack-bar-1" />
            <span class="w-1 bg-sky-400/80 rounded-xs stack-bar-2" />
            <span class="w-1 bg-sky-400/80 rounded-xs stack-bar-3" />
          </span>
          <span>dockerd · ready</span>
        </div>
      </CardHeader>

      <CardContent>
        <form class="space-y-4" @submit.prevent="handleLogin">
          <div class="space-y-1.5">
            <Label for="login-username" class="text-zinc-200">用户名</Label>
            <div class="relative">
              <User class="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-zinc-400 pointer-events-none" />
              <Input
                id="login-username"
                v-model="form.username"
                placeholder="请输入用户名"
                autocomplete="username"
                class="pl-9 bg-zinc-950/60 border-zinc-800 text-zinc-100 placeholder:text-zinc-500"
                @input="errors.username = ''"
              />
            </div>
            <p v-if="errors.username" class="text-xs text-destructive">{{ errors.username }}</p>
          </div>

          <div class="space-y-1.5">
            <Label for="login-password" class="text-zinc-200">密码</Label>
            <div class="relative">
              <Lock class="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-zinc-400 pointer-events-none" />
              <Input
                id="login-password"
                v-model="form.password"
                :type="showPassword ? 'text' : 'password'"
                placeholder="请输入密码"
                autocomplete="current-password"
                class="pl-9 pr-9 bg-zinc-950/60 border-zinc-800 text-zinc-100 placeholder:text-zinc-500"
                @input="errors.password = ''"
              />
              <button
                type="button"
                class="absolute right-2.5 top-1/2 -translate-y-1/2 text-zinc-400 hover:text-zinc-100 transition-colors"
                @click="showPassword = !showPassword"
              >
                <EyeOff v-if="showPassword" class="size-4" />
                <Eye v-else class="size-4" />
              </button>
            </div>
            <p v-if="errors.password" class="text-xs text-destructive">{{ errors.password }}</p>
          </div>

          <div class="space-y-1.5">
            <Label for="login-captcha" class="text-zinc-200">验证码</Label>
            <div class="flex items-center gap-2.5">
              <Input
                id="login-captcha"
                v-model="form.captcha"
                placeholder="验证码"
                class="flex-1 font-mono bg-zinc-950/60 border-zinc-800 text-zinc-100 placeholder:text-zinc-500"
                @input="errors.captcha = ''"
              />
              <button
                v-if="captchaImage"
                type="button"
                class="h-9 rounded-md border border-zinc-700 bg-white px-1 overflow-hidden hover:border-sky-400 transition-colors cursor-pointer shrink-0"
                title="点击刷新验证码"
                @click="loadCaptcha"
              >
                <img :src="captchaImage" alt="验证码" class="h-full w-auto object-contain" />
              </button>
              <div v-else class="h-9 w-24 rounded-md border border-zinc-800 flex items-center justify-center bg-zinc-950/60">
                <Loader2 class="size-4 animate-spin text-zinc-500" />
              </div>
            </div>
            <p v-if="errors.captcha" class="text-xs text-destructive">{{ errors.captcha }}</p>
          </div>

          <Button type="submit" class="w-full font-medium" size="lg" :disabled="loading">
            <Loader2 v-if="loading" class="animate-spin" />
            {{ loading ? '登录中...' : '登录' }}
          </Button>
        </form>
      </CardContent>

      <CardFooter class="justify-center border-t border-zinc-800/80 pt-4">
      </CardFooter>
    </Card>
  </div>
</template>

<style scoped>
@keyframes card-enter {
  from {
    opacity: 0;
    transform: translateY(14px) scale(0.985);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

@keyframes container-float {
  0%, 100% {
    transform: translateY(0px);
  }
  50% {
    transform: translateY(-10px);
  }
}

@keyframes network-dash {
  to {
    stroke-dashoffset: -48;
  }
}

@keyframes whale-bob {
  0%, 100% {
    transform: translateY(0px) rotate(-0.6deg);
  }
  50% {
    transform: translateY(-6px) rotate(0.6deg);
  }
}

@keyframes wave-slide {
  0% {
    transform: translateX(0);
  }
  100% {
    transform: translateX(-50%);
  }
}

@keyframes cube-pulse {
  0%, 100% {
    opacity: 0.45;
  }
  50% {
    opacity: 1;
  }
}

@keyframes stack-bar {
  0%, 100% {
    height: 4px;
    opacity: 0.5;
  }
  50% {
    height: 11px;
    opacity: 1;
  }
}

.card-enter {
  animation: card-enter 0.45s cubic-bezier(0.16, 1, 0.3, 1) both;
}

.container-float {
  animation-name: container-float;
  animation-timing-function: ease-in-out;
  animation-iteration-count: infinite;
}

.network-stream {
  animation: network-dash 4s linear infinite;
}

.network-stream-reverse {
  animation-direction: reverse;
}

.whale-bob {
  animation: whale-bob 5.5s ease-in-out infinite;
}

.wave-slide {
  animation: wave-slide 14s linear infinite;
}

.cube-pulse {
  animation: cube-pulse 3s ease-in-out infinite;
}

.cube-delay-1 {
  animation-delay: 0.7s;
}

.cube-delay-2 {
  animation-delay: 1.4s;
}

.stack-bar-1 {
  animation: stack-bar 1.6s ease-in-out infinite;
}

.stack-bar-2 {
  animation: stack-bar 1.6s ease-in-out 0.25s infinite;
}

.stack-bar-3 {
  animation: stack-bar 1.6s ease-in-out 0.5s infinite;
}

@media (prefers-reduced-motion: reduce) {
  .card-enter,
  .container-float,
  .network-stream,
  .whale-bob,
  .wave-slide,
  .cube-pulse,
  .stack-bar-1,
  .stack-bar-2,
  .stack-bar-3 {
    animation: none !important;
  }
}
</style>
