import request from '@/utils/request'

export type AgentAPIFormat = 'anthropic' | 'openai_chat' | 'openai_responses'
export type ThinkingEffort = 'low' | 'medium' | 'high' | 'xhigh'

export interface AgentConfig {
    api_url: string
    api_key: string
    api_format: AgentAPIFormat
    model: string
    enabled: boolean
    thinking: boolean
    thinking_effort: ThinkingEffort
}

export function getAgentConfig() {
    return request.get<AgentConfig>('/settings/agent')
}

export function saveAgentConfig(data: AgentConfig) {
    return request.post('/settings/agent', data)
}

export function listAgentModels(data: { api_url: string; api_format: string; api_key: string }) {
    return request.post<{ models: string[] }>('/settings/agent/models', data)
}

export function testAgentConnection(data: Pick<AgentConfig, 'api_url' | 'api_format' | 'api_key' | 'model'>) {
    return request.post<{ reply: string }>('/settings/agent/test', data, { timeout: 60000 })
}
