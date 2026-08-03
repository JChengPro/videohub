<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import AppIcon from '../components/AppIcon.vue'
import UserAvatar from '../components/UserAvatar.vue'
import { ApiError } from '../api/client'
import * as commentApi from '../api/comment'
import * as likeApi from '../api/like'
import type { Comment, Video } from '../api/types'
import * as videoApi from '../api/video'
import { useAuthStore } from '../stores/auth'
import { useSocialStore } from '../stores/social'
import { useToastStore } from '../stores/toast'
import { useDialogStore } from '../stores/dialog'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const social = useSocialStore()
const toast = useToastStore()
const dialog = useDialogStore()

const id = computed(() => Number(route.params.id))
const isOwner = computed(() => !!state.video && auth.claims?.account_id === state.video.author_id)

const state = reactive({
  loading: false,
  error: '',
  video: null as Video | null,
  isLiked: null as boolean | null,
  busy: false,
})

const muted = ref(true)
const paused = ref(true)
const mediaLoading = ref(false)
const playbackError = ref('')
const videoEl = ref<HTMLVideoElement | null>(null)
const commentInput = ref<HTMLTextAreaElement | null>(null)
let videoRequest = 0
let likeRequest = 0
let commentRequest = 0
let resumeAfterVisibility = false
let resumeAfterDrawer = false
let drawerTrigger: HTMLElement | null = null

const drawer = reactive({
  open: false,
  loading: false,
  error: '',
  comments: [] as Comment[],
  content: '',
})

async function needLogin() {
  toast.error('请先登录')
  await router.push('/account')
}

async function loadVideo() {
  const videoId = id.value
  const request = ++videoRequest
  state.video = null
  if (!Number.isFinite(videoId) || videoId <= 0) {
    state.loading = false
    state.error = '无效的 video id'
    return
  }
  state.loading = true
  state.error = ''
  try {
    const video = await videoApi.getDetail(videoId)
    if (request === videoRequest && videoId === id.value) state.video = video
  } catch (e) {
    if (request === videoRequest) state.error = e instanceof ApiError ? e.message : String(e)
  } finally {
    if (request === videoRequest) state.loading = false
  }
}

async function loadIsLiked() {
  const request = ++likeRequest
  const videoId = id.value
  if (!auth.isLoggedIn) {
    state.isLiked = null
    return
  }
  try {
    const res = await likeApi.isLiked(videoId)
    if (request === likeRequest && videoId === id.value) state.isLiked = res.is_liked
  } catch {
    if (request === likeRequest) state.isLiked = null
  }
}

async function play() {
  if (!videoEl.value || document.hidden || drawer.open) return
  videoEl.value.muted = muted.value
  playbackError.value = ''
  try {
    await videoEl.value.play()
    paused.value = false
  } catch {
    paused.value = true
    playbackError.value = '浏览器阻止了自动播放，点击继续'
  }
}

function toggleMute() {
  muted.value = !muted.value
  if (videoEl.value) videoEl.value.muted = muted.value
  toast.info(muted.value ? '声音已关闭' : '声音已开启')
}

async function togglePlayPause() {
  const v = videoEl.value
  if (!v) return
  if (v.paused) {
    playbackError.value = ''
    try {
      await v.play()
      paused.value = false
    } catch {
      paused.value = true
      playbackError.value = '视频暂时无法播放，请重试'
    }
  } else {
    v.pause()
    paused.value = true
  }
}

function onVideoPlaying() {
  mediaLoading.value = false
  playbackError.value = ''
  paused.value = false
}

function onVideoPause() {
  paused.value = true
}

function onVideoError() {
  mediaLoading.value = false
  paused.value = true
  playbackError.value = '视频加载失败，点击重试'
}

async function retryPlayback() {
  if (!videoEl.value) return
  videoEl.value.load()
  await togglePlayPause()
}

async function toggleLike() {
  if (!state.video) return
  if (!auth.isLoggedIn) return needLogin()
  if (state.busy) return

  state.busy = true
  try {
    if (state.isLiked) {
      const result = await likeApi.unlike(id.value)
      state.isLiked = result.is_liked
      state.video.likes_count = Math.max(0, result.likes_count)
    } else {
      const result = await likeApi.like(id.value)
      state.isLiked = result.is_liked
      state.video.likes_count = Math.max(0, result.likes_count)
    }
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
  } finally {
    state.busy = false
  }
}

async function toggleFollow() {
  if (!state.video) return
  if (!auth.isLoggedIn) return needLogin()
  if (state.busy) return
  if (auth.claims?.account_id && auth.claims.account_id === state.video.author_id) return

  state.busy = true
  try {
    if (social.isFollowing(state.video.author_id)) {
      await social.unfollow(state.video.author_id)
      toast.info('已取关')
    } else {
      await social.follow(state.video.author_id)
      toast.success('已关注')
    }
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
  } finally {
    state.busy = false
  }
}

async function share() {
  if (!state.video) return
  const url = `${location.origin}/video/${state.video.id}`
  try {
    if (navigator.share) {
      await navigator.share({ title: state.video.title, url })
      return
    }
    await navigator.clipboard.writeText(url)
    toast.success('链接已复制')
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') return
    toast.error('暂时无法自动复制，请从浏览器地址栏复制视频链接')
  }
}

async function deleteVideo() {
  if (!state.video) return
  if (!auth.isLoggedIn) return needLogin()
  if (!isOwner.value) {
    toast.error('无权限删除此视频')
    return
  }
  if (state.busy) return
  if (!await dialog.ask({
    title: '删除这个视频？',
    message: '删除后作品将不再公开展示，相关点赞和评论也会一并移除。',
    confirmLabel: '删除视频',
    tone: 'danger',
  })) return

  state.busy = true
  try {
    await videoApi.deleteVideo(state.video.id)
    closeDrawer()
    toast.info('视频已删除')
    await router.replace('/account')
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
  } finally {
    state.busy = false
  }
}

function closeDrawer() {
  commentRequest += 1
  drawer.open = false
  drawer.comments = []
  drawer.content = ''
  drawer.error = ''
  if (resumeAfterDrawer && !document.hidden) void play()
  else paused.value = videoEl.value?.paused ?? true
  resumeAfterDrawer = false
  drawerTrigger?.focus()
  drawerTrigger = null
}

async function focusCommentInput() {
  await nextTick()
  commentInput.value?.focus()
}

async function loadComments() {
  if (!state.video) return
  const videoId = state.video.id
  const request = ++commentRequest
  drawer.loading = true
  drawer.error = ''
  try {
    const comments = await commentApi.listAll(videoId)
    if (request === commentRequest && drawer.open && state.video?.id === videoId) {
      drawer.comments = comments
      state.video.comments_count = comments.length
    }
  } catch (e) {
    if (request === commentRequest && drawer.open) drawer.error = e instanceof ApiError ? e.message : String(e)
  } finally {
    if (request === commentRequest) drawer.loading = false
  }
}

async function openComments() {
  resumeAfterDrawer = !!videoEl.value && !videoEl.value.paused
  drawerTrigger = document.activeElement instanceof HTMLElement ? document.activeElement : null
  videoEl.value?.pause()
  drawer.open = true
  drawer.content = ''
  await loadComments()
  await focusCommentInput()
}

async function publishComment() {
  if (!state.video) return
  if (!auth.isLoggedIn) return needLogin()
  const content = drawer.content.trim()
  if (!content) return

  drawer.loading = true
  drawer.error = ''
  try {
    await commentApi.publish(state.video.id, content)
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

function formatCommentTime(value: string) {
  const time = new Date(value).getTime()
  if (!Number.isFinite(time)) return ''
  const seconds = Math.max(0, Math.floor((Date.now() - time) / 1000))
  if (seconds < 60) return '刚刚'
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟前`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)} 小时前`
  if (seconds < 604800) return `${Math.floor(seconds / 86400)} 天前`
  return new Date(time).toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })
}

async function deleteComment(commentId: number) {
  if (!state.video) return
  if (!auth.isLoggedIn) return needLogin()
  if (!await dialog.ask({
    title: '删除这条评论？',
    message: '评论删除后无法恢复。',
    confirmLabel: '删除评论',
    tone: 'danger',
  })) return

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
    else if (state.video) await openComments()
    return
  }

  if (drawer.open) {
    if (e.key === 'Escape') {
      e.preventDefault()
      closeDrawer()
    }
    return
  }

  if (e.key === ' ') {
    e.preventDefault()
    togglePlayPause()
  } else if (e.key.toLowerCase() === 'm') {
    e.preventDefault()
    toggleMute()
  }
}

function onVisibilityChange() {
  if (document.hidden) {
    resumeAfterVisibility = !!videoEl.value && !videoEl.value.paused && !drawer.open
    videoEl.value?.pause()
    return
  }
  if (resumeAfterVisibility && !drawer.open) void play()
  resumeAfterVisibility = false
}

watch(
  () => id.value,
  async () => {
    closeDrawer()
    await loadVideo()
    await loadIsLiked()
    await nextTick()
    await play()
  },
)

watch(
  () => auth.isLoggedIn,
  async () => {
    await loadIsLiked()
  },
)

onMounted(async () => {
  await loadVideo()
  await loadIsLiked()
  await nextTick()
  await play()
  window.addEventListener('keydown', onKeydown)
  document.addEventListener('visibilitychange', onVisibilityChange)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  document.removeEventListener('visibilitychange', onVisibilityChange)
  videoRequest += 1
  likeRequest += 1
  commentRequest += 1
  videoEl.value?.pause()
})
</script>

<template>
  <AppShell full>
    <div class="page">
      <div class="top">
        <div class="top-left">
          <RouterLink class="top-chip" to="/">返回推荐</RouterLink>
        </div>
        <div class="top-right">
          <button
            class="top-chip"
            type="button"
            :aria-label="muted ? '当前静音，点击开启声音' : '当前有声，点击关闭声音'"
            :title="muted ? '当前静音' : '当前有声'"
            @click="toggleMute"
          >
            {{ muted ? '开启声音' : '关闭声音' }}
          </button>
        </div>
      </div>

      <div class="wrap">
        <div v-if="state.loading" class="center-hint">加载中…</div>
        <div v-else-if="state.error" class="center-hint bad">{{ state.error }}</div>

        <div v-else-if="state.video" class="stage" @click="togglePlayPause" @dblclick.prevent="toggleLike">
          <video
            ref="videoEl"
            class="video"
            :src="state.video.play_url"
            :poster="state.video.cover_url"
            playsinline
            preload="metadata"
            loop
            @playing="onVideoPlaying"
            @pause="onVideoPause"
            @waiting="mediaLoading = true"
            @canplay="mediaLoading = false"
            @error="onVideoError"
          />
          <div class="grad" />
          <div v-if="mediaLoading && !playbackError" class="media-status" role="status"><span class="media-spinner" />正在缓冲</div>
          <button v-if="playbackError" class="media-error" type="button" @click.stop="retryPlayback"><span>{{ playbackError }}</span><b>重试</b></button>
          <button v-if="paused && !mediaLoading && !playbackError" class="pause-indicator" type="button" aria-label="继续播放" @click.stop="togglePlayPause">
            <AppIcon name="play" :size="29" />
          </button>

          <div class="meta">
            <RouterLink class="author-link" :to="`/u/${state.video.author_id}`" @click.stop>
              <UserAvatar :username="state.video.username" :id="state.video.author_id" :size="34" />
              <span class="author-name">{{ state.video.username }}</span>
            </RouterLink>
            <div class="title">{{ state.video.title }}</div>
            <div v-if="state.video.description" class="desc">{{ state.video.description }}</div>
            <div class="row" style="margin-top: 10px">
              <a class="asset-link" :href="state.video.play_url" target="_blank" rel="noreferrer">播放地址</a>
              <a class="asset-link" :href="state.video.cover_url" target="_blank" rel="noreferrer">封面地址</a>
            </div>
          </div>

          <div class="actions">
            <button class="act" type="button" :aria-label="state.isLiked ? `取消点赞，当前 ${state.video.likes_count} 赞` : `点赞，当前 ${state.video.likes_count} 赞`" :disabled="state.busy" @click.stop="toggleLike">
              <span class="icon" :class="{ liked: !!state.isLiked }" aria-hidden="true">
                <AppIcon name="heart" :size="20" :filled="!!state.isLiked" />
              </span>
              <span class="count">{{ state.video.likes_count }}</span>
            </button>

            <button class="act" type="button" :aria-label="`查看评论，当前 ${state.video.comments_count} 条`" @click.stop="openComments">
              <span class="icon" aria-hidden="true">
                <AppIcon name="comment" :size="20" />
              </span>
              <span class="count">{{ state.video.comments_count }}</span>
            </button>

            <button
              v-if="!auth.claims?.account_id || auth.claims.account_id !== state.video.author_id"
              class="act"
              type="button"
              :aria-label="social.isFollowing(state.video.author_id) ? `取消关注 ${state.video.username}` : `关注 ${state.video.username}`"
              :disabled="state.busy"
              @click.stop="toggleFollow"
            >
              <span class="icon" aria-hidden="true">
                <AppIcon name="following" :size="20" />
              </span>
              <span class="count">{{ social.isFollowing(state.video.author_id) ? '已关注' : '关注' }}</span>
            </button>

            <button class="act" type="button" aria-label="分享视频" @click.stop="share">
              <span class="icon" aria-hidden="true">
                <AppIcon name="share" :size="20" />
              </span>
              <span class="count">分享</span>
            </button>

            <button v-if="isOwner" class="act act-danger" type="button" aria-label="删除视频" :disabled="state.busy" @click.stop="deleteVideo">
              <span class="icon" aria-hidden="true">
                <AppIcon name="trash" :size="20" />
              </span>
              <span class="count">删除</span>
            </button>
          </div>

          <div class="hint">
            <span class="hint-pill"><span>Click</span>暂停/播放</span>
            <span class="hint-pill"><span>Double</span>点赞</span>
            <span class="hint-pill"><span>C</span>评论</span>
            <span class="hint-pill"><span>Esc</span>关闭</span>
          </div>
        </div>
      </div>

      <div v-if="drawer.open" class="drawer-backdrop" @click.self="closeDrawer">
        <div class="drawer" role="dialog" aria-modal="true" aria-labelledby="detail-comments-title">
          <div class="drawer-head">
            <div>
              <span class="drawer-kicker">COMMENTS</span>
              <div id="detail-comments-title" class="drawer-title">评论 <b>{{ drawer.comments.length }}</b></div>
              <p>{{ state.video?.title ?? '视频评论' }}</p>
            </div>
            <button class="drawer-x" type="button" aria-label="关闭评论" @click="closeDrawer">×</button>
          </div>

          <div class="drawer-body">
            <div v-if="drawer.loading" class="drawer-hint">加载中…</div>
            <div v-else-if="drawer.error" class="drawer-hint bad">{{ drawer.error }}</div>
            <div v-else-if="drawer.comments.length === 0" class="drawer-hint">暂无评论</div>

            <article class="comment" v-for="c in drawer.comments" :key="c.id">
              <RouterLink class="comment-avatar" :to="`/u/${c.author_id}`" :aria-label="`查看 ${c.username} 的主页`" @click="closeDrawer">
                <UserAvatar :username="c.username" :id="c.author_id" :size="40" />
              </RouterLink>
              <div class="comment-main">
                <div class="comment-top">
                  <RouterLink class="comment-user" :to="`/u/${c.author_id}`" @click="closeDrawer">{{ c.username }}</RouterLink>
                  <time class="comment-meta" :datetime="c.created_at">{{ formatCommentTime(c.created_at) }}</time>
                </div>
                <div class="comment-content">{{ c.content }}</div>
                <div class="comment-actions">
                  <button v-if="canDeleteComment(c)" class="comment-action danger" type="button" :disabled="drawer.loading" @click="deleteComment(c.id)">
                    删除
                  </button>
                </div>
              </div>
            </article>
          </div>

          <div class="drawer-foot">
            <UserAvatar :username="auth.claims?.username ?? 'User'" :id="auth.claims?.account_id ?? 0" :size="36" />
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
.page {
  height: 100%;
  display: flex;
  flex-direction: column;
  background:
    radial-gradient(440px 440px at 76% 8%, rgba(255, 255, 255, 0.055), transparent 68%),
    radial-gradient(520px 520px at 14% 88%, rgba(254, 44, 85, 0.1), transparent 70%),
    var(--surface-base);
}

.top {
  height: 58px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 18px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.09);
  background: var(--surface-overlay);
  backdrop-filter: blur(18px);
}

.top-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 9px 13px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.07);
  color: rgba(255, 255, 255, 0.82);
  font-size: 13px;
  font-weight: 800;
  letter-spacing: 0.02em;
  text-decoration: none;
  box-shadow: none;
}

.top-chip:hover {
  text-decoration: none;
}

.wrap {
  flex: 1;
  min-height: 0;
  display: grid;
  place-items: center;
  padding: 22px 18px;
}

.center-hint {
  color: rgba(255, 255, 255, 0.78);
}

.center-hint.bad {
  color: rgba(254, 44, 85, 0.92);
}

.stage {
  width: min(1120px, calc(100% - 36px));
  height: calc(100dvh - 64px - 58px - 44px);
  position: relative;
  border-radius: var(--radius-xl);
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.16);
  background: rgba(0, 0, 0, 0.35);
  box-shadow: 0 34px 120px rgba(0, 0, 0, 0.62), 0 0 0 1px rgba(255, 255, 255, 0.04) inset;
}

.stage::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 1;
  pointer-events: none;
  border-radius: inherit;
  background: linear-gradient(140deg, rgba(255, 255, 255, 0.13), transparent 22%, transparent 70%, rgba(254, 44, 85, 0.08));
  mix-blend-mode: screen;
}

.video {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: contain;
  background: rgba(0, 0, 0, 0.4);
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
  gap: 10px;
  border: 1px solid rgba(255, 255, 255, .15);
  border-radius: 999px;
  background: rgba(9, 9, 11, .78);
  color: rgba(255, 255, 255, .86);
  backdrop-filter: blur(14px);
  font-size: 12px;
}

.media-status {
  padding: 10px 15px;
  pointer-events: none;
}

.media-error {
  max-width: min(360px, calc(100% - 40px));
  padding: 10px 12px 10px 16px;
  text-align: left;
}

.media-error b {
  padding: 5px 9px;
  border-radius: 999px;
  background: #fff;
  color: #111;
  font-size: 10px;
  white-space: nowrap;
}

.media-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255, 255, 255, .24);
  border-top-color: #fff;
  border-radius: 50%;
  animation: detail-spin .75s linear infinite;
}

@keyframes detail-spin {
  to { transform: rotate(360deg); }
}

.pause-indicator {
  position: absolute;
  z-index: 4;
  top: 50%;
  left: 50%;
  width: 68px;
  height: 68px;
  padding: 0;
  transform: translate(-50%, -50%);
  border: 1px solid rgba(255, 255, 255, .2);
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: rgba(0, 0, 0, .48);
  backdrop-filter: blur(12px);
}

.pause-indicator svg {
  width: 29px;
  margin-left: 3px;
  stroke: #fff;
  stroke-width: 1.7;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.grad {
  position: absolute;
  inset: 0;
  background:
    linear-gradient(to top, rgba(0, 0, 0, 0.78), rgba(0, 0, 0, 0.18) 42%, rgba(0, 0, 0, 0) 72%),
    linear-gradient(90deg, rgba(0, 0, 0, 0.46), transparent 42%, rgba(0, 0, 0, 0.32));
  pointer-events: none;
}

.meta {
  position: absolute;
  z-index: 2;
  left: 22px;
  bottom: 24px;
  max-width: min(620px, calc(100% - 96px));
}

.author-link {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  font-weight: 800;
  letter-spacing: 0.2px;
  margin-bottom: 6px;
  text-decoration: none;
}

.author-link:hover {
  text-decoration: none;
}

.author-name {
  text-shadow: 0 14px 30px rgba(0, 0, 0, 0.55);
  font-weight: 900;
}

.title {
  overflow: hidden;
  display: -webkit-box;
  font-size: clamp(24px, 3.1vw, 48px);
  line-height: 0.98;
  font-weight: 950;
  letter-spacing: -0.055em;
  margin-bottom: 10px;
  text-shadow: 0 18px 44px rgba(0, 0, 0, 0.58);
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.desc {
  overflow: hidden;
  display: -webkit-box;
  color: rgba(255, 255, 255, 0.74);
  font-size: 14px;
  line-height: 1.45;
  max-width: 58ch;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
}

.asset-link {
  display: inline-flex;
  align-items: center;
  padding: 7px 11px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(34, 34, 37, 0.7);
  color: rgba(255, 255, 255, 0.7);
  font-size: 12px;
  font-weight: 750;
  letter-spacing: 0.01em;
  text-decoration: none;
  backdrop-filter: blur(14px);
}

.asset-link:hover {
  color: rgba(255, 255, 255, 0.92);
  text-decoration: none;
}

.actions {
  position: absolute;
  z-index: 2;
  right: 18px;
  bottom: 24px;
  display: grid;
  gap: 12px;
}

.act {
  width: 74px;
  border-radius: 22px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(34, 34, 37, 0.82);
  backdrop-filter: blur(16px);
  color: rgba(255, 255, 255, 0.92);
  padding: 12px 10px;
  cursor: pointer;
  display: grid;
  gap: 6px;
  justify-items: center;
  box-shadow: 0 18px 44px rgba(0, 0, 0, 0.32);
}

.act:hover {
  background: rgba(255, 255, 255, 0.1);
}

.act:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.act-danger {
  border-color: rgba(254, 44, 85, 0.42);
  background: rgba(254, 44, 85, 0.12);
}

.act-danger:hover {
  background: rgba(254, 44, 85, 0.18);
}

.icon {
  width: 22px;
  height: 22px;
  opacity: 0.92;
  display: grid;
  place-items: center;
  color: currentColor;
}

.icon svg {
  width: 22px;
  height: 22px;
  display: block;
}

.icon.liked {
  color: rgba(254, 44, 85, 1);
  text-shadow: 0 10px 20px rgba(254, 44, 85, 0.25);
}

.count {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.8);
}

.hint {
  position: absolute;
  z-index: 2;
  left: 18px;
  top: 18px;
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.hint-pill {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 7px 10px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(34, 34, 37, 0.7);
  color: rgba(255, 255, 255, 0.68);
  font-size: 12px;
  font-weight: 750;
  letter-spacing: 0.01em;
  backdrop-filter: blur(14px);
}

.hint-pill span {
  color: var(--accent);
  font-size: 11px;
  font-weight: 900;
  letter-spacing: 0.04em;
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(0, 0, 0, 0.34);
  color: rgba(255, 255, 255, 0.86);
  font-size: 12px;
  text-decoration: none;
  backdrop-filter: blur(14px);
}

.chip.primary {
  border-color: rgba(254, 44, 85, 0.45);
  background: rgba(254, 44, 85, 0.14);
}

.chip.danger {
  border-color: rgba(254, 44, 85, 0.55);
  background: rgba(254, 44, 85, 0.12);
}

.drawer-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.55);
  backdrop-filter: blur(10px);
  z-index: 120;
  display: grid;
  justify-items: end;
}

.drawer {
  width: min(480px, calc(100vw - 18px));
  height: 100dvh;
  background: linear-gradient(165deg, #1c1c20, #121215);
  border-left: 1px solid rgba(255, 255, 255, 0.12);
  display: grid;
  grid-template-rows: auto 1fr auto;
}

.drawer-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  min-height: 108px;
  padding: 23px 22px 18px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  background: radial-gradient(circle at 0 0, rgba(255, 59, 92, .09), transparent 48%);
}

.drawer-kicker { color: var(--accent); font-size: 8px; font-weight: 900; letter-spacing: .18em; }
.drawer-title {
  margin-top: 5px;
  font-weight: 800;
  font-size: 18px;
}
.drawer-title b { margin-left: 3px; color: var(--text-muted); font-size: 11px; font-weight: 500; }
.drawer-head p { max-width: 330px; margin-top: 5px; overflow: hidden; color: var(--text-muted); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }

.drawer-x {
  width: 34px;
  height: 34px;
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(255, 255, 255, 0.06);
  color: rgba(255, 255, 255, 0.9);
  cursor: pointer;
  display: grid;
  place-items: center;
  font-size: 0;
  transition:
    border-color 0.18s ease,
    background 0.18s ease,
    transform 0.18s ease;
}

.drawer-x::before {
  content: '×';
  font-family: 'Avenir Next', 'PingFang SC', 'Microsoft YaHei UI', sans-serif;
  font-size: 20px;
  font-weight: 700;
  line-height: 1;
  transform: translateY(-1px);
}

.drawer-x:hover {
  transform: translateY(-1px);
  border-color: rgba(254, 44, 85, 0.42);
  background: rgba(255, 255, 255, 0.11);
}

.drawer-body {
  overflow: auto;
  padding: 12px 18px 22px;
  display: block;
}

.drawer-foot {
  min-height: 88px;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  padding: 15px 16px;
  display: flex;
  align-items: flex-start;
  gap: 10px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.035), rgba(255, 255, 255, 0.015)),
    rgba(0, 0, 0, 0.24);
}

.comment-composer { position: relative; min-width: 0; flex: 1; }
.drawer-foot textarea {
  width: 100%;
  min-height: 52px;
  max-height: 110px;
  resize: none;
  border-radius: 14px;
  border: 1px solid rgba(255, 255, 255, 0.13);
  background: rgba(255, 255, 255, 0.075);
  color: rgba(255, 255, 255, 0.92);
  padding: 12px 64px 10px 13px;
  outline: none;
  font: inherit;
  line-height: 1.55;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.05);
  transition:
    border-color 0.18s ease,
    background 0.18s ease,
    box-shadow 0.18s ease;
}

.drawer-foot textarea:focus {
  border-color: rgba(254, 44, 85, 0.46);
  background: var(--surface-hover);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.06),
    0 0 0 3px rgba(254, 44, 85, 0.1);
}

.comment-composer .comment-action.primary { position: absolute; right: 7px; bottom: 8px; min-height: 32px; padding-inline: 11px; }

.drawer-hint {
  color: rgba(255, 255, 255, 0.78);
  padding: 12px 0;
}

.drawer-hint.bad {
  color: rgba(254, 44, 85, 0.92);
}

.comment {
  padding: 9px 0;
  display: grid;
  grid-template-columns: 40px minmax(0, 1fr);
  align-items: start;
  gap: 11px;
}

.comment-avatar { border-radius: 999px; transition: transform var(--duration-fast) ease; }
.comment-avatar:hover { transform: translateY(-1px); }
.comment-main { min-width: 0; padding: 12px 13px; border: 1px solid rgba(255,255,255,.075); border-radius: 15px; background: rgba(255,255,255,.035); transition: border-color var(--duration-fast) ease, background var(--duration-fast) ease; }
.comment:hover .comment-main { border-color: rgba(255,255,255,.13); background: rgba(255,255,255,.05); }
.comment-top {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
}

.comment-user {
  min-width: 0;
  overflow: hidden;
  color: #e8e8eb;
  font-weight: 750;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.comment-user:hover { color: var(--accent); }

.comment-meta {
  flex: 0 0 auto;
  font-size: 9px;
  color: var(--text-muted);
}

.comment-content {
  margin-top: 8px;
  font-size: 13px;
  line-height: 1.65;
  color: #f1f1f3;
  white-space: pre-wrap;
  word-break: break-word;
}

.comment-actions {
  margin-top: 7px;
  display: flex;
  justify-content: flex-start;
}

.comment-action {
  min-height: 34px;
  border: 1px solid rgba(255, 255, 255, 0.13);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.075);
  color: rgba(255, 255, 255, 0.82);
  padding: 0 14px;
  font: inherit;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.03em;
  cursor: pointer;
  transition:
    transform 0.18s ease,
    border-color 0.18s ease,
    background 0.18s ease;
}

.comment-action:hover:not(:disabled) {
  transform: translateY(-1px);
  border-color: rgba(254, 44, 85, 0.4);
  background: rgba(255, 255, 255, 0.12);
}

.comment-action.primary {
  border-color: rgba(254, 44, 85, 0.42);
  background: var(--accent);
  color: #fff;
}

.comment-action.danger {
  border-color: rgba(254, 44, 85, 0.42);
  background: rgba(254, 44, 85, 0.1);
  color: rgba(255, 255, 255, 0.9);
}

.comment-action:disabled {
  cursor: not-allowed;
  opacity: 0.58;
}

@media (max-width: 900px) {
  .stage {
    width: calc(100% - 12px);
    height: calc(100dvh - 64px - 58px - 36px);
    border-radius: var(--radius-lg);
  }
  .drawer-backdrop {
    justify-items: center;
    align-items: end;
  }
  .drawer {
    width: calc(100vw - 16px);
    height: min(72dvh, 560px);
    border-left: none;
    border-top: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 18px 18px 0 0;
    overflow: hidden;
  }
}

@media (max-width: 768px) {
  .top { height: 52px; padding-inline: 12px; }
  .wrap { padding: 12px 8px; }
  .stage { height: calc(100dvh - 56px - 52px - 62px - env(safe-area-inset-bottom) - 24px); }
  .hint { display: none; }
  .meta { left: 14px; bottom: 16px; }
  .actions { right: 10px; bottom: 14px; }
  .act { width: 62px; padding: 9px 7px; border-radius: 17px; }
}
/* Keep the standalone player visually identical to the feed player. */
.page { background: var(--surface-base); }
.top { height: 52px; padding: 0 20px; background: rgba(11,11,13,.95); }
.top-chip { padding: 7px 11px; border: 0; border-radius: 6px; background: var(--surface-raised); font-size: 12px; }
.wrap { padding: 14px 20px; }

@media (min-width: 901px) {
  .stage {
    width: min(1260px, calc(100% - 32px));
    height: calc(100dvh - 62px - 52px - 28px);
    overflow: hidden;
    border: 0;
    border-radius: 14px;
    background: #050506;
    box-shadow: 0 22px 64px rgba(0,0,0,.55);
  }
  .stage::before { display: none; }
  .video,
  .grad { border-radius: 14px; }
  .grad { background: linear-gradient(to top, rgba(0,0,0,.82), transparent 48%); }
  .meta { left: 20px; right: auto; bottom: 20px; max-width: min(760px, calc(100% - 116px)); }
  .author-link { margin-bottom: 5px; font-size: 14px; }
  .title { margin-bottom: 7px; font-size: clamp(22px, 2.2vw, 34px); line-height: 1.16; letter-spacing: -.025em; }
  .desc { max-width: 100%; font-size: 12px; line-height: 1.5; }
  .asset-link { margin-top: 7px; padding: 5px 8px; border: 0; border-radius: 6px; font-size: 10px; }
  .actions { right: 14px; bottom: 18px; gap: 10px; }
  .act {
    width: 60px;
    min-height: 60px;
    padding: 6px 4px;
    gap: 3px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    box-shadow: none;
    backdrop-filter: none;
  }
  .act:hover { background: transparent; }
  .act .icon {
    width: 45px;
    height: 45px;
    border-radius: 50%;
    background: #29292d;
    transition: background var(--duration-fast) ease, transform var(--duration-fast) ease;
  }
  .act:hover .icon { background: #38383d; transform: translateY(-1px); }
  .act-danger .icon { background: rgba(254,44,85,.18); }
  .count { color: #d7d7db; font-size: 10px; font-weight: 750; }
  .hint { top: 12px; left: 12px; }
  .hint-pill,
  .chip { padding: 5px 8px; border: 0; background: rgba(0,0,0,.58); font-size: 10px; }
}

.drawer { background: #151517; }
.drawer-head { background: transparent; }
.drawer-foot { background: #19191c; }
.drawer-foot textarea,
.comment-main { border-radius: 8px; background: #202024; }
</style>
