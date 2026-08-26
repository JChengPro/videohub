<script setup lang="ts">
import { computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import BottomNav from './components/BottomNav.vue'
import ToastHost from './components/ToastHost.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'
import SessionEndedDialog from './components/SessionEndedDialog.vue'
import { api } from './api'
import { useAuthStore } from './stores/auth'
import { useChatStore } from './stores/chat'
import { useNotificationStore } from './stores/notification'
import { useRealtimeStore } from './stores/realtime'

const route = useRoute()
const auth = useAuthStore()
const chat = useChatStore()
const notifications = useNotificationStore()
const realtime = useRealtimeStore()
let sessionCheckTimer = 0
const showBottomNav = computed(() =>
  !route.path.startsWith('/video/')
  && !route.path.startsWith('/user/')
  && !/^\/chat\/\d+/.test(route.path),
)

async function checkSession() {
  if (!auth.isLoggedIn) return
  try { await api.me() } catch { /* The API client handles unauthorized sessions. */ }
}

function stopSessionChecks() {
  window.clearInterval(sessionCheckTimer)
  sessionCheckTimer = 0
}

function startSessionChecks() {
  stopSessionChecks()
  void checkSession()
  sessionCheckTimer = window.setInterval(() => void checkSession(), 15_000)
}

function onVisibilityChange() {
  if (!document.hidden) void checkSession()
}

watch(() => auth.isLoggedIn, (loggedIn) => {
  if (loggedIn) {
    void Promise.all([chat.refresh(), notifications.refresh()])
    void realtime.connect()
    startSessionChecks()
  } else {
    stopSessionChecks()
    realtime.disconnect()
    chat.clear()
    notifications.clear()
  }
}, { immediate: true })

function syncAuth(event: StorageEvent) {
  if (event.key === 'jwt_token') auth.syncFromStorage()
}

onMounted(() => {
  window.addEventListener('storage', syncAuth)
  document.addEventListener('visibilitychange', onVisibilityChange)
})
onUnmounted(() => {
  stopSessionChecks()
  realtime.disconnect()
  window.removeEventListener('storage', syncAuth)
  document.removeEventListener('visibilitychange', onVisibilityChange)
})
</script>

<template>
  <RouterView />
  <BottomNav v-if="showBottomNav" />
  <ToastHost />
  <ConfirmDialog />
  <SessionEndedDialog />
</template>
