<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import AppIcon from './AppIcon.vue'
import { useAuthStore } from '../stores/auth'
import { api } from '../api'

const route = useRoute()
const auth = useAuthStore()
const unread = defineModel<number>('unread', { default: 0 })

const items = computed(() => [
  { to: '/', icon: 'home', label: '首页' },
  { to: '/hot', icon: 'hot', label: '热门' },
  { to: '/publish', icon: 'plus', label: '发布', create: true },
  { to: '/messages', icon: 'message', label: '消息', badge: unread.value },
  { to: '/me', icon: 'user', label: auth.isLoggedIn ? '我' : '登录' },
])

watch(() => auth.isLoggedIn, async (loggedIn) => {
  if (!loggedIn) {
    unread.value = 0
    return
  }
  try { unread.value = Math.max(0, (await api.unread()).count) } catch { unread.value = 0 }
}, { immediate: true })
</script>

<template>
  <nav class="bottom-nav" aria-label="手机端主导航">
    <RouterLink v-for="item in items" :key="item.to" :to="item.to" :class="{ active: route.path === item.to, create: item.create }">
      <span class="nav-icon"><AppIcon :name="item.icon" :size="item.create ? 27 : 23" :filled="route.path === item.to && !item.create" /><i v-if="item.badge" class="badge">{{ item.badge > 99 ? '99+' : item.badge }}</i></span>
      <small v-if="!item.create">{{ item.label }}</small>
    </RouterLink>
  </nav>
</template>

<style scoped>
.bottom-nav { position: fixed; z-index: 80; right: 0; bottom: 0; left: 0; height: calc(62px + env(safe-area-inset-bottom)); padding: 7px 10px env(safe-area-inset-bottom); display: grid; grid-template-columns: repeat(5, 1fr); background: rgba(17,17,20,.96); border-top: 1px solid rgba(255,255,255,.08); backdrop-filter: blur(18px); }
.bottom-nav a { min-width: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 2px; color: #777780; }
.bottom-nav a.active { color: #fff; }
.bottom-nav small { font-size: 10px; font-weight: 600; }
.nav-icon { position: relative; height: 27px; display: grid; place-items: center; }
.create .nav-icon { width: 44px; height: 29px; border-radius: 8px; background: linear-gradient(90deg, #24d7e8 0 18%, #fff 18% 82%, #ff315a 82%); color: #111114; }
.badge { position: absolute; top: -5px; right: -10px; min-width: 16px; height: 16px; padding: 0 4px; display: grid; place-items: center; border: 2px solid #111114; border-radius: 10px; background: #fe2c55; color: #fff; font-size: 8px; font-style: normal; font-weight: 800; }
</style>
