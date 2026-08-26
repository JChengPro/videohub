<script setup lang="ts">
import { onBeforeUnmount, onMounted, watch } from 'vue'

import * as accountApi from './api/account'
import { useAuthStore } from './stores/auth'
import { useChatStore } from './stores/chat'
import { useNotificationStore } from './stores/notification'
import { useRealtimeStore } from './stores/realtime'
import ConfirmDialog from './components/ConfirmDialog.vue'
import SessionEndedDialog from './components/SessionEndedDialog.vue'

const auth = useAuthStore()
const chat = useChatStore()
const notifications = useNotificationStore()
const realtime = useRealtimeStore()
let sessionCheckTimer = 0

async function checkSession() {
  if (!auth.isLoggedIn) return
  try { await accountApi.me() } catch { /* The API client handles unauthorized sessions. */ }
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
    void Promise.all([chat.refreshUnread(), notifications.refreshUnread()])
    void realtime.connect()
    startSessionChecks()
  } else {
    stopSessionChecks()
    realtime.disconnect()
    chat.clear()
    notifications.clear()
  }
}, { immediate: true })

onMounted(() => document.addEventListener('visibilitychange', onVisibilityChange))
onBeforeUnmount(() => {
  stopSessionChecks()
  document.removeEventListener('visibilitychange', onVisibilityChange)
  realtime.disconnect()
})
</script>

<template>
  <RouterView />
  <ConfirmDialog />
  <SessionEndedDialog />
</template>
