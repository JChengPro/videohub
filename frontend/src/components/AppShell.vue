<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useSocialStore } from '../stores/social'
import { useNotificationStore } from '../stores/notification'
import { useChatStore } from '../stores/chat'
import Toaster from './Toaster.vue'
import AppIcon from './AppIcon.vue'
import UserAvatar from './UserAvatar.vue'

const props = defineProps<{ full?: boolean }>()

const auth = useAuthStore()
const social = useSocialStore()
const notifications = useNotificationStore()
const chat = useChatStore()
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
  if (!q) return
  await router.push({ path: '/search', query: { q } })
}

function syncAuthFromStorage(event: StorageEvent) {
  if (event.key === 'jwt_token') auth.syncFromStorage()
}

onMounted(() => window.addEventListener('storage', syncAuthFromStorage))
onBeforeUnmount(() => window.removeEventListener('storage', syncAuthFromStorage))
</script>

<template>
  <div class="shell">
    <a class="skip-link" href="#main-content">跳到主要内容</a>
    <aside class="sidebar" aria-label="主导航">
      <RouterLink class="logo" to="/" aria-label="VideoHub 首页">
        <span class="logo-icon"><AppIcon name="brand" :size="29" /></span>
        <span class="logo-text">VideoHub</span>
      </RouterLink>

      <nav class="nav" aria-label="内容导航">
        <RouterLink class="nav-item" to="/" exact-active-class="router-link-active">
          <AppIcon name="home" :size="23" :filled="route.path === '/'" />
          <span>推荐</span>
        </RouterLink>
        <RouterLink class="nav-item" to="/hot">
          <AppIcon name="hot" :size="23" />
          <span>热门</span>
        </RouterLink>
        <RouterLink class="nav-item" to="/following">
          <AppIcon name="following" :size="23" />
          <span>关注</span>
        </RouterLink>
        <RouterLink class="nav-item" to="/messages">
          <AppIcon name="message" :size="23" />
          <span>消息</span>
          <b v-if="notifications.unread + chat.unread > 0" class="nav-badge">{{ notifications.unread + chat.unread > 99 ? '99+' : notifications.unread + chat.unread }}</b>
        </RouterLink>
        <RouterLink class="nav-item" to="/video">
          <AppIcon name="upload" :size="23" />
          <span>发布</span>
        </RouterLink>
        <RouterLink class="nav-item" to="/account">
          <AppIcon name="user" :size="23" />
          <span>个人主页</span>
        </RouterLink>
      </nav>

      <div class="sidebar-foot">
        <RouterLink v-if="auth.isLoggedIn" class="account-chip" to="/settings">
          <UserAvatar :username="userLabel" :id="auth.claims?.account_id ?? 0" :size="36" />
          <span><b>@{{ userLabel }}</b><small>账号与设置</small></span>
          <AppIcon name="more" :size="18" />
        </RouterLink>
        <RouterLink v-else class="login-btn" to="/account">登录 VideoHub</RouterLink>
      </div>
    </aside>

    <main id="main-content" class="main">
      <header class="topbar">
        <form class="search-box" role="search" @submit.prevent="onSearch">
          <AppIcon name="search" :size="19" />
          <input v-model="search" aria-label="搜索用户或视频" autocomplete="off" placeholder="搜索用户或视频" />
          <button class="search-submit" type="submit" aria-label="提交搜索">搜索</button>
        </form>
        <RouterLink v-if="auth.isLoggedIn" class="publish-btn" to="/video">
          <AppIcon name="upload" :size="18" /> 上传
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

<style scoped>
.skip-link { position: fixed; z-index: 10000; top: 8px; left: 12px; padding: 9px 14px; border-radius: 8px; background: #fff; color: #111; font-weight: 800; transform: translateY(-150%); transition: transform 160ms ease; }
.skip-link:focus { transform: translateY(0); }
.shell { grid-template-columns: 240px minmax(0, 1fr); background: #0b0b0d; }
.sidebar { padding: 22px 14px 16px; gap: 28px; border-right-color: rgba(255,255,255,.08); background: #0b0b0d; }
.logo { min-height: 50px; padding: 6px 10px; gap: 11px; }
.logo-icon { width: 34px; height: 34px; border-radius: 0; background: transparent; box-shadow: none; }
.logo-text { font-size: 21px; font-weight: 900; letter-spacing: -.045em; }
.nav { gap: 3px; }
.nav-item { min-height: 50px; padding: 0 13px; gap: 14px; border-radius: 9px; color: #d7d7dc; font-size: 15px; font-weight: 650; }
.nav-item:hover { background: #1c1c20; color: #fff; }
.nav-item.router-link-active { background: #202024; color: #fff; font-weight: 850; }
.nav-item.router-link-active::before { display: none; }
.nav-item :deep(svg) { transition: color 160ms ease; }
.nav-item.router-link-active :deep(svg) { color: #fe2c55; }
.nav-badge { min-width: 20px; height: 20px; border: 2px solid #202024; font-size: 9px; font-weight: 850; }
.sidebar-foot { padding-top: 14px; }
.account-chip { min-width: 0; padding: 9px 8px; display: grid; grid-template-columns: 36px minmax(0,1fr) 18px; align-items: center; gap: 9px; border-radius: 10px; transition: background 160ms ease; }
.account-chip:hover { background: #1c1c20; color: #fff; }
.account-chip > span { min-width: 0; }
.account-chip b,.account-chip small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.account-chip b { font-size: 12px; }.account-chip small { margin-top: 2px; color: #77777f; font-size: 9px; }
.login-btn { min-height: 44px; padding: 11px 14px; border: 0; border-radius: 7px; background: #fe2c55; color: #fff; font-weight: 800; }
.login-btn:hover { background: #ea1d48; color: #fff; }
.topbar { height: 62px; justify-content: center; padding: 0 22px; border-bottom-color: rgba(255,255,255,.075); background: rgba(11,11,13,.92); }
.search-box { max-width: 500px; }
.search-box > :deep(svg) { position: absolute; z-index: 1; top: 50%; left: 15px; transform: translateY(-50%); color: #77777f; }
.search-box input { height: 42px; padding: 0 64px 0 43px; border: 1px solid transparent; border-radius: 6px; background: #1d1d21; }
.search-box input:focus { border-color: rgba(255,255,255,.25); box-shadow: none; background: #232328; }
.search-submit { top: 5px; right: 5px; height: 32px; color: #a7a7ae; font-weight: 700; }
.publish-btn { min-height: 40px; padding: 0 16px; display: inline-flex; align-items: center; gap: 7px; border: 1px solid rgba(255,255,255,.18); border-radius: 6px; background: #fff; color: #111; font-size: 13px; font-weight: 850; }
.publish-btn:hover { background: #ececef; color: #111; }
@media (max-width: 1180px) and (min-width: 769px) {
  .shell { grid-template-columns: 76px minmax(0,1fr); }
  .nav-item :deep(svg) { width: 24px; height: 24px; }
}
@media (max-width: 768px) {
  .sidebar { background: rgba(11,11,13,.96); }
  .nav-item { font-size: 9px; }
  .topbar { background: rgba(11,11,13,.94); }
}
</style>
