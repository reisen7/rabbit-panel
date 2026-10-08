import axios, { type AxiosError, type AxiosInstance, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'
import router from '@/router'
import { getTargetNodeId } from '@/utils/targetNode'

declare module 'axios' {
  interface AxiosRequestConfig {
    suppressErrorMessage?: boolean
  }
  interface InternalAxiosRequestConfig {
    suppressErrorMessage?: boolean
  }
}

// Create axios instance
const request: AxiosInstance = axios.create({
  baseURL: '/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Token storage key
const TOKEN_KEY = 'rabbit_panel_token'

// Get token from localStorage
export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

// Set token to localStorage
export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

// Remove token from localStorage
export function removeToken(): void {
  localStorage.removeItem(TOKEN_KEY)
}

// Request interceptor
request.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const token = getToken()
    if (token && config.headers) {
      config.headers.Authorization = `Bearer ${token}`
    }
    const targetNodeId = getTargetNodeId()
    const url = (config.url || '').split('?')[0] || ''
    if (targetNodeId && config.headers && isNodeScopedPath(url)) {
      config.headers['X-Target-Node'] = targetNodeId
    }
    return config
  },
  (error: AxiosError) => {
    return Promise.reject(error)
  }
)

// Response interceptor
request.interceptors.response.use(
  (response: AxiosResponse) => {
    return response
  },
  (error: AxiosError) => {
    const status = error.response?.status
    const data = error.response?.data
    const suppressErrorMessage = (error.config as InternalAxiosRequestConfig | undefined)?.suppressErrorMessage

    if (suppressErrorMessage) {
      return Promise.reject(error)
    }

    if (status === 401) {
      // Token expired or invalid
      removeToken()
      router.push('/login')
      ElMessage.error('登录已过期，请重新登录')
    } else if (status === 403) {
      // Check if password change is required
      if (typeof data === 'object' && data !== null && 'need_change_password' in data) {
        // This will be handled by the auth store
        return Promise.reject({ ...error, needChangePassword: true })
      }
      ElMessage.error('没有权限执行此操作')
    } else if (status && status >= 500) {
      // 尝试获取后端返回的错误信息
      const errorMsg = getErrorMessage(data)
      ElMessage.error(errorMsg || '服务器错误，请稍后重试')
    } else if (status === 400 || status === 404 || status === 409) {
      // 客户端错误，显示后端返回的具体信息
      const errorMsg = getErrorMessage(data)
      if (errorMsg) {
        ElMessage.error(errorMsg)
      }
    } else {
      const errorMsg = getErrorMessage(data)
      if (errorMsg) {
        ElMessage.error(errorMsg)
      } else if (error.message) {
        ElMessage.error(error.message)
      }
    }

    return Promise.reject(error)
  }
)

/**
 * 从响应数据中提取错误信息
 * 支持纯文本和 JSON 格式
 */
function isNodeScopedPath(url: string): boolean {
  return ['/containers', '/networks', '/volumes', '/compose', '/registries'].some(
    (prefix) => url === prefix || url.startsWith(`${prefix}/`)
  )
}

function getErrorMessage(data: unknown): string | null {
  if (!data) return null
  
  // 如果是字符串，直接返回
  if (typeof data === 'string') {
    return data.trim() || null
  }
  
  // 如果是对象，尝试获取 error 或 message 字段
  if (typeof data === 'object' && data !== null) {
    const obj = data as Record<string, unknown>
    if (typeof obj.error === 'string') return obj.error
    if (typeof obj.message === 'string') return obj.message
  }
  
  return null
}

export default request
