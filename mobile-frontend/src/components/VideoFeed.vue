<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { api } from '../api'
import type { FeedVideo } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'
import AppIcon from './AppIcon.vue'
import Avatar from './Avatar.vue'
import CommentsSheet from './CommentsSheet.vue'

type FeedMode = 'latest' | 'following' | 'hot'

const props = defineProps<{ mode: FeedMode }>()
const auth = useAuthStore()
const toast = useToastStore()
const router = useRouter()

const feedElement = ref<HTMLElement | null>(null)
const items = ref<FeedVideo[]>([])
const loading = ref(false)
const loadingMore = ref(false)
const error = ref('')
const hasMore = ref(false)
const nextTime = ref(0)
const nextOffset = ref(0)
const asOf = ref(0)
const activeIndex = ref(0)
const commentsVideo = ref<FeedVideo | null>(null)
const muted = ref(true)
const followed = ref(new Set<number>())
const expandedDescriptions = ref(new Set<number>())
const playing = reactive<Record<number, boolean>>({})
const mediaLoading = reactive<Record<number, boolean>>({})
const mediaError = reactive<Record<number, string>>({})
const likeBusy = ref(new Set<number>())
const followBusy = ref(new Set<number>())

const videoElements = new Map<number, HTMLVideoElement>()
let observer: IntersectionObserver | null = null
let loadRequest = 0
let tapTimer = 0
let lastTapAt = 0
let lastTapVideoId = 0
let resumeAfterComments = false
let resumeAfterVisibility = false

const activeVideo = computed(() => items.value[activeIndex.value] ?? null)

function mergeUnique(current: FeedVideo[], incoming: FeedVideo[]) {
  const seen = new Set(current.map((item) => item.id))
  return current.concat(incoming.filter((item) => !seen.has(item.id)))
}

function clearPlaybackState() {
  observer?.disconnect()
  for (const video of videoElements.values()) video.pause()
  videoElements.clear()
  for (const key of Object.keys(playing)) delete playing[Number(key)]
  for (const key of Object.keys(mediaLoading)) delete mediaLoading[Number(key)]
  for (const key of Object.keys(mediaError)) delete mediaError[Number(key)]
}

async function loadFollowingState(request: number) {
  if (!auth.isLoggedIn) {
    followed.value = new Set()
    return
  }
  try {
    const response = await api.following()
    if (request === loadRequest) followed.value = new Set(response.vloggers.map((item) => item.id))
  } catch {
    if (request === loadRequest) followed.value = new Set()
  }
}

async function load(reset: boolean) {
  if (props.mode === 'following' && !auth.isLoggedIn) {
    items.value = []
    await router.push('/me')
    return
  }
  if (!reset && (loadingMore.value || !hasMore.value)) return

  const request = reset ? ++loadRequest : loadRequest
  if (reset) {
    loading.value = true
    error.value = ''
    hasMore.value = false
    nextTime.value = 0
    nextOffset.value = 0
    asOf.value = 0
    activeIndex.value = 0
    clearPlaybackState()
  } else {
    loadingMore.value = true
    error.value = ''
  }

  try {
    let nextItems: FeedVideo[] = []
    if (props.mode === 'following') {
      const response = await api.followingFeed(reset ? 0 : nextTime.value)
      nextItems = response.video_list
      if (request !== loadRequest) return
      nextTime.value = response.next_time
      hasMore.value = response.has_more
    } else if (props.mode === 'hot') {
      const response = await api.hot(reset ? 0 : nextOffset.value, reset ? 0 : asOf.value)
      nextItems = response.video_list
      if (request !== loadRequest) return
      nextOffset.value = response.next_offset
      asOf.value = response.as_of
      hasMore.value = response.has_more
    } else {
      const response = await api.latest(reset ? 0 : nextTime.value)
      nextItems = response.video_list
      if (request !== loadRequest) return
      nextTime.value = response.next_time
      hasMore.value = response.has_more
    }

    items.value = reset ? nextItems : mergeUnique(items.value, nextItems)
    await loadFollowingState(request)
    await nextTick()
    if (request !== loadRequest) return
    observeVideos()
  } catch (cause) {
    if (request === loadRequest) error.value = cause instanceof Error ? cause.message : String(cause)
  } finally {
    if (request === loadRequest) {
      loading.value = false
      loadingMore.value = false
    }
  }
}

function bindVideo(element: unknown, id: number) {
  const previous = videoElements.get(id)
  if (previous && previous !== element) observer?.unobserve(previous)
  if (!(element instanceof HTMLVideoElement)) {
    videoElements.delete(id)
    return
  }
  element.muted = muted.value
  videoElements.set(id, element)
  observer?.observe(element)
}

function observeVideos() {
  observer?.disconnect()
  observer = new IntersectionObserver((entries) => {
    const visible = entries
      .filter((entry) => entry.isIntersecting)
      .sort((a, b) => b.intersectionRatio - a.intersectionRatio)[0]
    if (!visible || visible.intersectionRatio < 0.68) return
    const index = Number((visible.target as HTMLVideoElement).dataset.index)
    if (!Number.isInteger(index)) return
    activateVideo(index)
  }, { root: feedElement.value, threshold: [0.25, 0.68, 0.9] })
  for (const video of videoElements.values()) observer.observe(video)
}

function pauseAll() {
  for (const video of videoElements.values()) video.pause()
}

async function playActive() {
  const item = activeVideo.value
  if (!item || commentsVideo.value || document.hidden) return
  for (const [id, video] of videoElements) {
    if (id !== item.id) video.pause()
  }
  const video = videoElements.get(item.id)
  if (!video) return
  video.muted = muted.value
  mediaError[item.id] = ''
  try {
    await video.play()
    if (activeVideo.value?.id !== item.id || commentsVideo.value || document.hidden) video.pause()
  } catch {
    playing[item.id] = false
    mediaLoading[item.id] = false
    mediaError[item.id] = '自动播放受限，点击继续'
  }
}

function activateVideo(index: number) {
  if (index < 0 || index >= items.value.length) return
  activeIndex.value = index
  void playActive()
  if (index >= items.value.length - 3 && hasMore.value) void load(false)
}

async function togglePlay(index: number) {
  const item = items.value[index]
  if (!item) return
  activeIndex.value = index
  const video = videoElements.get(item.id)
  if (!video) return
  if (video.paused) {
    mediaError[item.id] = ''
    try {
      await video.play()
    } catch {
      mediaError[item.id] = '视频暂时无法播放，请重试'
    }
  } else {
    video.pause()
  }
}

function onVideoTap(item: FeedVideo, index: number) {
  const now = Date.now()
  if (lastTapVideoId === item.id && now - lastTapAt < 280) {
    window.clearTimeout(tapTimer)
    lastTapAt = 0
    lastTapVideoId = 0
    void toggleLike(item)
    return
  }
  lastTapAt = now
  lastTapVideoId = item.id
  window.clearTimeout(tapTimer)
  tapTimer = window.setTimeout(() => {
    void togglePlay(index)
    lastTapAt = 0
    lastTapVideoId = 0
  }, 230)
}

function toggleMute() {
  muted.value = !muted.value
  for (const video of videoElements.values()) video.muted = muted.value
  toast.info(muted.value ? '声音已关闭' : '声音已开启')
  if (!muted.value) void playActive()
}

function onVideoPlaying(id: number) {
  playing[id] = true
  mediaLoading[id] = false
  mediaError[id] = ''
}

function onVideoPause(id: number) {
  playing[id] = false
}

function onVideoWaiting(id: number) {
  mediaLoading[id] = true
}

function onVideoCanPlay(id: number) {
  mediaLoading[id] = false
}

function onVideoError(id: number) {
  playing[id] = false
  mediaLoading[id] = false
  mediaError[id] = '视频加载失败，点击重试'
}

async function retryVideo(id: number) {
  const video = videoElements.get(id)
  if (!video) return
  mediaError[id] = ''
  mediaLoading[id] = true
  video.load()
  await playActive()
}

async function toggleLike(item: FeedVideo) {
  if (!auth.isLoggedIn) {
    toast.error('登录后才能点赞')
    return
  }
  if (likeBusy.value.has(item.id)) return
  likeBusy.value.add(item.id)
  try {
    const state = item.is_liked ? await api.unlike(item.id) : await api.like(item.id)
    item.is_liked = state.is_liked
    item.likes_count = Math.max(0, state.likes_count)
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : String(cause))
  } finally {
    likeBusy.value.delete(item.id)
  }
}

async function toggleFollow(item: FeedVideo) {
  if (!auth.isLoggedIn) {
    toast.error('登录后才能关注')
    return
  }
  if (followBusy.value.has(item.author.id)) return
  followBusy.value.add(item.author.id)
  try {
    if (followed.value.has(item.author.id)) {
      await api.unfollow(item.author.id)
      followed.value.delete(item.author.id)
      toast.info('已取消关注')
    } else {
      await api.follow(item.author.id)
      followed.value.add(item.author.id)
      toast.success('已关注')
    }
    followed.value = new Set(followed.value)
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : String(cause))
  } finally {
    followBusy.value.delete(item.author.id)
  }
}

async function share(item: FeedVideo) {
  const url = `${location.origin}/video/${item.id}`
  try {
    if (navigator.share) await navigator.share({ title: item.title, url })
    else {
      await navigator.clipboard.writeText(url)
      toast.success('链接已复制')
    }
  } catch (cause) {
    if (cause instanceof DOMException && cause.name === 'AbortError') return
    toast.error('分享失败，请稍后重试')
  }
}

function openComments(item: FeedVideo) {
  const video = videoElements.get(item.id)
  resumeAfterComments = !!video && !video.paused
  video?.pause()
  commentsVideo.value = item
}

function closeComments() {
  commentsVideo.value = null
  if (resumeAfterComments && !document.hidden) void playActive()
  resumeAfterComments = false
}

function updateCommentCount(count: number) {
  if (commentsVideo.value) commentsVideo.value.comments_count = Math.max(0, count)
}

function toggleDescription(id: number) {
  const next = new Set(expandedDescriptions.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expandedDescriptions.value = next
}

function formatCount(value: number) {
  const count = Math.max(0, value)
  if (count >= 10000) return `${(count / 10000).toFixed(count >= 100000 ? 0 : 1)}万`
  return String(count)
}

function onVisibilityChange() {
  const item = activeVideo.value
  const video = item ? videoElements.get(item.id) : undefined
  if (document.hidden) {
    resumeAfterVisibility = !!video && !video.paused && !commentsVideo.value
    pauseAll()
    return
  }
  if (resumeAfterVisibility && !commentsVideo.value) void playActive()
  resumeAfterVisibility = false
}

watch(() => props.mode, () => void load(true))
watch(() => auth.isLoggedIn, () => {
  if (props.mode === 'following' || auth.isLoggedIn) void load(true)
  else {
    followed.value = new Set()
    items.value.forEach((item) => { item.is_liked = false })
  }
})

onMounted(() => {
  document.addEventListener('visibilitychange', onVisibilityChange)
  void load(true)
})

onUnmounted(() => {
  loadRequest += 1
  observer?.disconnect()
  window.clearTimeout(tapTimer)
  document.removeEventListener('visibilitychange', onVisibilityChange)
  pauseAll()
  videoElements.clear()
})
</script>

<template>
  <main ref="feedElement" class="feed" aria-label="视频流">
    <div v-if="loading" class="state" role="status">
      <span class="loader" />
      <p>正在准备视频</p>
    </div>
    <div v-else-if="error && !items.length" class="state error-state" role="alert">
      <AppIcon name="warning" :size="34" />
      <strong>暂时无法加载</strong>
      <p>{{ error }}</p>
      <button type="button" @click="load(true)">重新加载</button>
    </div>
    <div v-else-if="!items.length" class="state">
      <AppIcon name="video" :size="36" />
      <strong>这里暂时没有视频</strong>
      <p>{{ mode === 'following' ? '关注创作者后，他们的新作品会出现在这里' : '稍后再来看看新的内容' }}</p>
    </div>

    <article v-for="(item, index) in items" v-else :key="item.id" class="video-card" :aria-label="`视频：${item.title}`">
      <video
        :ref="(element) => bindVideo(element, item.id)"
        :data-index="index"
        :src="item.play_url"
        :poster="item.cover_url"
        :preload="Math.abs(index - activeIndex) <= 1 ? 'metadata' : 'none'"
        :muted="muted"
        playsinline
        loop
        tabindex="0"
        :aria-label="`${item.title}，点击播放或暂停，双击点赞`"
        @loadstart="mediaLoading[item.id] = true"
        @playing="onVideoPlaying(item.id)"
        @pause="onVideoPause(item.id)"
        @waiting="onVideoWaiting(item.id)"
        @canplay="onVideoCanPlay(item.id)"
        @error="onVideoError(item.id)"
        @click="onVideoTap(item, index)"
        @keydown.enter.prevent="togglePlay(index)"
        @keydown.space.prevent="togglePlay(index)"
      />

      <div class="shade" />

      <div v-if="activeIndex === index && mediaLoading[item.id] && !mediaError[item.id]" class="media-status" role="status">
        <span class="loader small" />
        <span>缓冲中</span>
      </div>
      <button v-if="activeIndex === index && mediaError[item.id]" class="media-error" type="button" @click.stop="retryVideo(item.id)">
        <span>{{ mediaError[item.id] }}</span>
        <b>重试</b>
      </button>
      <button
        v-if="activeIndex === index && !playing[item.id] && !mediaLoading[item.id] && !mediaError[item.id]"
        class="play-indicator"
        type="button"
        aria-label="播放视频"
        @click.stop="togglePlay(index)"
      >
        <AppIcon name="play" :size="42" filled />
      </button>

      <section class="copy">
        <button class="author-name" type="button" @click.stop="router.push(`/user/${item.author.id}`)">{{ item.author.username }}</button>
        <button class="video-title" type="button" @click.stop="router.push(`/video/${item.id}`)">{{ item.title }}</button>
        <div v-if="item.description" class="description-row">
          <p :class="{ expanded: expandedDescriptions.has(item.id) }">{{ item.description }}</p>
          <button v-if="item.description.length > 46" type="button" @click.stop="toggleDescription(item.id)">
            {{ expandedDescriptions.has(item.id) ? '收起' : '展开' }}
          </button>
        </div>
        <button class="sound-control" type="button" :aria-label="muted ? '当前静音，点击开启声音' : '当前有声，点击关闭声音'" @click.stop="toggleMute">
          <AppIcon :name="muted ? 'volume-off' : 'volume'" :size="15" />
          <span>{{ muted ? '开启声音' : '关闭声音' }}</span>
        </button>
      </section>

      <aside class="actions" aria-label="视频互动">
        <div class="author">
          <button type="button" :aria-label="`查看 ${item.author.username} 的主页`" @click.stop="router.push(`/user/${item.author.id}`)">
            <Avatar :name="item.author.username" :id="item.author.id" :size="46" />
          </button>
          <button
            v-if="item.author.id !== auth.claims?.account_id"
            class="follow-mark"
            :class="{ followed: followed.has(item.author.id) }"
            type="button"
            :disabled="followBusy.has(item.author.id)"
            :aria-label="followed.has(item.author.id) ? `取消关注 ${item.author.username}` : `关注 ${item.author.username}`"
            @click.stop="toggleFollow(item)"
          >
            {{ followed.has(item.author.id) ? '✓' : '+' }}
          </button>
        </div>
        <button
          :class="{ liked: item.is_liked }"
          type="button"
          :disabled="likeBusy.has(item.id)"
          :aria-label="item.is_liked ? `取消点赞，当前 ${item.likes_count} 赞` : `点赞，当前 ${item.likes_count} 赞`"
          @click.stop="toggleLike(item)"
        >
          <span><AppIcon name="heart" :size="30" :filled="item.is_liked" /></span>
          <b>{{ formatCount(item.likes_count) }}</b>
        </button>
        <button type="button" :aria-label="`查看评论，当前 ${item.comments_count} 条`" @click.stop="openComments(item)">
          <span><AppIcon name="comment" :size="29" filled /></span>
          <b>{{ formatCount(item.comments_count) }}</b>
        </button>
        <button type="button" aria-label="分享视频" @click.stop="share(item)">
          <span><AppIcon name="share" :size="28" filled /></span>
          <b>分享</b>
        </button>
      </aside>

      <div v-if="index === items.length - 1" class="feed-progress" role="status">
        <button v-if="error" type="button" @click.stop="load(false)">加载失败，点击重试</button>
        <template v-else-if="loadingMore"><span class="loader small" />加载更多</template>
        <template v-else-if="!hasMore">已经看完了</template>
      </div>
    </article>

    <CommentsSheet v-if="commentsVideo" :video="commentsVideo" @close="closeComments" @count-change="updateCommentCount" />
  </main>
</template>

<style scoped>
.feed {
  height: 100dvh;
  overflow-y: auto;
  overscroll-behavior: contain;
  scroll-snap-type: y mandatory;
  scrollbar-width: none;
  background: #050506;
}
.feed::-webkit-scrollbar { display: none; }
.video-card {
  position: relative;
  height: 100dvh;
  overflow: hidden;
  scroll-snap-align: start;
  scroll-snap-stop: always;
  background: #050506;
}
video { width: 100%; height: 100%; display: block; object-fit: contain; background: #050506; }
.shade {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    linear-gradient(to bottom, rgba(0, 0, 0, .28), transparent 24%, transparent 50%, rgba(0, 0, 0, .86) 96%),
    linear-gradient(to left, rgba(0, 0, 0, .16), transparent 34%);
}
.play-indicator {
  position: absolute;
  z-index: 4;
  top: 50%;
  left: 50%;
  width: 68px;
  height: 68px;
  transform: translate(-50%, -50%);
  display: grid;
  place-items: center;
  border: 1px solid rgba(255, 255, 255, .18);
  border-radius: 50%;
  background: rgba(10, 10, 12, .46);
  color: rgba(255, 255, 255, .86);
  backdrop-filter: blur(10px);
  filter: drop-shadow(0 8px 20px rgba(0, 0, 0, .55));
}
.media-status,
.media-error {
  position: absolute;
  z-index: 5;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  display: flex;
  align-items: center;
  gap: 9px;
  border: 1px solid var(--mobile-border-strong);
  border-radius: 999px;
  background: rgba(14, 14, 16, .78);
  color: var(--mobile-text-secondary);
  backdrop-filter: blur(14px);
  font-size: 11px;
}
.media-status { padding: 10px 14px; pointer-events: none; }
.media-error { width: min(286px, calc(100% - 48px)); justify-content: space-between; padding: 10px 10px 10px 15px; text-align: left; }
.media-error b { padding: 6px 10px; border-radius: 999px; background: var(--mobile-text); color: var(--mobile-bg); font-size: 10px; white-space: nowrap; }
.copy {
  position: absolute;
  z-index: 2;
  right: 78px;
  bottom: calc(78px + env(safe-area-inset-bottom));
  left: 16px;
  color: #fff;
  text-shadow: 0 2px 8px rgba(0, 0, 0, .7);
}
.author-name { min-height: 34px; display: block; color: #fff; font-size: 15px; font-weight: 850; }
.video-title {
  max-width: 100%;
  margin-top: 3px;
  overflow: hidden;
  display: -webkit-box;
  color: #fff;
  font-size: 14px;
  font-weight: 700;
  line-height: 1.45;
  text-align: left;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}
.description-row { margin-top: 5px; }
.description-row p {
  overflow: hidden;
  display: -webkit-box;
  color: rgba(255, 255, 255, .8);
  font-size: 12px;
  line-height: 1.5;
  word-break: break-word;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}
.description-row p.expanded { display: block; max-height: 30dvh; overflow-y: auto; }
.description-row button { min-height: 28px; color: rgba(255, 255, 255, .9); font-size: 10px; font-weight: 800; }
.sound-control {
  min-height: 38px;
  margin-top: 5px;
  padding-right: 8px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: rgba(255, 255, 255, .84);
  font-size: 11px;
}
.actions {
  position: absolute;
  z-index: 3;
  right: 9px;
  bottom: calc(76px + env(safe-area-inset-bottom));
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}
.actions > button,
.actions .author > button:first-child {
  width: 56px;
  min-height: 54px;
  display: grid;
  place-items: center;
  gap: 2px;
  color: #fff;
  filter: drop-shadow(0 3px 7px rgba(0, 0, 0, .55));
}
.actions span { width: 46px; height: 38px; display: grid; place-items: center; }
.actions b { font-size: 10px; font-weight: 750; }
.actions .liked { color: var(--mobile-accent); }
.author { position: relative; margin-bottom: 2px; }
.author .follow-mark {
  position: absolute;
  right: 17px;
  bottom: -3px;
  width: 22px;
  min-height: 22px;
  display: grid;
  place-items: center;
  border: 2px solid #111;
  border-radius: 50%;
  background: var(--mobile-accent);
  color: #fff;
  font-size: 15px;
  font-weight: 800;
  line-height: 1;
  filter: none;
}
.author .follow-mark.followed { background: #f4f4f5; color: #18181b; font-size: 10px; }
.feed-progress {
  position: absolute;
  z-index: 4;
  right: 0;
  bottom: calc(66px + env(safe-area-inset-bottom));
  left: 0;
  min-height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  color: rgba(255, 255, 255, .54);
  font-size: 10px;
  pointer-events: none;
}
.feed-progress button { min-height: 30px; padding: 0 12px; border-radius: 999px; background: rgba(15, 15, 17, .76); color: rgba(255, 255, 255, .82); pointer-events: auto; }
.state {
  height: 100dvh;
  padding: 32px;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 12px;
  color: var(--mobile-text-muted);
  text-align: center;
  font-size: 12px;
}
.state strong { color: var(--mobile-text); font-size: 16px; }
.state p { max-width: 280px; line-height: 1.6; }
.state button { min-height: 44px; margin-top: 4px; padding: 9px 18px; border-radius: 999px; background: var(--mobile-text); color: var(--mobile-bg); font-weight: 750; }
.error-state { color: var(--mobile-text-secondary); }
.loader { width: 28px; height: 28px; border: 3px solid rgba(255, 255, 255, .16); border-top-color: var(--mobile-text); border-radius: 50%; animation: spin .8s linear infinite; }
.loader.small { width: 14px; height: 14px; border-width: 2px; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (orientation: landscape) and (max-height: 560px) {
  .copy { right: 72px; bottom: calc(62px + env(safe-area-inset-bottom)); }
  .actions { bottom: calc(58px + env(safe-area-inset-bottom)); gap: 4px; }
  .actions > button,
  .actions .author > button:first-child { min-height: 46px; }
  .description-row,
  .sound-control { display: none; }
}

.copy {
  right: 74px;
  bottom: calc(68px + env(safe-area-inset-bottom));
  left: 13px;
}
.author-name { font-size: 14px; }
.video-title { font-size: 13px; }
.actions {
  right: 5px;
  bottom: calc(64px + env(safe-area-inset-bottom));
  gap: 7px;
}
.actions > button,
.actions .author > button:first-child { width: 58px; min-height: 55px; }
.actions span {
  width: 43px;
  height: 43px;
  border-radius: 50%;
  background: rgba(30,30,33,.78);
  backdrop-filter: blur(8px);
}
.actions .author > button:first-child span { background: transparent; }
.actions b { font-size: 9px; text-shadow: 0 2px 5px #000; }
.state button,
.feed-progress button { border-radius: var(--mobile-radius); }
</style>
