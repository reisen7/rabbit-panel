import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useToast } from '@/composables/useToast'
import { handleUnauthorized } from '@/utils/request'
import { getTargetNodeId } from '@/utils/targetNode'

export interface AgentMessage {
  role: 'user' | 'assistant'
  content: string
}

export type ConfirmState = 'pending' | 'approved' | 'done' | 'rejected' | 'expired'

export interface ConfirmRequest {
  id: string
  command: string
  summary: string
  state: ConfirmState
}

// 后端 SSE 事件协议（见 backend/service/agent_confirm.go ChatEvent）
interface ChatEvent {
  type: 'chunk' | 'confirm' | 'error'
  text?: string
  id?: string
  command?: string
  summary?: string
}

// useAgentChat 封装 AI 对话的完整客户端逻辑：
// 结构化 SSE 事件解析（文本片段/确认请求/错误）、危险操作确认回执、历史记录加载。
// AgentChat 页面与浮动聊天窗共用，避免协议解析出现两份实现。
export function useAgentChat(scrollToBottom: () => void) {
  const { t } = useI18n()
  const toast = useToast()

  const messages = ref<AgentMessage[]>([])
  // isThinking 在整个流式响应期间为 true（输入锁定），
  // 避免用户在等待确认的暂停间隙再次发送消息。
  const isThinking = ref(false)
  // waitingReply 仅控制"思考中"气泡：收到首个内容（文本或确认请求）后消失。
  const waitingReply = ref(false)
  const confirmations = ref<ConfirmRequest[]>([])

  const renderMarkdown = (text: string) => {
    if (!text) return ''
    const formattedText = text
      .replace(/\[\[TOOL:\s*(.*?)\]\]/g, '> 🛠️ **正在执行工具**: `$1`')
      .replace(/\$\$TOOL_CALL:\s*(.*?)\$\$/g, '> ⚙️ *$1*')
      .replace(/\$\$/g, '')
    // marked 不做任何消毒，LLM 输出/容器日志可能带 HTML，必须过 DOMPurify
    return DOMPurify.sanitize(marked(formattedText) as string)
  }

  const loadHistory = async () => {
    try {
      const response = await fetch('/api/agent/history')
      if (response.status === 401) handleUnauthorized()
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

  const clearHistory = async () => {
    try {
      const response = await fetch('/api/agent/history', { method: 'DELETE' })
      if (response.status === 401) handleUnauthorized()
      messages.value = []
      confirmations.value = []
    } catch (e) {
      console.error('Failed to clear history:', e)
    }
  }

  // resolveConfirm 把用户的允许/拒绝回执给后端，解除执行流的阻塞。
  // 404 说明确认已超时（后端按拒绝处理）；网络错误时保持 pending 允许重试。
  const resolveConfirm = async (item: ConfirmRequest, approve: boolean) => {
    if (item.state !== 'pending') return
    try {
      const response = await fetch('/api/agent/confirm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: item.id, approve }),
      })
      if (response.status === 401) handleUnauthorized()
      if (response.status === 404) {
        item.state = 'expired'
        return
      }
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`)
      }
      item.state = approve ? 'approved' : 'rejected'
    } catch (e) {
      console.error('Confirm error:', e)
      toast.error(t('agent.confirmFailed'))
    }
  }

  // 后端在 executeTool 结束后才推下一个事件：新 chunk 出现即表示已批准的执行已完成。
  const markApprovedDone = () => {
    for (const item of confirmations.value) {
      if (item.state === 'approved') item.state = 'done'
    }
  }

  const sendMessage = async (userMsg: string) => {
    if (!userMsg.trim() || isThinking.value) return

    messages.value.push({ role: 'user', content: userMsg })
    // 上一轮会话的确认卡片已失效，清掉避免误操作旧 ID
    confirmations.value = []
    isThinking.value = true
    waitingReply.value = true
    scrollToBottom()

    try {
      const response = await fetch('/api/agent/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          message: userMsg,
          node_id: getTargetNodeId(),
          history: messages.value.slice(0, -1).map((m) => ({ role: m.role, content: m.content })),
        }),
      })

      if (response.status === 401) handleUnauthorized()
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

      const appendChunk = (text: string) => {
        markApprovedDone()
        fullContent += text
        if (!started) {
          started = true
          waitingReply.value = false
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

      const processEvent = (eventBlock: string) => {
        for (const line of eventBlock.split('\n')) {
          if (!line.startsWith('data: ')) continue
          let evt: ChatEvent
          try {
            evt = JSON.parse(line.slice(6))
          } catch {
            continue // 忽略畸形事件，不断流
          }
          if (evt.type === 'chunk' && evt.text) {
            appendChunk(evt.text)
          } else if (evt.type === 'confirm' && evt.id) {
            // 执行流暂停等待用户决策：思考气泡消失，确认卡片出现
            waitingReply.value = false
            confirmations.value.push({
              id: evt.id,
              command: evt.command ?? '',
              summary: evt.summary ?? '',
              state: 'pending',
            })
            scrollToBottom()
          } else if (evt.type === 'error' && evt.text) {
            appendChunk(`\n\n**${t('agent.error')}**: ${evt.text}`)
          }
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
        for (const event of buffer.split('\n\n')) {
          processEvent(event)
        }
      }

      if (started && assistantMsgIndex >= 0) {
        messages.value.splice(assistantMsgIndex, 1, {
          role: 'assistant',
          content: fullContent,
        })
      }
    } catch (error) {
      console.error('Chat error:', error)
      toast.error(t('agent.connectionError'))
      messages.value.push({
        role: 'assistant',
        content: `**${t('agent.error')}**: ${t('agent.responseError')}`,
      })
    } finally {
      markApprovedDone()
      // SSE 流已结束，后端不再处理仍 pending 的确认，标记为过期
      for (const item of confirmations.value) {
        if (item.state === 'pending') item.state = 'expired'
      }
      isThinking.value = false
      waitingReply.value = false
    }
  }

  return {
    messages,
    isThinking,
    waitingReply,
    confirmations,
    renderMarkdown,
    loadHistory,
    clearHistory,
    resolveConfirm,
    sendMessage,
  }
}
