<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import UserAvatar from '../components/UserAvatar.vue'
import { ApiError } from '../api/client'
import * as commentApi from '../api/comment'
import * as likeApi from '../api/like'
import type { Comment, Video } from '../api/types'
import * as videoApi from '../api/video'
import { useAuthStore } from '../stores/auth'
import { useSocialStore } from '../stores/social'
import { useToastStore } from '../stores/toast'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const social = useSocialStore()
const toast = useToastStore()

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
const videoEl = ref<HTMLVideoElement | null>(null)
const commentInput = ref<HTMLTextAreaElement | null>(null)

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
  if (!Number.isFinite(id.value) || id.value <= 0) {
    state.error = '无效的 video id'
    return
  }
  state.loading = true
  state.error = ''
  try {
    state.video = await videoApi.getDetail(id.value)
  } catch (e) {
    state.error = e instanceof ApiError ? e.message : String(e)
  } finally {
    state.loading = false
  }
}

async function loadIsLiked() {
  if (!auth.isLoggedIn) {
    state.isLiked = null
    return
  }
  try {
    const res = await likeApi.isLiked(id.value)
    state.isLiked = res.is_liked
  } catch {
    state.isLiked = null
  }
}

async function play() {
  if (!videoEl.value) return
  videoEl.value.muted = muted.value
  try {
    await videoEl.value.play()
  } catch {
    // ignore
  }
}

function toggleMute() {
  muted.value = !muted.value
  if (videoEl.value) videoEl.value.muted = muted.value
  toast.info(muted.value ? '已静音' : '已取消静音')
}

function togglePlayPause() {
  const v = videoEl.value
  if (!v) return
  if (v.paused) void v.play()
  else v.pause()
}

async function toggleLike() {
  if (!state.video) return
  if (!auth.isLoggedIn) return needLogin()
  if (state.busy) return

  state.busy = true
  try {
    if (state.isLiked) {
      await likeApi.unlike(id.value)
      state.isLiked = false
      state.video.likes_count = Math.max(0, state.video.likes_count - 1)
    } else {
      await likeApi.like(id.value)
      state.isLiked = true
      state.video.likes_count += 1
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
    await navigator.clipboard.writeText(url)
    toast.success('链接已复制')
  } catch {
    window.prompt('复制链接', url)
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
  if (!window.confirm('确认删除这个视频？相关点赞和评论也会一起删除。')) return

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
  drawer.open = false
  drawer.comments = []
  drawer.content = ''
  drawer.error = ''
}

async function focusCommentInput() {
  await nextTick()
  commentInput.value?.focus()
}

async function loadComments() {
  if (!state.video) return
  drawer.loading = true
  drawer.error = ''
  try {
    drawer.comments = await commentApi.listAll(state.video.id)
  } catch (e) {
    drawer.error = e instanceof ApiError ? e.message : String(e)
  } finally {
    drawer.loading = false
  }
}

async function openComments() {
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

async function deleteComment(commentId: number) {
  if (!state.video) return
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
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
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
          <button class="top-chip" type="button" @click="toggleMute">{{ muted ? '静音' : '有声' }}</button>
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
          />
          <div class="grad" />

          <div class="meta">
            <RouterLink class="author-link" :to="`/u/${state.video.author_id}`" @click.stop>
              <UserAvatar :username="state.video.username" :id="state.video.author_id" :size="34" />
              <span class="author-name">@{{ state.video.username }}</span>
            </RouterLink>
            <div class="title">{{ state.video.title }}</div>
            <div v-if="state.video.description" class="desc">{{ state.video.description }}</div>
            <div class="row" style="margin-top: 10px">
              <a class="asset-link" :href="state.video.play_url" target="_blank" rel="noreferrer">播放地址</a>
              <a class="asset-link" :href="state.video.cover_url" target="_blank" rel="noreferrer">封面地址</a>
            </div>
          </div>

          <div class="actions">
            <button class="act" type="button" :disabled="state.busy" @click.stop="toggleLike">
              <span class="icon" :class="{ liked: !!state.isLiked }">♥</span>
              <span class="count">{{ state.video.likes_count }}</span>
            </button>

            <button class="act" type="button" @click.stop="openComments">
              <span class="icon">💬</span>
              <span class="count">评论</span>
            </button>

            <button
              v-if="!auth.claims?.account_id || auth.claims.account_id !== state.video.author_id"
              class="act"
              type="button"
              :disabled="state.busy"
              @click.stop="toggleFollow"
            >
              <span class="icon">＋</span>
              <span class="count">{{ social.isFollowing(state.video.author_id) ? '已关注' : '关注' }}</span>
            </button>

            <button class="act" type="button" @click.stop="share">
              <span class="icon">↗</span>
              <span class="count">分享</span>
            </button>

            <button v-if="isOwner" class="act act-danger" type="button" :disabled="state.busy" @click.stop="deleteVideo">
              <span class="icon">删</span>
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
        <div class="drawer">
          <div class="drawer-head">
            <div class="drawer-title">评论</div>
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
.page {
  height: 100%;
  display: flex;
  flex-direction: column;
  background:
    radial-gradient(440px 440px at 76% 8%, rgba(37, 244, 238, 0.1), transparent 68%),
    radial-gradient(520px 520px at 14% 88%, rgba(254, 44, 85, 0.11), transparent 70%);
}

.top {
  height: 58px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 18px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.09);
  background: rgba(0, 0, 0, 0.18);
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
  width: min(1040px, calc(100vw - 36px));
  height: calc(100vh - 68px - 58px - 44px);
  position: relative;
  border-radius: 30px;
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
  background: linear-gradient(140deg, rgba(255, 255, 255, 0.16), transparent 22%, transparent 70%, rgba(37, 244, 238, 0.12));
  mix-blend-mode: screen;
}

.video {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  background: rgba(0, 0, 0, 0.4);
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
  font-size: clamp(24px, 3.1vw, 48px);
  line-height: 0.98;
  font-weight: 950;
  letter-spacing: -0.055em;
  margin-bottom: 10px;
  text-shadow: 0 18px 44px rgba(0, 0, 0, 0.58);
}

.desc {
  color: rgba(255, 255, 255, 0.74);
  font-size: 14px;
  line-height: 1.45;
  max-width: 58ch;
}

.asset-link {
  display: inline-flex;
  align-items: center;
  padding: 7px 11px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(5, 6, 10, 0.34);
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
  background: rgba(5, 6, 10, 0.54);
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
  font-size: 20px;
  line-height: 1;
  opacity: 0.92;
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
  background: rgba(5, 6, 10, 0.34);
  color: rgba(255, 255, 255, 0.68);
  font-size: 12px;
  font-weight: 750;
  letter-spacing: 0.01em;
  backdrop-filter: blur(14px);
}

.hint-pill span {
  color: rgba(37, 244, 238, 0.86);
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
  width: min(420px, calc(100vw - 18px));
  height: 100vh;
  background: rgba(0, 0, 0, 0.65);
  border-left: 1px solid rgba(255, 255, 255, 0.12);
  display: grid;
  grid-template-rows: auto 1fr auto;
}

.drawer-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.drawer-title {
  font-weight: 800;
  font-size: 14px;
}

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
  border-color: rgba(37, 244, 238, 0.42);
  background: rgba(255, 255, 255, 0.11);
}

.drawer-body {
  overflow: auto;
  padding: 12px 14px;
  display: grid;
  gap: 10px;
}

.drawer-foot {
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  padding: 12px 14px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.035), rgba(255, 255, 255, 0.015)),
    rgba(0, 0, 0, 0.24);
}

.drawer-foot textarea {
  width: 100%;
  min-height: 82px;
  resize: none;
  border-radius: 18px;
  border: 1px solid rgba(255, 255, 255, 0.13);
  background: rgba(255, 255, 255, 0.075);
  color: rgba(255, 255, 255, 0.92);
  padding: 12px 13px;
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
  border-color: rgba(37, 244, 238, 0.46);
  background: rgba(255, 255, 255, 0.1);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.06),
    0 0 0 3px rgba(37, 244, 238, 0.1);
}

.drawer-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-top: 10px;
}

.drawer-hint {
  color: rgba(255, 255, 255, 0.78);
  padding: 12px 0;
}

.drawer-hint.bad {
  color: rgba(254, 44, 85, 0.92);
}

.comment {
  border: 1px solid rgba(255, 255, 255, 0.105);
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.08), rgba(255, 255, 255, 0.035)),
    rgba(255, 255, 255, 0.03);
  border-radius: 18px;
  padding: 12px 12px;
  box-shadow: 0 14px 26px rgba(0, 0, 0, 0.18);
}

.comment-top {
  display: grid;
  gap: 3px;
}

.comment-user {
  font-weight: 700;
  font-size: 13.5px;
  letter-spacing: 0.01em;
}

.comment-meta {
  font-size: 11.5px;
  font-weight: 700;
  letter-spacing: 0.02em;
  color: rgba(255, 255, 255, 0.46);
}

.comment-content {
  margin-top: 8px;
  font-size: 14px;
  line-height: 1.55;
  color: rgba(255, 255, 255, 0.9);
  white-space: pre-wrap;
  word-break: break-word;
}

.comment-actions {
  margin-top: 10px;
  display: flex;
  justify-content: flex-end;
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
  border-color: rgba(37, 244, 238, 0.4);
  background: rgba(255, 255, 255, 0.12);
}

.comment-action.primary {
  border-color: rgba(37, 244, 238, 0.38);
  background: linear-gradient(135deg, rgba(37, 244, 238, 0.22), rgba(254, 44, 85, 0.16));
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
    width: calc(100vw - 28px);
    height: calc(100vh - 68px - 58px - 36px);
    border-radius: 24px;
  }
  .drawer-backdrop {
    justify-items: center;
    align-items: end;
  }
  .drawer {
    width: calc(100vw - 16px);
    height: min(72vh, 560px);
    border-left: none;
    border-top: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 18px 18px 0 0;
    overflow: hidden;
  }
}
</style>
