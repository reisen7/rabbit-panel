import request from '@/utils/request'

export interface WeChatQR {
  image?: string
  status: string
  message?: string
}

export interface WeChatStatus {
  linked: boolean
  token_mask: string
  account_id: string
  saved_at: string
  online: boolean
  session_expired: boolean
  qr?: WeChatQR | null
}

export function getWeChatStatus() {
  return request.get<WeChatStatus>('/channels/wechat')
}

export function createWeChatQR() {
  return request.post<WeChatQR>('/channels/wechat/qrcode')
}

export function getWeChatQRStatus() {
  return request.get<WeChatQR>('/channels/wechat/qrcode/status')
}

export function submitWeChatVerify(code: string) {
  return request.post('/channels/wechat/verify', { code })
}

export function unbindWeChat() {
  return request.delete('/channels/wechat')
}
