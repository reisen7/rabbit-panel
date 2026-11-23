import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '@/api/auth'
import { getToken, setToken, removeToken } from '@/utils/request'
import { useUpdateStore } from '@/stores/update'

/**
 * Authentication store
 * Manages user authentication state including token, username, and password change requirement
 */
export const useAuthStore = defineStore('auth', () => {
  const updateStore = useUpdateStore()
  // State
  const token = ref<string | null>(getToken())
  const username = ref<string | null>(null)
  const needChangePassword = ref(false)

  // Getters
  const isAuthenticated = computed(() => !!token.value)

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

    // Store token
    token.value = response.token
    setToken(response.token)

    // Store user info
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
    token.value = null
    username.value = null
    needChangePassword.value = false
    removeToken()
    updateStore.resetAllState()
  }

  /**
   * Change user password
   * @param oldPassword Current password
   * @param newPassword New password
   */
  async function changePassword(oldPassword: string, newPassword: string): Promise<void> {
    const response = await authApi.changePassword({
      old_password: oldPassword,
      new_password: newPassword,
    })
    
    // Update token if provided
    if (response.token) {
      token.value = response.token
      setToken(response.token)
    }
    
    // Password changed, no longer need to change
    needChangePassword.value = false
  }

  /**
   * Check authentication status
   * Verifies token validity by fetching current user info
   * @returns true if authenticated
   */
  async function checkAuth(): Promise<boolean> {
    if (!token.value) {
      return false
    }

    try {
      const userInfo = await authApi.getCurrentUser()
      username.value = userInfo.username
      needChangePassword.value = userInfo.need_change_password
      return true
    } catch {
      // Token invalid, clear state
      clearSession()
      return false
    }
  }

  /**
   * Initialize store from localStorage
   * Called on app startup
   */
  function initialize(): void {
    const storedToken = getToken()
    if (storedToken) {
      token.value = storedToken
    }
  }

  return {
    // State
    token,
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
    initialize,
  }
})
