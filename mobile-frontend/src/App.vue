<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import BottomNav from './components/BottomNav.vue'
import ToastHost from './components/ToastHost.vue'
import { useAuthStore } from './stores/auth'

const route = useRoute()
const auth = useAuthStore()
const showBottomNav = computed(() => !route.path.startsWith('/video/') && !route.path.startsWith('/user/'))

function syncAuth(event: StorageEvent) {
  if (event.key === 'jwt_token') auth.syncFromStorage()
}

onMounted(() => window.addEventListener('storage', syncAuth))
onUnmounted(() => window.removeEventListener('storage', syncAuth))
</script>

<template>
  <RouterView />
  <BottomNav v-if="showBottomNav" />
  <ToastHost />
</template>
