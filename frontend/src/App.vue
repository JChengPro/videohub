<script setup lang="ts">
import { onBeforeUnmount, watch } from 'vue'

import { useAuthStore } from './stores/auth'
import { useChatStore } from './stores/chat'
import { useNotificationStore } from './stores/notification'
import { useRealtimeStore } from './stores/realtime'
import ConfirmDialog from './components/ConfirmDialog.vue'

const auth = useAuthStore()
const chat = useChatStore()
const notifications = useNotificationStore()
const realtime = useRealtimeStore()

watch(() => auth.isLoggedIn, (loggedIn) => {
  if (loggedIn) {
    void Promise.all([chat.refreshUnread(), notifications.refreshUnread()])
    void realtime.connect()
  } else {
    realtime.disconnect()
    chat.clear()
    notifications.clear()
  }
}, { immediate: true })

onBeforeUnmount(() => realtime.disconnect())
</script>

<template>
  <RouterView />
  <ConfirmDialog />
</template>
