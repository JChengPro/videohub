<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import VideoFeed from '../components/VideoFeed.vue'
import AppIcon from '../components/AppIcon.vue'

const route = useRoute()
const router = useRouter()
const mode = computed(() => route.path === '/following' ? 'following' : route.path === '/hot' ? 'hot' : 'latest')
</script>

<template>
  <div class="feed-page">
    <button class="search-entry" type="button" aria-label="搜索用户和视频" @click="router.push('/search')"><AppIcon name="search" :size="20" /></button>
    <nav class="feed-tabs" aria-label="视频流分类">
      <button type="button" :aria-current="mode === 'following' ? 'page' : undefined" :class="{ active: mode === 'following' }" @click="router.push('/following')">关注</button>
      <button type="button" :aria-current="mode === 'latest' ? 'page' : undefined" :class="{ active: mode === 'latest' }" @click="router.push('/')">推荐</button>
      <button type="button" :aria-current="mode === 'hot' ? 'page' : undefined" :class="{ active: mode === 'hot' }" @click="router.push('/hot')">热门</button>
    </nav>
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

@media (min-width: 700px) {
  .search-entry { right: calc(50% - 203px); }
}
</style>
