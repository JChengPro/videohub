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
  <div class="feed-card">
    <div class="cover">
      <img :src="item.cover_url" :alt="item.title" loading="lazy" />
    </div>
    <div class="content">
      <div class="row" style="justify-content: space-between">
        <div>
          <div class="title">
            <RouterLink :to="`/video/${item.id}`">{{ item.title }}</RouterLink>
          </div>
          <div class="subtle">
            作者：{{ item.author.username }} (#{{ item.author.id }}) · 创建时间：{{ new Date(item.create_time * 1000).toLocaleString() }}
          </div>
        </div>
        <div class="row">
          <span class="pill">❤️ {{ item.likes_count }}</span>
          <button
            v-if="canLike"
            class="primary"
            type="button"
            :disabled="busy"
            @click="onToggle"
            :title="item.is_liked ? '取消点赞' : '点赞'"
          >
            {{ item.is_liked ? '已赞' : '点赞' }}
          </button>
        </div>
      </div>
      <div v-if="item.description" class="muted" style="margin-top: 8px">{{ item.description }}</div>
      <div class="row" style="margin-top: 10px">
        <a class="pill" :href="item.play_url" target="_blank" rel="noreferrer">播放地址</a>
        <RouterLink class="pill" :to="`/video/${item.id}`">查看详情 / 评论</RouterLink>
      </div>
    </div>
  </div>
</template>

<style scoped>
.feed-card {
  display: grid;
  grid-template-columns: 230px minmax(0, 1fr);
  gap: 0;
  border: 1px solid rgba(255, 255, 255, 0.13);
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.11), rgba(255, 255, 255, 0.045)),
    rgba(255, 255, 255, 0.06);
  border-radius: 22px;
  overflow: hidden;
  box-shadow: 0 20px 58px rgba(0, 0, 0, 0.26);
  transition: transform 150ms ease, border-color 150ms ease, box-shadow 150ms ease;
}

.feed-card:hover {
  transform: translateY(-2px);
  border-color: rgba(37, 244, 238, 0.24);
  box-shadow: 0 28px 80px rgba(0, 0, 0, 0.34);
}

.cover {
  background: rgba(0, 0, 0, 0.25);
  aspect-ratio: 10/13;
  position: relative;
  overflow: hidden;
}

.cover::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(to top, rgba(0, 0, 0, 0.32), transparent 56%);
}

.cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.content {
  padding: 16px 16px 18px;
  min-width: 0;
}

@media (max-width: 900px) {
  .feed-card {
    grid-template-columns: 1fr;
  }
}
</style>
