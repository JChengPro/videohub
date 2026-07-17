<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import AppIcon from './AppIcon.vue'
import { useAuthStore } from '../stores/auth'
import { useNotificationStore } from '../stores/notification'

const route = useRoute()
const auth = useAuthStore()
const notifications = useNotificationStore()

const items = computed(() => [
  { to: '/', icon: 'home', label: '首页' },
  { to: '/hot', icon: 'hot', label: '热门' },
  { to: '/publish', icon: 'plus', label: '发布', create: true },
  { to: '/messages', icon: 'message', label: '消息', badge: notifications.unread },
  { to: '/me', icon: 'user', label: auth.isLoggedIn ? '我' : '登录' },
])

watch(() => auth.isLoggedIn, async (loggedIn) => {
  if (!loggedIn) {
    notifications.clear()
    return
  }
  await notifications.refresh()
}, { immediate: true })
</script>

<template>
  <nav class="bottom-nav" aria-label="手机端主导航">
    <RouterLink v-for="item in items" :key="item.to" :to="item.to" :class="{ active: route.path === item.to, create: item.create }" :aria-label="item.label">
      <span class="nav-icon"><AppIcon :name="item.icon" :size="item.create ? 27 : 23" :filled="route.path === item.to && !item.create" /><i v-if="item.badge" class="badge">{{ item.badge > 99 ? '99+' : item.badge }}</i></span>
      <small v-if="!item.create">{{ item.label }}</small>
    </RouterLink>
  </nav>
</template>

<style scoped>
.bottom-nav {
  position: fixed;
  z-index: 80;
  right: 0;
  bottom: 0;
  left: 0;
  height: calc(64px + env(safe-area-inset-bottom));
  padding: 6px 8px env(safe-area-inset-bottom);
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  border-top: 1px solid var(--mobile-border);
  background: rgba(11, 11, 13, .92);
  backdrop-filter: blur(20px) saturate(125%);
}

.bottom-nav a {
  position: relative;
  min-width: 44px;
  min-height: 50px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  color: var(--mobile-text-muted);
  transition: color var(--mobile-duration) ease;
}

.bottom-nav a.active {
  color: var(--mobile-text);
}

.bottom-nav a.active:not(.create)::after {
  position: absolute;
  bottom: 1px;
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: var(--mobile-accent);
  content: '';
}

.bottom-nav small {
  font-size: 10px;
  font-weight: 650;
}

.nav-icon {
  position: relative;
  height: 28px;
  display: grid;
  place-items: center;
}

.create .nav-icon {
  width: 46px;
  height: 30px;
  border: 1px solid rgba(255, 255, 255, .75);
  border-radius: 9px;
  background: #f4f4f5;
  color: #111114;
  box-shadow: 3px 0 0 var(--mobile-accent);
}

.badge {
  position: absolute;
  top: -5px;
  right: -11px;
  min-width: 17px;
  height: 17px;
  padding: 0 4px;
  display: grid;
  place-items: center;
  border: 2px solid var(--mobile-bg);
  border-radius: 10px;
  background: var(--mobile-accent);
  color: #fff;
  font-size: 8px;
  font-style: normal;
  font-weight: 800;
}
</style>
