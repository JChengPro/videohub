<script setup lang="ts">
import { computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import BottomNav from './components/BottomNav.vue'
import ToastHost from './components/ToastHost.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'
import { useAuthStore } from './stores/auth'
import { useChatStore } from './stores/chat'
import { useNotificationStore } from './stores/notification'
import { useRealtimeStore } from './stores/realtime'

const route = useRoute()
const auth = useAuthStore()
const chat = useChatStore()
const notifications = useNotificationStore()
const realtime = useRealtimeStore()
const showBottomNav = computed(() =>
  !route.path.startsWith('/video/')
  && !route.path.startsWith('/user/')
  && !/^\/chat\/\d+/.test(route.path),
)

watch(() => auth.isLoggedIn, (loggedIn) => {
  if (loggedIn) {
    void Promise.all([chat.refresh(), notifications.refresh()])
    void realtime.connect()
  } else {
    realtime.disconnect()
    chat.clear()
    notifications.clear()
  }
}, { immediate: true })

function syncAuth(event: StorageEvent) {
  if (event.key === 'jwt_token') auth.syncFromStorage()
}

onMounted(() => window.addEventListener('storage', syncAuth))
onUnmounted(() => {
  realtime.disconnect()
  window.removeEventListener('storage', syncAuth)
})
</script>

<template>
  <RouterView />
  <BottomNav v-if="showBottomNav" />
  <ToastHost />
  <ConfirmDialog />
</template>
