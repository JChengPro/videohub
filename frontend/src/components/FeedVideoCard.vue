<script setup lang="ts">
import type { FeedVideoItem } from '../api/types'

const props = defineProps<{
  item: FeedVideoItem
  canLike: boolean
  busy?: boolean
}>()

const emit = defineEmits<{
  (e: 'toggle-like', item: FeedVideoItem): void
}>()

function onToggle() {
  emit('toggle-like', props.item)
}
</script>

<template>
  <div class="video-card">
    <RouterLink class="cover" :to="`/video/${item.id}`">
      <img :src="item.cover_url" :alt="item.title" loading="lazy" />
      <div class="play-icon">
        <svg width="36" height="36" viewBox="0 0 36 36" fill="none"><circle cx="18" cy="18" r="18" fill="rgba(0,0,0,0.5)"/><polygon points="15,11 26,18 15,25" fill="#fff"/></svg>
      </div>
    </RouterLink>
    <div class="info">
      <RouterLink class="title" :to="`/video/${item.id}`">{{ item.title }}</RouterLink>
      <RouterLink class="author" :to="`/user/${item.author.id}`">@{{ item.author.username }}</RouterLink>
      <div class="meta">
        <span>{{ item.likes_count }} 赞</span>
        <span>{{ new Date(item.create_time).toLocaleDateString() }}</span>
      </div>
      <div class="actions">
        <button
          v-if="canLike"
          class="like-btn"
          :class="{ liked: item.is_liked }"
          :disabled="busy"
          @click="onToggle"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" :fill="item.is_liked ? '#fe2c55' : 'none'" :stroke="item.is_liked ? '#fe2c55' : '#aaa'" stroke-width="2"><path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z"/></svg>
          {{ item.is_liked ? '已赞' : '点赞' }}
        </button>
        <RouterLink class="action-link" :to="`/video/${item.id}`">详情</RouterLink>
      </div>
    </div>
  </div>
</template>

<style scoped>
.video-card {
  display: grid;
  grid-template-columns: 200px 1fr;
  gap: 16px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  overflow: hidden;
  transition: border-color 150ms ease;
}

.video-card:hover {
  border-color: var(--border-hover);
}

.cover {
  position: relative;
  aspect-ratio: 9/13;
  overflow: hidden;
  background: #1a1a1a;
}

.cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.play-icon {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  opacity: 0;
  transition: opacity 150ms ease;
}

.cover:hover .play-icon {
  opacity: 1;
}

.info {
  padding: 14px 14px 14px 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.title {
  font-size: 16px;
  font-weight: 600;
  line-height: 1.3;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.author {
  font-size: 13px;
  color: var(--text-secondary);
}

.author:hover {
  color: var(--accent);
}

.meta {
  display: flex;
  gap: 14px;
  font-size: 12px;
  color: var(--text-muted);
}

.actions {
  margin-top: auto;
  display: flex;
  gap: 8px;
}

.like-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  background: rgba(255,255,255,0.06);
  color: var(--text-secondary);
  padding: 6px 12px;
  border-radius: 999px;
  font-size: 13px;
  transition: background 120ms;
}

.like-btn:hover {
  background: var(--accent-dim);
  color: var(--accent);
}

.like-btn.liked {
  color: var(--accent);
}

.action-link {
  display: inline-flex;
  align-items: center;
  padding: 6px 12px;
  border-radius: 999px;
  background: rgba(255,255,255,0.06);
  color: var(--text-secondary);
  font-size: 13px;
}

.action-link:hover {
  background: rgba(255,255,255,0.1);
  color: var(--text);
}

@media (max-width: 600px) {
  .video-card {
    grid-template-columns: 1fr;
  }
  .cover {
    aspect-ratio: 16/9;
  }
  .info {
    padding: 0 14px 14px;
  }
}
</style>
