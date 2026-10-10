<template>
  <div class="agent-settings">
    <div class="settings-grid">
      <el-card>
        <template #header>
          <div class="card-header">
            <span>{{ t('agent.settingsTitle') }}</span>
          </div>
        </template>

        <el-form :model="form" label-position="top" v-loading="loading">
          <el-form-item :label="t('agent.enable')">
            <el-switch v-model="form.enabled" />
          </el-form-item>

          <el-form-item :label="t('agent.apiUrl')">
            <el-input v-model="form.api_url" placeholder="https://api.openai.com/v1" />
            <div class="form-tip">{{ t('agent.apiTip') }}</div>
          </el-form-item>

          <el-form-item :label="t('agent.format')">
            <el-select v-model="form.api_format" class="full-control">
              <el-option value="openai_chat" :label="t('agent.formatOpenAIChat')" />
              <el-option value="openai_responses" :label="t('agent.formatOpenAIResponses')" />
              <el-option value="anthropic" :label="t('agent.formatAnthropic')" />
            </el-select>
          </el-form-item>

          <el-form-item :label="t('agent.apiKey')">
            <div class="key-row">
              <span class="key-mask">{{ displayApiKey }}</span>
              <el-button @click="showKeyDialog = true">{{ t('agent.changeKey') }}</el-button>
            </div>
          </el-form-item>

          <el-form-item :label="t('agent.model')">
            <div class="model-row">
              <el-select
                v-model="form.model"
                class="model-select"
                filterable
                allow-create
                default-first-option
                :placeholder="t('agent.modelPlaceholder')"
              >
                <el-option v-for="name in modelChoices" :key="name" :label="name" :value="name" />
              </el-select>
              <el-tooltip :content="t('agent.loadModels')" placement="top">
                <el-button :loading="modelsLoading" @click="handleLoadModels">
                  <el-icon><Refresh /></el-icon>
                </el-button>
              </el-tooltip>
            </div>
          </el-form-item>

          <el-form-item :label="t('agent.thinkingSwitch')">
            <el-switch v-model="form.thinking" />
            <div class="form-tip">{{ t('agent.thinkingTip') }}</div>
          </el-form-item>

          <el-form-item :label="t('agent.thinkingEffort')">
            <el-select v-model="form.thinking_effort" class="full-control" :disabled="!form.thinking">
              <el-option value="low" :label="t('agent.effortLow')" />
              <el-option value="medium" :label="t('agent.effortMedium')" />
              <el-option value="high" :label="t('agent.effortHigh')" />
              <el-option value="xhigh" :label="t('agent.effortXHigh')" />
            </el-select>
          </el-form-item>

          <el-form-item>
            <div class="action-row">
              <el-button :loading="testing" :disabled="loading" @click="handleTest">{{ t('agent.test') }}</el-button>
              <el-button type="primary" :disabled="testing" @click="handleSave">{{ t('agent.save') }}</el-button>
            </div>
          </el-form-item>
        </el-form>
      </el-card>

      <el-card>
        <template #header>
          <div class="card-header">
            <span>{{ t('agent.wechat.title') }}</span>
          </div>
        </template>

        <el-alert class="wechat-alert" type="info" :closable="false" show-icon :title="t('agent.wechat.notice')" />

        <div v-loading="wechatLoading">
          <div v-if="wechat.linked" class="bind-list">
            <div class="bind-row">
              <div>
                <div class="bind-label">{{ t('agent.wechat.boundToken') }}</div>
                <div class="key-mask">{{ wechat.token_mask || t('agent.notSet') }}</div>
                <div v-if="wechat.account_id" class="form-tip">{{ t('agent.wechat.account') }} {{ wechat.account_id }}</div>
                <div v-if="wechat.saved_at" class="form-tip">{{ t('agent.wechat.savedAt') }} {{ wechat.saved_at }}</div>
              </div>
              <el-button type="danger" plain @click="handleUnbind">{{ t('agent.wechat.unbind') }}</el-button>
            </div>
            <p class="form-tip section-tip">{{ t('agent.wechat.rebindTip') }}</p>
          </div>
          <template v-else-if="wechatLoaded">
            <el-alert
              v-if="wechat.session_expired"
              class="wechat-alert"
              type="warning"
              :closable="false"
              show-icon
              :title="t('agent.wechat.sessionExpired')"
            />
            <p class="form-tip section-tip">{{ t('agent.wechat.bindTip') }}</p>
            <WeChatQRPanel :initial="wechat.qr" @linked="loadWeChat" />
          </template>
        </div>
      </el-card>
    </div>

    <el-dialog v-model="showKeyDialog" :title="t('agent.changeKey')" width="460px">
      <el-form label-width="100px">
        <el-form-item :label="t('agent.apiKey')">
          <el-input v-model="newApiKey" type="password" show-password placeholder="sk-..." />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="handleCancelKeyDialog">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="handleConfirmKeyChange">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Refresh } from '@element-plus/icons-vue'
import { getAgentConfig, listAgentModels, saveAgentConfig, testAgentConnection, type AgentConfig } from '@/api/agent'
import { getWeChatStatus, unbindWeChat, type WeChatStatus } from '@/api/wechat'
import WeChatQRPanel from '@/components/common/WeChatQRPanel.vue'
import { ElMessage, ElMessageBox } from 'element-plus'

const { t } = useI18n()

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
    ElMessage.success(t('agent.saveSuccess'))
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
    ElMessage.warning(t('agent.testNeed'))
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
      ElMessage.success(t('agent.testSuccessReply', { reply }))
    } else {
      ElMessage.success(t('agent.testSuccess'))
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
    ElMessage.warning(t('agent.loadModelsNeed'))
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
      ElMessage.warning(t('agent.loadModelsEmpty'))
      return
    }
    ElMessage.success(t('agent.loadModelsSuccess'))
  } catch (error) {
    console.error(error)
  } finally {
    modelsLoading.value = false
  }
}

const handleCancelKeyDialog = () => {
  showKeyDialog.value = false
  newApiKey.value = ''
}

const handleConfirmKeyChange = async () => {
  const key = newApiKey.value.trim()
  if (!key) {
    return
  }
  loading.value = true
  try {
    await saveAgentConfig({
      ...form.value,
      api_key: key,
    })
    await loadConfig()
    showKeyDialog.value = false
    newApiKey.value = ''
    ElMessage.success(t('agent.saveSuccess'))
  } catch (error) {
    console.error(error)
  } finally {
    loading.value = false
  }
}

const handleUnbind = async () => {
  try {
    await ElMessageBox.confirm(t('agent.wechat.unbindConfirm'), t('common.confirm'))
  } catch {
    return
  }
  try {
    await unbindWeChat()
    await loadWeChat()
    ElMessage.success(t('agent.wechat.unbound'))
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

<style scoped>
.agent-settings {
  max-width: 1180px;
  margin: 0 auto;
}
.settings-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(0, 0.85fr);
  gap: 16px;
  align-items: start;
}
.form-tip {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
  line-height: 1.5;
}
.full-control {
  width: 100%;
}
.action-row {
  display: flex;
  gap: 8px;
}
.model-row {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
}
.model-select {
  flex: 1;
  min-width: 0;
}
.key-row {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.key-mask {
  color: var(--el-text-color-regular);
  font-family: monospace;
  word-break: break-all;
}
.wechat-alert {
  margin-bottom: 16px;
}
.section-tip {
  margin: 0 0 12px;
}
.bind-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.bind-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
}
.bind-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 4px;
}
@media (max-width: 960px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 768px) {
  .key-row,
  .bind-row {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
