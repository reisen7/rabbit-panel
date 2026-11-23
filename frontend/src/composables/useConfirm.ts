import { ref } from 'vue'

export interface ConfirmOptions {
  title?: string
  description?: string
  confirmText?: string
  cancelText?: string
  destructive?: boolean
}

interface PendingConfirm extends ConfirmOptions {
  resolve: (value: boolean) => void
}

const pending = ref<PendingConfirm | null>(null)

export function useConfirm() {
  function confirm(options: ConfirmOptions): Promise<boolean> {
    return new Promise((resolve) => {
      pending.value = { ...options, resolve }
    })
  }

  function handleResult(value: boolean) {
    pending.value?.resolve(value)
    pending.value = null
  }

  return {
    pending,
    confirm,
    handleResult,
  }
}
