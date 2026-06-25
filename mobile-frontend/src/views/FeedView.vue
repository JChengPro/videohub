<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import VideoFeed from '../components/VideoFeed.vue'

const route = useRoute()
const router = useRouter()
const mode = computed(() => route.path === '/following' ? 'following' : route.path === '/hot' ? 'hot' : 'latest')
</script>

<template>
  <div class="feed-page">
    <header class="feed-tabs">
      <button :class="{ active: mode === 'following' }" @click="router.push('/following')">关注</button>
      <button :class="{ active: mode === 'latest' }" @click="router.push('/')">推荐</button>
      <button :class="{ active: mode === 'hot' }" @click="router.push('/hot')">热门</button>
    </header>
    <VideoFeed :mode="mode" />
  </div>
</template>

<style scoped>
.feed-tabs { position: fixed; z-index: 50; top: calc(10px + env(safe-area-inset-top)); left: 50%; display: flex; gap: 22px; transform: translateX(-50%); filter: drop-shadow(0 2px 5px #000); }.feed-tabs button { position: relative; padding: 5px 0; color: rgba(255,255,255,.58); font-size: 14px; font-weight: 700; white-space: nowrap; }.feed-tabs button.active { color: #fff; }.feed-tabs button.active::after { content: ''; position: absolute; right: 25%; bottom: 0; left: 25%; height: 2px; border-radius: 2px; background: #fff; }
</style>
