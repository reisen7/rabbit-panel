import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { containerApi } from '@/api/containers'
import { toast } from '@/composables/useToast'
import { useI18n } from '@/composables/useI18n'
import type { CreateContainerRequest } from '@/types'

export interface ContainerCreateForm {
  image: string
  name: string
  restart: string
  network: string
  ports: Array<{ host: string; container: string }>
  volumes: Array<{ host: string; container: string }>
  envs: Array<{ key: string; value: string }>
}

export const useContainerCreateStore = defineStore('containerCreate', () => {
  const { t } = useI18n()

  const loading = ref(false)
  const isStreaming = ref(false)
  const streamLogs = ref<Array<{ type: string; message: string }>>([])
  const minimized = ref(false)
  const restorePending = ref(false)
  const createdSignal = ref(0)
  const lastForm = ref<ContainerCreateForm | null>(null)
  let abortController: AbortController | null = null

  const hasTaskState = computed(() => loading.value || isStreaming.value || streamLogs.value.length > 0)
  const latestLogLine = computed(() => {
    if (streamLogs.value.length === 0) return t('container.output')
    return streamLogs.value[streamLogs.value.length - 1]?.message || t('container.output')
  })

  async function start(form: ContainerCreateForm, payload: CreateContainerRequest): Promise<void> {
    lastForm.value = JSON.parse(JSON.stringify(form)) as ContainerCreateForm
    loading.value = true
    isStreaming.value = true
    streamLogs.value = []

    try {
      abortController = new AbortController()
      const result = await containerApi.createStream(
        payload,
        (entry) => {
          streamLogs.value.push(entry)
        },
        abortController.signal
      )
      if (result.container_id) {
        toast.success(t('container.createSuccess'))
        createdSignal.value++
      }
    } catch (error: unknown) {
      if ((error as { name?: string })?.name === 'AbortError') {
        // cancelled by user
      }
    } finally {
      abortController = null
      loading.value = false
      isStreaming.value = false
    }
  }

  function cancel(): void {
    abortController?.abort()
    abortController = null
    streamLogs.value.push({ type: 'error', message: '已取消创建任务' })
    loading.value = false
    isStreaming.value = false
  }

  function minimize(): void {
    minimized.value = true
  }

  function restore(): void {
    minimized.value = false
    restorePending.value = true
  }

  function consumeRestore(): boolean {
    const pending = restorePending.value
    restorePending.value = false
    return pending
  }

  function dismiss(): void {
    minimized.value = false
    streamLogs.value = []
  }

  function reset(): void {
    if (loading.value || isStreaming.value) return
    streamLogs.value = []
    minimized.value = false
  }

  return {
    loading,
    isStreaming,
    streamLogs,
    minimized,
    restorePending,
    createdSignal,
    lastForm,
    hasTaskState,
    latestLogLine,
    start,
    cancel,
    minimize,
    restore,
    consumeRestore,
    dismiss,
    reset,
  }
})
