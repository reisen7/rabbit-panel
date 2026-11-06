<template>
  <div class="notify-page" v-loading="loading">
    <!-- 顶部：标题 / 简介 / 总开关 / 教程入口 -->
    <section class="notify-hero">
      <div class="hero-main">
        <div class="hero-icon">
          <el-icon><Bell /></el-icon>
        </div>
        <div class="hero-copy">
          <h1>{{ t('notify.title') }}</h1>
          <p>{{ t('notify.pageIntro') }}</p>
        </div>
      </div>

      <div class="hero-side">
        <div class="hero-switch" :class="{ 'is-on': form.enabled }">
          <span class="hero-switch-label">{{ t('notify.enabled') }}</span>
          <el-switch v-model="form.enabled" size="large" />
        </div>
        <el-button class="hero-guide" text bg @click="showGuide = true">
          <el-icon><Reading /></el-icon>
          <span>{{ t('notify.openGuide') }}</span>
        </el-button>
      </div>
    </section>

    <!-- 推送凭证 -->
    <section class="section-card">
      <header class="section-head">
        <span class="section-icon"><el-icon><Key /></el-icon></span>
        <div class="section-title">
          <h2>{{ t('notify.token') }}</h2>
          <p>{{ t('notify.tokenTip') }}</p>
        </div>
      </header>

      <div class="section-body">
        <el-input
          v-model="form.token"
          class="token-input"
          type="password"
          show-password
          size="large"
          :placeholder="tokenPlaceholder"
        >
          <template #prefix>
            <el-icon><Lock /></el-icon>
          </template>
        </el-input>

        <div v-if="saved.has_token" class="token-chip">
          <el-icon class="chip-icon"><CircleCheck /></el-icon>
          <span>{{ t('notify.tokenCurrent') }}</span>
          <code>{{ saved.token_mask }}</code>
        </div>
      </div>
    </section>

    <!-- 推送范围 -->
    <section class="section-card">
      <header class="section-head">
        <span class="section-icon"><el-icon><Aim /></el-icon></span>
        <div class="section-title">
          <h2>{{ t('notify.scope') }}</h2>
          <p>{{ t('notify.containersTip') }}</p>
        </div>
      </header>

      <div class="section-body">
        <el-radio-group v-model="form.all_containers" class="scope-group">
          <el-radio :value="true" class="scope-option">
            <span class="option-title">{{ t('notify.allContainers') }}</span>
          </el-radio>
          <el-radio :value="false" class="scope-option">
            <span class="option-title">{{ t('notify.selectedContainers') }}</span>
          </el-radio>
        </el-radio-group>

        <el-collapse-transition>
          <div v-if="!form.all_containers" class="scope-detail">
            <el-select
              v-model="form.containers"
              class="full"
              multiple
              filterable
              allow-create
              default-first-option
              :placeholder="t('notify.containersPlaceholder')"
            >
              <el-option v-for="name in containerOptions" :key="name" :label="name" :value="name" />
            </el-select>
          </div>
        </el-collapse-transition>
      </div>
    </section>

    <!-- 推送规则 -->
    <section class="section-card">
      <header class="section-head">
        <span class="section-icon"><el-icon><Filter /></el-icon></span>
        <div class="section-title">
          <h2>{{ t('notify.rules') }}</h2>
          <p>{{ t('notify.rulesTip') }}</p>
        </div>
      </header>

      <div class="section-body">
        <div class="rule-grid">
          <el-checkbox v-model="form.exit_nonzero" class="rule-tile">
            {{ t('notify.ruleExit') }}
          </el-checkbox>
          <el-checkbox v-model="form.unhealthy" class="rule-tile">
            {{ t('notify.ruleUnhealthy') }}
          </el-checkbox>
          <el-checkbox v-model="form.restart_storm" class="rule-tile">
            {{ t('notify.ruleRestart') }}
          </el-checkbox>
          <el-checkbox v-model="form.recovery" class="rule-tile">
            {{ t('notify.ruleRecovery') }}
          </el-checkbox>
        </div>
      </div>
    </section>

    <!-- 冷却时间 -->
    <section class="section-card">
      <header class="section-head">
        <span class="section-icon"><el-icon><Timer /></el-icon></span>
        <div class="section-title">
          <h2>{{ t('notify.cooldown') }}</h2>
          <p>{{ t('notify.cooldownTip') }}</p>
        </div>
      </header>

      <div class="section-body">
        <el-input-number v-model="form.cooldown_minutes" :min="1" :max="1440" size="large" />
      </div>
    </section>

    <!-- 底部操作条 -->
    <div class="notify-actions">
      <el-button :loading="testing" :disabled="loading" @click="handleTest">
        <el-icon><Promotion /></el-icon>
        <span>{{ t('notify.test') }}</span>
      </el-button>
      <el-button type="primary" :loading="saving" :disabled="testing" @click="handleSave">
        {{ t('common.save') }}
      </el-button>
    </div>

    <ShowdocGuideDialog v-model="showGuide" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Aim, Bell, CircleCheck, Filter, Key, Lock, Promotion, Reading, Timer } from '@element-plus/icons-vue'
import { useI18n } from '@/composables/useI18n'
import { containerApi } from '@/api/containers'
import { getNotifySettings, saveNotifySettings, testNotify, type NotifySettings } from '@/api/notify'
import ShowdocGuideDialog from '@/components/notify/ShowdocGuideDialog.vue'

const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const showGuide = ref(false)
const containerNames = ref<string[]>([])
const saved = ref<NotifySettings>(emptySettings())
const form = ref<NotifySettings>(emptySettings())

const tokenPlaceholder = computed(() => (
  saved.value.has_token ? t('notify.tokenKeep') : t('notify.tokenPlaceholder')
))

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

async function loadContainers() {
  try {
    const list = await containerApi.list()
    containerNames.value = list.map(item => item.name).filter(Boolean)
  } catch {
    containerNames.value = []
  }
}

async function loadSettings() {
  loading.value = true
  try {
    const { data } = await getNotifySettings()
    saved.value = { ...emptySettings(), ...data, token: '', containers: data.containers || [] }
    form.value = { ...saved.value, token: '' }
  } catch (error) {
    ElMessage.error(errorText(error, t('notify.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    const { data } = await saveNotifySettings({ ...form.value, containers: form.value.containers || [] })
    saved.value = { ...emptySettings(), ...data, token: '', containers: data.containers || [] }
    form.value = { ...saved.value, token: '' }
    ElMessage.success(t('notify.saved'))
  } catch (error) {
    ElMessage.error(errorText(error, t('notify.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function handleTest() {
  testing.value = true
  try {
    await testNotify(form.value.token || '')
    ElMessage.success(t('notify.testSent'))
  } catch (error) {
    ElMessage.error(errorText(error, t('notify.testFailed')))
  } finally {
    testing.value = false
  }
}

onMounted(() => {
  loadSettings()
  loadContainers()
})
</script>

<style scoped>
.notify-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-width: 900px;
  margin: 0 auto;
  padding: 2px 2px 8px;
}

/* ---------- 顶部 ---------- */
.notify-hero {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 18px 24px;
  padding: 20px 22px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 14px;
  overflow: hidden;
  background:
    radial-gradient(110% 150% at 0% 0%, var(--el-color-primary-light-8) 0%, transparent 58%),
    var(--el-bg-color);
}

.notify-hero::after {
  content: '';
  position: absolute;
  top: -60px;
  right: -40px;
  width: 180px;
  height: 180px;
  border-radius: 50%;
  background: var(--el-color-primary-light-8);
  opacity: 0.45;
  filter: blur(28px);
  pointer-events: none;
}

.hero-main {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  position: relative;
  z-index: 1;
  min-width: 0;
}

.hero-icon {
  display: grid;
  place-items: center;
  flex: none;
  width: 42px;
  height: 42px;
  border-radius: 12px;
  font-size: 20px;
  color: var(--el-color-primary);
  background: var(--el-bg-color);
  box-shadow: inset 0 0 0 1px var(--el-color-primary-light-7);
}

.hero-copy {
  min-width: 0;
}

.hero-copy h1 {
  margin: 0;
  font-size: 17px;
  font-weight: 600;
  line-height: 1.45;
  letter-spacing: 0.2px;
  color: var(--el-text-color-primary);
}

.hero-copy p {
  margin: 5px 0 0;
  max-width: 58ch;
  font-size: 12.5px;
  line-height: 1.7;
  color: var(--el-text-color-regular);
}

.hero-side {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  position: relative;
  z-index: 1;
}

.hero-switch {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 14px 6px 16px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 999px;
  background: var(--el-bg-color);
  transition: border-color 0.2s, background-color 0.2s;
}

.hero-switch.is-on {
  border-color: var(--el-color-primary-light-5);
  background: var(--el-color-primary-light-9);
}

.hero-switch-label {
  font-size: 12.5px;
  font-weight: 500;
  color: var(--el-text-color-regular);
  white-space: nowrap;
}

.hero-switch.is-on .hero-switch-label {
  color: var(--el-color-primary);
}

.hero-guide {
  gap: 6px;
}

/* ---------- 分区卡片 ---------- */
.section-card {
  padding: 18px 20px 20px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 14px;
  background: var(--el-bg-color);
  transition: border-color 0.2s, box-shadow 0.2s;
}

.section-card:hover {
  border-color: var(--el-border-color-light);
  box-shadow: 0 2px 12px -6px var(--el-color-info-light-5);
}

.section-head {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 16px;
}

.section-icon {
  display: grid;
  place-items: center;
  flex: none;
  width: 30px;
  height: 30px;
  border-radius: 9px;
  font-size: 15px;
  color: var(--el-color-primary);
  background: var(--el-fill-color-light);
}

.section-title {
  min-width: 0;
}

.section-title h2 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  line-height: 1.5;
  color: var(--el-text-color-primary);
}

.section-title p {
  margin: 3px 0 0;
  font-size: 12px;
  line-height: 1.65;
  color: var(--el-text-color-secondary);
}

.section-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

/* ---------- Token ---------- */
.token-input {
  max-width: 520px;
}

.token-chip {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  align-self: flex-start;
  max-width: 100%;
  padding: 5px 12px;
  border-radius: 999px;
  font-size: 12px;
  color: var(--el-color-success);
  background: var(--el-color-success-light-9);
  border: 1px solid var(--el-color-success-light-7);
}

.chip-icon {
  font-size: 13px;
}

.token-chip code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 11.5px;
  letter-spacing: 0.3px;
  color: var(--el-text-color-regular);
  overflow-wrap: anywhere;
}

/* ---------- 范围 ---------- */
.scope-group {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  width: 100%;
}

.scope-option {
  display: flex;
  align-items: center;
  height: auto;
  margin: 0;
  padding: 13px 15px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 10px;
  background: var(--el-fill-color-blank);
  transition: border-color 0.2s, background-color 0.2s;
}

.scope-option:hover {
  border-color: var(--el-color-primary-light-5);
}

.scope-option.is-checked {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}

.scope-option :deep(.el-radio__label) {
  padding-left: 8px;
}

.option-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--el-text-color-regular);
}

.scope-option.is-checked .option-title {
  color: var(--el-color-primary);
}

.scope-detail {
  max-width: 520px;
}

.full {
  width: 100%;
}

/* ---------- 规则 ---------- */
.rule-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  width: 100%;
}

.rule-tile {
  display: flex;
  align-items: center;
  height: auto;
  margin: 0;
  padding: 12px 14px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 10px;
  background: var(--el-fill-color-blank);
  transition: border-color 0.2s, background-color 0.2s;
}

.rule-tile:hover {
  border-color: var(--el-color-primary-light-5);
}

.rule-tile.is-checked {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}

.rule-tile :deep(.el-checkbox__label) {
  font-size: 13px;
  line-height: 1.5;
  color: var(--el-text-color-regular);
  white-space: normal;
}

.rule-tile.is-checked :deep(.el-checkbox__label) {
  color: var(--el-color-primary);
}

/* ---------- 操作条 ---------- */
.notify-actions {
  position: sticky;
  bottom: 12px;
  z-index: 5;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 2px;
  padding: 12px 16px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 12px;
  background: var(--el-bg-color);
  background: color-mix(in srgb, var(--el-bg-color) 88%, transparent);
  backdrop-filter: blur(10px);
  box-shadow: 0 6px 20px -12px var(--el-color-info);
}

/* ---------- 响应式 ---------- */
@media (max-width: 768px) {
  .notify-hero {
    padding: 18px;
  }

  .notify-hero::after {
    display: none;
  }

  .hero-side {
    width: 100%;
  }

  .hero-switch {
    flex: 1 1 auto;
    justify-content: space-between;
  }

  .section-card {
    padding: 16px;
  }

  .scope-group,
  .rule-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .token-input,
  .scope-detail {
    max-width: none;
  }

  .notify-actions {
    bottom: 8px;
  }
}
</style>
