<template>
  <div class="agent-chat-container">
    <el-card class="chat-card" body-style="height: 100%; display: flex; flex-direction: column; padding: 0;">
      <template #header>
        <div class="card-header">
          <span>{{ t('agent.chatTitle') }}</span>
          <el-tag type="success" size="small" v-if="connected">{{ t('agent.connected') }}</el-tag>
          <el-tag type="info" size="small" v-else>{{ t('agent.disconnected') }}</el-tag>
        </div>
      </template>
      
      <!-- 聊天消息区域 -->
      <transition-group name="message-fade" tag="div" class="chat-messages" ref="messagesRef">
        <div v-if="messages.length === 0" key="empty" class="empty-state">
          <el-empty :description="t('agent.welcome')" />
          <div class="suggestions">
            <el-button size="small" @click="setInput(t('agent.checkStatus'))">{{ t('agent.checkStatus') }}</el-button>
            <el-button size="small" @click="setInput(t('agent.listContainers'))">{{ t('agent.listContainers') }}</el-button>
          </div>
        </div>
        
        <div v-for="(msg, index) in messages" :key="index" :class="['message-row', msg.role]">
          <div class="message-avatar">
            <el-avatar :icon="msg.role === 'user' ? UserFilled : Monitor" :size="32" 
              :style="{ backgroundColor: msg.role === 'user' ? '#409EFF' : '#67C23A' }" />
          </div>
          <div class="message-content">
            <div class="content-text" v-if="msg.role === 'user'">{{ msg.content }}</div>
            <div class="content-markdown" v-else v-html="renderMarkdown(msg.content)"></div>
          </div>
        </div>
        
        <div v-if="isThinking" key="thinking" class="message-row assistant">
           <div class="message-avatar">
            <el-avatar :icon="Loading" :size="32" style="background-color: #67C23A" class="thinking-avatar" />
          </div>
          <div class="message-content thinking-content">
            <div class="typing-indicator">
              <span></span><span></span><span></span>
            </div>
            <span class="thinking-text">{{ t('agent.thinking') }}</span>
          </div>
        </div>
      </transition-group>
      
      <!-- 输入区域 -->
      <div class="chat-input">
        <el-input
          v-model="inputMessage"
          type="textarea"
          :rows="3"
          :placeholder="t('agent.inputPlaceholder')"
          @keydown.enter.prevent="sendMessage"
          :disabled="isThinking"
        />
        <div class="input-actions">
          <el-button type="primary" @click="sendMessage" :loading="isThinking">{{ t('agent.send') }}</el-button>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick, onMounted, onUpdated } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import { ElMessage } from 'element-plus'
import { UserFilled, Monitor, Loading } from '@element-plus/icons-vue'
import { getToken } from '@/utils/request'
import { getTargetNodeId } from '@/utils/targetNode'

const { t } = useI18n()

interface Message {
  role: 'user' | 'assistant'
  content: string
}

const messages = ref<Message[]>([])
const inputMessage = ref('')
const isThinking = ref(false)
const connected = ref(true)
const messagesRef = ref<HTMLElement>()

const setInput = (text: string) => {
  inputMessage.value = text
  nextTick(() => {
    // optional: auto focus
  })
}

const renderMarkdown = (text: string) => {
  // Format tool calls to be user friendly (handle complete and partial)
  let formattedText = text
    .replace(/\$\$TOOL_CALL:\s*(.*?)\$\$/g, '> ⚙️ *$1*') // Complete
    .replace(/\$\$TOOL_CALL:\s*(.*)$/g, '> ⚙️ *$1*') // Partial at end
    .replace(/\$\$/g, '') // Remove isolated markers if any remain
  
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
  messages.value.push({ role: 'user', content: userMsg })
  inputMessage.value = ''
  isThinking.value = true
  scrollToBottom()

  // 准备流式请求
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
        history: messages.value.slice(0, -1).map(m => ({ role: m.role, content: m.content })) // Simple history
      })
    })

    if (!response.ok) {
       throw new Error(`HTTP error! status: ${response.status}`)
    }

    if (!response.body) {
         throw new Error('Response body is null')
    }

    // 添加助手消息占位
    const reader = response.body.getReader()
    const decoder = new TextDecoder()
    
    let started = false
    let assistantMsgIndex = -1
    let fullContent = ''
    let buffer = ''
    
    const processLine = (line: string) => {
      if (line.startsWith('data: ')) {
        const data = line.slice(6)
        fullContent += data
        
        if (!started) {
          started = true
          isThinking.value = false
          // Push initial message
          messages.value.push({ role: 'assistant', content: fullContent })
          assistantMsgIndex = messages.value.length - 1
        } else {
          // Use Vue.set pattern - replace the object to trigger reactivity
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
      const lines = buffer.split('\n\n')
      // Keep the last part in buffer if it's not empty (incomplete chunk)
      buffer = lines.pop() || '' 
      
      for (const line of lines) {
        processLine(line)
      }
    }
    
    // Process remaining buffer after stream ends
    buffer += decoder.decode() // Flush decoder
    if (buffer) {
      const lines = buffer.split('\n\n')
      for (const line of lines) {
        processLine(line)
      }
    }
    
    // Final update to ensure message is complete
    if (started && assistantMsgIndex >= 0) {
      messages.value.splice(assistantMsgIndex, 1, { 
        role: 'assistant', 
        content: fullContent 
      })
    }
  } catch (error) {
    console.error('Chat error:', error)
    ElMessage.error(t('agent.connectionError'))
    isThinking.value = false
    messages.value.push({ role: 'assistant', content: `**${t('agent.error')}**: ${t('agent.responseError')}` })
  }
}

onMounted(() => {
    // Optional: Load history?
})

onUpdated(() => {
    scrollToBottom()
})
</script>

<style scoped>
.agent-chat-container {
  height: calc(100vh - 120px); /* Adjust based on layout */
  display: flex;
  flex-direction: column;
}

.chat-card {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  background-color: var(--el-fill-color-light);
}

.message-row {
  display: flex;
  margin-bottom: 20px;
  align-items: flex-start;
}

.message-row.user {
  flex-direction: row-reverse;
}

.message-content {
  max-width: 70%;
  padding: 12px 16px;
  border-radius: 8px;
  margin: 0 12px;
  font-size: 14px;
  line-height: 1.5;
  box-shadow: 0 2px 4px rgba(0,0,0,0.05);
}

.message-row.user .message-content {
  background-color: #409EFF;
  color: white;
  border-top-right-radius: 2px;
}

.message-row.assistant .message-content {
  background-color: var(--el-bg-color);
  color: var(--el-text-color-primary);
  border-top-left-radius: 2px;
}

.content-markdown :deep(pre) {
  background-color: var(--el-fill-color);
  padding: 10px;
  border-radius: 4px;
  overflow-x: auto;
}

.content-markdown :deep(p) {
    margin: 0 0 10px 0;
}
.content-markdown :deep(p:last-child) {
    margin: 0;
}

.chat-input {
  padding: 20px;
  background-color: var(--el-bg-color);
  border-top: 1px solid var(--el-border-color-lighter);
}

.input-actions {
  margin-top: 10px;
  display: flex;
  justify-content: flex-end;
}

.typing-indicator {
  display: flex;
  align-items: center;
  height: 20px;
}

.typing-indicator span {
  display: inline-block;
  width: 8px;
  height: 8px;
  background-color: #409EFF;
  border-radius: 50%;
  margin: 0 3px;
  animation: bounce 1.4s infinite ease-in-out both;
}

.typing-indicator span:nth-child(1) { animation-delay: -0.32s; }
.typing-indicator span:nth-child(2) { animation-delay: -0.16s; }

@keyframes bounce {
  0%, 80%, 100% { 
    transform: scale(0);
    opacity: 0.5;
  }
  40% { 
    transform: scale(1);
    opacity: 1;
  }
}

.thinking-content {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 24px;
}

.thinking-text {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  animation: fadeIn 0.5s ease-in;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.thinking-avatar {
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0% { box-shadow: 0 0 0 0 rgba(103, 194, 58, 0.4); }
  70% { box-shadow: 0 0 0 10px rgba(103, 194, 58, 0); }
  100% { box-shadow: 0 0 0 0 rgba(103, 194, 58, 0); }
}

/* Message Transition Animations */
.message-fade-enter-active,
.message-fade-leave-active {
  transition: all 0.4s ease;
}

.message-fade-enter-from {
  opacity: 0;
  transform: translateY(20px);
}

.message-fade-leave-to {
  opacity: 0;
  transform: translateY(-20px);
}

.empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
}
.suggestions {
    margin-top: 20px;
    display: flex;
    gap: 10px;
}
</style>
