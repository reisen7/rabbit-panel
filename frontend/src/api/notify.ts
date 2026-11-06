import request from '@/utils/request'

export interface NotifySettings {
  enabled: boolean
  token?: string
  token_mask: string
  has_token: boolean
  all_containers: boolean
  containers: string[]
  exit_nonzero: boolean
  unhealthy: boolean
  restart_storm: boolean
  recovery: boolean
  cooldown_minutes: number
}

export function getNotifySettings() {
  return request.get<NotifySettings>('/settings/notify')
}

export function saveNotifySettings(data: NotifySettings) {
  return request.post<NotifySettings>('/settings/notify', data)
}

export function testNotify(token: string) {
  return request.post('/settings/notify/test', { token }, { timeout: 20000 })
}
