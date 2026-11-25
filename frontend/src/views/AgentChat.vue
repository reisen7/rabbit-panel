<script setup lang="ts">
import { ref, nextTick, onMounted, onUpdated } from 'vue'
import { useI18n } from 'vue-i18n'
import { Bot, Loader2, Send, User } from 'lucide-vue-next'
import { useAgentChat } from '@/composables/useAgentChat'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import AgentConfirmCard from '@/components/common/AgentConfirmCard.vue'

const { t } = useI18n()

const inputMessage = ref('')
const connected = ref(true)
const messagesRef = ref<HTMLElement>()

const scrollToBottom = () => {
  nextTick(() => {
    if (messagesRef.value) {
      messagesRef.value.scrollTop = messagesRef.value.scrollHeight
    }
  })
}

const {
  messages,
  isThinking,
  waitingReply,
  confirmations,
  renderMarkdown,
  loadHistory,
  resolveConfirm,
  sendMessage: sendChatMessage,
} = useAgentChat(scrollToBottom)

const setInput = (text: string) => {
  inputMessage.value = text
}

const sendMessage = async () => {
  const userMsg = inputMessage.value.trim()
  if (!userMsg) return
  inputMessage.value = ''
  await sendChatMessage(userMsg)
}

onMounted(() => {
  loadHistory()
})

onUpdated(() => {
  scrollToBottom()
})
</script>

<template>
  <div class="h-[calc(100vh-5.25rem)] md:h-[calc(100vh-6.5rem)] flex flex-col">
    <Card class="flex-1 flex flex-col overflow-hidden py-0 gap-0">
      <CardHeader class="flex flex-row items-center justify-between px-5 py-3.5 border-b space-y-0">
        <div class="flex items-center gap-2">
          <Bot class="size-4 text-primary" />
          <CardTitle class="text-base">{{ t('agent.chatTitle') }}</CardTitle>
        </div>
        <Badge v-if="connected" variant="success">{{ t('agent.connected') }}</Badge>
        <Badge v-else variant="info">{{ t('agent.disconnected') }}</Badge>
      </CardHeader>

      <CardContent class="flex-1 flex flex-col min-h-0 p-0">
        <div ref="messagesRef" class="flex-1 overflow-y-auto p-5 bg-muted/20 space-y-5">
          <div
            v-if="messages.length === 0"
            class="h-full flex flex-col items-center justify-center text-center gap-4"
          >
            <div class="size-12 rounded-full bg-muted flex items-center justify-center text-muted-foreground">
              <Bot class="size-6" />
            </div>
            <p class="text-sm text-muted-foreground">{{ t('agent.welcome') }}</p>
            <div class="flex flex-wrap justify-center gap-2">
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
            class="flex items-start gap-3"
            :class="msg.role === 'user' ? 'flex-row-reverse' : ''"
          >
            <Avatar class="size-8 shrink-0">
              <AvatarFallback
                :class="msg.role === 'user' ? 'bg-primary text-primary-foreground' : 'bg-secondary text-secondary-foreground'"
              >
                <User v-if="msg.role === 'user'" class="size-4" />
                <Bot v-else class="size-4" />
              </AvatarFallback>
            </Avatar>
            <div
              class="max-w-[72%] rounded-xl px-4 py-2.5 text-sm leading-relaxed"
              :class="msg.role === 'user'
                ? 'bg-primary text-primary-foreground rounded-tr-xs'
                : 'bg-card text-card-foreground border rounded-tl-xs shadow-2xs'"
            >
              <div v-if="msg.role === 'user'" class="whitespace-pre-wrap">{{ msg.content }}</div>
              <div v-else class="content-markdown" v-html="renderMarkdown(msg.content)" />
            </div>
          </div>

          <div v-for="item in confirmations" :key="item.id" class="flex items-start gap-3">
            <Avatar class="size-8 shrink-0">
              <AvatarFallback class="bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-400">
                <Bot class="size-4" />
              </AvatarFallback>
            </Avatar>
            <div class="max-w-[72%] flex-1">
              <AgentConfirmCard :item="item" @resolve="resolveConfirm" />
            </div>
          </div>

          <div v-if="waitingReply" class="flex items-start gap-3">
            <Avatar class="size-8 shrink-0 animate-pulse">
              <AvatarFallback class="bg-secondary text-secondary-foreground">
                <Loader2 class="size-4 animate-spin" />
              </AvatarFallback>
            </Avatar>
            <div class="rounded-xl rounded-tl-xs border bg-card px-4 py-2.5 shadow-2xs flex items-center gap-2.5">
              <div class="flex items-center gap-1">
                <span class="size-1.5 rounded-full bg-primary animate-bounce [animation-delay:0ms]" />
                <span class="size-1.5 rounded-full bg-primary animate-bounce [animation-delay:150ms]" />
                <span class="size-1.5 rounded-full bg-primary animate-bounce [animation-delay:300ms]" />
              </div>
              <span class="text-xs text-muted-foreground">{{ t('agent.thinking') }}</span>
            </div>
          </div>
        </div>

        <div class="p-4 border-t bg-card space-y-2.5">
          <Textarea
            v-model="inputMessage"
            rows="3"
            :placeholder="t('agent.inputPlaceholder')"
            :disabled="isThinking"
            class="resize-none"
            @keydown.enter.prevent="sendMessage"
          />
          <div class="flex justify-end">
            <Button :disabled="isThinking" @click="sendMessage">
              <Loader2 v-if="isThinking" class="animate-spin" />
              <Send v-else />
              {{ t('agent.send') }}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>

<style scoped>
.content-markdown :deep(p) {
  margin: 0 0 8px 0;
}
.content-markdown :deep(p:last-child) {
  margin-bottom: 0;
}
.content-markdown :deep(pre) {
  background: var(--muted);
  padding: 10px 12px;
  border-radius: 6px;
  overflow-x: auto;
  border: 1px solid var(--border);
  font-size: 12px;
  margin: 8px 0;
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
</style>
