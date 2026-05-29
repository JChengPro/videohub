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
const activeIndex = ref(0)
const videoMap = new Map<number, HTMLVideoElement>()

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
  if (!item) return
  for (const [id, v] of videoMap.entries()) {
    if (id === item.id) continue
    v.pause()
  }
  const video = videoMap.get(item.id)
  if (!video) return
  video.muted = muted.value
  try {
    await video.play()
  } catch {
    // ignore autoplay errors
  }
}

function toggleMute() {
  muted.value = !muted.value
  for (const v of videoMap.values()) v.muted = muted.value
  toast.info(muted.value ? '已静音' : '已取消静音')
}

function togglePlayPause() {
  const item = activeItem.value
  if (!item) return
  const video = videoMap.get(item.id)
  if (!video) return
  if (video.paused) {
    void video.play()
  } else {
    video.pause()
  }
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
    following.error = '登录后才能查看关注流'
    return
  }
  if (following.loading) return
  following.loading = true
  following.error = ''
  try {
    const res = await feedApi.listByFollowing({ limit: 10, latest_time: reset ? 0 : following.nextTime })
    following.hasMore = res.has_more
    following.nextTime = res.next_time
    following.items = reset ? res.video_list : following.items.concat(res.video_list)
  } catch (e) {
    following.error = e instanceof ApiError ? e.message : String(e)
  } finally {
    following.loading = false
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
    if (item.is_liked) await likeApi.unlike(item.id)
    else await likeApi.like(item.id)
    item.is_liked = !item.is_liked
    item.likes_count = Math.max(0, item.likes_count + (item.is_liked ? 1 : -1))
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
    await navigator.clipboard.writeText(url)
    toast.success('链接已复制')
  } catch {
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
  drawer.open = false
  drawer.video = null
  drawer.comments = []
  drawer.content = ''
  drawer.error = ''
}

async function focusCommentInput() {
  await nextTick()
  commentInput.value?.focus()
}

async function openComments(item: FeedVideoItem) {
  drawer.open = true
  drawer.video = item
  drawer.content = ''
  await loadComments()
  await focusCommentInput()
}

async function loadComments() {
  if (!drawer.video) return
  drawer.loading = true
  drawer.error = ''
  try {
    drawer.comments = await commentApi.listAll(drawer.video.id)
  } catch (e) {
    drawer.error = e instanceof ApiError ? e.message : String(e)
  } finally {
    drawer.loading = false
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
  () => filteredItems.value.length,
  (len) => {
    if (len === 0) activeIndex.value = 0
    else if (activeIndex.value > len - 1) activeIndex.value = len - 1
  },
)

watch(
  () => auth.isLoggedIn,
  async (v) => {
    if (tab.value === 'following' && v && following.items.length === 0) {
      await loadFollowing(true)
    }
  },
)

onMounted(async () => {
  await ensureTabLoaded()
  await nextTick()
  await playActive()
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <AppShell full>
    <div class="page">
      <div class="tabs">
        <button class="tab" :class="{ on: tab === 'recommend' }" type="button" @click="tab = 'recommend'">推荐</button>
        <button class="tab" :class="{ on: tab === 'following' }" type="button" @click="tab = 'following'">关注</button>
        <button class="tab" :class="{ on: tab === 'hot' }" type="button" @click="tab = 'hot'">热度榜</button>

        <div class="tabs-right">
          <button class="top-chip" type="button" @click="toggleMute">{{ muted ? '静音' : '有声' }}</button>
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
          <div class="stage" @click="togglePlayPause" @dblclick.prevent="toggleLike(item)">
            <video
              class="video"
              :ref="(el) => setVideoRef(item.id, el as HTMLVideoElement | null)"
              :src="item.play_url"
              :poster="item.cover_url"
              playsinline
              preload="metadata"
              loop
            />
            <div class="grad" />

            <div class="meta">
              <RouterLink class="author-link" :to="`/u/${item.author.id}`" @click.stop>
                <UserAvatar :username="item.author.username" :id="item.author.id" :size="34" />
                <span class="author-name">@{{ item.author.username }}</span>
              </RouterLink>
              <div class="title">{{ item.title }}</div>
              <div v-if="item.description" class="desc">{{ item.description }}</div>
            </div>

            <div class="actions">
              <button class="act" type="button" :disabled="!!likeBusy[String(item.id)]" @click.stop="toggleLike(item)">
                <span class="icon" :class="{ liked: item.is_liked }" aria-hidden="true">
                  <svg viewBox="0 0 24 24">
                    <path d="M12 21s-7.2-4.7-9.4-9.2C.9 8.2 2.8 4.5 6.6 4.5c2 0 3.5 1 4.4 2.3.9-1.3 2.4-2.3 4.4-2.3 3.8 0 5.7 3.7 4 7.3C19.2 16.3 12 21 12 21Z" />
                  </svg>
                </span>
                <span class="count">{{ item.likes_count }}</span>
              </button>

              <button class="act" type="button" @click.stop="openComments(item)">
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

              <button class="act" type="button" @click.stop="share(item)">
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
              <span class="hint-pill"><span>M</span>静音</span>
              <span class="hint-pill"><span>C</span>评论</span>
            </div>
          </div>
        </section>
      </div>

      <div v-if="drawer.open" class="drawer-backdrop" @click.self="closeDrawer">
        <div class="drawer">
          <div class="drawer-head">
            <div class="drawer-title">{{ drawer.video?.title ?? '评论' }}</div>
            <button class="drawer-x" type="button" aria-label="关闭评论" @click="closeDrawer">×</button>
          </div>

          <div class="drawer-body">
            <div v-if="drawer.loading" class="drawer-hint">加载中…</div>
            <div v-else-if="drawer.error" class="drawer-hint bad">{{ drawer.error }}</div>
            <div v-else-if="drawer.comments.length === 0" class="drawer-hint">暂无评论</div>

            <div class="comment" v-for="c in drawer.comments" :key="c.id">
              <div class="comment-top">
                <div class="comment-user">{{ c.username }}</div>
                <div class="comment-meta">
                  #{{ c.id }} · {{ new Date(c.created_at).toLocaleString() }}
                </div>
              </div>
              <div class="comment-content">{{ c.content }}</div>
              <div class="comment-actions">
                <button v-if="canDeleteComment(c)" class="comment-action danger" type="button" :disabled="drawer.loading" @click="deleteComment(c.id)">
                  删除
                </button>
              </div>
            </div>
          </div>

          <div class="drawer-foot">
            <textarea ref="commentInput" v-model="drawer.content" placeholder="说点什么…" :disabled="drawer.loading" @keydown.esc.prevent="closeDrawer" />
            <div class="drawer-actions">
              <button class="comment-action" type="button" :disabled="drawer.loading" @click="loadComments">刷新</button>
              <button class="comment-action primary" type="button" :disabled="drawer.loading || !drawer.content.trim()" @click="publishComment">
                发送
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppShell>
</template>

<style scoped>
.page { height: 100%; display: flex; flex-direction: column; background: var(--bg); }

.tabs {
  height: 56px; display: flex; align-items: center; gap: 6px;
  padding: 0 18px; border-bottom: 1px solid var(--border);
  background: rgba(0,0,0,0.4); backdrop-filter: blur(14px);
}
.tab {
  border: 1px solid var(--border); background: transparent; color: var(--text-secondary);
  border-radius: 999px; padding: 7px 14px; cursor: pointer; font-weight: 600; font-size: 14px;
}
.tab.on {
  color: #fff; border-color: var(--accent); background: var(--accent);
}
.tabs-right { margin-left: auto; display: flex; gap: 8px; align-items: center; }
.top-chip {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 7px 12px; border-radius: 999px; border: 1px solid var(--border);
  background: rgba(255,255,255,0.05); color: var(--text-secondary);
  font-size: 13px; text-decoration: none;
}
.top-chip:hover { background: rgba(255,255,255,0.1); color: var(--text); }

.scroller {
  flex: 1; min-height: 0; overflow-y: auto;
  scroll-snap-type: y mandatory; scroll-behavior: smooth;
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

.slide { height: 100%; scroll-snap-align: start; padding: 18px 14px; display: grid; place-items: center; }
.stage {
  width: min(1040px, calc(100vw - 28px));
  height: calc(100vh - 56px - 56px - 36px);
  position: relative; border-radius: 20px; overflow: hidden;
  border: 1px solid var(--border); background: #000;
}
.video { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; }
.grad {
  position: absolute; inset: 0; pointer-events: none;
  background: linear-gradient(to top, rgba(0,0,0,0.75), transparent 55%);
}
.meta { position: absolute; z-index: 2; left: 18px; bottom: 20px; max-width: min(600px, calc(100% - 90px)); }
.author-link { display: inline-flex; align-items: center; gap: 8px; font-weight: 800; margin-bottom: 4px; }
.author-name { text-shadow: 0 10px 20px rgba(0,0,0,0.6); font-weight: 900; }
.title { font-size: clamp(20px,2.5vw,38px); font-weight: 900; margin-bottom: 6px; text-shadow: 0 10px 30px rgba(0,0,0,0.6); }
.desc { color: rgba(255,255,255,0.7); font-size: 13px; }

.actions { position: absolute; z-index: 2; right: 14px; bottom: 20px; display: grid; gap: 10px; }
.act {
  width: 64px; border-radius: 16px; border: 1px solid rgba(255,255,255,0.14);
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

.drawer-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,0.5); backdrop-filter: blur(8px); z-index: 120; display: grid; justify-items: end; }
.drawer {
  width: min(400px, calc(100vw - 16px)); height: 100vh;
  background: rgba(10,10,10,0.95); border-left: 1px solid var(--border);
  display: grid; grid-template-rows: auto 1fr auto;
}
.drawer-head { display: flex; align-items: center; justify-content: space-between; padding: 12px 14px; border-bottom: 1px solid var(--border); }
.drawer-title { font-weight: 800; font-size: 14px; }
.drawer-x {
  width: 32px; height: 32px; border-radius: 8px; border: 1px solid var(--border);
  background: transparent; color: var(--text-secondary); cursor: pointer; display: grid; place-items: center;
}
.drawer-x:hover { background: rgba(255,255,255,0.1); color: var(--text); }
.drawer-body { overflow: auto; padding: 12px 14px; display: grid; gap: 8px; }
.drawer-foot { border-top: 1px solid var(--border); padding: 10px 14px; }
.drawer-foot textarea {
  width: 100%; min-height: 72px; resize: none; border-radius: 12px;
  border: 1px solid var(--border); background: var(--bg-input); color: var(--text);
  padding: 10px 12px; outline: none; font: inherit;
}
.drawer-foot textarea:focus { border-color: var(--accent); }
.drawer-actions { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-top: 8px; }
.drawer-hint { color: var(--text-muted); padding: 10px 0; }
.drawer-hint.bad { color: var(--danger); }

.comment {
  border: 1px solid var(--border); background: var(--bg-card);
  border-radius: 12px; padding: 10px 12px;
}
.comment-top { display: grid; gap: 2px; }
.comment-user { font-weight: 700; font-size: 13px; }
.comment-meta { font-size: 11px; color: var(--text-muted); }
.comment-content { margin-top: 6px; font-size: 14px; color: var(--text); white-space: pre-wrap; word-break: break-word; }
.comment-actions { margin-top: 8px; display: flex; justify-content: flex-end; }
.comment-action {
  border: 1px solid var(--border); background: transparent; color: var(--text-secondary);
  border-radius: 999px; padding: 5px 12px; font-size: 12px; font-weight: 700; cursor: pointer;
}
.comment-action:hover:not(:disabled) { background: rgba(255,255,255,0.06); color: var(--text); }
.comment-action.primary { background: var(--accent); color: #fff; border-color: var(--accent); }
.comment-action.danger { border-color: rgba(254,44,85,0.35); color: var(--danger); }
.comment-action:disabled { opacity: 0.5; cursor: not-allowed; }

@media (max-width: 900px) {
  .stage { width: calc(100vw - 20px); height: calc(100vh - 56px - 56px - 28px); border-radius: 14px; }
  .drawer-backdrop { justify-items: center; align-items: end; }
  .drawer { width: calc(100vw - 12px); height: min(70vh, 500px); border-left: none; border-top: 1px solid var(--border); border-radius: 14px 14px 0 0; overflow: hidden; }
}
</style>
