<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import type { FeedVideo, Video } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'
import AppIcon from '../components/AppIcon.vue'
import CommentsSheet from '../components/CommentsSheet.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const toast = useToastStore()
const video = ref<Video | null>(null)
const liked = ref(false)
const commentsOpen = ref(false)
const likeBusy = ref(false)
const id = computed(() => Number(route.params.id))
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
  video.value = null
  liked.value = false
  commentsOpen.value = false
  if (!Number.isInteger(id.value) || id.value <= 0) {
    toast.error('无效的视频 ID')
    return
  }
  try {
    video.value = await api.videoDetail(id.value)
    liked.value = auth.isLoggedIn ? (await api.isLiked(id.value)).is_liked : false
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
}

async function toggleLike() {
  if (!auth.isLoggedIn) return router.push('/me')
  if (!video.value || likeBusy.value) return
  likeBusy.value = true
  try {
    const state = liked.value ? await api.unlike(id.value) : await api.like(id.value)
    liked.value = state.is_liked
    video.value.likes_count = Math.max(0, state.likes_count)
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
  finally { likeBusy.value = false }
}

watch(id, load, { immediate: true })
</script>

<template>
  <main class="detail">
    <button class="back" @click="router.back()"><AppIcon name="back" /></button>
    <video v-if="video" :src="video.play_url" :poster="video.cover_url" controls autoplay playsinline />
    <section v-if="video" class="info">
      <button class="author" @click="router.push(`/user/${video.author_id}`)">@{{ video.username }}</button>
      <h1>{{ video.title }}</h1><p>{{ video.description }}</p>
      <div><button :class="{ liked }" :disabled="likeBusy" @click="toggleLike"><AppIcon name="heart" :filled="liked" />{{ Math.max(0, video.likes_count) }}</button><button @click="commentsOpen = true"><AppIcon name="comment" />评论</button></div>
    </section>
    <CommentsSheet v-if="commentsOpen && feedVideo" :video="feedVideo" @close="commentsOpen = false" />
  </main>
</template>

<style scoped>
.detail { min-height: 100dvh; padding-bottom: 30px; background: #09090b; }.back { position: fixed; z-index: 10; top: calc(12px + env(safe-area-inset-top)); left: 12px; width: 38px; height: 38px; display: grid; place-items: center; border-radius: 50%; background: rgba(0,0,0,.5); }.detail video { width: 100%; height: 68dvh; display: block; object-fit: contain; background: #000; }.info { padding: 17px 16px; }.author { font-size: 14px; font-weight: 800; }.info h1 { margin-top: 9px; font-size: 18px; }.info p { margin-top: 7px; color: #888; font-size: 12px; line-height: 1.6; }.info div { margin-top: 18px; display: flex; gap: 12px; }.info div button { min-width: 80px; padding: 9px 14px; display: flex; align-items: center; justify-content: center; gap: 6px; border-radius: 22px; background: #242429; }.info div button.liked { color: #fe2c55; }
</style>
