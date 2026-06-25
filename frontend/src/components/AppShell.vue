<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useSocialStore } from '../stores/social'
import { useNotificationStore } from '../stores/notification'
import Toaster from './Toaster.vue'

const props = defineProps<{ full?: boolean }>()

const auth = useAuthStore()
const social = useSocialStore()
const notifications = useNotificationStore()
const router = useRouter()
const route = useRoute()

const search = ref(typeof route.query.q === 'string' ? route.query.q : '')
watch(() => route.query.q, (v) => { search.value = typeof v === 'string' ? v : '' })

watch(() => auth.isLoggedIn, (v) => {
  if (v) {
    void social.refreshMine()
    void notifications.refreshUnread()
  } else {
    social.clear()
    notifications.clear()
  }
}, { immediate: true })

watch(() => route.path, () => {
  if (auth.isLoggedIn) void notifications.refreshUnread()
})

const userLabel = computed(() => {
  if (!auth.isLoggedIn) return '未登录'
  const name = auth.claims?.username ?? '(unknown)'
  return name
})

async function onSearch() {
  const q = search.value.trim()
  await router.push({ path: '/', query: q ? { q } : {} })
}
</script>

<template>
  <div class="shell">
    <aside class="sidebar">
      <RouterLink class="logo" to="/">
        <span class="logo-icon">VH</span>
        <span class="logo-text">VideoHub</span>
      </RouterLink>

      <nav class="nav">
        <RouterLink class="nav-item" to="/" exact-active-class="router-link-active">
          <svg viewBox="0 0 24 24" fill="none"><path d="m4 10 8-7 8 7v9a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2v-9Z"/><path d="M9 21v-7h6v7"/></svg>
          <span>首页</span>
        </RouterLink>
        <RouterLink class="nav-item" to="/following">
          <svg viewBox="0 0 24 24" fill="none"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M19 8v6m3-3h-6"/></svg>
          <span>关注</span>
        </RouterLink>
        <RouterLink class="nav-item" to="/hot">
          <svg viewBox="0 0 24 24" fill="none"><path d="M12 22c4.4 0 8-3.3 8-7.8 0-3.3-1.8-6.2-5.3-8.8.1 2.8-1.2 4.2-2.7 4.9.1-3.4-1.8-6.4-5-8.3.3 4.6-3 6.7-3 12.2C4 18.7 7.6 22 12 22Z"/><path d="M9.5 18.5c-1.1-2.4.2-4.1 2.5-6.1.1 2 1.5 2.9 2.5 4.2"/></svg>
          <span>热榜</span>
        </RouterLink>
        <RouterLink class="nav-item" to="/messages">
          <svg viewBox="0 0 24 24" fill="none"><path d="M20 15a2 2 0 0 1-2 2H8l-4 4v-16a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v10Z"/><path d="M8 8h8M8 12h5"/></svg>
          <span>消息</span>
          <b v-if="notifications.unread > 0" class="nav-badge">{{ notifications.unread > 99 ? '99+' : notifications.unread }}</b>
        </RouterLink>
        <RouterLink class="nav-item" to="/video">
          <svg viewBox="0 0 24 24" fill="none"><path d="M12 5v14m-7-7h14"/><rect x="3" y="3" width="18" height="18" rx="5"/></svg>
          <span>发布视频</span>
        </RouterLink>
        <RouterLink class="nav-item" to="/account">
          <svg viewBox="0 0 24 24" fill="none"><circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0 1 16 0"/></svg>
          <span>个人中心</span>
        </RouterLink>
      </nav>

      <div class="sidebar-foot">
        <div v-if="auth.isLoggedIn" class="user-info">
          <span class="user-dot ok" />
          <span class="user-name">{{ userLabel }}</span>
        </div>
        <RouterLink v-if="!auth.isLoggedIn" class="login-btn" to="/account">登录 / 注册</RouterLink>
        <RouterLink v-else class="login-btn" to="/settings">账号设置</RouterLink>
      </div>
    </aside>

    <main class="main">
      <header class="topbar">
        <div class="search-box">
          <svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/></svg>
          <input v-model="search" placeholder="搜索你感兴趣的视频" @keydown.enter="onSearch" />
        </div>
        <RouterLink v-if="auth.isLoggedIn" class="publish-btn" to="/video">
          <span>+</span> 发布
        </RouterLink>
      </header>

      <div class="content" :class="props.full ? 'full' : 'padded'">
        <template v-if="props.full">
          <slot />
        </template>
        <template v-else>
          <div class="container">
            <slot />
          </div>
        </template>
      </div>
    </main>

    <Toaster />
  </div>
</template>

<style scoped>
.shell {
  height: 100vh;
  display: grid;
  grid-template-columns: 208px 1fr;
  background: linear-gradient(145deg, #1b1b1e, #151517);
}

/* ---- sidebar ---- */
.sidebar {
  border-right: 1px solid var(--border);
  background: var(--surface-sidebar);
  padding: 18px 12px;
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.logo {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 6px;
}

.logo-icon {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  display: grid;
  place-items: center;
  font-weight: 800;
  font-size: 14px;
  background: #fff;
  color: #080808;
  box-shadow: -3px 0 0 #20d5ec, 3px 0 0 #fe2c55;
}

.logo-text {
  font-size: 18px;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.nav {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.nav-item {
  min-height: 46px;
  padding: 0 13px;
  border-radius: 7px;
  display: flex;
  align-items: center;
  gap: 13px;
  font-size: 15px;
  color: var(--text-secondary);
  transition: background 120ms ease, color 120ms ease;
}

.nav-item svg {
  width: 21px;
  height: 21px;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.nav-item:hover {
  background: var(--bg-hover);
  color: var(--text);
}

.nav-item.router-link-active {
  background: var(--surface-hover);
  color: #fff;
  font-weight: 600;
}

.nav-badge {
  min-width: 17px;
  height: 17px;
  margin-left: auto;
  padding: 0 5px;
  border-radius: 999px;
  display: grid;
  place-items: center;
  background: var(--accent);
  color: #fff;
  font-size: 9px;
  line-height: 1;
}

.sidebar-foot {
  margin-top: auto;
  padding-top: 16px;
  border-top: 1px solid var(--border);
}

.user-info {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}

.user-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: #555;
  flex-shrink: 0;
}

.user-dot.ok {
  background: var(--ok);
}

.user-name {
  font-size: 14px;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.login-btn {
  display: block;
  text-align: center;
  padding: 8px;
  border-radius: 8px;
  background: var(--bg-hover);
  color: var(--text-secondary);
  font-size: 14px;
}

.login-btn:hover {
  background: var(--accent-dim);
  color: var(--accent);
}

/* ---- main ---- */
.main {
  height: 100vh;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.topbar {
  height: 56px;
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 0 20px;
  background: rgba(30, 30, 33, .9);
  backdrop-filter: blur(16px);
}

.search-box {
  flex: 1;
  max-width: 520px;
  position: relative;
}

.search-box svg {
  position: absolute;
  z-index: 1;
  left: 14px;
  top: 50%;
  width: 17px;
  transform: translateY(-50%);
  stroke: #777;
  stroke-width: 1.8;
  stroke-linecap: round;
}

.search-box input {
  height: 38px;
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 0 16px 0 40px;
  font-size: 14px;
  color: var(--text);
}

.search-box input:focus {
  border-color: var(--accent);
}

.publish-btn {
  min-width: 88px;
  padding: 8px 17px;
  border-radius: 6px;
  background: var(--accent);
  color: #fff;
  font-size: 14px;
  font-weight: 600;
}

.publish-btn span {
  margin-right: 3px;
  font-size: 18px;
  line-height: 0;
}

.publish-btn:hover {
  background: var(--accent-hover);
  color: #fff;
}

/* ---- content ---- */
.content {
  flex: 1;
  min-height: 0;
}

.content.padded {
  overflow: auto;
}

.content.full {
  overflow: hidden;
}

@media (max-width: 768px) {
  .shell {
    grid-template-columns: 1fr;
  }
  .sidebar {
    position: fixed;
    z-index: 100;
    right: 0;
    bottom: 0;
    left: 0;
    height: 62px;
    padding: 6px 8px;
    border-top: 1px solid var(--border);
    border-right: 0;
    display: block;
    background: rgba(29, 29, 32, .96);
    backdrop-filter: blur(18px);
  }
  .logo, .sidebar-foot { display: none; }
  .nav { height: 100%; display: grid; grid-template-columns: repeat(6, 1fr); gap: 2px; }
  .nav-item { min-height: 48px; padding: 4px 2px; flex-direction: column; justify-content: center; gap: 2px; border-radius: 6px; font-size: 9px; }
  .nav-item svg { width: 19px; height: 19px; }
  .nav-badge { position: absolute; top: 2px; right: 15%; min-width: 14px; height: 14px; padding: 0 3px; font-size: 7px; }
  .main { padding-bottom: 62px; }
  .topbar { padding: 0 12px; }
  .publish-btn { display: none; }
}
</style>
