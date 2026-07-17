<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { api } from '../api'
import type { FeedVideo, Video } from '../api/types'
import AppIcon from '../components/AppIcon.vue'
import CommentsSheet from '../components/CommentsSheet.vue'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const toast = useToastStore()

const videoElement = ref<HTMLVideoElement | null>(null)
const video = ref<Video | null>(null)
const loading = ref(false)
const error = ref('')
const liked = ref(false)
const commentsOpen = ref(false)
const likeBusy = ref(false)
const id = computed(() => Number(route.params.id))
let requestId = 0
let resumeAfterComments = false
let resumeAfterVisibility = false

const feedVideo = computed<FeedVideo | null>(() => video.value ? {
  id: video.value.id,
  author: { id: video.value.author_id, username: video.value.username },
  title: video.value.title,
  description: video.value.description,
  play_url: video.value.play_url,
  cover_url: video.value.cover_url,
  create_time: Date.parse(video.value.create_time),
  likes_count: Math.max(0, video.value.likes_count),
  is_liked: liked.value,
} : null)

async function load() {
  const request = ++requestId
  videoElement.value?.pause()
  video.value = null
  liked.value = false
  commentsOpen.value = false
  error.value = ''

  if (!Number.isInteger(id.value) || id.value <= 0) {
    error.value = '无效的视频 ID'
    return
  }

  loading.value = true
  try {
    const detail = await api.videoDetail(id.value)
    if (request !== requestId) return
    video.value = detail
    if (auth.isLoggedIn) {
      try {
        const state = await api.isLiked(id.value)
        if (request === requestId) liked.value = state.is_liked
      } catch {
        if (request === requestId) liked.value = false
      }
    }
    await nextTick()
    if (request === requestId && !document.hidden) {
      try { await videoElement.value?.play() } catch { /* Native controls remain available. */ }
    }
  } catch (cause) {
    if (request === requestId) error.value = cause instanceof Error ? cause.message : String(cause)
  } finally {
    if (request === requestId) loading.value = false
  }
}

async function toggleLike() {
  if (!auth.isLoggedIn) {
    await router.push('/me')
    return
  }
  if (!video.value || likeBusy.value) return
  likeBusy.value = true
  try {
    const state = liked.value ? await api.unlike(id.value) : await api.like(id.value)
    liked.value = state.is_liked
    video.value.likes_count = Math.max(0, state.likes_count)
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : String(cause))
  } finally {
    likeBusy.value = false
  }
}

async function share() {
  if (!video.value) return
  const url = location.href
  try {
    if (navigator.share) await navigator.share({ title: video.value.title, url })
    else {
      await navigator.clipboard.writeText(url)
      toast.success('链接已复制')
    }
  } catch (cause) {
    if (cause instanceof DOMException && cause.name === 'AbortError') return
    toast.error('分享失败，请稍后重试')
  }
}

function openComments() {
  resumeAfterComments = !!videoElement.value && !videoElement.value.paused
  videoElement.value?.pause()
  commentsOpen.value = true
}

function closeComments() {
  commentsOpen.value = false
  if (resumeAfterComments && !document.hidden) void videoElement.value?.play().catch(() => {})
  resumeAfterComments = false
}

function onVisibilityChange() {
  if (document.hidden) {
    resumeAfterVisibility = !!videoElement.value && !videoElement.value.paused && !commentsOpen.value
    videoElement.value?.pause()
    return
  }
  if (resumeAfterVisibility && !commentsOpen.value) void videoElement.value?.play().catch(() => {})
  resumeAfterVisibility = false
}

watch(id, load, { immediate: true })
watch(() => auth.isLoggedIn, () => {
  if (!auth.isLoggedIn) liked.value = false
  else if (video.value) {
    const currentRequest = requestId
    const currentVideoId = id.value
    void api.isLiked(currentVideoId).then((state) => {
      if (currentRequest === requestId && currentVideoId === id.value) liked.value = state.is_liked
    }).catch(() => {})
  }
})

onMounted(() => document.addEventListener('visibilitychange', onVisibilityChange))
onUnmounted(() => {
  requestId += 1
  document.removeEventListener('visibilitychange', onVisibilityChange)
  videoElement.value?.pause()
})
</script>

<template>
  <main class="detail">
    <header class="detail-bar">
      <button class="back" type="button" aria-label="返回上一页" @click="router.back()"><AppIcon name="back" /></button>
      <strong>视频详情</strong>
      <button class="share-top" type="button" aria-label="分享视频" :disabled="!video" @click="share"><AppIcon name="share" :size="21" /></button>
    </header>

    <section v-if="loading" class="detail-state" role="status">
      <span class="detail-loader" />
      <p>正在加载视频</p>
    </section>
    <section v-else-if="error" class="detail-state" role="alert">
      <AppIcon name="warning" :size="36" />
      <strong>视频暂时无法打开</strong>
      <p>{{ error }}</p>
      <button type="button" @click="load">重新加载</button>
    </section>

    <template v-else-if="video">
      <div class="player">
        <video
          ref="videoElement"
          :src="video.play_url"
          :poster="video.cover_url"
          controls
          autoplay
          muted
          playsinline
          preload="metadata"
        />
        <span class="sound-tip">默认静音，可在播放器中开启声音</span>
      </div>

      <section class="info">
        <button class="author" type="button" @click="router.push(`/user/${video.author_id}`)">@{{ video.username }}</button>
        <h1>{{ video.title }}</h1>
        <p v-if="video.description">{{ video.description }}</p>
        <div class="info-actions">
          <button :class="{ liked }" type="button" :disabled="likeBusy" :aria-label="liked ? '取消点赞' : '点赞'" @click="toggleLike">
            <AppIcon name="heart" :filled="liked" />
            {{ Math.max(0, video.likes_count) }}
          </button>
          <button type="button" aria-label="查看评论" @click="openComments"><AppIcon name="comment" />评论</button>
          <button type="button" aria-label="分享视频" @click="share"><AppIcon name="share" />分享</button>
        </div>
      </section>
    </template>

    <CommentsSheet v-if="commentsOpen && feedVideo" :video="feedVideo" @close="closeComments" />
  </main>
</template>

<style scoped>
.detail {
  min-height: 100dvh;
  padding-bottom: calc(26px + env(safe-area-inset-bottom));
  background: var(--mobile-bg);
}

.detail-bar {
  position: sticky;
  z-index: 10;
  top: 0;
  min-height: calc(54px + env(safe-area-inset-top));
  padding: env(safe-area-inset-top) 8px 0;
  display: grid;
  grid-template-columns: 48px 1fr 48px;
  align-items: center;
  border-bottom: 1px solid var(--mobile-border);
  background: rgba(11, 11, 13, .9);
  backdrop-filter: blur(18px);
}

.detail-bar strong {
  text-align: center;
  font-size: 14px;
}

.back,
.share-top {
  width: 44px;
  height: 44px;
  display: grid;
  place-items: center;
  color: var(--mobile-text-secondary);
}

.player {
  position: relative;
  background: #050506;
}

.player video {
  width: 100%;
  height: min(68dvh, 720px);
  display: block;
  object-fit: contain;
  background: #050506;
}

.sound-tip {
  position: absolute;
  right: 12px;
  bottom: 50px;
  padding: 6px 9px;
  border: 1px solid rgba(255, 255, 255, .11);
  border-radius: 999px;
  background: rgba(0, 0, 0, .58);
  color: rgba(255, 255, 255, .66);
  font-size: 9px;
  pointer-events: none;
}

.info {
  padding: 18px 16px;
}

.author {
  min-height: 32px;
  color: var(--mobile-text);
  font-size: 14px;
  font-weight: 800;
}

.info h1 {
  margin-top: 7px;
  font-size: 20px;
  line-height: 1.3;
  letter-spacing: -.03em;
  word-break: break-word;
}

.info > p {
  margin-top: 9px;
  color: var(--mobile-text-secondary);
  font-size: 12px;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
}

.info-actions {
  margin-top: 20px;
  display: flex;
  gap: 10px;
}

.info-actions button {
  min-width: 84px;
  min-height: 44px;
  padding: 9px 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border: 1px solid var(--mobile-border);
  border-radius: 22px;
  background: var(--mobile-surface-raised);
  color: var(--mobile-text-secondary);
  font-size: 12px;
}

.info-actions button.liked {
  border-color: rgba(255, 59, 92, .3);
  color: var(--mobile-accent);
}

.detail-state {
  min-height: calc(100dvh - 54px - env(safe-area-inset-top));
  padding: 30px;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 11px;
  color: var(--mobile-text-muted);
  text-align: center;
  font-size: 12px;
}

.detail-state strong {
  color: var(--mobile-text);
  font-size: 16px;
}

.detail-state p {
  max-width: 280px;
  line-height: 1.6;
}

.detail-state button {
  min-height: 44px;
  margin-top: 4px;
  padding: 0 17px;
  border-radius: 999px;
  background: var(--mobile-text);
  color: var(--mobile-bg);
  font-weight: 750;
}

.detail-loader {
  width: 28px;
  height: 28px;
  border: 3px solid rgba(255, 255, 255, .15);
  border-top-color: var(--mobile-text);
  border-radius: 50%;
  animation: spin .8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
