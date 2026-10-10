import { onUnmounted, ref } from 'vue'
import { createWeChatQR, getWeChatQRStatus, submitWeChatVerify, type WeChatQR } from '@/api/wechat'

export function useWeChatQR(onLinked?: () => void) {
  const qr = ref<WeChatQR | null>(null)
  const creating = ref(false)
  const code = ref('')
  let timer: ReturnType<typeof setInterval> | undefined
  let linked = false

  function stop() {
    if (timer) {
      clearInterval(timer)
      timer = undefined
    }
  }

  function start() {
    stop()
    timer = setInterval(() => {
      poll().catch(() => undefined)
    }, 2000)
  }

  async function poll() {
    const res = await getWeChatQRStatus()
    qr.value = { ...qr.value, ...res.data, image: res.data.image || qr.value?.image }
    if (res.data.status === 'linked' || res.data.status === 'expired' || res.data.status === 'blocked' || res.data.status === 'idle') {
      stop()
    }
    if (res.data.status === 'linked' && !linked) {
      linked = true
      onLinked?.()
    }
  }

  async function create() {
    stop()
    linked = false
    code.value = ''
    creating.value = true
    try {
      const res = await createWeChatQR()
      qr.value = res.data
      start()
    } finally {
      creating.value = false
    }
  }

  function resume(initial?: WeChatQR | null) {
    if (!initial?.image) {
      return
    }
    if (['idle', 'expired', 'blocked', 'error', 'linked'].includes(initial.status)) {
      qr.value = initial
      return
    }
    qr.value = initial
    start()
  }

  async function submit() {
    const value = code.value.trim()
    if (!value) {
      return
    }
    await submitWeChatVerify(value)
    code.value = ''
  }

  onUnmounted(stop)

  return { qr, creating, code, create, resume, submit, stop }
}
