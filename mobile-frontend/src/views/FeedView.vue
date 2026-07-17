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
    <nav class="feed-tabs" aria-label="视频流分类">
      <button type="button" :aria-current="mode === 'following' ? 'page' : undefined" :class="{ active: mode === 'following' }" @click="router.push('/following')">关注</button>
      <button type="button" :aria-current="mode === 'latest' ? 'page' : undefined" :class="{ active: mode === 'latest' }" @click="router.push('/')">推荐</button>
      <button type="button" :aria-current="mode === 'hot' ? 'page' : undefined" :class="{ active: mode === 'hot' }" @click="router.push('/hot')">热门</button>
    </nav>
    <VideoFeed :mode="mode" />
  </div>
</template>

<style scoped>
.feed-tabs {
  position: fixed;
  z-index: 50;
  top: calc(8px + env(safe-area-inset-top));
  left: 50%;
  padding: 3px 5px;
  display: flex;
  gap: 3px;
  transform: translateX(-50%);
  border: 1px solid rgba(255, 255, 255, .1);
  border-radius: 999px;
  background: rgba(10, 10, 12, .46);
  backdrop-filter: blur(14px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, .22);
}

.feed-tabs button {
  position: relative;
  min-width: 52px;
  min-height: 34px;
  padding: 0 10px;
  border-radius: 999px;
  color: rgba(255, 255, 255, .58);
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
  transition: background var(--mobile-duration) ease, color var(--mobile-duration) ease;
}

.feed-tabs button.active {
  background: rgba(255, 255, 255, .12);
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
</style>
