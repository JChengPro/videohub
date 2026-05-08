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
watch(
  () => route.query.q,
  (v) => {
    search.value = typeof v === 'string' ? v : ''
  },
)

watch(
  () => auth.isLoggedIn,
  (v) => {
    if (v) void social.refreshMine()
    else social.clear()
  },
  { immediate: true },
)

const userLabel = computed(() => {
  if (!auth.isLoggedIn) return '未登录'
  const username = auth.claims?.username ?? '(unknown)'
  const accountId = auth.claims?.account_id
  return accountId ? `${username} #${accountId}` : username
})

async function onSearch() {
  const q = search.value.trim()
  await router.push({ path: '/', query: q ? { q } : {} })
}

async function goLogin() {
  await router.push('/account')
}

async function goSettings() {
  await router.push('/settings')
}
</script>

<template>
  <div class="dy-shell">
    <aside class="dy-aside">
      <RouterLink class="dy-logo" to="/">
        <span class="dy-logo-mark">SV</span>
        <span>
          <span class="dy-logo-main">ShortVideo</span>
          <span class="dy-logo-sub">Feed Studio</span>
        </span>
      </RouterLink>

      <nav class="dy-nav">
        <div class="dy-nav-caption">Browse</div>
        <RouterLink class="dy-nav-link" to="/"><span>推荐</span><small>For You</small></RouterLink>
        <RouterLink class="dy-nav-link" to="/hot"><span>热榜</span><small>Hot Rank</small></RouterLink>
        <div class="dy-nav-caption">Create</div>
        <RouterLink class="dy-nav-link" to="/video"><span>发布</span><small>Upload</small></RouterLink>
        <RouterLink class="dy-nav-link" to="/account"><span>账号</span><small>Profile</small></RouterLink>
        <RouterLink class="dy-nav-link" to="/settings"><span>设置</span><small>Safety</small></RouterLink>
      </nav>

      <div class="dy-aside-foot">
        <div class="dy-user">
          <span class="dy-user-dot" :class="auth.isLoggedIn ? 'ok' : 'bad'" />
          <span class="dy-user-name">{{ userLabel }}</span>
        </div>
        <div class="dy-user-actions">
          <button v-if="!auth.isLoggedIn" class="dy-btn dy-btn-primary" type="button" @click="goLogin">登录</button>
          <button v-else class="dy-btn dy-btn-primary" type="button" @click="goSettings">设置</button>
        </div>
      </div>
    </aside>

    <div class="dy-main">
      <header class="dy-topbar">
        <div class="dy-top-left">
          <div class="dy-tabs-hint">{{ route.name }}</div>
          <div class="dy-top-title">发现短视频</div>
        </div>

        <div class="dy-search">
          <input v-model="search" class="dy-search-input" placeholder="搜索标题 / 作者（本地过滤）" @keydown.enter="onSearch" />
          <button class="dy-btn dy-btn-primary" type="button" @click="onSearch">搜索</button>
        </div>

        <div class="dy-top-right">
          <RouterLink class="dy-btn dy-btn-ghost" to="/video">+ 发布视频</RouterLink>
        </div>
      </header>

      <div class="dy-content" :class="props.full ? 'full' : 'padded'">
        <template v-if="props.full">
          <slot />
        </template>
        <template v-else>
          <div class="container">
            <slot />
          </div>
        </template>
      </div>
    </div>

    <Toaster />
  </div>
</template>

<style scoped>
.dy-shell {
  height: 100vh;
  display: grid;
  grid-template-columns: 268px 1fr;
  background:
    linear-gradient(90deg, rgba(255, 255, 255, 0.03), transparent 18%),
    transparent;
}

.dy-aside {
  position: relative;
  border-right: 1px solid rgba(255, 255, 255, 0.1);
  background:
    radial-gradient(260px 260px at 20% 10%, rgba(254, 44, 85, 0.18), transparent 62%),
    rgba(4, 5, 10, 0.72);
  backdrop-filter: blur(22px) saturate(140%);
  padding: 18px 14px;
  display: flex;
  flex-direction: column;
  gap: 18px;
  box-shadow: 20px 0 80px rgba(0, 0, 0, 0.28);
}

.dy-logo {
  display: grid;
  grid-template-columns: auto 1fr;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border-radius: 20px;
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.14), rgba(255, 255, 255, 0.045)),
    rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  text-decoration: none;
  box-shadow: 0 18px 50px rgba(0, 0, 0, 0.26);
}

.dy-logo:hover {
  text-decoration: none;
}

.dy-logo-mark {
  width: 42px;
  height: 42px;
  border-radius: 16px;
  display: grid;
  place-items: center;
  font-weight: 950;
  letter-spacing: -0.05em;
  background: linear-gradient(135deg, #fe2c55, #ff8a3d 58%, #25f4ee);
  color: white;
  box-shadow: 0 16px 36px rgba(254, 44, 85, 0.28);
}

.dy-logo-main,
.dy-logo-sub {
  display: block;
}

.dy-logo-main {
  font-weight: 950;
  letter-spacing: 0.02em;
  font-size: 17px;
}

.dy-logo-sub {
  margin-top: 1px;
  font-size: 11px;
  color: rgba(255, 255, 255, 0.55);
  letter-spacing: 0.18em;
  text-transform: uppercase;
}

.dy-nav {
  display: grid;
  gap: 9px;
}

.dy-nav-caption {
  margin: 10px 8px 2px;
  color: rgba(255, 255, 255, 0.38);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.18em;
  text-transform: uppercase;
}

.dy-nav-link {
  display: grid;
  gap: 2px;
  padding: 12px 13px;
  border-radius: 16px;
  border: 1px solid transparent;
  background: rgba(255, 255, 255, 0.035);
  text-decoration: none;
  transition: transform 140ms ease, background 140ms ease, border-color 140ms ease;
}

.dy-nav-link:hover {
  transform: translateX(3px);
  text-decoration: none;
  background: rgba(255, 255, 255, 0.08);
}

.dy-nav-link span {
  font-weight: 850;
}

.dy-nav-link small {
  color: rgba(255, 255, 255, 0.48);
  font-size: 11px;
  letter-spacing: 0.04em;
}

.dy-nav-link.router-link-active {
  border-color: rgba(254, 44, 85, 0.5);
  background:
    linear-gradient(135deg, rgba(254, 44, 85, 0.22), rgba(37, 244, 238, 0.08)),
    rgba(255, 255, 255, 0.06);
  box-shadow: inset 3px 0 0 rgba(254, 44, 85, 0.92), 0 16px 42px rgba(0, 0, 0, 0.2);
}

.dy-aside-foot {
  margin-top: auto;
  display: grid;
  gap: 10px;
  padding-top: 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
}

.dy-user {
  display: flex;
  gap: 10px;
  align-items: center;
}

.dy-user-dot {
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.25);
  box-shadow: 0 0 0 3px rgba(255, 255, 255, 0.06);
}

.dy-user-dot.ok {
  background: rgba(34, 197, 94, 1);
  box-shadow: 0 0 0 3px rgba(34, 197, 94, 0.14);
}

.dy-user-dot.bad {
  background: rgba(254, 44, 85, 1);
  box-shadow: 0 0 0 3px rgba(254, 44, 85, 0.14);
}

.dy-user-name {
  font-size: 13px;
  color: rgba(255, 255, 255, 0.86);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dy-user-actions {
  display: flex;
  gap: 10px;
}

.dy-btn {
  appearance: none;
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(255, 255, 255, 0.06);
  color: rgba(255, 255, 255, 0.9);
  border-radius: 15px;
  padding: 10px 12px;
  cursor: pointer;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  justify-content: center;
  font-size: 13px;
}

.dy-btn:hover {
  background: rgba(255, 255, 255, 0.1);
}

.dy-btn-primary {
  border-color: rgba(254, 44, 85, 0.5);
  background: linear-gradient(135deg, rgba(254, 44, 85, 0.88), rgba(255, 138, 61, 0.7));
  box-shadow: 0 16px 38px rgba(254, 44, 85, 0.18);
}

.dy-btn-primary:hover {
  background: linear-gradient(135deg, rgba(255, 70, 104, 0.96), rgba(255, 151, 80, 0.78));
}

.dy-btn-ghost {
  border-color: rgba(255, 255, 255, 0.14);
  background: rgba(0, 0, 0, 0.15);
}

.dy-main {
  height: 100vh;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.dy-topbar {
  height: 68px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(6, 7, 12, 0.54);
  backdrop-filter: blur(20px) saturate(140%);
  display: grid;
  grid-template-columns: 220px 1fr 190px;
  gap: 16px;
  align-items: center;
  padding: 0 18px;
  box-shadow: 0 16px 55px rgba(0, 0, 0, 0.22);
}

.dy-tabs-hint {
  font-size: 12px;
  color: rgba(37, 244, 238, 0.72);
  text-transform: uppercase;
  letter-spacing: 0.16em;
  font-weight: 900;
}

.dy-top-title {
  margin-top: 1px;
  font-size: 15px;
  font-weight: 900;
}

.dy-search {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 10px;
  align-items: center;
  max-width: 680px;
  width: 100%;
  justify-self: center;
}

.dy-search-input {
  width: 100%;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 999px;
  color: rgba(255, 255, 255, 0.9);
  padding: 12px 16px;
  outline: none;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08);
}

.dy-search-input:focus {
  border-color: rgba(37, 244, 238, 0.42);
  box-shadow: 0 0 0 3px rgba(37, 244, 238, 0.14);
}

.dy-top-right {
  display: flex;
  justify-content: flex-end;
}

.dy-content {
  flex: 1;
  min-height: 0;
}

.dy-content.padded {
  overflow: auto;
}

.dy-content.full {
  overflow: hidden;
}

@media (max-width: 900px) {
  .dy-shell {
    grid-template-columns: 1fr;
  }
  .dy-aside {
    display: none;
  }
  .dy-topbar {
    grid-template-columns: 1fr auto;
  }
  .dy-top-left {
    display: none;
  }
  .dy-top-right {
    display: none;
  }
}
</style>
