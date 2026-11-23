<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { AlertTriangle, ChevronDown, Eye, EyeOff, Info, Loader2, RefreshCw } from 'lucide-vue-next'
import {
  getAgentConfig,
  listAgentModels,
  saveAgentConfig,
  testAgentConnection,
  type AgentConfig,
} from '@/api/agent'
import { getWeChatStatus, unbindWeChat, type WeChatStatus } from '@/api/wechat'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'
import WeChatQRPanel from '@/components/common/WeChatQRPanel.vue'
import LoadingOverlay from '@/components/common/LoadingOverlay.vue'
import { Alert, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
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
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

const { t } = useI18n()
const toast = useToast()
const { confirm } = useConfirm()

const loading = ref(false)
const testing = ref(false)
const modelsLoading = ref(false)
const modelOptions = ref<string[]>([])
const form = ref<AgentConfig>({
  api_url: 'https://api.openai.com/v1',
  api_key: '',
  api_format: 'openai_chat',
  model: 'gpt-3.5-turbo',
  enabled: false,
  thinking: false,
  thinking_effort: 'medium',
})
const showKeyDialog = ref(false)
const newApiKey = ref('')
const showNewKey = ref(false)

// 主流大模型服务商预设：选择后自动填充 API 地址、接口格式与默认模型
interface ProviderPreset {
  label: string
  api_url: string
  api_format: AgentConfig['api_format']
  model: string
}
const providerPresets: ProviderPreset[] = [
  { label: 'OpenAI', api_url: 'https://api.openai.com/v1', api_format: 'openai_chat', model: 'gpt-4o-mini' },
  { label: 'Anthropic', api_url: 'https://api.anthropic.com', api_format: 'anthropic', model: 'claude-sonnet-4-5' },
  { label: 'Gemini', api_url: 'https://generativelanguage.googleapis.com/v1beta/openai', api_format: 'openai_chat', model: 'gemini-2.5-flash' },
  { label: 'Grok (xAI)', api_url: 'https://api.x.ai/v1', api_format: 'openai_chat', model: 'grok-4' },
  { label: 'DeepSeek', api_url: 'https://api.deepseek.com/v1', api_format: 'openai_chat', model: 'deepseek-chat' },
  { label: 'GLM (智谱)', api_url: 'https://open.bigmodel.cn/api/paas/v4', api_format: 'openai_chat', model: 'glm-4.5' },
]
const providerPreset = ref('')
function applyProviderPreset(value: unknown) {
  const preset = providerPresets.find((p) => p.label === String(value))
  providerPreset.value = '' // 一次性填充动作，不持久选中
  if (!preset) return
  form.value.api_url = preset.api_url
  form.value.api_format = preset.api_format
  form.value.model = preset.model
  modelOptions.value = []
  toast.success(t('agent.providerApplied').replace('{name}', preset.label))
}

const wechatLoading = ref(false)
const wechatLoaded = ref(false)
const wechat = ref<WeChatStatus>({
  linked: false,
  token_mask: '',
  account_id: '',
  saved_at: '',
  online: false,
  session_expired: false,
})

const displayApiKey = computed(() => form.value.api_key || t('agent.notSet'))
const modelChoices = computed(() => {
  const names = [...modelOptions.value]
  if (form.value.model && !names.includes(form.value.model)) {
    names.unshift(form.value.model)
  }
  return names
})

const loadConfig = async () => {
  loading.value = true
  try {
    const res = await getAgentConfig()
    const data = res.data
    form.value = {
      api_url: data.api_url || 'https://api.openai.com/v1',
      api_key: data.api_key || '',
      api_format: data.api_format || 'openai_chat',
      model: data.model || 'gpt-3.5-turbo',
      enabled: !!data.enabled,
      thinking: !!data.thinking,
      thinking_effort: data.thinking_effort || 'medium',
    }
  } catch (error) {
    console.error(error)
  } finally {
    loading.value = false
  }
}

const loadWeChat = async () => {
  wechatLoading.value = true
  try {
    const res = await getWeChatStatus()
    wechat.value = res.data
  } catch (error) {
    console.error(error)
  } finally {
    wechatLoading.value = false
    wechatLoaded.value = true
  }
}

const handleSave = async () => {
  loading.value = true
  try {
    await saveAgentConfig({
      ...form.value,
      api_key: '',
    })
    await loadConfig()
    toast.success(t('agent.saveSuccess'))
  } catch (error) {
    console.error(error)
  } finally {
    loading.value = false
  }
}

const handleTest = async () => {
  const url = form.value.api_url.trim()
  const format = form.value.api_format
  const model = form.value.model.trim()
  const key = newApiKey.value.trim() || form.value.api_key.trim()
  if (!url || !format || !key || !model) {
    toast.warning(t('agent.testNeed'))
    return
  }
  testing.value = true
  try {
    const res = await testAgentConnection({
      api_url: url,
      api_format: format,
      api_key: key,
      model,
    })
    const reply = (res.data.reply || '').trim()
    if (reply) {
      toast.success(t('agent.testSuccessReply', { reply }))
    } else {
      toast.success(t('agent.testSuccess'))
    }
  } catch (error) {
    console.error(error)
  } finally {
    testing.value = false
  }
}

const handleLoadModels = async () => {
  const url = form.value.api_url.trim()
  const format = form.value.api_format
  const typedKey = newApiKey.value.trim()
  const savedKey = form.value.api_key.trim()
  const key = typedKey || savedKey
  if (!url || !format || !key) {
    toast.warning(t('agent.loadModelsNeed'))
    return
  }
  modelsLoading.value = true
  try {
    const res = await listAgentModels({
      api_url: url,
      api_format: format,
      api_key: key,
    })
    const models = res.data.models || []
    modelOptions.value = models
    if (models.length === 0) {
      toast.warning(t('agent.loadModelsEmpty'))
      return
    }
    toast.success(t('agent.loadModelsSuccess'))
  } catch (error) {
    console.error(error)
  } finally {
    modelsLoading.value = false
  }
}

const handleCancelKeyDialog = () => {
  showKeyDialog.value = false
  newApiKey.value = ''
  showNewKey.value = false
}

const handleConfirmKeyChange = async () => {
  const key = newApiKey.value.trim()
  if (!key) return
  loading.value = true
  try {
    await saveAgentConfig({
      ...form.value,
      api_key: key,
    })
    await loadConfig()
    showKeyDialog.value = false
    newApiKey.value = ''
    showNewKey.value = false
    toast.success(t('agent.saveSuccess'))
  } catch (error) {
    console.error(error)
  } finally {
    loading.value = false
  }
}

const handleUnbind = async () => {
  const ok = await confirm({
    title: t('common.confirm'),
    description: t('agent.wechat.unbindConfirm'),
    confirmText: t('common.confirm'),
    cancelText: t('common.cancel'),
    destructive: true,
  })
  if (!ok) return

  try {
    await unbindWeChat()
    await loadWeChat()
    toast.success(t('agent.wechat.unbound'))
  } catch (error) {
    console.error(error)
  }
}

let wechatTimer: ReturnType<typeof setInterval> | undefined

onMounted(() => {
  loadConfig()
  loadWeChat()
  wechatTimer = setInterval(() => {
    if (wechat.value.linked) {
      loadWeChat()
    }
  }, 15000)
})

onUnmounted(() => {
  if (wechatTimer) {
    clearInterval(wechatTimer)
  }
})
</script>

<template>
  <TooltipProvider>
    <div class="max-w-[1180px] mx-auto">
      <div class="grid grid-cols-1 lg:grid-cols-[1.15fr_0.85fr] gap-5 items-start">
        <!-- Agent Config Card -->
        <Card class="relative">
          <LoadingOverlay :loading="loading" />
          <CardHeader>
            <CardTitle class="text-base">{{ t('agent.settingsTitle') }}</CardTitle>
          </CardHeader>

          <CardContent class="space-y-5">
            <div class="flex items-center justify-between rounded-lg border p-3.5">
              <Label for="agent-enable">{{ t('agent.enable') }}</Label>
              <Switch id="agent-enable" v-model="form.enabled" />
            </div>

            <div class="space-y-1.5">
              <Label>{{ t('agent.provider') }}</Label>
              <Select
                :model-value="providerPreset"
                @update:model-value="applyProviderPreset"
              >
                <SelectTrigger class="w-full">
                  <SelectValue :placeholder="t('agent.providerPlaceholder')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="p in providerPresets" :key="p.label" :value="p.label">
                    {{ p.label }}
                  </SelectItem>
                </SelectContent>
              </Select>
              <p class="text-xs text-muted-foreground">{{ t('agent.providerHint') }}</p>
            </div>

            <div class="space-y-1.5">
              <Label for="agent-api-url">{{ t('agent.apiUrl') }}</Label>
              <Input
                id="agent-api-url"
                v-model="form.api_url"
                placeholder="https://api.openai.com/v1"
                class="font-mono text-xs"
              />
              <p class="text-xs text-muted-foreground leading-relaxed">{{ t('agent.apiTip') }}</p>
            </div>

            <div class="space-y-1.5">
              <Label>{{ t('agent.format') }}</Label>
              <Select v-model="form.api_format">
                <SelectTrigger class="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="openai_chat">{{ t('agent.formatOpenAIChat') }}</SelectItem>
                  <SelectItem value="openai_responses">{{ t('agent.formatOpenAIResponses') }}</SelectItem>
                  <SelectItem value="anthropic">{{ t('agent.formatAnthropic') }}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div class="space-y-1.5">
              <Label>{{ t('agent.apiKey') }}</Label>
              <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 rounded-lg border bg-muted/30 px-3 py-2">
                <span class="font-mono text-xs break-all text-foreground">{{ displayApiKey }}</span>
                <Button variant="outline" size="sm" @click="showKeyDialog = true">
                  {{ t('agent.changeKey') }}
                </Button>
              </div>
            </div>

            <div class="space-y-1.5">
              <Label for="agent-model">{{ t('agent.model') }}</Label>
              <div class="flex items-center gap-2">
                <Input
                  id="agent-model"
                  v-model="form.model"
                  :placeholder="t('agent.modelPlaceholder')"
                  class="flex-1 font-mono text-xs"
                />
                <DropdownMenu v-if="modelChoices.length > 0">
                  <DropdownMenuTrigger as-child>
                    <Button variant="outline" size="icon" title="Select model">
                      <ChevronDown class="size-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" class="max-h-64 overflow-y-auto">
                    <DropdownMenuItem
                      v-for="name in modelChoices"
                      :key="name"
                      class="font-mono text-xs"
                      @click="form.model = name"
                    >
                      {{ name }}
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button
                      variant="outline"
                      size="icon"
                      :disabled="modelsLoading"
                      @click="handleLoadModels"
                    >
                      <RefreshCw :class="{ 'animate-spin': modelsLoading }" class="size-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{{ t('agent.loadModels') }}</TooltipContent>
                </Tooltip>
              </div>
            </div>

            <div class="flex items-center justify-between gap-4 rounded-lg border p-3.5">
              <div class="space-y-0.5">
                <Label for="agent-thinking">{{ t('agent.thinkingSwitch') }}</Label>
                <p class="text-xs text-muted-foreground">{{ t('agent.thinkingTip') }}</p>
              </div>
              <Switch id="agent-thinking" v-model="form.thinking" />
            </div>

            <div class="space-y-1.5">
              <Label>{{ t('agent.thinkingEffort') }}</Label>
              <Select v-model="form.thinking_effort" :disabled="!form.thinking">
                <SelectTrigger class="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="low">{{ t('agent.effortLow') }}</SelectItem>
                  <SelectItem value="medium">{{ t('agent.effortMedium') }}</SelectItem>
                  <SelectItem value="high">{{ t('agent.effortHigh') }}</SelectItem>
                  <SelectItem value="xhigh">{{ t('agent.effortXHigh') }}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div class="flex flex-wrap gap-2 pt-1">
              <Button variant="outline" :disabled="loading || testing" @click="handleTest">
                <Loader2 v-if="testing" class="animate-spin" />
                {{ t('agent.test') }}
              </Button>
              <Button :disabled="testing || loading" @click="handleSave">
                {{ t('agent.save') }}
              </Button>
            </div>
          </CardContent>
        </Card>

        <!-- WeChat Card -->
        <Card>
          <CardHeader>
            <CardTitle class="text-base">{{ t('agent.wechat.title') }}</CardTitle>
          </CardHeader>

          <CardContent class="space-y-4">
            <Alert variant="info">
              <Info class="size-4" />
              <AlertTitle>{{ t('agent.wechat.notice') }}</AlertTitle>
            </Alert>

            <div class="relative min-h-[120px]">
              <LoadingOverlay :loading="wechatLoading" />

              <div v-if="wechat.linked" class="space-y-3">
                <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 rounded-lg border p-3.5">
                  <div class="space-y-1">
                    <div class="text-xs text-muted-foreground">{{ t('agent.wechat.boundToken') }}</div>
                    <div class="font-mono text-xs break-all">{{ wechat.token_mask || t('agent.notSet') }}</div>
                    <div v-if="wechat.account_id" class="text-xs text-muted-foreground">
                      {{ t('agent.wechat.account') }} {{ wechat.account_id }}
                    </div>
                    <div v-if="wechat.saved_at" class="text-xs text-muted-foreground">
                      {{ t('agent.wechat.savedAt') }} {{ wechat.saved_at }}
                    </div>
                  </div>
                  <Button variant="outline" class="text-destructive border-destructive/40 hover:bg-destructive/10" @click="handleUnbind">
                    {{ t('agent.wechat.unbind') }}
                  </Button>
                </div>
                <p class="text-xs text-muted-foreground">{{ t('agent.wechat.rebindTip') }}</p>
              </div>

              <div v-else-if="wechatLoaded" class="space-y-3">
                <Alert v-if="wechat.session_expired" variant="warning">
                  <AlertTriangle class="size-4" />
                  <AlertTitle>{{ t('agent.wechat.sessionExpired') }}</AlertTitle>
                </Alert>
                <p class="text-xs text-muted-foreground">{{ t('agent.wechat.bindTip') }}</p>
                <WeChatQRPanel :initial="wechat.qr" @linked="loadWeChat" />
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      <!-- Change API Key Dialog -->
      <Dialog v-model:open="showKeyDialog">
        <DialogContent class="sm:max-w-[460px]">
          <DialogHeader>
            <DialogTitle>{{ t('agent.changeKey') }}</DialogTitle>
          </DialogHeader>
          <div class="space-y-1.5 py-2">
            <Label for="new-api-key">{{ t('agent.apiKey') }}</Label>
            <div class="relative">
              <Input
                id="new-api-key"
                v-model="newApiKey"
                :type="showNewKey ? 'text' : 'password'"
                placeholder="sk-..."
                class="pr-9 font-mono"
              />
              <button
                type="button"
                class="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                @click="showNewKey = !showNewKey"
              >
                <EyeOff v-if="showNewKey" class="size-4" />
                <Eye v-else class="size-4" />
              </button>
            </div>
          </div>
          <DialogFooter class="gap-2">
            <Button variant="outline" @click="handleCancelKeyDialog">{{ t('common.cancel') }}</Button>
            <Button @click="handleConfirmKeyChange">{{ t('common.confirm') }}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  </TooltipProvider>
</template>
