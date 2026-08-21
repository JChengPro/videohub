<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import type { Account } from '../api/types'
import { useAuthStore } from '../stores/auth'
import VideoFeed from '../components/VideoFeed.vue'
import AppIcon from '../components/AppIcon.vue'
import Avatar from '../components/Avatar.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const mode = computed(() => route.path === '/following' ? 'following' : route.path === '/hot' ? 'hot' : 'latest')
const followingUsers = ref<Account[]>([])

async function loadFollowingUsers() {
  if (mode.value !== 'following' || !auth.isLoggedIn) {
    followingUsers.value = []
    return
  }
  try {
    followingUsers.value = (await api.following()).vloggers
  } catch {
    followingUsers.value = []
  }
}

watch([mode, () => auth.isLoggedIn], () => { void loadFollowingUsers() }, { immediate: true })
</script>

<template>
  <div class="feed-page">
    <button class="search-entry" type="button" aria-label="搜索用户和视频" @click="router.push('/search')"><AppIcon name="search" :size="20" /></button>
    <nav class="feed-tabs" aria-label="视频流分类">
      <button type="button" :aria-current="mode === 'following' ? 'page' : undefined" :class="{ active: mode === 'following' }" @click="router.push('/following')">关注</button>
      <button type="button" :aria-current="mode === 'latest' ? 'page' : undefined" :class="{ active: mode === 'latest' }" @click="router.push('/')">推荐</button>
      <button type="button" :aria-current="mode === 'hot' ? 'page' : undefined" :class="{ active: mode === 'hot' }" @click="router.push('/hot')">热门</button>
    </nav>
    <section v-if="mode === 'following' && auth.isLoggedIn" class="following-users" aria-label="我关注的用户">
      <button v-for="user in followingUsers" :key="user.id" type="button" @click="router.push(`/user/${user.id}`)">
        <Avatar :name="user.username" :id="user.id" :size="40" />
        <span>{{ user.username }}</span>
      </button>
      <p v-if="followingUsers.length === 0">还没有关注用户</p>
    </section>
    <VideoFeed :mode="mode" />
  </div>
</template>

<style scoped>
.search-entry { position: fixed; z-index: 51; top: calc(9px + env(safe-area-inset-top)); right: 12px; width: 40px; height: 40px; display: grid; place-items: center; border: 0; border-radius: 50%; background: rgba(10,10,12,.42); color: rgba(255,255,255,.9); backdrop-filter: blur(14px); }
.feed-tabs {
  position: fixed;
  z-index: 50;
  top: calc(8px + env(safe-area-inset-top));
  left: 50%;
  padding: 3px 5px;
  display: flex;
  gap: 3px;
  transform: translateX(-50%);
  border: 0;
  border-radius: 0;
  background: rgba(10, 10, 12, .28);
  backdrop-filter: blur(14px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, .22);
}

.feed-tabs button {
  position: relative;
  min-width: 52px;
  min-height: 34px;
  padding: 0 10px;
  border-radius: 0;
  color: rgba(255, 255, 255, .58);
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
  transition: background var(--mobile-duration) ease, color var(--mobile-duration) ease;
}

.feed-tabs button.active {
  background: transparent;
  color: #fff;
}

.feed-tabs button.active::after {
  position: absolute;
  right: 22px;
  bottom: 3px;
  left: 22px;
  height: 2px;
  border-radius: 2px;
  background: var(--mobile-accent);
  content: '';
}

.following-users { position: fixed; z-index: 49; top: calc(53px + env(safe-area-inset-top)); right: 0; left: 0; min-height: 66px; padding: 8px 12px; display: flex; align-items: flex-start; gap: 14px; overflow-x: auto; background: linear-gradient(to bottom, rgba(10,10,12,.88), rgba(10,10,12,.18)); scrollbar-width: none; }
.following-users::-webkit-scrollbar { display: none; }
.following-users button { flex: 0 0 52px; min-width: 52px; padding: 0; display: grid; justify-items: center; gap: 4px; color: rgba(255,255,255,.84); }
.following-users button span { width: 52px; overflow: hidden; font-size: 9px; text-align: center; text-overflow: ellipsis; white-space: nowrap; }
.following-users p { width: 100%; padding: 15px 0; color: rgba(255,255,255,.55); font-size: 10px; text-align: center; }

@media (min-width: 700px) {
  .search-entry { right: calc(50% - 203px); }
}
</style>
