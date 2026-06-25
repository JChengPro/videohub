<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import type { FeedVideo } from '../api/types'
import { api } from '../api'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'
import AppIcon from './AppIcon.vue'
import Avatar from './Avatar.vue'
import CommentsSheet from './CommentsSheet.vue'

const props = defineProps<{ mode: 'latest' | 'following' | 'hot' }>()
const auth = useAuthStore()
const toast = useToastStore()
const router = useRouter()
const items = ref<FeedVideo[]>([])
const loading = ref(false)
const error = ref('')
const activeIndex = ref(0)
const commentsVideo = ref<FeedVideo | null>(null)
const followed = ref(new Set<number>())
const videos = ref<HTMLVideoElement[]>([])
const playing = ref<Record<number, boolean>>({})
const likeBusy = ref(new Set<number>())
const followBusy = ref(new Set<number>())
let observer: IntersectionObserver | null = null
let loadRequest = 0

async function load() {
  const request = ++loadRequest
  if (props.mode === 'following' && !auth.isLoggedIn) {
    await router.push('/me')
    return
  }
  loading.value = true
  error.value = ''
  observer?.disconnect()
  videos.value = []
  playing.value = {}
  try {
    const response = props.mode === 'following' ? await api.followingFeed() : props.mode === 'hot' ? await api.hot() : await api.latest()
    if (request !== loadRequest) return
    items.value = response.video_list
    if (auth.isLoggedIn) {
      try {
        const response = await api.following()
        if (request === loadRequest) followed.value = new Set(response.vloggers.map((item) => item.id))
      } catch {
        if (request === loadRequest) followed.value = new Set()
      }
    } else {
      followed.value = new Set()
    }
    if (request !== loadRequest) return
    await nextTick()
    observeVideos()
  } catch (cause) {
    if (request === loadRequest) error.value = cause instanceof Error ? cause.message : String(cause)
  } finally {
    if (request === loadRequest) loading.value = false
  }
}

function observeVideos() {
  observer?.disconnect()
  observer = new IntersectionObserver((entries) => {
    entries.forEach((entry) => {
      const video = entry.target as HTMLVideoElement
      const index = Number(video.dataset.index)
      if (entry.isIntersecting && entry.intersectionRatio > .72) {
        activeIndex.value = index
        videos.value.forEach((item, i) => {
          if (i === index) item.play().catch(() => {})
          else item.pause()
        })
      }
    })
  }, { threshold: [.72] })
  videos.value.forEach((video) => observer?.observe(video))
}

function bindVideo(element: unknown, index: number) {
  if (element instanceof HTMLVideoElement) videos.value[index] = element
}

function togglePlay(index: number) {
  const video = videos.value[index]
  if (!video) return
  video.paused ? video.play().catch(() => {}) : video.pause()
}

async function toggleLike(item: FeedVideo) {
  if (!auth.isLoggedIn) return toast.error('登录后才能点赞')
  if (likeBusy.value.has(item.id)) return
  likeBusy.value.add(item.id)
  try {
    const state = item.is_liked ? await api.unlike(item.id) : await api.like(item.id)
    item.is_liked = state.is_liked
    item.likes_count = Math.max(0, state.likes_count)
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
  finally { likeBusy.value.delete(item.id) }
}

async function toggleFollow(item: FeedVideo) {
  if (!auth.isLoggedIn) return toast.error('登录后才能关注')
  if (followBusy.value.has(item.author.id)) return
  followBusy.value.add(item.author.id)
  try {
    if (followed.value.has(item.author.id)) {
      await api.unfollow(item.author.id)
      followed.value.delete(item.author.id)
      followed.value = new Set(followed.value)
      toast.info('已取消关注')
    } else {
      await api.follow(item.author.id)
      followed.value.add(item.author.id)
      followed.value = new Set(followed.value)
      toast.success('已关注')
    }
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
  finally { followBusy.value.delete(item.author.id) }
}

async function share(item: FeedVideo) {
  const url = `${location.origin}/video/${item.id}`
  try {
    if (navigator.share) await navigator.share({ title: item.title, url })
    else { await navigator.clipboard.writeText(url); toast.success('链接已复制') }
  } catch { /* User cancelled native share. */ }
}

watch(() => props.mode, load)
onMounted(load)
onUnmounted(() => observer?.disconnect())
</script>

<template>
  <main class="feed">
    <div v-if="loading" class="state"><span class="loader" /><p>视频正在赶来</p></div>
    <div v-else-if="error" class="state"><p>{{ error }}</p><button @click="load">重新加载</button></div>
    <div v-else-if="!items.length" class="state"><p>这里暂时没有视频</p></div>
    <article v-for="(item, index) in items" :key="item.id" class="video-card">
      <video
        :ref="(el) => bindVideo(el, index)"
        :data-index="index"
        :src="item.play_url"
        :poster="item.cover_url"
        playsinline
        loop
        preload="metadata"
        @play="playing[index] = true"
        @pause="playing[index] = false"
        @click="togglePlay(index)"
      />
      <div class="shade" />
      <button v-if="activeIndex === index && !playing[index]" class="play-indicator" aria-label="播放视频" @click="togglePlay(index)"><AppIcon name="play" :size="42" filled /></button>
      <section class="copy">
        <button class="author-name" @click.stop="router.push(`/user/${item.author.id}`)">@{{ item.author.username }}</button>
        <button class="video-title" @click.stop="router.push(`/video/${item.id}`)">{{ item.title }}</button>
        <p v-if="item.description">{{ item.description }}</p>
        <span class="sound"><i /> 原声 · {{ item.author.username }}</span>
      </section>
      <aside class="actions">
        <div class="author">
          <button aria-label="查看作者" @click="router.push(`/user/${item.author.id}`)"><Avatar :name="item.author.username" :size="46" /></button>
          <button v-if="item.author.id !== auth.claims?.account_id" class="follow-mark" :class="{ followed: followed.has(item.author.id) }" :disabled="followBusy.has(item.author.id)" aria-label="关注作者" @click="toggleFollow(item)">{{ followed.has(item.author.id) ? '✓' : '+' }}</button>
        </div>
        <button :class="{ liked: item.is_liked }" :disabled="likeBusy.has(item.id)" aria-label="点赞" @click="toggleLike(item)"><span><AppIcon name="heart" :size="30" :filled="item.is_liked" /></span><b>{{ item.likes_count }}</b></button>
        <button aria-label="评论" @click="commentsVideo = item"><span><AppIcon name="comment" :size="29" filled /></span><b>评论</b></button>
        <button aria-label="分享" @click="share(item)"><span><AppIcon name="share" :size="28" filled /></span><b>分享</b></button>
      </aside>
    </article>
    <CommentsSheet v-if="commentsVideo" :video="commentsVideo" @close="commentsVideo = null" />
  </main>
</template>

<style scoped>
.feed { height: 100dvh; overflow-y: auto; scroll-snap-type: y mandatory; background: #08080a; scrollbar-width: none; }.feed::-webkit-scrollbar { display: none; }
.video-card { position: relative; height: 100dvh; overflow: hidden; scroll-snap-align: start; scroll-snap-stop: always; background: #0a0a0c; }
video { width: 100%; height: 100%; display: block; object-fit: cover; }
.shade { position: absolute; inset: 0; pointer-events: none; background: linear-gradient(to bottom, rgba(0,0,0,.18), transparent 25%, transparent 52%, rgba(0,0,0,.8) 94%); }
.play-indicator { position: absolute; top: 50%; left: 50%; transform: translate(-50%,-50%); color: rgba(255,255,255,.72); filter: drop-shadow(0 4px 12px #000); }
.copy { position: absolute; z-index: 2; right: 78px; bottom: calc(78px + env(safe-area-inset-bottom)); left: 14px; color: #fff; text-shadow: 0 1px 5px rgba(0,0,0,.65); }.author-name { display: block; font-size: 15px; font-weight: 800; }.video-title { margin-top: 8px; display: block; font-size: 14px; font-weight: 600; text-align: left; }.copy p { margin-top: 5px; display: -webkit-box; overflow: hidden; color: #e6e6e8; font-size: 12px; line-height: 1.45; -webkit-line-clamp: 2; -webkit-box-orient: vertical; }.sound { margin-top: 12px; display: flex; align-items: center; gap: 7px; font-size: 11px; }.sound i { width: 13px; height: 13px; border: 2px solid #fff; border-radius: 50%; }
.actions { position: absolute; z-index: 3; right: 10px; bottom: calc(78px + env(safe-area-inset-bottom)); display: flex; flex-direction: column; align-items: center; gap: 16px; }.actions button { width: 54px; display: grid; place-items: center; gap: 3px; color: #fff; filter: drop-shadow(0 2px 5px rgba(0,0,0,.5)); }.actions span { width: 45px; height: 38px; display: grid; place-items: center; }.actions b { font-size: 10px; font-weight: 700; }.actions .liked { color: #fe2c55; }.author { position: relative; margin-bottom: 2px; }.author .follow-mark { position: absolute; right: 17px; bottom: -6px; width: 20px; height: 20px; display: grid; place-items: center; border-radius: 50%; background: #fe2c55; color: #fff; font-size: 16px; font-weight: 700; }.author .follow-mark.followed { background: #fff; color: #222; font-size: 11px; }
.state { height: 100dvh; display: grid; place-content: center; justify-items: center; gap: 14px; color: #888; font-size: 13px; }.state button { padding: 9px 16px; border-radius: 20px; background: #fff; color: #111; }.loader { width: 28px; height: 28px; border: 3px solid #333; border-top-color: #fe2c55; border-radius: 50%; animation: spin .8s linear infinite; } @keyframes spin { to { transform: rotate(360deg); } }
</style>
