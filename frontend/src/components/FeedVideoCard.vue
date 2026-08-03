<script setup lang="ts">
import type { FeedVideoItem } from '../api/types'
import AppIcon from './AppIcon.vue'

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
        <AppIcon name="play" :size="17" />
      </div>
    </RouterLink>
    <div class="info">
      <RouterLink class="title" :to="`/video/${item.id}`">{{ item.title }}</RouterLink>
      <RouterLink class="author" :to="`/u/${item.author.id}`">{{ item.author.username }}</RouterLink>
      <div class="meta">
        <span>{{ item.likes_count }} 赞</span>
        <span>{{ item.comments_count }} 评论</span>
        <span>{{ new Date(item.create_time).toLocaleDateString() }}</span>
      </div>
      <div class="actions">
        <button
          v-if="canLike"
          class="like-btn"
          :class="{ liked: item.is_liked }"
          type="button"
          :disabled="busy"
          :aria-label="item.is_liked ? `取消点赞，当前 ${item.likes_count} 赞` : `点赞，当前 ${item.likes_count} 赞`"
          @click="onToggle"
        >
          <AppIcon name="heart" :size="16" :filled="item.is_liked" />
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
  grid-template-columns: 164px 1fr;
  gap: 15px;
  background: var(--surface-panel);
  border: 1px solid var(--border);
  border-radius: 10px;
  overflow: hidden;
  transition: border-color 150ms ease;
}

.video-card {
  grid-template-columns: 148px minmax(0, 1fr);
  gap: 14px;
  border-color: transparent;
  border-radius: 8px;
  background: #1b1b1e;
}
.video-card:hover { border-color: var(--border-hover); background: #202024; }
.cover { border-radius: 8px; }
.play-icon {
  width: 38px;
  height: 38px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: rgba(0,0,0,.62);
  color: #fff;
}
.play-icon :deep(svg) { margin-left: 2px; }
.info { padding: 12px 12px 12px 0; }
.actions { gap: 6px; }
.like-btn,
.action-link { border-radius: 6px; }

.video-card:hover {
  border-color: var(--border-hover);
}

.cover {
  position: relative;
  aspect-ratio: 4/5;
  overflow: hidden;
  background: var(--surface-raised);
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
  padding: 16px 16px 16px 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.title {
  font-size: 15px;
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
  border-radius: 6px;
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
  border-radius: 6px;
  background: rgba(255,255,255,0.06);
  color: var(--text-secondary);
  font-size: 13px;
}

.action-link:hover {
  background: rgba(255,255,255,0.1);
  color: var(--text);
}

/* Compact ranked-list card. */
.video-card { grid-template-columns: 148px minmax(0, 1fr); gap: 14px; }
.cover { border-radius: 8px; }
.cover .play-icon {
  inset: auto;
  top: 50%;
  left: 50%;
  width: 38px;
  height: 38px;
  transform: translate(-50%, -50%);
}
.info { padding: 12px 12px 12px 0; }
.actions { gap: 6px; }

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
