<script setup lang="ts">
import { ref, nextTick, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import {
  Bot,
  Loader2,
  Maximize2,
  MessageSquare,
  Minimize2,
  Send,
  Trash2,
  User,
  X,
} from 'lucide-vue-next'
import { useToast } from '@/composables/useToast'
import { getToken } from '@/utils/request'
import { getTargetNodeId } from '@/utils/targetNode'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

const { t } = useI18n()
const toast = useToast()

interface Message {
  role: 'user' | 'assistant'
  content: string
}

const isOpen = ref(false)
const isExpanded = ref(false)
const messages = ref<Message[]>([])
const inputMessage = ref('')
const isThinking = ref(false)
const connected = ref(true)
const messagesRef = ref<HTMLElement>()
const inputWrapperRef = ref<HTMLElement>()

const focusInput = () => {
  nextTick(() => {
    const el = inputWrapperRef.value?.querySelector('input')
    el?.focus()
  })
}

watch(isOpen, (val) => {
  if (val) {
    loadHistory()
    focusInput()
    scrollToBottom()
  }
})

const toggleExpand = () => {
  isExpanded.value = !isExpanded.value
}

const setInput = (text: string) => {
  inputMessage.value = text
  focusInput()
}

const loadHistory = async () => {
  const token = getToken()
  try {
    const response = await fetch('/api/agent/history', {
      headers: { Authorization: `Bearer ${token}` },
    })
    if (response.ok) {
      const data = await response.json()
      if (Array.isArray(data)) {
        messages.value = data.map((m: { role: string; content: string }) => ({
          role: m.role as 'user' | 'assistant',
          content: m.content,
        }))
        scrollToBottom()
      }
    }
  } catch (e) {
    console.error('Failed to load history:', e)
  }
}

const clearMessages = async () => {
  const token = getToken()
  try {
    await fetch('/api/agent/history', {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${token}` },
    })
    messages.value = []
  } catch (e) {
    console.error('Failed to clear history:', e)
  }
}

const renderMarkdown = (text: string) => {
  if (!text) return ''
  const formattedText = text.replace(/\[\[TOOL:\s*(.*?)\]\]/g, '> 🛠️ **正在执行工具**: `$1`')
  return marked(formattedText)
}

const scrollToBottom = () => {
  nextTick(() => {
    if (messagesRef.value) {
      messagesRef.value.scrollTop = messagesRef.value.scrollHeight
    }
  })
}

const sendMessage = async () => {
  if (!inputMessage.value.trim() || isThinking.value) return

  const userMsg = inputMessage.value.trim()
  const userMessage: Message = { role: 'user', content: userMsg }
  messages.value.push(userMessage)
  inputMessage.value = ''
  isThinking.value = true
  scrollToBottom()
  focusInput()

  const token = getToken()
  try {
    const response = await fetch('/api/agent/chat', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify({
        message: userMsg,
        node_id: getTargetNodeId(),
        history: messages.value.slice(0, -1).map((m) => ({ role: m.role, content: m.content })),
      }),
    })

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`)
    }

    if (!response.body) {
      throw new Error('Response body is null')
    }

    const reader = response.body.getReader()
    const decoder = new TextDecoder()

    let started = false
    let assistantMsgIndex = -1
    let fullContent = ''
    let buffer = ''

    const processEvent = (eventBlock: string) => {
      if (!eventBlock.trim()) return

      const lines = eventBlock.split('\n')
      const dataLines: string[] = []

      for (const line of lines) {
        if (line.startsWith('data: ')) {
          dataLines.push(line.slice(6))
        }
      }

      if (dataLines.length > 0) {
        const content = dataLines.join('\n')
        fullContent += content

        if (!started) {
          started = true
          messages.value.push({ role: 'assistant', content: fullContent })
          assistantMsgIndex = messages.value.length - 1
        } else {
          messages.value.splice(assistantMsgIndex, 1, {
            role: 'assistant',
            content: fullContent,
          })
        }
        scrollToBottom()
      }
    }

    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const events = buffer.split('\n\n')
      buffer = events.pop() || ''

      for (const event of events) {
        processEvent(event)
      }
    }

    buffer += decoder.decode()
    if (buffer) {
      const events = buffer.split('\n\n')
      for (const event of events) {
        processEvent(event)
      }
    }

    if (started && assistantMsgIndex >= 0) {
      messages.value.splice(assistantMsgIndex, 1, {
        role: 'assistant',
        content: fullContent,
      })
    }
    isThinking.value = false
    focusInput()
  } catch (error) {
    console.error('Chat error:', error)
    toast.error(t('agent.connectionError'))
    isThinking.value = false
    messages.value.push({
      role: 'assistant',
      content: `**${t('agent.error')}**: ${t('agent.responseError')}`,
    })
  }
}

onMounted(() => {
  loadHistory()
})
</script>

<template>
  <div class="fixed right-5 bottom-5 z-40">
    <!-- Floating Button -->
    <transition name="bounce">
      <Button
        v-show="!isOpen"
        size="icon"
        class="size-13 rounded-full shadow-lg hover:scale-105 transition-transform"
        :aria-label="t('agent.chatTitle')"
        @click="isOpen = true"
      >
        <MessageSquare class="size-5" />
      </Button>
    </transition>

    <!-- Chat Window -->
    <transition name="chat-popup">
      <div
        v-show="isOpen"
        class="absolute right-0 bottom-0 flex flex-col rounded-xl border bg-card text-card-foreground shadow-2xl overflow-hidden origin-bottom-right transition-all duration-200"
        :class="isExpanded ? 'w-[min(52vw,760px)] h-[72vh]' : 'w-[380px] max-w-[calc(100vw-2.5rem)] h-[520px]'"
      >
        <div class="flex items-center justify-between px-4 py-3 border-b bg-muted/40">
          <div class="flex items-center gap-2 font-semibold text-sm">
            <Bot class="size-4 text-primary" />
            <span>{{ t('agent.chatTitle') }}</span>
            <Badge v-if="connected" variant="success">{{ t('agent.connected') }}</Badge>
          </div>
          <div class="flex items-center gap-1">
            <Button
              variant="ghost"
              size="icon-sm"
              :title="isExpanded ? '缩小' : '放大'"
              @click="toggleExpand"
            >
              <Minimize2 v-if="isExpanded" class="size-4" />
              <Maximize2 v-else class="size-4" />
            </Button>
            <Button variant="ghost" size="icon-sm" @click="clearMessages">
              <Trash2 class="size-4" />
            </Button>
            <Button variant="ghost" size="icon-sm" @click="isOpen = false">
              <X class="size-4" />
            </Button>
          </div>
        </div>

        <div ref="messagesRef" class="flex-1 overflow-y-auto p-3.5 bg-muted/20 space-y-3">
          <div
            v-if="messages.length === 0"
            class="h-full flex flex-col items-center justify-center text-center gap-4 px-4"
          >
            <div class="size-11 rounded-full bg-muted flex items-center justify-center text-muted-foreground">
              <Bot class="size-5" />
            </div>
            <p class="text-sm text-muted-foreground">{{ t('agent.welcome') }}</p>
            <div class="flex flex-wrap gap-2 justify-center">
              <Button variant="outline" size="sm" @click="setInput(t('agent.checkStatus'))">
                {{ t('agent.checkStatus') }}
              </Button>
              <Button variant="outline" size="sm" @click="setInput(t('agent.listContainers'))">
                {{ t('agent.listContainers') }}
              </Button>
            </div>
          </div>

          <div
            v-for="(msg, index) in messages"
            :key="index"
            class="flex gap-2.5 items-start"
            :class="msg.role === 'user' ? 'flex-row-reverse' : ''"
          >
            <Avatar class="size-7 shrink-0">
              <AvatarFallback
                :class="msg.role === 'user' ? 'bg-primary text-primary-foreground' : 'bg-secondary text-secondary-foreground'"
              >
                <User v-if="msg.role === 'user'" class="size-3.5" />
                <Bot v-else class="size-3.5" />
              </AvatarFallback>
            </Avatar>
            <div
              class="max-w-[78%] rounded-xl px-3 py-2 text-[13px] leading-relaxed"
              :class="msg.role === 'user'
                ? 'bg-primary text-primary-foreground rounded-br-xs'
                : 'bg-card text-card-foreground border rounded-bl-xs shadow-xs'"
            >
              <div v-if="msg.role === 'user'" class="whitespace-pre-wrap">{{ msg.content }}</div>
              <div v-else class="content-markdown" v-html="renderMarkdown(msg.content)" />
            </div>
          </div>

          <div v-if="isThinking" class="flex gap-2.5 items-start">
            <Avatar class="size-7 shrink-0 animate-pulse">
              <AvatarFallback class="bg-secondary text-secondary-foreground">
                <Loader2 class="size-3.5 animate-spin" />
              </AvatarFallback>
            </Avatar>
            <div class="rounded-xl rounded-bl-xs border bg-card px-3 py-2.5 shadow-xs flex items-center gap-1.5">
              <span class="size-1.5 rounded-full bg-muted-foreground/70 animate-bounce [animation-delay:0ms]" />
              <span class="size-1.5 rounded-full bg-muted-foreground/70 animate-bounce [animation-delay:150ms]" />
              <span class="size-1.5 rounded-full bg-muted-foreground/70 animate-bounce [animation-delay:300ms]" />
            </div>
          </div>
        </div>

        <div ref="inputWrapperRef" class="flex items-center gap-2 p-3 border-t bg-card">
          <Input
            v-model="inputMessage"
            :placeholder="t('agent.inputPlaceholder')"
            :disabled="isThinking"
            class="h-8 text-sm"
            @keydown.enter.prevent="sendMessage"
          />
          <Button size="icon-sm" :disabled="isThinking" @click="sendMessage">
            <Send class="size-3.5" />
          </Button>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.content-markdown :deep(p) {
  margin: 0 0 8px 0;
}
.content-markdown :deep(p:last-child) {
  margin-bottom: 0;
}
.content-markdown :deep(hr) {
  border: none;
  border-top: 1px dashed var(--border);
  margin: 12px 0;
}
.content-markdown :deep(ul),
.content-markdown :deep(ol) {
  margin: 8px 0;
  padding-left: 20px;
}
.content-markdown :deep(li) {
  margin-bottom: 4px;
}
.content-markdown :deep(pre) {
  background: var(--muted);
  padding: 10px 12px;
  border-radius: 8px;
  overflow-x: auto;
  font-size: 12px;
  margin: 8px 0;
  border: 1px solid var(--border);
}
.content-markdown :deep(code) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  background: var(--muted);
  padding: 1px 4px;
  border-radius: 4px;
  font-size: 12px;
}
.content-markdown :deep(pre code) {
  padding: 0;
  background: transparent;
}
.content-markdown :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 8px 0;
  font-size: 12px;
}
.content-markdown :deep(th),
.content-markdown :deep(td) {
  border: 1px solid var(--border);
  padding: 6px 10px;
  text-align: left;
}
.content-markdown :deep(th) {
  background: var(--muted);
  font-weight: 600;
}
.content-markdown :deep(blockquote) {
  margin: 8px 0;
  padding: 8px 12px;
  border-left: 3px solid var(--primary);
  background: var(--muted);
  border-radius: 6px;
  font-size: 12px;
  word-break: break-all;
}
.content-markdown :deep(blockquote p) {
  margin: 0;
}

.bounce-enter-active {
  transition: all 0.25s cubic-bezier(0.34, 1.56, 0.64, 1);
}
.bounce-leave-active {
  transition: all 0.15s ease-in;
}
.bounce-enter-from,
.bounce-leave-to {
  transform: scale(0);
  opacity: 0;
}

.chat-popup-enter-active,
.chat-popup-leave-active {
  transition: all 0.2s ease;
}
.chat-popup-enter-from,
.chat-popup-leave-to {
  opacity: 0;
  transform: scale(0.92) translateY(12px);
}
</style>
