<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
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

function syncAuthFromStorage(event: StorageEvent) {
  if (event.key === 'jwt_token') auth.syncFromStorage()
}

onMounted(() => window.addEventListener('storage', syncAuthFromStorage))
onBeforeUnmount(() => window.removeEventListener('storage', syncAuthFromStorage))
</script>

<template>
  <div class="shell">
    <aside class="sidebar" aria-label="主导航">
      <RouterLink class="logo" to="/" aria-label="VideoHub 首页">
        <span class="logo-icon">VH</span>
        <span class="logo-text">VideoHub</span>
      </RouterLink>

      <nav class="nav" aria-label="内容导航">
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
        <form class="search-box" role="search" @submit.prevent="onSearch">
          <svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="m20 20-4-4"/></svg>
          <input v-model="search" aria-label="搜索视频或作者" autocomplete="off" placeholder="搜索视频或作者" />
          <button class="search-submit" type="submit" aria-label="提交搜索">搜索</button>
        </form>
        <RouterLink v-if="auth.isLoggedIn" class="publish-btn" to="/video">
          <span aria-hidden="true">+</span> 发布
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
  height: 100dvh;
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  background: var(--surface-base);
}

/* ---- sidebar ---- */
.sidebar {
  border-right: 1px solid var(--border);
  background: var(--surface-sidebar);
  padding: 20px 14px 16px;
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
  border-radius: var(--radius);
  display: grid;
  place-items: center;
  font-weight: 800;
  font-size: 14px;
  background: #f7f7f8;
  color: #080808;
  box-shadow: -3px 0 0 var(--accent-cyan), 3px 0 0 var(--accent);
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
  position: relative;
  min-height: 46px;
  padding: 0 13px;
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  gap: 13px;
  font-size: 15px;
  color: var(--text-secondary);
  transition: background var(--duration-fast) ease, color var(--duration-fast) ease;
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
  background: linear-gradient(90deg, var(--accent-dim), rgba(255, 255, 255, .035));
  color: #fff;
  font-weight: 600;
}

.nav-item.router-link-active::before {
  position: absolute;
  top: 12px;
  bottom: 12px;
  left: 0;
  width: 3px;
  border-radius: 3px;
  background: var(--accent);
  content: '';
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
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
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
  height: 100dvh;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.topbar {
  height: 64px;
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 0 24px;
  background: rgba(13, 13, 15, .86);
  backdrop-filter: blur(18px) saturate(120%);
}

.search-box {
  flex: 1;
  max-width: 520px;
  position: relative;
  display: flex;
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
  height: 40px;
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 0 16px 0 40px;
  font-size: 14px;
  color: var(--text);
}

.search-submit {
  position: absolute;
  top: 4px;
  right: 5px;
  width: 54px;
  height: 32px;
  overflow: hidden;
  border-radius: 999px;
  background: transparent;
  color: var(--text-muted);
  font-size: 11px;
}

.search-submit:hover {
  background: var(--surface-hover);
  color: var(--text);
}

.search-box input:focus {
  border-color: var(--accent);
}

.publish-btn {
  min-width: 88px;
  padding: 8px 17px;
  border-radius: var(--radius-sm);
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

@media (max-width: 1180px) and (min-width: 769px) {
  .shell {
    grid-template-columns: 78px minmax(0, 1fr);
  }

  .sidebar {
    padding-inline: 10px;
  }

  .logo {
    justify-content: center;
  }

  .logo-text,
  .nav-item span,
  .sidebar-foot {
    display: none;
  }

  .nav-item {
    min-height: 50px;
    justify-content: center;
    padding: 0;
  }

  .nav-item.router-link-active::before {
    top: 14px;
    bottom: 14px;
  }

  .nav-badge {
    position: absolute;
    top: 5px;
    right: 7px;
  }
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
    height: calc(62px + env(safe-area-inset-bottom));
    padding: 6px 8px env(safe-area-inset-bottom);
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
  .nav-item.router-link-active::before { display: none; }
  .main { padding-bottom: calc(62px + env(safe-area-inset-bottom)); }
  .topbar { height: 56px; padding: 0 12px; }
  .publish-btn { display: none; }
  .search-submit { display: none; }
  .search-box input { padding-right: 14px; }
}
</style>
