<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  Bell,
  BookOpen,
  CheckCircle2,
  Clock,
  Crosshair,
  Eye,
  EyeOff,
  Filter,
  KeyRound,
  Loader2,
  Lock,
  Plus,
  Send,
  X,
} from 'lucide-vue-next'
import { useI18n } from '@/composables/useI18n'
import { useToast } from '@/composables/useToast'
import { containerApi } from '@/api/containers'
import { getNotifySettings, saveNotifySettings, testNotify, type NotifySettings } from '@/api/notify'
import ShowdocGuideDialog from '@/components/notify/ShowdocGuideDialog.vue'
import LoadingOverlay from '@/components/common/LoadingOverlay.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

const { t } = useI18n()
const toast = useToast()

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const showGuide = ref(false)
const showToken = ref(false)
const customContainerInput = ref('')
const containerNames = ref<string[]>([])
const saved = ref<NotifySettings>(emptySettings())
const form = ref<NotifySettings>(emptySettings())

const tokenPlaceholder = computed(() =>
  saved.value.has_token ? t('notify.tokenKeep') : t('notify.tokenPlaceholder')
)

const containerOptions = computed(() => {
  const names = [...containerNames.value]
  for (const name of form.value.containers) {
    if (name && !names.includes(name)) names.push(name)
  }
  return names
})

function emptySettings(): NotifySettings {
  return {
    enabled: false,
    token: '',
    token_mask: '',
    has_token: false,
    all_containers: true,
    containers: [],
    exit_nonzero: true,
    unhealthy: true,
    restart_storm: true,
    recovery: true,
    cooldown_minutes: 10,
  }
}

function errorText(error: unknown, fallback: string) {
  const data = (error as { response?: { data?: { error?: string } } })?.response?.data?.error
  return data || fallback
}

function toggleContainer(name: string) {
  const idx = form.value.containers.indexOf(name)
  if (idx >= 0) {
    form.value.containers.splice(idx, 1)
  } else {
    form.value.containers.push(name)
  }
}

function addCustomContainer() {
  const name = customContainerInput.value.trim()
  if (!name) return
  if (!form.value.containers.includes(name)) {
    form.value.containers.push(name)
  }
  customContainerInput.value = ''
}

async function loadContainers() {
  try {
    const list = await containerApi.list()
    containerNames.value = list.map((item) => item.name).filter(Boolean)
  } catch {
    containerNames.value = []
  }
}

async function loadSettings() {
  loading.value = true
  try {
    const { data } = await getNotifySettings()
    saved.value = { ...emptySettings(), ...data, token: '', containers: data.containers || [] }
    form.value = { ...saved.value, token: '', containers: [...(saved.value.containers || [])] }
  } catch (error) {
    toast.error(errorText(error, t('notify.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    const { data } = await saveNotifySettings({ ...form.value, containers: form.value.containers || [] })
    saved.value = { ...emptySettings(), ...data, token: '', containers: data.containers || [] }
    form.value = { ...saved.value, token: '', containers: [...(saved.value.containers || [])] }
    toast.success(t('notify.saved'))
  } catch (error) {
    toast.error(errorText(error, t('notify.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function handleTest() {
  testing.value = true
  try {
    await testNotify(form.value.token || '')
    toast.success(t('notify.testSent'))
  } catch (error) {
    toast.error(errorText(error, t('notify.testFailed')))
  } finally {
    testing.value = false
  }
}

onMounted(() => {
  loadSettings()
  loadContainers()
})
</script>

<template>
  <div class="relative max-w-[920px] mx-auto space-y-5">
    <LoadingOverlay :loading="loading" />

    <!-- Hero -->
    <Card>
      <CardContent class="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div class="flex items-start gap-3.5">
          <div class="size-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center shrink-0">
            <Bell class="size-5" />
          </div>
          <div class="space-y-1">
            <h1 class="text-lg font-semibold tracking-tight">{{ t('notify.title') }}</h1>
            <p class="text-xs text-muted-foreground leading-relaxed">{{ t('notify.pageIntro') }}</p>
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-3 shrink-0">
          <div class="flex items-center gap-2.5 rounded-lg border px-3 py-1.5 bg-muted/30">
            <Label for="notify-enabled" class="text-xs font-medium cursor-pointer">{{ t('notify.enabled') }}</Label>
            <Switch id="notify-enabled" v-model="form.enabled" />
          </div>
          <Button variant="outline" size="sm" @click="showGuide = true">
            <BookOpen />
            {{ t('notify.openGuide') }}
          </Button>
        </div>
      </CardContent>
    </Card>

    <!-- Token -->
    <Card>
      <CardHeader>
        <div class="flex items-start gap-3">
          <div class="size-8 rounded-lg bg-muted flex items-center justify-center shrink-0 mt-0.5">
            <KeyRound class="size-4 text-muted-foreground" />
          </div>
          <div>
            <CardTitle class="text-base">{{ t('notify.token') }}</CardTitle>
            <CardDescription class="mt-1">{{ t('notify.tokenTip') }}</CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent class="space-y-3">
        <div class="relative max-w-xl">
          <Lock class="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
          <Input
            v-model="form.token"
            :type="showToken ? 'text' : 'password'"
            :placeholder="tokenPlaceholder"
            class="pl-9 pr-9 font-mono"
          />
          <button
            type="button"
            class="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
            @click="showToken = !showToken"
          >
            <EyeOff v-if="showToken" class="size-4" />
            <Eye v-else class="size-4" />
          </button>
        </div>

        <div v-if="saved.has_token" class="inline-flex items-center gap-2 rounded-md border bg-muted/40 px-3 py-1.5 text-xs">
          <CheckCircle2 class="size-3.5 text-emerald-600 dark:text-emerald-400" />
          <span class="text-muted-foreground">{{ t('notify.tokenCurrent') }}</span>
          <code class="font-mono font-semibold">{{ saved.token_mask }}</code>
        </div>
      </CardContent>
    </Card>

    <!-- Scope -->
    <Card>
      <CardHeader>
        <div class="flex items-start gap-3">
          <div class="size-8 rounded-lg bg-muted flex items-center justify-center shrink-0 mt-0.5">
            <Crosshair class="size-4 text-muted-foreground" />
          </div>
          <div>
            <CardTitle class="text-base">{{ t('notify.scope') }}</CardTitle>
            <CardDescription class="mt-1">{{ t('notify.containersTip') }}</CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent class="space-y-4">
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <button
            type="button"
            class="flex items-center gap-2.5 rounded-lg border px-4 py-3 text-left text-sm font-medium transition-colors cursor-pointer"
            :class="form.all_containers
              ? 'border-primary bg-primary/5 text-foreground'
              : 'hover:bg-muted/50 text-muted-foreground'"
            @click="form.all_containers = true"
          >
            <span
              class="size-4 rounded-full border flex items-center justify-center shrink-0"
              :class="form.all_containers ? 'border-primary' : 'border-muted-foreground/40'"
            >
              <span v-if="form.all_containers" class="size-2 rounded-full bg-primary" />
            </span>
            <span>{{ t('notify.allContainers') }}</span>
          </button>

          <button
            type="button"
            class="flex items-center gap-2.5 rounded-lg border px-4 py-3 text-left text-sm font-medium transition-colors cursor-pointer"
            :class="!form.all_containers
              ? 'border-primary bg-primary/5 text-foreground'
              : 'hover:bg-muted/50 text-muted-foreground'"
            @click="form.all_containers = false"
          >
            <span
              class="size-4 rounded-full border flex items-center justify-center shrink-0"
              :class="!form.all_containers ? 'border-primary' : 'border-muted-foreground/40'"
            >
              <span v-if="!form.all_containers" class="size-2 rounded-full bg-primary" />
            </span>
            <span>{{ t('notify.selectedContainers') }}</span>
          </button>
        </div>

        <div v-if="!form.all_containers" class="space-y-3 rounded-lg border bg-muted/20 p-3.5">
          <div v-if="form.containers.length > 0" class="flex flex-wrap gap-1.5">
            <Badge
              v-for="name in form.containers"
              :key="name"
              variant="secondary"
              class="gap-1 font-mono"
            >
              {{ name }}
              <button type="button" class="hover:text-destructive" @click="toggleContainer(name)">
                <X class="size-3" />
              </button>
            </Badge>
          </div>

          <div class="flex gap-2">
            <Input
              v-model="customContainerInput"
              :placeholder="t('notify.containersPlaceholder')"
              class="h-8 text-xs font-mono bg-background"
              @keyup.enter.prevent="addCustomContainer"
            />
            <Button type="button" variant="outline" size="sm" @click="addCustomContainer">
              <Plus />
              {{ t('common.create') }}
            </Button>
          </div>

          <div v-if="containerOptions.length > 0" class="flex flex-wrap gap-2 pt-1">
            <button
              v-for="name in containerOptions"
              :key="name"
              type="button"
              class="inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1 text-xs font-mono transition-colors cursor-pointer"
              :class="form.containers.includes(name)
                ? 'bg-primary text-primary-foreground border-primary'
                : 'bg-background hover:bg-muted'"
              @click="toggleContainer(name)"
            >
              {{ name }}
            </button>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Rules -->
    <Card>
      <CardHeader>
        <div class="flex items-start gap-3">
          <div class="size-8 rounded-lg bg-muted flex items-center justify-center shrink-0 mt-0.5">
            <Filter class="size-4 text-muted-foreground" />
          </div>
          <div>
            <CardTitle class="text-base">{{ t('notify.rules') }}</CardTitle>
            <CardDescription class="mt-1">{{ t('notify.rulesTip') }}</CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <label class="flex items-center gap-2.5 rounded-lg border p-3 cursor-pointer hover:bg-muted/40 transition-colors">
            <Checkbox v-model="form.exit_nonzero" />
            <span class="text-sm">{{ t('notify.ruleExit') }}</span>
          </label>
          <label class="flex items-center gap-2.5 rounded-lg border p-3 cursor-pointer hover:bg-muted/40 transition-colors">
            <Checkbox v-model="form.unhealthy" />
            <span class="text-sm">{{ t('notify.ruleUnhealthy') }}</span>
          </label>
          <label class="flex items-center gap-2.5 rounded-lg border p-3 cursor-pointer hover:bg-muted/40 transition-colors">
            <Checkbox v-model="form.restart_storm" />
            <span class="text-sm">{{ t('notify.ruleRestart') }}</span>
          </label>
          <label class="flex items-center gap-2.5 rounded-lg border p-3 cursor-pointer hover:bg-muted/40 transition-colors">
            <Checkbox v-model="form.recovery" />
            <span class="text-sm">{{ t('notify.ruleRecovery') }}</span>
          </label>
        </div>
      </CardContent>
    </Card>

    <!-- Cooldown -->
    <Card>
      <CardHeader>
        <div class="flex items-start gap-3">
          <div class="size-8 rounded-lg bg-muted flex items-center justify-center shrink-0 mt-0.5">
            <Clock class="size-4 text-muted-foreground" />
          </div>
          <div>
            <CardTitle class="text-base">{{ t('notify.cooldown') }}</CardTitle>
            <CardDescription class="mt-1">{{ t('notify.cooldownTip') }}</CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div class="flex items-center gap-2.5">
          <Input
            type="number"
            min="1"
            max="1440"
            :model-value="form.cooldown_minutes"
            class="w-32 font-mono tabular-nums"
            @update:model-value="(v) => form.cooldown_minutes = Math.min(1440, Math.max(1, Number(v) || 1))"
          />
          <span class="text-xs text-muted-foreground">min (1 - 1440)</span>
        </div>
      </CardContent>
    </Card>

    <!-- Actions -->
    <div class="flex flex-wrap justify-end gap-3 pt-1">
      <Button variant="outline" :disabled="loading || testing" @click="handleTest">
        <Loader2 v-if="testing" class="animate-spin" />
        <Send v-else />
        {{ t('notify.test') }}
      </Button>
      <Button :disabled="testing || saving" @click="handleSave">
        <Loader2 v-if="saving" class="animate-spin" />
        {{ t('common.save') }}
      </Button>
    </div>

    <ShowdocGuideDialog v-model="showGuide" />
  </div>
</template>
