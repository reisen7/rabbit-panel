<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useUpdateStore } from '@/stores/update'
import { useTheme } from '@/composables/useTheme'
import { useI18n } from '@/composables/useI18n'
import ChangePasswordDialog from '@/components/common/ChangePasswordDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { Sonner } from '@/components/ui/sonner'

const route = useRoute()
const authStore = useAuthStore()
const updateStore = useUpdateStore()
const { initTheme } = useTheme()
const { initLanguage } = useI18n()

// Forced password change dialog
const showPasswordDialog = ref(false)

// Watch for need_change_password flag
watch(
  () => authStore.needChangePassword,
  (needChange) => {
    // Only show dialog if user is authenticated and on a protected route
    if (needChange && authStore.isAuthenticated && route.name !== 'login') {
      showPasswordDialog.value = true
    }
  },
  { immediate: true }
)

// Also check when route changes
watch(
  () => route.name,
  (name) => {
    if (authStore.needChangePassword && authStore.isAuthenticated && route.name !== 'login') {
      showPasswordDialog.value = true
    }
    if (name !== 'login' && authStore.isAuthenticated && !updateStore.info && !updateStore.loading) {
      updateStore.fetchUpdateInfo()
    }
  }
)

watch(
  () => authStore.isAuthenticated,
  (isAuthenticated, wasAuthenticated) => {
    if (isAuthenticated && !wasAuthenticated) {
      updateStore.fetchUpdateInfo()
    }
    if (!isAuthenticated && wasAuthenticated) {
      updateStore.resetAllState()
    }
  }
)

// Initialize on mount
onMounted(() => {
  authStore.initialize()
  initTheme()
  initLanguage()
  if (authStore.isAuthenticated) {
    updateStore.fetchUpdateInfo()
  }
})

function handlePasswordChangeSuccess() {
  showPasswordDialog.value = false
}
</script>

<template>
  <router-view />
  
  <!-- Forced password change dialog -->
  <ChangePasswordDialog
    v-model="showPasswordDialog"
    :forced="authStore.needChangePassword"
    @success="handlePasswordChangeSuccess"
  />
  <Sonner position="top-center" rich-colors />
  <ConfirmDialog />
</template>

<style>
/* Global app styles handled in styles/index.css */
</style>
