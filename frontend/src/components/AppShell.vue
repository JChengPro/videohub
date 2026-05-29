<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useSocialStore } from '../stores/social'
import Toaster from './Toaster.vue'

const props = defineProps<{ full?: boolean }>()

const auth = useAuthStore()
const social = useSocialStore()
const router = useRouter()
const route = useRoute()

const search = ref(typeof route.query.q === 'string' ? route.query.q : '')
watch(() => route.query.q, (v) => { search.value = typeof v === 'string' ? v : '' })

watch(() => auth.isLoggedIn, (v) => {
  if (v) void social.refreshMine()
  else social.clear()
}, { immediate: true })

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
        <RouterLink class="nav-item" to="/">首页</RouterLink>
        <RouterLink class="nav-item" to="/hot">热榜</RouterLink>
        <RouterLink class="nav-item" to="/video">发布视频</RouterLink>
        <RouterLink class="nav-item" to="/account">个人中心</RouterLink>
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
          <input v-model="search" placeholder="搜索视频..." @keydown.enter="onSearch" />
        </div>
        <RouterLink v-if="auth.isLoggedIn" class="publish-btn" to="/video">+ 发布</RouterLink>
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
  grid-template-columns: 220px 1fr;
  background: var(--bg);
}

/* ---- sidebar ---- */
.sidebar {
  border-right: 1px solid var(--border);
  background: #050505;
  padding: 20px 14px;
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
  border-radius: 8px;
  display: grid;
  place-items: center;
  font-weight: 800;
  font-size: 14px;
  background: linear-gradient(135deg, #fe2c55, #20d5ec);
  color: #fff;
}

.logo-text {
  font-size: 18px;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.nav-item {
  padding: 10px 12px;
  border-radius: 8px;
  font-size: 15px;
  color: var(--text-secondary);
  transition: background 120ms ease, color 120ms ease;
}

.nav-item:hover {
  background: var(--bg-hover);
  color: var(--text);
}

.nav-item.router-link-active {
  background: var(--accent-dim);
  color: var(--accent);
  font-weight: 600;
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
  background: var(--bg);
}

.search-box {
  flex: 1;
  max-width: 520px;
}

.search-box input {
  height: 36px;
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 0 16px;
  font-size: 14px;
  color: var(--text);
}

.search-box input:focus {
  border-color: var(--accent);
}

.publish-btn {
  padding: 7px 18px;
  border-radius: 8px;
  background: var(--accent);
  color: #fff;
  font-size: 14px;
  font-weight: 600;
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
  .sidebar { display: none; }
}
</style>
