<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import UserAvatar from '../components/UserAvatar.vue'
import { ApiError } from '../api/client'
import * as commentApi from '../api/comment'
import * as feedApi from '../api/feed'
import * as likeApi from '../api/like'
import type { Comment, FeedVideoItem } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { useSocialStore } from '../stores/social'
import { useToastStore } from '../stores/toast'

type TabKey = 'recommend' | 'hot' | 'following'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const social = useSocialStore()
const toast = useToastStore()

const tab = ref<TabKey>('recommend')
const scroller = ref<HTMLDivElement | null>(null)
const commentInput = ref<HTMLTextAreaElement | null>(null)

const q = computed(() => (typeof route.query.q === 'string' ? route.query.q.trim().toLowerCase() : ''))

function syncTabWithRoute() {
  if (route.path === '/following') tab.value = 'following'
  else if (route.path === '/') tab.value = 'recommend'
}

async function selectTab(next: TabKey) {
  tab.value = next
  if (next === 'following') await router.push('/following')
  else if (next === 'hot') await router.push('/hot')
  else await router.push('/')
}

const recommend = reactive({
  items: [] as FeedVideoItem[],
  loading: false,
  error: '',
  hasMore: false,
  nextTime: 0,
})

const hot = reactive({
  items: [] as FeedVideoItem[],
  loading: false,
  error: '',
  hasMore: false,
  nextLikesCountBefore: undefined as number | undefined,
  nextIdBefore: undefined as number | undefined,
})

const following = reactive({
  items: [] as FeedVideoItem[],
  loading: false,
  error: '',
  hasMore: false,
  nextTime: 0,
})

const likeBusy = reactive<Record<string, boolean>>({})
const followBusy = reactive<Record<string, boolean>>({})

const muted = ref(true)
const paused = ref(false)
const mediaLoading = ref(false)
const playbackError = ref('')
const activeIndex = ref(0)
const videoMap = new Map<number, HTMLVideoElement>()
let resumeAfterVisibility = false
let resumeAfterDrawer = false
let drawerTrigger: HTMLElement | null = null
let followingRequest = 0
let commentRequest = 0

const currentState = computed(() => {
  if (tab.value === 'hot') return hot
  if (tab.value === 'following') return following
  return recommend
})

const filteredItems = computed(() => {
  const items = currentState.value.items
  if (!q.value) return items
  return items.filter((v) => v.title.toLowerCase().includes(q.value) || v.author.username.toLowerCase().includes(q.value))
})

const activeItem = computed(() => filteredItems.value[activeIndex.value] ?? null)
const myAccountId = computed(() => auth.claims?.account_id ?? 0)

function setVideoRef(id: number, el: HTMLVideoElement | null) {
  if (el) {
    el.muted = muted.value
    videoMap.set(id, el)
  } else {
    videoMap.delete(id)
  }
}

function getScrollerHeight() {
  return scroller.value?.clientHeight ?? 0
}

function scrollToIndex(idx: number) {
  const el = scroller.value
  if (!el) return
  const h = getScrollerHeight()
  if (!h) return
  const next = Math.max(0, Math.min(idx, Math.max(0, filteredItems.value.length - 1)))
  el.scrollTo({ top: next * h, behavior: 'smooth' })
}

let scrollRaf = 0
function onScroll() {
  if (!scroller.value) return
  if (scrollRaf) return
  scrollRaf = window.requestAnimationFrame(() => {
    scrollRaf = 0
    const el = scroller.value
    if (!el) return
    const h = el.clientHeight
    if (!h) return
    const idx = Math.round(el.scrollTop / h)
    if (idx !== activeIndex.value) activeIndex.value = idx
  })
}

async function playActive() {
  const item = activeItem.value
  if (!item || document.hidden || drawer.open) return
  for (const [id, v] of videoMap.entries()) {
    if (id === item.id) continue
    v.pause()
  }
  const video = videoMap.get(item.id)
  if (!video) return
  video.muted = muted.value
  playbackError.value = ''
  try {
    await video.play()
    paused.value = false
  } catch {
    paused.value = true
    playbackError.value = '浏览器阻止了自动播放，点击继续'
  }
}

function toggleMute() {
  muted.value = !muted.value
  for (const v of videoMap.values()) v.muted = muted.value
  toast.info(muted.value ? '声音已关闭' : '声音已开启')
}

async function togglePlayPause() {
  const item = activeItem.value
  if (!item) return
  const video = videoMap.get(item.id)
  if (!video) return
  if (video.paused) {
    playbackError.value = ''
    try {
      await video.play()
      paused.value = false
    } catch {
      paused.value = true
      playbackError.value = '视频暂时无法播放，请重试'
    }
  } else {
    video.pause()
    paused.value = true
  }
}

function onVideoPlaying(id: number) {
  if (activeItem.value?.id !== id) return
  mediaLoading.value = false
  playbackError.value = ''
  paused.value = false
}

function onVideoPause(id: number) {
  if (activeItem.value?.id === id) paused.value = true
}

function onVideoWaiting(id: number) {
  if (activeItem.value?.id === id) mediaLoading.value = true
}

function onVideoCanPlay(id: number) {
  if (activeItem.value?.id === id) mediaLoading.value = false
}

function onVideoError(id: number) {
  if (activeItem.value?.id !== id) return
  mediaLoading.value = false
  paused.value = true
  playbackError.value = '视频加载失败，点击重试'
}

async function retryPlayback() {
  const item = activeItem.value
  if (!item) return
  const video = videoMap.get(item.id)
  if (!video) return
  video.load()
  await togglePlayPause()
}

async function needLogin() {
  toast.error('请先登录')
  await router.push('/account')
}

async function loadRecommend(reset: boolean) {
  if (recommend.loading) return
  recommend.loading = true
  recommend.error = ''
  try {
    const res = await feedApi.listLatest({ limit: 10, latest_time: reset ? 0 : recommend.nextTime })
    recommend.hasMore = res.has_more
    recommend.nextTime = res.next_time
    recommend.items = reset ? res.video_list : recommend.items.concat(res.video_list)
  } catch (e) {
    recommend.error = e instanceof ApiError ? e.message : String(e)
  } finally {
    recommend.loading = false
  }
}

async function loadHot(reset: boolean) {
  if (hot.loading) return
  hot.loading = true
  hot.error = ''
  try {
    const res = await feedApi.listLikesCount({
      limit: 10,
      likes_count_before: reset ? undefined : hot.nextLikesCountBefore,
      id_before: reset ? undefined : hot.nextIdBefore,
    })
    hot.hasMore = res.has_more
    hot.nextLikesCountBefore = res.next_likes_count_before
    hot.nextIdBefore = res.next_id_before
    hot.items = reset ? res.video_list : hot.items.concat(res.video_list)
  } catch (e) {
    hot.error = e instanceof ApiError ? e.message : String(e)
  } finally {
    hot.loading = false
  }
}

async function loadFollowing(reset: boolean) {
  if (!auth.isLoggedIn) {
    followingRequest += 1
    following.loading = false
    following.items = []
    following.hasMore = false
    following.nextTime = 0
    following.error = '登录后才能查看关注流'
    return
  }
  if (!reset && following.loading) return
  const request = reset ? ++followingRequest : followingRequest
  following.loading = true
  following.error = ''
  try {
    const res = await feedApi.listByFollowing({ limit: 10, latest_time: reset ? 0 : following.nextTime })
    if (request !== followingRequest) return
    following.hasMore = res.has_more
    following.nextTime = res.next_time
    following.items = reset ? res.video_list : following.items.concat(res.video_list)
  } catch (e) {
    if (request === followingRequest) following.error = e instanceof ApiError ? e.message : String(e)
  } finally {
    if (request === followingRequest) following.loading = false
  }
}

async function ensureTabLoaded() {
  if (tab.value === 'recommend' && recommend.items.length === 0) await loadRecommend(true)
  if (tab.value === 'hot' && hot.items.length === 0) await loadHot(true)
  if (tab.value === 'following' && following.items.length === 0) await loadFollowing(true)
}

async function loadMoreIfNeeded() {
  const idx = activeIndex.value
  const items = filteredItems.value
  if (items.length === 0) return
  if (idx < items.length - 3) return

  if (tab.value === 'recommend' && recommend.hasMore) await loadRecommend(false)
  if (tab.value === 'hot' && hot.hasMore) await loadHot(false)
  if (tab.value === 'following' && following.hasMore) await loadFollowing(false)
}

async function toggleLike(item: FeedVideoItem) {
  if (!auth.isLoggedIn) return needLogin()
  const key = String(item.id)
  if (likeBusy[key]) return
  likeBusy[key] = true
  try {
    const state = item.is_liked ? await likeApi.unlike(item.id) : await likeApi.like(item.id)
    item.is_liked = state.is_liked
    item.likes_count = Math.max(0, state.likes_count)
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
  } finally {
    likeBusy[key] = false
  }
}

async function toggleFollow(authorId: number) {
  if (!auth.isLoggedIn) return needLogin()
  const key = String(authorId)
  if (followBusy[key]) return
  followBusy[key] = true
  try {
    if (social.isFollowing(authorId)) {
      await social.unfollow(authorId)
      toast.info('已取关')
    } else {
      await social.follow(authorId)
      toast.success('已关注')
    }
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
  } finally {
    followBusy[key] = false
  }
}

async function share(item: FeedVideoItem) {
  const url = `${location.origin}/video/${item.id}`
  try {
    if (navigator.share) {
      await navigator.share({ title: item.title, url })
      return
    }
    await navigator.clipboard.writeText(url)
    toast.success('链接已复制')
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') return
    window.prompt('复制链接', url)
  }
}

const drawer = reactive({
  open: false,
  video: null as FeedVideoItem | null,
  loading: false,
  error: '',
  comments: [] as Comment[],
  content: '',
})

function closeDrawer() {
  commentRequest += 1
  drawer.open = false
  drawer.video = null
  drawer.comments = []
  drawer.content = ''
  drawer.error = ''
  if (resumeAfterDrawer && !document.hidden) void playActive()
  else paused.value = true
  resumeAfterDrawer = false
  drawerTrigger?.focus()
  drawerTrigger = null
}

async function focusCommentInput() {
  await nextTick()
  commentInput.value?.focus()
}

async function openComments(item: FeedVideoItem) {
  const video = videoMap.get(item.id)
  resumeAfterDrawer = !!video && !video.paused
  drawerTrigger = document.activeElement instanceof HTMLElement ? document.activeElement : null
  video?.pause()
  paused.value = true
  drawer.open = true
  drawer.video = item
  drawer.content = ''
  await loadComments()
  await focusCommentInput()
}

async function loadComments() {
  if (!drawer.video) return
  const videoId = drawer.video.id
  const request = ++commentRequest
  drawer.loading = true
  drawer.error = ''
  try {
    const comments = await commentApi.listAll(videoId)
    if (request === commentRequest && drawer.open && drawer.video?.id === videoId) drawer.comments = comments
  } catch (e) {
    if (request === commentRequest && drawer.open) drawer.error = e instanceof ApiError ? e.message : String(e)
  } finally {
    if (request === commentRequest) drawer.loading = false
  }
}

async function publishComment() {
  if (!drawer.video) return
  if (!auth.isLoggedIn) return needLogin()
  const content = drawer.content.trim()
  if (!content) return
  drawer.loading = true
  drawer.error = ''
  try {
    await commentApi.publish(drawer.video.id, content)
    drawer.content = ''
    await loadComments()
    await focusCommentInput()
    toast.success('评论已发布')
  } catch (e) {
    drawer.error = e instanceof ApiError ? e.message : String(e)
    toast.error(drawer.error)
  } finally {
    drawer.loading = false
  }
}

function canDeleteComment(c: Comment) {
  const myId = auth.claims?.account_id
  return !!myId && myId === c.author_id
}

async function deleteComment(commentId: number) {
  if (!drawer.video) return
  if (!auth.isLoggedIn) return needLogin()
  if (!window.confirm('确认删除这条评论？')) return
  drawer.loading = true
  drawer.error = ''
  try {
    await commentApi.remove(commentId)
    await loadComments()
    toast.info('评论已删除')
  } catch (e) {
    drawer.error = e instanceof ApiError ? e.message : String(e)
    toast.error(drawer.error)
  } finally {
    drawer.loading = false
  }
}

async function onKeydown(e: KeyboardEvent) {
  const t = e.target as HTMLElement | null
  const isTyping = t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA')
  if (isTyping) {
    if (e.key === 'Escape' && drawer.open) {
      e.preventDefault()
      closeDrawer()
    }
    return
  }

  if (e.key.toLowerCase() === 'c') {
    e.preventDefault()
    if (drawer.open) closeDrawer()
    else if (activeItem.value) await openComments(activeItem.value)
    return
  }

  if (drawer.open) {
    if (e.key === 'Escape') {
      e.preventDefault()
      closeDrawer()
    }
    return
  }

  if (e.key === 'ArrowDown') {
    e.preventDefault()
    scrollToIndex(activeIndex.value + 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    scrollToIndex(activeIndex.value - 1)
  } else if (e.key === ' ') {
    e.preventDefault()
    togglePlayPause()
  } else if (e.key.toLowerCase() === 'm') {
    e.preventDefault()
    toggleMute()
  }
}

function onVisibilityChange() {
  const item = activeItem.value
  const video = item ? videoMap.get(item.id) : undefined
  if (document.hidden) {
    resumeAfterVisibility = !!video && !video.paused && !drawer.open
    video?.pause()
    return
  }
  if (resumeAfterVisibility && !drawer.open) void playActive()
  resumeAfterVisibility = false
}

watch(activeItem, async () => {
  await nextTick()
  await playActive()
  await loadMoreIfNeeded()
})

watch(
  () => tab.value,
  async () => {
    activeIndex.value = 0
    videoMap.clear()
    if (scroller.value) scroller.value.scrollTop = 0
    await ensureTabLoaded()
    await nextTick()
    await playActive()
  },
)

watch(
  () => q.value,
  async () => {
    activeIndex.value = 0
    if (scroller.value) scroller.value.scrollTop = 0
    await nextTick()
    await playActive()
  },
)

watch(
  () => route.path,
  () => syncTabWithRoute(),
)

watch(
  () => filteredItems.value.length,
  (len) => {
    if (len === 0) activeIndex.value = 0
    else if (activeIndex.value > len - 1) activeIndex.value = len - 1
  },
)

watch(
  () => auth.isLoggedIn,
  async (v) => {
    if (!v) await loadFollowing(true)
    else if (tab.value === 'following') await loadFollowing(true)
  },
)

onMounted(async () => {
  syncTabWithRoute()
  await ensureTabLoaded()
  await nextTick()
  await playActive()
  window.addEventListener('keydown', onKeydown)
  document.addEventListener('visibilitychange', onVisibilityChange)
})

onBeforeUnmount(() => {
  followingRequest += 1
  commentRequest += 1
  window.removeEventListener('keydown', onKeydown)
  document.removeEventListener('visibilitychange', onVisibilityChange)
  if (scrollRaf) window.cancelAnimationFrame(scrollRaf)
  for (const video of videoMap.values()) video.pause()
  videoMap.clear()
})
</script>

<template>
  <AppShell full>
    <div class="page">
      <div class="tabs">
        <button class="tab" :class="{ on: tab === 'recommend' }" type="button" @click="selectTab('recommend')">推荐</button>
        <button class="tab" :class="{ on: tab === 'following' }" type="button" @click="selectTab('following')">关注</button>

        <div class="tabs-right">
          <button
            class="top-chip"
            type="button"
            :aria-label="muted ? '当前静音，点击开启声音' : '当前有声，点击关闭声音'"
            :title="muted ? '当前静音' : '当前有声'"
            @click="toggleMute"
          >
            {{ muted ? '开启声音' : '关闭声音' }}
          </button>
          <RouterLink class="top-chip" :to="activeItem ? `/video/${activeItem.id}` : '/video'">详情</RouterLink>
        </div>
      </div>

      <div ref="scroller" class="scroller" @scroll="onScroll">
        <div v-if="currentState.loading && currentState.items.length === 0" class="center-hint">加载中…</div>
        <div v-else-if="currentState.error && currentState.items.length === 0" class="center-hint bad">
          {{ currentState.error }}
        </div>
        <div v-else-if="filteredItems.length === 0" class="center-hint empty">
          <div>
            <div class="empty-title">还没有可播放的视频</div>
            <div class="empty-sub">发布一个视频后，它会出现在推荐流里。</div>
            <RouterLink class="empty-link" to="/video">去发布</RouterLink>
          </div>
        </div>

        <section
          v-for="(item, idx) in filteredItems"
          :key="`${tab}-${item.id}`"
          class="slide"
          :class="{ active: idx === activeIndex }"
        >
          <div class="stage" @dblclick.prevent="toggleLike(item)">
            <video
              class="video"
              :ref="(el) => setVideoRef(item.id, el as HTMLVideoElement | null)"
              :src="item.play_url"
              :poster="item.cover_url"
              playsinline
              preload="metadata"
              loop
              @click.stop="togglePlayPause"
              @playing="onVideoPlaying(item.id)"
              @pause="onVideoPause(item.id)"
              @waiting="onVideoWaiting(item.id)"
              @canplay="onVideoCanPlay(item.id)"
              @error="onVideoError(item.id)"
            />
            <div class="grad" />
            <div v-if="idx === activeIndex && mediaLoading && !playbackError" class="media-status" role="status">
              <span class="media-spinner" />
              <span>正在缓冲</span>
            </div>
            <button v-if="idx === activeIndex && playbackError" class="media-error" type="button" @click.stop="retryPlayback">
              <span>{{ playbackError }}</span>
              <b>重试</b>
            </button>
            <button v-if="idx === activeIndex && paused && !mediaLoading && !playbackError" class="pause-indicator" type="button" aria-label="继续播放" @click.stop="togglePlayPause">
              <svg viewBox="0 0 24 24" fill="none"><path d="m9 6 10 6-10 6V6Z"/></svg>
            </button>

            <div class="meta">
              <RouterLink class="author-link" :to="`/u/${item.author.id}`" @click.stop>
                <UserAvatar :username="item.author.username" :id="item.author.id" :size="34" />
                <span class="author-name">@{{ item.author.username }}</span>
              </RouterLink>
              <div class="title">{{ item.title }}</div>
              <div v-if="item.description" class="desc">{{ item.description }}</div>
            </div>

            <div class="actions">
              <button class="act" type="button" :aria-label="item.is_liked ? `取消点赞，当前 ${item.likes_count} 赞` : `点赞，当前 ${item.likes_count} 赞`" :disabled="!!likeBusy[String(item.id)]" @click.stop="toggleLike(item)">
                <span class="icon" :class="{ liked: item.is_liked }" aria-hidden="true">
                  <svg viewBox="0 0 24 24">
                    <path d="M12 21s-7.2-4.7-9.4-9.2C.9 8.2 2.8 4.5 6.6 4.5c2 0 3.5 1 4.4 2.3.9-1.3 2.4-2.3 4.4-2.3 3.8 0 5.7 3.7 4 7.3C19.2 16.3 12 21 12 21Z" />
                  </svg>
                </span>
                <span class="count">{{ item.likes_count }}</span>
              </button>

              <button class="act" type="button" aria-label="查看评论" @click.stop="openComments(item)">
                <span class="icon" aria-hidden="true">
                  <svg viewBox="0 0 24 24">
                    <path d="M5 5.5A3.5 3.5 0 0 1 8.5 2h7A3.5 3.5 0 0 1 19 5.5v5A3.5 3.5 0 0 1 15.5 14H11l-5.2 4.1A.5.5 0 0 1 5 17.7V5.5Z" />
                  </svg>
                </span>
                <span class="count">评论</span>
              </button>

              <button
                v-if="!myAccountId || myAccountId !== item.author.id"
                class="act"
                type="button"
                :aria-label="social.isFollowing(item.author.id) ? `取消关注 ${item.author.username}` : `关注 ${item.author.username}`"
                :disabled="!!followBusy[String(item.author.id)]"
                @click.stop="toggleFollow(item.author.id)"
              >
                <span class="icon" aria-hidden="true">
                  <svg viewBox="0 0 24 24">
                    <path d="M11 5a1 1 0 1 1 2 0v6h6a1 1 0 1 1 0 2h-6v6a1 1 0 1 1-2 0v-6H5a1 1 0 1 1 0-2h6V5Z" />
                  </svg>
                </span>
                <span class="count">{{ social.isFollowing(item.author.id) ? '已关注' : '关注' }}</span>
              </button>

              <button class="act" type="button" aria-label="分享视频" @click.stop="share(item)">
                <span class="icon" aria-hidden="true">
                  <svg viewBox="0 0 24 24">
                    <path d="M14 4h5a1 1 0 0 1 1 1v5a1 1 0 1 1-2 0V7.4l-8.3 8.3a1 1 0 0 1-1.4-1.4L16.6 6H14a1 1 0 1 1 0-2Z" />
                    <path d="M5 6a2 2 0 0 1 2-2h3a1 1 0 1 1 0 2H7v11h11v-3a1 1 0 1 1 2 0v3a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6Z" />
                  </svg>
                </span>
                <span class="count">分享</span>
              </button>
            </div>

            <div class="hint">
              <span class="hint-pill"><span>↑↓</span>切换</span>
              <span class="hint-pill"><span>Space</span>暂停</span>
              <span class="hint-pill"><span>M</span>切换声音</span>
              <span class="hint-pill"><span>C</span>评论</span>
            </div>
          </div>
        </section>
      </div>

      <div v-if="drawer.open" class="drawer-backdrop" @click.self="closeDrawer">
        <div class="drawer" role="dialog" aria-modal="true" aria-labelledby="home-comments-title">
          <div class="drawer-head">
            <div>
              <span class="drawer-kicker">COMMENTS</span>
              <div id="home-comments-title" class="drawer-title">评论 <b>{{ drawer.comments.length }}</b></div>
              <p>{{ drawer.video?.title ?? '视频评论' }}</p>
            </div>
            <button class="drawer-x" type="button" aria-label="关闭评论" @click="closeDrawer">×</button>
          </div>

          <div class="drawer-body">
            <div v-if="drawer.loading" class="drawer-hint">加载中…</div>
            <div v-else-if="drawer.error" class="drawer-hint bad">{{ drawer.error }}</div>
            <div v-else-if="drawer.comments.length === 0" class="drawer-hint">暂无评论</div>

            <div class="comment" v-for="c in drawer.comments" :key="c.id">
              <UserAvatar :username="c.username" :id="c.author_id" :size="36" />
              <div class="comment-main">
                <div class="comment-top">
                  <div class="comment-user">@{{ c.username }}</div>
                  <div class="comment-meta">{{ new Date(c.created_at).toLocaleString() }}</div>
                </div>
                <div class="comment-content">{{ c.content }}</div>
                <div class="comment-actions">
                  <button v-if="canDeleteComment(c)" class="comment-action danger" type="button" :disabled="drawer.loading" @click="deleteComment(c.id)">
                    删除
                  </button>
                </div>
              </div>
            </div>
          </div>

          <div class="drawer-foot">
            <UserAvatar :username="auth.claims?.username ?? 'User'" :id="myAccountId" :size="34" />
            <div class="comment-composer">
              <textarea ref="commentInput" v-model="drawer.content" aria-label="评论内容" maxlength="300" placeholder="留下你的评论" :disabled="drawer.loading" @keydown.esc.prevent="closeDrawer" />
              <button class="comment-action primary" type="button" :disabled="drawer.loading || !drawer.content.trim()" @click="publishComment">发送</button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppShell>
</template>

<style scoped>
.page { height: 100%; display: flex; flex-direction: column; background: var(--surface-base); }

.tabs {
  height: 56px; display: flex; align-items: center; justify-content: center; gap: 28px;
  padding: 0 18px; border-bottom: 1px solid var(--border);
  background: rgba(17, 17, 20, .9); backdrop-filter: blur(16px);
}
.tab {
  position: relative; border: 0; background: transparent; color: var(--text-secondary);
  border-radius: 0; padding: 17px 3px 15px; cursor: pointer; font-weight: 600; font-size: 15px;
}
.tab:hover { background: transparent; color: var(--text); }
.tab.on {
  color: #fff; background: transparent;
}
.tab.on::after { content: ''; position: absolute; right: 3px; bottom: 7px; left: 3px; height: 2px; border-radius: 2px; background: var(--accent); }
.tabs-right { margin-left: auto; display: flex; gap: 8px; align-items: center; }
.top-chip {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 7px 12px; border-radius: 999px; border: 1px solid var(--border);
  background: var(--surface-raised); color: var(--text-secondary);
  font-size: 13px; text-decoration: none;
}
.top-chip:hover { background: var(--surface-hover); color: var(--text); }

.scroller {
  flex: 1; min-height: 0; overflow-y: auto;
  scroll-snap-type: y mandatory; scroll-behavior: smooth;
  overscroll-behavior: contain;
  scrollbar-width: none;
}
.scroller::-webkit-scrollbar { width:0; height:0; }

.center-hint {
  height: calc(100% - 60px); display: grid; place-items: center;
  color: var(--text-muted); text-align: center;
}
.center-hint.bad { color: var(--danger); }
.empty-title { font-size: clamp(24px,4vw,40px); font-weight: 900; }
.empty-sub { margin-top: 6px; color: var(--text-muted); }
.empty-link {
  margin-top: 16px; display: inline-flex; padding: 10px 20px; border-radius: 999px;
  background: var(--accent); color: #fff; text-decoration: none; font-weight: 700;
}

.slide { height: 100%; scroll-snap-align: start; scroll-snap-stop: always; padding: 18px 14px; display: grid; place-items: center; }
.stage {
  width: min(1120px, calc(100% - 28px));
  height: calc(100dvh - 64px - 56px - 36px);
  position: relative; border-radius: var(--radius-xl); overflow: hidden;
  border: 1px solid var(--border); background: #050506;
  box-shadow: 0 24px 64px rgba(0, 0, 0, .28);
}
.video { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: contain; background: #050506; }
.media-status,
.media-error {
  position: absolute;
  z-index: 5;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  display: flex;
  align-items: center;
  gap: 10px;
  border: 1px solid rgba(255, 255, 255, .15);
  border-radius: 999px;
  background: rgba(9, 9, 11, .74);
  color: rgba(255, 255, 255, .86);
  backdrop-filter: blur(14px);
  font-size: 12px;
}
.media-status { padding: 10px 15px; pointer-events: none; }
.media-error { max-width: min(360px, calc(100% - 40px)); padding: 10px 12px 10px 16px; text-align: left; }
.media-error b { padding: 5px 9px; border-radius: 999px; background: #fff; color: #111; font-size: 10px; white-space: nowrap; }
.media-spinner { width: 14px; height: 14px; border: 2px solid rgba(255,255,255,.24); border-top-color: #fff; border-radius: 50%; animation: media-spin .75s linear infinite; }
@keyframes media-spin { to { transform: rotate(360deg); } }
.pause-indicator {
  position: absolute; z-index: 4; top: 50%; left: 50%; width: 68px; height: 68px;
  padding: 0; transform: translate(-50%, -50%); border: 1px solid rgba(255,255,255,.2);
  border-radius: 50%; display: grid; place-items: center;
  background: rgba(0,0,0,.46); backdrop-filter: blur(10px);
}
.pause-indicator:hover { background: rgba(0,0,0,.62); }
.pause-indicator svg { width: 29px; margin-left: 3px; stroke: #fff; stroke-width: 1.7; stroke-linecap: round; stroke-linejoin: round; }
.grad {
  position: absolute; inset: 0; pointer-events: none;
  background: linear-gradient(to top, rgba(0,0,0,0.75), transparent 55%);
}
.meta { position: absolute; z-index: 2; left: 18px; bottom: 20px; max-width: min(600px, calc(100% - 90px)); }
.author-link { display: inline-flex; align-items: center; gap: 8px; font-weight: 800; margin-bottom: 4px; }
.author-name { text-shadow: 0 10px 20px rgba(0,0,0,0.6); font-weight: 900; }
.title { max-width: 720px; overflow: hidden; display: -webkit-box; font-size: clamp(20px,2.5vw,38px); font-weight: 900; margin-bottom: 6px; text-shadow: 0 10px 30px rgba(0,0,0,0.6); -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.desc { max-width: 680px; overflow: hidden; display: -webkit-box; color: rgba(255,255,255,0.74); font-size: 13px; line-height: 1.55; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }

.actions { position: absolute; z-index: 2; right: 14px; bottom: 20px; display: grid; gap: 10px; }
.act {
  width: 64px; min-height: 60px; border-radius: 18px; border: 1px solid rgba(255,255,255,0.14);
  background: rgba(0,0,0,0.5); backdrop-filter: blur(10px);
  color: rgba(255,255,255,0.9); padding: 10px 8px; cursor: pointer;
  display: grid; gap: 4px; justify-items: center;
}
.act:hover { background: rgba(255,255,255,0.1); }
.act:disabled { opacity: 0.5; cursor: not-allowed; }
.icon { width: 20px; height: 20px; display: grid; place-items: center; }
.icon svg { width: 20px; height: 20px; fill: currentColor; }
.icon.liked { color: var(--accent); }
.count { font-size: 11px; color: rgba(255,255,255,0.75); }
.hint { position: absolute; z-index: 2; left: 14px; top: 14px; display: flex; gap: 6px; flex-wrap: wrap; }
.hint-pill {
  display: inline-flex; align-items: center; gap: 5px;
  padding: 5px 8px; border-radius: 999px; border: 1px solid rgba(255,255,255,0.12);
  background: rgba(0,0,0,0.4); color: rgba(255,255,255,0.7);
  font-size: 11px; font-weight: 700; backdrop-filter: blur(10px);
}
.hint-pill span { color: var(--accent); }

.drawer-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,.48); backdrop-filter: blur(5px); z-index: 120; display: grid; justify-items: end; }
.drawer {
  width: min(440px, calc(100vw - 16px)); height: 100dvh;
  background: var(--surface-panel); border-left: 1px solid rgba(255,255,255,.1);
  display: grid; grid-template-rows: auto 1fr auto;
  box-shadow: -24px 0 60px rgba(0,0,0,.34);
}
.drawer-head { min-height: 98px; display: flex; align-items: flex-start; justify-content: space-between; padding: 21px 20px 16px; border-bottom: 1px solid var(--border); }
.drawer-kicker { color: #fe2c55; font-size: 8px; font-weight: 900; letter-spacing: .18em; }
.drawer-title { margin-top: 5px; font-weight: 800; font-size: 18px; }
.drawer-title b { margin-left: 3px; color: #777; font-size: 11px; font-weight: 500; }
.drawer-head p { max-width: 320px; margin-top: 5px; overflow: hidden; color: #777; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.drawer-x {
  width: 32px; height: 32px; padding: 0; border-radius: 50%; border: 1px solid var(--border);
  background: var(--surface-raised); color: var(--text-secondary); cursor: pointer; display: grid; place-items: center;
}
.drawer-x:hover { background: rgba(255,255,255,0.1); color: var(--text); }
.drawer-body { overflow: auto; padding: 8px 20px 18px; display: block; }
.drawer-foot { min-height: 82px; border-top: 1px solid var(--border); padding: 14px 16px; display: flex; align-items: flex-start; gap: 10px; background: var(--surface-raised); }
.comment-composer { flex: 1; position: relative; }
.drawer-foot textarea {
  width: 100%; min-height: 48px; max-height: 110px; resize: none; border-radius: 8px;
  border: 1px solid transparent; background: var(--surface-hover); color: var(--text);
  padding: 12px 62px 10px 12px; outline: none; font: inherit; font-size: 12px;
}
.drawer-foot textarea:focus { border-color: var(--accent); }
.comment-composer .comment-action.primary { position: absolute; right: 7px; bottom: 8px; padding: 6px 10px; }
.drawer-hint { color: var(--text-muted); padding: 70px 0; text-align: center; font-size: 12px; }
.drawer-hint.bad { color: var(--danger); }

.comment {
  padding: 16px 0; border-bottom: 1px solid rgba(255,255,255,.06);
  display: grid; grid-template-columns: auto 1fr; gap: 10px; background: transparent;
}
.comment-main { min-width: 0; }
.comment-top { display: flex; align-items: baseline; justify-content: space-between; gap: 10px; }
.comment-user { font-weight: 600; font-size: 12px; color: #aaa; }
.comment-meta { flex: 0 0 auto; font-size: 9px; color: #555; }
.comment-content { margin-top: 7px; font-size: 13px; line-height: 1.65; color: #eee; white-space: pre-wrap; word-break: break-word; }
.comment-actions { margin-top: 7px; display: flex; justify-content: flex-start; }
.comment-action {
  border: 0; background: transparent; color: var(--text-secondary);
  border-radius: 5px; padding: 4px 7px; font-size: 10px; font-weight: 700; cursor: pointer;
}
.comment-action:hover:not(:disabled) { background: rgba(255,255,255,0.06); color: var(--text); }
.comment-action.primary { background: var(--accent); color: #fff; }
.comment-action.danger { color: #777; }
.comment-action.danger:hover:not(:disabled) { color: var(--danger); }
.comment-action:disabled { opacity: 0.5; cursor: not-allowed; }

@media (max-width: 900px) {
  .stage { width: calc(100% - 12px); height: calc(100dvh - 64px - 56px - 28px); border-radius: var(--radius-lg); }
  .drawer-backdrop { justify-items: center; align-items: end; }
  .drawer { width: calc(100vw - 12px); height: min(72dvh, 540px); border-left: none; border-top: 1px solid var(--border); border-radius: var(--radius-lg) var(--radius-lg) 0 0; overflow: hidden; }
}

@media (max-width: 768px) {
  .tabs { height: 52px; padding-inline: 12px; }
  .tab { padding-block: 15px 13px; }
  .top-chip { padding: 6px 9px; font-size: 11px; }
  .stage { height: calc(100dvh - 56px - 52px - 62px - env(safe-area-inset-bottom) - 24px); }
  .hint { display: none; }
  .meta { left: 14px; bottom: 16px; }
  .actions { right: 10px; bottom: 14px; }
}
</style>
