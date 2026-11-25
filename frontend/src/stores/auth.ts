import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '@/api/auth'
import { useUpdateStore } from '@/stores/update'

/**
 * Authentication store
 *
 * 认证完全依赖 httpOnly cookie：前端 JS 不接触 token，
 * XSS 无法窃取凭证。authenticated 只是会话状态的内存镜像，
 * 页面刷新后由路由守卫触发 checkAuth 向服务端重建。
 */
export const useAuthStore = defineStore('auth', () => {
  const updateStore = useUpdateStore()
  // State
  const authenticated = ref(false)
  const authChecked = ref(false)
  const username = ref<string | null>(null)
  const needChangePassword = ref(false)

  // Getters
  const isAuthenticated = computed(() => authenticated.value)

  // Actions

  /**
   * Login with username, password and captcha
   * @param loginUsername User's username
   * @param password User's password
   * @param captcha User's captcha input
   * @param captchaId Captcha ID
   * @returns true if login successful
   */
  async function login(loginUsername: string, password: string, captcha?: string, captchaId?: string): Promise<boolean> {
    const response = await authApi.login({
      username: loginUsername,
      password,
      captcha,
      captcha_id: captchaId,
    })

    // 后端已通过 Set-Cookie 写入 httpOnly 会话，这里只记录状态
    authenticated.value = true
    authChecked.value = true
    username.value = loginUsername
    needChangePassword.value = response.need_change_password

    return true
  }

  /**
   * Logout current user
   * Clears local state and server session
   */
  async function logout(): Promise<void> {
    try {
      await authApi.logout()
    } catch {
      // Ignore logout errors, still clear local state
    } finally {
      clearSession()
    }
  }

  /**
   * Clear local auth state without calling the API
   * Used when the session is already invalid (e.g. a 401 response)
   */
  function clearSession(): void {
    authenticated.value = false
    username.value = null
    needChangePassword.value = false
    updateStore.resetAllState()
  }

  /**
   * Change user password
   * @param oldPassword Current password
   * @param newPassword New password
   */
  async function changePassword(oldPassword: string, newPassword: string): Promise<void> {
    await authApi.changePassword({
      old_password: oldPassword,
      new_password: newPassword,
    })

    // Password changed, no longer need to change
    needChangePassword.value = false
  }

  /**
   * Check authentication status
   * Verifies the cookie session by fetching current user info
   * @returns true if authenticated
   */
  async function checkAuth(): Promise<boolean> {
    try {
      const userInfo = await authApi.getCurrentUser()
      username.value = userInfo.username
      needChangePassword.value = userInfo.need_change_password ?? false
      authenticated.value = true
      authChecked.value = true
      return true
    } catch {
      // Session invalid or absent
      clearSession()
      authChecked.value = true
      return false
    }
  }

  return {
    // State
    authenticated,
    authChecked,
    username,
    needChangePassword,
    // Getters
    isAuthenticated,
    // Actions
    login,
    logout,
    clearSession,
    changePassword,
    checkAuth,
  }
})
