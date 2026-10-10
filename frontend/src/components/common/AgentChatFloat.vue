<template>
  <div class="agent-chat-float">
    <!-- Floating Button -->
    <transition name="bounce">
      <el-button
        v-show="!isOpen"
        class="float-btn"
        type="success"
        circle
        size="large"
        @click="isOpen = true"
      >
        <el-icon :size="24"><ChatDotRound /></el-icon>
      </el-button>
    </transition>

    <!-- Chat Window -->
    <transition name="chat-popup">
      <div v-show="isOpen" :class="['chat-window', { expanded: isExpanded }]">
        <div class="chat-header">
          <div class="header-title">
            <el-icon><ChatDotRound /></el-icon>
            <span>{{ t('agent.chatTitle') }}</span>
            <el-tag type="success" size="small" v-if="connected">{{ t('agent.connected') }}</el-tag>
          </div>
          <div class="header-actions">
            <el-button text circle size="small" @click="toggleExpand" :title="isExpanded ? '缩小' : '放大'">
              <el-icon><FullScreen /></el-icon>
            </el-button>
            <el-button text circle size="small" @click="clearMessages">
              <el-icon><Delete /></el-icon>
            </el-button>
            <el-button text circle size="small" @click="isOpen = false">
              <el-icon><Close /></el-icon>
            </el-button>
          </div>
        </div>

        <div class="chat-body" ref="messagesRef">
          <div v-if="messages.length === 0" class="empty-state">
            <el-empty :description="t('agent.welcome')" :image-size="80" />
            <div class="suggestions">
              <el-button size="small" @click="setInput(t('agent.checkStatus'))">{{ t('agent.checkStatus') }}</el-button>
              <el-button size="small" @click="setInput(t('agent.listContainers'))">{{ t('agent.listContainers') }}</el-button>
            </div>
          </div>

          <div v-for="(msg, index) in messages" :key="index" :class="['message-row', msg.role]">
            <div class="message-avatar">
              <el-avatar :icon="msg.role === 'user' ? UserFilled : Monitor" :size="28" 
                :style="{ backgroundColor: msg.role === 'user' ? '#409EFF' : '#67C23A' }" />
            </div>
            <div class="message-content">
              <div class="content-text" v-if="msg.role === 'user'">{{ msg.content }}</div>
              <div class="content-markdown" v-else v-html="renderMarkdown(msg.content)"></div>
            </div>
          </div>

          <div v-if="isThinking" class="message-row assistant">
            <div class="message-avatar">
              <el-avatar :icon="Loading" :size="28" style="background-color: #67C23A" class="thinking-avatar" />
            </div>
            <div class="message-content thinking-content">
              <div class="typing-indicator">
                <span></span><span></span><span></span>
              </div>
            </div>
          </div>
        </div>

        <div class="chat-footer">
          <el-input
            ref="inputRef"
            v-model="inputMessage"
            :placeholder="t('agent.inputPlaceholder')"
            @keydown.enter.prevent="sendMessage"
            :disabled="isThinking"
            size="small"
          />
          <el-button type="primary" size="small" @click="sendMessage" :disabled="isThinking">
            <el-icon><Promotion /></el-icon>
          </el-button>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import { ElMessage } from 'element-plus'
import { ChatDotRound, Close, Delete, UserFilled, Monitor, Loading, Promotion, FullScreen } from '@element-plus/icons-vue'
import { getToken } from '@/utils/request'
import { getTargetNodeId } from '@/utils/targetNode'

const { t } = useI18n()

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
const inputRef = ref<any>()

const focusInput = () => {
  nextTick(() => {
    if (inputRef.value) {
      inputRef.value.focus()
    }
  })
}

// Watch for window open to focus input and scroll to bottom
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

// Load chat history from server
const loadHistory = async () => {
  const token = getToken()
  try {
    const response = await fetch('/api/agent/history', {
      headers: { 'Authorization': `Bearer ${token}` }
    })
    if (response.ok) {
      const data = await response.json()
      if (Array.isArray(data)) {
        messages.value = data.map((m: { role: string; content: string }) => ({
          role: m.role as 'user' | 'assistant',
          content: m.content
        }))
        scrollToBottom()
      }
    }
  } catch (e) {
    console.error('Failed to load history:', e)
  }
}

// Clear all history
const clearMessages = async () => {
  const token = getToken()
  try {
    await fetch('/api/agent/history', {
      method: 'DELETE',
      headers: { 'Authorization': `Bearer ${token}` }
    })
    messages.value = []
  } catch (e) {
    console.error('Failed to clear history:', e)
  }
}

const renderMarkdown = (text: string) => {
  if (!text) return ''
  // 优化工具调用显示: 将 [[TOOL: name]] 转换为更美观的引用块
  let formattedText = text
    .replace(/\[\[TOOL:\s*(.*?)\]\]/g, '> 🛠️ **正在执行工具**: `$1`')
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
        'Authorization': `Bearer ${token}`
      },
      body: JSON.stringify({
        message: userMsg,
        node_id: getTargetNodeId(),
        history: messages.value.slice(0, -1).map(m => ({ role: m.role, content: m.content }))
      })
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
          // Keep isThinking true until stream fully completes
          
          messages.value.push({ role: 'assistant', content: fullContent })
          assistantMsgIndex = messages.value.length - 1
        } else {
          messages.value.splice(assistantMsgIndex, 1, { 
            role: 'assistant', 
            content: fullContent 
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
      const finalMessage: Message = { role: 'assistant', content: fullContent }
      messages.value.splice(assistantMsgIndex, 1, finalMessage)
    }
    // Stream complete - now enable input
    isThinking.value = false
    focusInput()
  } catch (error) {
    console.error('Chat error:', error)
    ElMessage.error(t('agent.connectionError'))
    isThinking.value = false
    const errorMessage: Message = { role: 'assistant', content: `**${t('agent.error')}**: ${t('agent.responseError')}` }
    messages.value.push(errorMessage)
  }
}

// Load history when component mounts
onMounted(() => {
  loadHistory()
})
</script>

<style scoped>
.agent-chat-float {
  position: fixed;
  right: 20px;
  bottom: 20px;
  z-index: 2000;
}

.float-btn {
  width: 56px;
  height: 56px;
  box-shadow: 0 4px 16px rgba(103, 194, 58, 0.4);
}

.float-btn:hover {
  transform: scale(1.1);
  box-shadow: 0 6px 20px rgba(103, 194, 58, 0.5);
}

.chat-window {
  position: absolute;
  right: 0;
  bottom: 70px;
  width: 380px;
  height: 520px;
  background: var(--el-bg-color);
  border-radius: 12px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
  border: 1px solid var(--el-border-color);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transform-origin: bottom right;
  transition: width 0.3s cubic-bezier(0.4, 0, 0.2, 1), 
              height 0.3s cubic-bezier(0.4, 0, 0.2, 1), 
              right 0.3s cubic-bezier(0.4, 0, 0.2, 1), 
              bottom 0.3s cubic-bezier(0.4, 0, 0.2, 1),
              opacity 0.3s cubic-bezier(0.4, 0, 0.2, 1),
              transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.chat-window.expanded {
  width: 50vw;
  height: 70vh;
  right: 20px;
  bottom: 80px;
}

.chat-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: var(--el-color-success);
  color: white;
}

.header-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}

.header-title .el-tag {
  margin-left: 4px;
}

.header-actions {
  display: flex;
  gap: 4px;
}

.header-actions .el-button {
  color: white;
}

.chat-body {
  flex: 1;
  overflow-y: auto;
  padding: 12px;
  background: var(--el-fill-color-light);
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  gap: 16px;
}

.suggestions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: center;
}

.message-row {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.message-row.user {
  flex-direction: row-reverse;
}

.message-content {
  max-width: 75%;
  padding: 8px 12px;
  border-radius: 12px;
  font-size: 13px;
  line-height: 1.5;
}

.message-row.user .message-content {
  background: var(--el-color-primary);
  color: white;
  border-bottom-right-radius: 4px;
}

.message-row.assistant .message-content {
  background: var(--el-bg-color);
  color: var(--el-text-color-primary);
  border-bottom-left-radius: 4px;
  border: 1px solid var(--el-border-color-lighter);
}

.content-markdown :deep(p) {
  margin: 0 0 8px 0;
}

.content-markdown :deep(p:last-child) {
  margin-bottom: 0;
}

.content-markdown :deep(hr) {
  border: none;
  border-top: 1px dashed var(--el-border-color);
  margin: 12px 0;
}

.content-markdown :deep(ul), .content-markdown :deep(ol) {
  margin: 8px 0;
  padding-left: 20px;
}

.content-markdown :deep(li) {
  margin-bottom: 4px;
}

.content-markdown :deep(pre) {
  background: var(--el-fill-color-darker);
  padding: 12px;
  border-radius: 8px;
  overflow-x: auto;
  font-size: 12px;
  margin: 8px 0;
  border: 1px solid var(--el-border-color-lighter);
}

.content-markdown :deep(code) {
  font-family: 'JetBrains Mono', 'Consolas', monospace;
  background: var(--el-fill-color-light);
  padding: 2px 4px;
  border-radius: 4px;
  color: var(--el-color-primary-light-3);
}

.content-markdown :deep(pre code) {
  padding: 0;
  background: transparent;
  color: inherit;
}

/* Table Styling */
.content-markdown :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 8px 0;
  font-size: 12px;
}

.content-markdown :deep(th), .content-markdown :deep(td) {
  border: 1px solid var(--el-border-color);
  padding: 6px 10px;
  text-align: left;
}

.content-markdown :deep(th) {
  background: var(--el-fill-color-light);
  font-weight: 600;
}

/* Tool Call Styling */
.content-markdown :deep(blockquote) {
  margin: 8px 0;
  padding: 10px 14px;
  border-left: 4px solid var(--el-color-success);
  background: var(--el-fill-color-light);
  border-radius: 6px;
  color: var(--el-text-color-primary);
  font-size: 12px;
  word-break: break-all;
  overflow-wrap: break-word;
}

.content-markdown :deep(blockquote strong) {
  color: var(--el-color-success);
}

.content-markdown :deep(blockquote p) {
  margin: 0;
}

.content-markdown :deep(blockquote code) {
  background: var(--el-fill-color-darker);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 11px;
  word-break: break-all;
}

/* Scrollbar Styling */
.chat-body::-webkit-scrollbar {
  width: 6px;
}

.chat-body::-webkit-scrollbar-thumb {
  background: var(--el-border-color-lighter);
  border-radius: 3px;
}

.chat-body::-webkit-scrollbar-thumb:hover {
  background: var(--el-border-color);
}

.thinking-content {
  display: flex;
  align-items: center;
  gap: 8px;
}

.thinking-avatar {
  animation: pulse 1.5s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.typing-indicator {
  display: flex;
  gap: 4px;
}

.typing-indicator span {
  width: 6px;
  height: 6px;
  background: var(--el-color-success);
  border-radius: 50%;
  animation: bounce 1.4s infinite ease-in-out;
}

.typing-indicator span:nth-child(2) { animation-delay: 0.2s; }
.typing-indicator span:nth-child(3) { animation-delay: 0.4s; }

@keyframes bounce {
  0%, 80%, 100% { transform: scale(0); }
  40% { transform: scale(1); }
}

.chat-footer {
  display: flex;
  gap: 8px;
  padding: 12px;
  border-top: 1px solid var(--el-border-color);
  background: var(--el-bg-color);
}

.chat-footer .el-input {
  flex: 1;
}

/* Animations */
.bounce-enter-active {
  transition: all 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
}
.bounce-leave-active {
  transition: all 0.2s ease-in;
}
.bounce-enter-from,
.bounce-leave-to {
  transform: scale(0);
  opacity: 0;
}

.chat-popup-enter-from,
.chat-popup-leave-to {
  opacity: 0;
  transform: scale(0.85) translateY(20px);
}

.chat-popup-enter-to,
.chat-popup-leave-from {
  opacity: 1;
  transform: scale(1) translateY(0);
}
</style>
