<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import UserAvatar from '../components/UserAvatar.vue'
import { ApiError } from '../api/client'
import * as accountApi from '../api/account'
import * as likeApi from '../api/like'
import type { Video } from '../api/types'
import * as videoApi from '../api/video'
import { useAuthStore } from '../stores/auth'
import { useSocialStore } from '../stores/social'
import { useToastStore } from '../stores/toast'
import { useDialogStore } from '../stores/dialog'

const router = useRouter()
const auth = useAuthStore()
const social = useSocialStore()
const toast = useToastStore()
const dialog = useDialogStore()

const busy = ref(false)
const loginForm = reactive({ accountName: '', password: '' })

const me = computed(() => ({
  id: auth.claims?.account_id ?? 0,
  accountName: auth.claims?.account_name ?? '',
  username: auth.claims?.username ?? '',
}))

const myVideos = reactive({
  loading: false,
  error: '',
  items: [] as Video[],
})
const deletingVideoId = ref<number | null>(null)

type VideoTab = 'works' | 'likes'
const videoTab = ref<VideoTab>('works')

let myVideosReq = 0
async function loadMyVideos() {
  const id = me.value.id
  if (!auth.isLoggedIn || !id) {
    myVideos.items = []
    myVideos.error = ''
    myVideos.loading = false
    return
  }
  if (myVideos.loading) return

  const req = ++myVideosReq
  myVideos.loading = true
  myVideos.error = ''
  try {
    const vids = await videoApi.listByAuthorId(id)
    if (req !== myVideosReq) return
    myVideos.items = vids
  } catch (e) {
    if (req !== myVideosReq) return
    myVideos.error = e instanceof ApiError ? e.message : String(e)
    myVideos.items = []
  } finally {
    if (req === myVideosReq) myVideos.loading = false
  }
}

const likedVideos = reactive({
  loading: false,
  loaded: false,
  error: '',
  items: [] as Video[],
})

let likedVideosReq = 0
async function loadLikedVideos() {
  if (!auth.isLoggedIn || !me.value.id) {
    likedVideosReq += 1
    likedVideos.loading = false
    likedVideos.loaded = false
    likedVideos.error = ''
    likedVideos.items = []
    return
  }
  if (likedVideos.loading) return

  const req = ++likedVideosReq
  likedVideos.loading = true
  likedVideos.error = ''
  try {
    const vids = await likeApi.listMyLikedVideos()
    if (req !== likedVideosReq) return
    likedVideos.items = vids
    likedVideos.loaded = true
  } catch (e) {
    if (req !== likedVideosReq) return
    likedVideos.error = e instanceof ApiError ? e.message : String(e)
    likedVideos.items = []
    likedVideos.loaded = true
  } finally {
    if (req === likedVideosReq) likedVideos.loading = false
  }
}

async function goVideo(id: number) {
  await router.push(`/video/${id}`)
}

async function deleteMyVideo(videoId: number) {
  if (deletingVideoId.value === videoId) return
  if (!await dialog.ask({
    title: '删除这个视频？',
    message: '删除后作品将不再公开展示，相关点赞和评论也会一并移除。',
    confirmLabel: '删除视频',
    tone: 'danger',
  })) return

  deletingVideoId.value = videoId
  try {
    await videoApi.deleteVideo(videoId)
    myVideos.items = myVideos.items.filter((item) => item.id !== videoId)
    likedVideos.items = likedVideos.items.filter((item) => item.id !== videoId)
    toast.info('视频已删除')
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
  } finally {
    deletingVideoId.value = null
  }
}

function openWorksVideos() {
  videoTab.value = 'works'
  void loadMyVideos()
}

function openLikedVideos() {
  videoTab.value = 'likes'
  void loadLikedVideos()
}

async function onLogin() {
  if (busy.value) return
  const accountName = loginForm.accountName.trim().toLowerCase()
  const password = loginForm.password.trim()
  if (!accountName || !password) {
    toast.error('请输入账号名和密码')
    return
  }

  busy.value = true
  try {
    const res = await accountApi.login(accountName, password)
    auth.setToken(res.token)
    toast.success('登录成功')
    await social.refreshMine()
    await loadMyVideos()
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
  } finally {
    busy.value = false
  }
}

async function goRegister() {
  await router.push('/account/register')
}

async function goChangePassword() {
  await router.push('/account/change-password')
}

async function goSettings() {
  await router.push('/settings')
}

type ListTab = 'followers' | 'following'
const drawer = reactive({
  open: false,
  tab: 'followers' as ListTab,
})

function openFollowers() {
  drawer.tab = 'followers'
  drawer.open = true
}

function openFollowing() {
  drawer.tab = 'following'
  drawer.open = true
}

function closeDrawer() {
  drawer.open = false
}

const listTitle = computed(() => (drawer.tab === 'followers' ? '粉丝' : '关注'))
const listItems = computed(() => (drawer.tab === 'followers' ? social.followers : social.vloggers))
const drawerLoading = computed(() => (drawer.tab === 'followers' ? social.followersLoading : social.vloggersLoading))
const drawerError = computed(() => (drawer.tab === 'followers' ? social.followersError : social.vloggersError))
const socialErrorHint = computed(() => social.followersError || social.vloggersError)

async function goUser(id: number) {
  drawer.open = false
  await router.push(`/u/${id}`)
}

watch(
  () => auth.isLoggedIn,
  (v) => {
    if (!v) {
      drawer.open = false
      myVideosReq += 1
      myVideos.loading = false
      myVideos.items = []
      myVideos.error = ''

      likedVideosReq += 1
      likedVideos.loading = false
      likedVideos.loaded = false
      likedVideos.items = []
      likedVideos.error = ''

      videoTab.value = 'works'
    }
  },
)

watch(
  () => me.value.id,
  (id) => {
    if (auth.isLoggedIn && id) {
      void loadMyVideos()
      if (videoTab.value === 'likes') void loadLikedVideos()
    }
  },
  { immediate: true },
)
</script>

<template>
  <AppShell>
    <div v-if="!auth.isLoggedIn" class="login-wrap">
      <div class="login-atmosphere">
        <span class="brand-mark">VH</span>
        <div>
          <span class="brand-kicker">VIDEOHUB COMMUNITY</span>
          <h1>发现每一种<br />真实表达</h1>
          <p>登录后继续浏览关注内容，与喜欢的作品互动。</p>
        </div>
      </div>

      <div class="login-card">
        <div class="login-heading">
          <span>账号登录</span>
          <h2>欢迎回来</h2>
          <p>使用你的 VideoHub 账号继续</p>
        </div>

        <form class="login-form" @submit.prevent="onLogin">
          <label>
            <span>账号名</span>
            <div class="login-input">
              <svg viewBox="0 0 24 24" fill="none"><circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0 1 16 0"/></svg>
              <input v-model.trim="loginForm.accountName" autocomplete="username" placeholder="输入唯一账号名" />
            </div>
          </label>
          <label>
            <span>密码</span>
            <div class="login-input">
              <svg viewBox="0 0 24 24" fill="none"><rect x="4" y="10" width="16" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/></svg>
              <input v-model.trim="loginForm.password" type="password" autocomplete="current-password" placeholder="输入密码" @keydown.enter="onLogin" />
            </div>
          </label>
          <button class="login-submit" type="submit" :disabled="busy">{{ busy ? '登录中...' : '登录' }}</button>
        </form>

        <div class="login-links">
          <button type="button" :disabled="busy" @click="goRegister">注册新账号</button>
          <button type="button" :disabled="busy" @click="goChangePassword">修改密码</button>
        </div>

        <div class="login-benefits">
          <span><b />点赞与评论</span>
          <span><b />关注视频流</span>
          <span><b />发布你的作品</span>
        </div>
      </div>
    </div>

    <template v-else>
      <div class="profile-page">
        <section class="profile-hero">
          <div class="profile-banner">
            <span class="banner-orb cyan" />
            <span class="banner-orb red" />
            <div class="banner-grid" />
          </div>

          <div class="profile-info">
            <div class="profile-avatar">
              <UserAvatar :username="me.username" :id="me.id" :size="94" />
            </div>

            <div class="profile-copy">
              <span class="profile-kicker">VIDEOHUB CREATOR</span>
              <h1>{{ me.username }}</h1>
              <p class="profile-id">@{{ me.accountName || 'loading' }} · VideoHub ID {{ me.id }}</p>
              <p class="profile-bio">记录生活片段，分享每一种真实表达。</p>
            </div>

            <div class="profile-actions">
              <button class="edit-profile" type="button" @click="goSettings">
                <svg viewBox="0 0 24 24" fill="none"><path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L8 18l-4 1 1-4L16.5 3.5Z"/></svg>
                编辑资料
              </button>
              <button class="publish-profile" type="button" @click="router.push('/video')">发布作品</button>
            </div>
          </div>

          <div class="profile-stats">
            <button type="button" :disabled="social.vloggersLoading" @click="openFollowing">
              <strong>{{ social.vloggersLoading ? '…' : social.followingCount }}</strong><span>关注</span>
            </button>
            <button type="button" :disabled="social.followersLoading" @click="openFollowers">
              <strong>{{ social.followersLoading ? '…' : social.followerCount }}</strong><span>粉丝</span>
            </button>
            <div><strong>{{ myVideos.loading ? '…' : myVideos.items.length }}</strong><span>作品</span></div>
            <div><strong>{{ myVideos.items.reduce((sum, video) => sum + video.likes_count, 0) }}</strong><span>获赞</span></div>
          </div>
        </section>

        <section class="profile-content">
          <div class="profile-tabs">
            <button type="button" :class="{ active: videoTab === 'works' }" @click="openWorksVideos">
              作品 <span>{{ myVideos.items.length }}</span>
            </button>
            <button type="button" :class="{ active: videoTab === 'likes' }" @click="openLikedVideos">
              喜欢 <span>{{ likedVideos.loaded ? likedVideos.items.length : '—' }}</span>
            </button>
          </div>

          <div class="content-toolbar">
            <div>
              <h2>{{ videoTab === 'works' ? '我的作品' : '喜欢的视频' }}</h2>
              <p>{{ videoTab === 'works' ? '管理你发布的视频内容' : '浏览你点赞收藏的作品' }}</p>
            </div>
            <RouterLink v-if="videoTab === 'works'" class="create-link" to="/video">+ 发布新作品</RouterLink>
          </div>

        <template v-if="videoTab === 'works'">
          <div v-if="myVideos.loading" class="profile-empty">正在加载作品...</div>
          <div v-else-if="myVideos.error" class="profile-empty bad">{{ myVideos.error }}</div>
          <div v-else-if="myVideos.items.length === 0" class="profile-empty">
            <svg viewBox="0 0 24 24" fill="none"><rect x="3" y="4" width="18" height="16" rx="3"/><path d="m10 9 6 3-6 3V9Z"/></svg>
            <strong>还没有发布作品</strong>
            <span>分享你的第一个视频，让更多人看见</span>
            <RouterLink to="/video">去发布</RouterLink>
          </div>

          <div v-else class="video-grid">
            <div v-for="v in myVideos.items" :key="v.id" class="video-tile">
              <button class="video-card manageable" type="button" @click="goVideo(v.id)">
                <img class="video-cover" :src="v.cover_url" :alt="v.title" loading="lazy" />
                <span class="cover-likes">♥ {{ v.likes_count }}</span>
                <div class="video-meta">
                  <div class="video-title">{{ v.title }}</div>
                  <div class="video-sub subtle">{{ new Date(v.create_time).toLocaleDateString() }}</div>
                </div>
              </button>
              <button
                class="video-delete"
                type="button"
                :disabled="deletingVideoId === v.id"
                @click.stop="deleteMyVideo(v.id)"
              >
                {{ deletingVideoId === v.id ? '删除中…' : '删除' }}
              </button>
            </div>
          </div>
        </template>
        <template v-else>
          <div v-if="likedVideos.loading" class="profile-empty">正在加载喜欢的视频...</div>
          <div v-else-if="likedVideos.error" class="profile-empty bad">{{ likedVideos.error }}</div>
          <div v-else-if="likedVideos.items.length === 0" class="profile-empty">暂无喜欢的视频</div>

          <div v-else class="video-grid">
            <button v-for="v in likedVideos.items" :key="v.id" class="video-card" type="button" @click="goVideo(v.id)">
              <img class="video-cover" :src="v.cover_url" :alt="v.title" loading="lazy" />
              <span class="cover-likes">♥ {{ v.likes_count }}</span>
              <div class="video-meta">
                <div class="video-title">{{ v.title }}</div>
                <div class="video-sub subtle">{{ new Date(v.create_time).toLocaleDateString() }}</div>
              </div>
            </button>
          </div>
        </template>
        </section>
        <div v-if="socialErrorHint" class="profile-error">社交信息加载失败：{{ socialErrorHint }}</div>
      </div>
    </template>

    <div v-if="drawer.open" class="drawer-backdrop" @click.self="closeDrawer">
      <div class="drawer">
        <div class="drawer-head">
          <div class="drawer-title">{{ listTitle }}</div>
          <button class="drawer-x" type="button" aria-label="关闭列表" @click="closeDrawer">×</button>
        </div>
        <div class="drawer-body">
          <div v-if="drawerLoading" class="drawer-hint">加载中…</div>
          <div v-else-if="drawerError" class="drawer-hint bad">{{ drawerError }}</div>
          <div v-else-if="listItems.length === 0" class="drawer-hint">暂无</div>

          <button v-for="u in listItems" v-if="!drawerLoading && !drawerError" :key="u.id" class="user-row" type="button" @click="goUser(u.id)">
            <UserAvatar :username="u.username" :id="u.id" :size="40" />
            <div class="user-meta">
              <div class="user-name">{{ u.username }}</div>
              <div class="user-id mono">@{{ u.account_name }}</div>
            </div>
          </button>
        </div>
      </div>
    </div>
  </AppShell>
</template>

<style scoped>
.login-wrap {
  min-height: calc(100vh - 96px);
  position: relative;
  overflow: hidden;
  display: grid;
  grid-template-columns: 1fr 440px;
  border: 1px solid var(--border);
  border-radius: 16px;
  background: #202023;
}

.login-card {
  padding: 60px 48px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  background: #242428;
}

.login-atmosphere {
  position: relative;
  overflow: hidden;
  padding: 64px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  background:
    radial-gradient(circle at 22% 18%, rgba(32, 213, 236, .15), transparent 30%),
    radial-gradient(circle at 78% 80%, rgba(254, 44, 85, .2), transparent 34%),
    linear-gradient(145deg, #29292d, #1d1d20);
}

.login-atmosphere::after {
  content: '';
  position: absolute;
  inset: 0;
  opacity: .13;
  background-image: linear-gradient(rgba(255,255,255,.18) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,.18) 1px, transparent 1px);
  background-size: 46px 46px;
}

.brand-mark,
.login-atmosphere > div {
  position: relative;
  z-index: 1;
}

.brand-mark {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: grid;
  place-items: center;
  background: #fff;
  color: #080808;
  box-shadow: -4px 0 #20d5ec, 4px 0 #fe2c55;
  font-size: 14px;
  font-weight: 900;
}

.brand-kicker,
.login-heading > span {
  color: #fe2c55;
  font-size: 10px;
  font-weight: 900;
  letter-spacing: .18em;
}

.login-atmosphere h1 {
  margin-top: 16px;
  font-size: clamp(36px, 3.8vw, 52px);
  line-height: 1.15;
  letter-spacing: -.035em;
}

.login-atmosphere p {
  max-width: 390px;
  margin-top: 18px;
  color: #888;
  font-size: 13px;
  line-height: 1.8;
}

.login-heading h2 {
  margin-top: 9px;
  font-size: 28px;
  letter-spacing: -.04em;
}

.login-heading p {
  margin-top: 7px;
  color: #777;
  font-size: 12px;
}

.login-form {
  margin-top: 30px;
  display: grid;
  gap: 18px;
}

.login-form label {
  margin: 0;
}

.login-form label > span {
  display: block;
  margin-bottom: 8px;
  color: #bbb;
  font-size: 12px;
  font-weight: 600;
}

.login-input {
  position: relative;
}

.login-input svg {
  position: absolute;
  top: 50%;
  left: 13px;
  width: 18px;
  transform: translateY(-50%);
  stroke: #666;
  stroke-width: 1.7;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.login-input input {
  height: 46px;
  padding-left: 42px;
  border-radius: 7px;
  background: var(--surface-raised);
}

.login-submit {
  height: 46px;
  margin-top: 5px;
  border-radius: 7px;
  background: #fe2c55;
  color: #fff;
  font-weight: 700;
}

.login-submit:hover {
  background: #ff405e;
}

.login-links {
  margin-top: 14px;
  display: flex;
  justify-content: space-between;
}

.login-links button {
  padding: 7px 0;
  background: transparent;
  color: #888;
  font-size: 11px;
}

.login-links button:hover {
  color: #fff;
}

.login-benefits {
  margin-top: 34px;
  padding-top: 18px;
  border-top: 1px solid var(--border);
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

.login-benefits span {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #727272;
  font-size: 10px;
}

.login-benefits b {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: #fe2c55;
}

.profile-page {
  display: grid;
  gap: 16px;
}

.profile-hero,
.profile-content {
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 14px;
  background: #222225;
}

.profile-banner {
  position: relative;
  height: 170px;
  overflow: hidden;
  background:
    radial-gradient(circle at 18% 30%, rgba(32, 213, 236, .16), transparent 28%),
    radial-gradient(circle at 82% 70%, rgba(254, 44, 85, .18), transparent 30%),
    linear-gradient(130deg, #333338, #242428 48%, #2c2c31);
}

.banner-grid {
  position: absolute;
  inset: 0;
  opacity: .16;
  background-image: linear-gradient(rgba(255,255,255,.15) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,.15) 1px, transparent 1px);
  background-size: 38px 38px;
  mask-image: linear-gradient(to right, black, transparent);
}

.banner-orb {
  position: absolute;
  width: 160px;
  height: 160px;
  border-radius: 50%;
  filter: blur(65px);
}

.banner-orb.cyan { top: -60px; left: 10%; background: rgba(32, 213, 236, .32); }
.banner-orb.red { right: 8%; bottom: -80px; background: rgba(254, 44, 85, .36); }

.profile-info {
  position: relative;
  min-height: 142px;
  padding: 24px 28px 22px 150px;
  display: flex;
  align-items: flex-start;
  gap: 24px;
}

.profile-avatar {
  position: absolute;
  top: -54px;
  left: 28px;
  padding: 5px;
  border: 4px solid #222225;
  border-radius: 50%;
  background: #29292d;
}

.profile-copy {
  flex: 1;
  min-width: 0;
}

.profile-kicker {
  color: #fe2c55;
  font-size: 9px;
  font-weight: 900;
  letter-spacing: .16em;
}

.profile-copy h1 {
  margin-top: 5px;
  font-size: 25px;
  letter-spacing: -.035em;
}

.profile-id,
.profile-bio {
  color: #85858b;
  font-size: 11px;
}

.profile-id { margin-top: 4px; }
.profile-bio { margin-top: 12px; color: #aaaab0; }

.profile-actions {
  display: flex;
  gap: 8px;
}

.profile-actions button,
.create-link {
  height: 38px;
  padding: 0 16px;
  border-radius: 6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  font-size: 11px;
  font-weight: 700;
}

.edit-profile {
  border: 1px solid rgba(255,255,255,.12);
  background: #303034;
}

.edit-profile svg {
  width: 15px;
  stroke: currentColor;
  stroke-width: 1.7;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.publish-profile,
.create-link {
  background: #fe2c55;
  color: #fff;
}

.profile-stats {
  padding: 0 28px 22px 150px;
  display: flex;
  gap: 38px;
}

.profile-stats button,
.profile-stats div {
  min-width: auto;
  padding: 0;
  border: 0;
  display: flex;
  align-items: baseline;
  gap: 6px;
  background: transparent;
}

.profile-stats button:hover { background: transparent; }
.profile-stats strong { font-size: 17px; }
.profile-stats span { color: #85858b; font-size: 10px; }

.profile-content {
  padding: 0 24px 28px;
}

.profile-tabs {
  height: 58px;
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: stretch;
  gap: 30px;
}

.profile-tabs button {
  position: relative;
  padding: 0 3px;
  border-radius: 0;
  background: transparent;
  color: #8d8d93;
  font-size: 13px;
  font-weight: 700;
}

.profile-tabs button:hover,
.profile-tabs button.active {
  background: transparent;
  color: #fff;
}

.profile-tabs button.active::after {
  content: '';
  position: absolute;
  right: 3px;
  bottom: 0;
  left: 3px;
  height: 2px;
  background: #fe2c55;
}

.profile-tabs span {
  margin-left: 4px;
  color: #6e6e74;
  font-size: 9px;
}

.content-toolbar {
  padding: 22px 0 18px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
}

.content-toolbar h2 { font-size: 16px; }
.content-toolbar p { margin-top: 4px; color: #77777d; font-size: 10px; }
.create-link { height: 34px; padding: 0 13px; }

.profile-empty {
  min-height: 270px;
  border: 1px dashed rgba(255,255,255,.1);
  border-radius: 10px;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 8px;
  background: #28282c;
  color: #85858b;
  font-size: 11px;
}

.profile-empty svg {
  width: 34px;
  margin-bottom: 4px;
  stroke: #77777d;
  stroke-width: 1.4;
}

.profile-empty strong { color: #d6d6d9; font-size: 13px; }
.profile-empty a { margin-top: 7px; padding: 7px 14px; border-radius: 5px; background: #fe2c55; color: #fff; }
.profile-empty.bad, .profile-error { color: #fe6b87; }
.profile-error { font-size: 10px; }

@media (max-width: 980px) {
  .login-wrap { grid-template-columns: 1fr; }
  .login-atmosphere { display: none; }
  .login-card { min-height: calc(100vh - 100px); padding: 42px 28px; }
}

.ghost {
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(0, 0, 0, 0.18);
  color: rgba(255, 255, 255, 0.86);
  border-radius: 12px;
  padding: 10px 12px;
  cursor: pointer;
}

.ghost:hover {
  background: rgba(255, 255, 255, 0.1);
}

.metric {
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.06);
  border-radius: 16px;
  padding: 12px 14px;
  min-width: 120px;
  cursor: pointer;
  display: grid;
  gap: 4px;
  text-align: left;
}

.metric:hover {
  background: rgba(255, 255, 255, 0.1);
}

.metric.active {
  background: rgba(254, 44, 85, 0.14);
  border-color: rgba(254, 44, 85, 0.55);
}

.metric.static {
  cursor: default;
}

.metric:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.metric-num {
  font-size: 18px;
  font-weight: 900;
  letter-spacing: 0.2px;
}

.metric-label {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.65);
}

.hint {
  color: rgba(255, 255, 255, 0.78);
}

.hint.bad {
  color: rgba(254, 44, 85, 0.92);
}

.drawer-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.55);
  backdrop-filter: blur(10px);
  z-index: 120;
  display: grid;
  justify-items: center;
  align-items: center;
  padding: 16px;
}

.drawer {
  width: min(520px, calc(100vw - 18px));
  max-height: min(78vh, 720px);
  background: rgba(0, 0, 0, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 18px;
  overflow: hidden;
  display: grid;
  grid-template-rows: auto 1fr;
}

.drawer-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.drawer-title {
  font-weight: 900;
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
  border-color: rgba(254, 44, 85, 0.42);
  background: rgba(255, 255, 255, 0.11);
}

.drawer-body {
  overflow: auto;
  padding: 12px 14px;
  display: grid;
  gap: 10px;
}

.drawer-hint {
  color: rgba(255, 255, 255, 0.78);
  padding: 12px 0;
}

.drawer-hint.bad {
  color: rgba(254, 44, 85, 0.92);
}

.video-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

@media (max-width: 1100px) {
  .video-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 800px) {
  .video-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

.video-card {
  position: relative;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: #29292d;
  border-radius: 9px;
  overflow: hidden;
  cursor: pointer;
  padding: 0;
  text-align: left;
}

.video-card.manageable {
  width: 100%;
}

.video-card:hover {
  background: #303034;
  border-color: rgba(255,255,255,.17);
}

.video-tile {
  position: relative;
}

.video-delete {
  position: absolute;
  top: 10px;
  right: 10px;
  border: 1px solid rgba(254, 44, 85, 0.45);
  background: rgba(35, 35, 38, .88);
  color: rgba(255, 255, 255, 0.92);
  border-radius: 999px;
  padding: 6px 10px;
  font-size: 12px;
  cursor: pointer;
}

.video-delete:hover {
  background: rgba(254, 44, 85, 0.18);
}

.video-delete:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.video-cover {
  width: 100%;
  aspect-ratio: 9/12;
  object-fit: cover;
  display: block;
  background: #303034;
}

.cover-likes {
  position: absolute;
  top: 10px;
  left: 10px;
  padding: 5px 7px;
  border-radius: 5px;
  background: rgba(37,37,40,.82);
  color: #fff;
  font-size: 9px;
  backdrop-filter: blur(8px);
}

.video-meta {
  padding: 10px 10px;
}

.video-title {
  font-weight: 800;
  font-size: 13px;
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

@media (max-width: 760px) {
  .profile-banner { height: 125px; }
  .profile-info { padding: 64px 18px 18px; display: block; }
  .profile-avatar { top: -48px; left: 18px; }
  .profile-actions { margin-top: 18px; }
  .profile-stats { padding: 0 18px 18px; gap: 18px; justify-content: space-between; }
  .profile-stats button, .profile-stats div { display: grid; justify-items: center; gap: 2px; }
  .profile-content { padding: 0 14px 20px; }
}

.video-sub {
  margin-top: 6px;
  font-size: 12px;
}

.user-row {
  text-align: left;
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 12px;
  align-items: center;
  padding: 10px 10px;
  border-radius: 14px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.05);
  cursor: pointer;
}

.user-row:hover {
  background: rgba(255, 255, 255, 0.08);
}

.user-meta {
  min-width: 0;
}

.user-name {
  font-weight: 800;
}

.user-id {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.6);
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
}

.login-wrap { min-height: calc(100dvh - 110px); border-radius: 12px; background: #151517; }
.login-card { background: #19191c; }
.login-atmosphere { background: #101012; }
.login-atmosphere::after { opacity: .06; }
.profile-page { max-width: 1040px; margin: 0 auto; gap: 10px; }
.profile-hero,
.profile-content { border-color: transparent; border-radius: 8px; background: var(--surface-panel); }
.profile-banner {
  height: 128px;
  background: linear-gradient(120deg, #18181b, #232327 58%, rgba(254,44,85,.16));
}
.banner-grid { opacity: .06; }
.banner-orb { opacity: .32; }
.profile-info { min-height: 128px; }
.profile-content { padding-bottom: 20px; }
.profile-empty { border-style: solid; border-radius: 8px; background: #1b1b1e; }
.video-grid { gap: 4px; }
.video-card { border: 0; border-radius: 6px; background: #1b1b1e; }
.user-row { border: 0; border-radius: 7px; }
.drawer { border-radius: 10px; }

/* TikTok-like account entry: focused auth panel, no marketing split screen. */
.login-wrap {
  width: min(460px, 100%);
  min-height: auto;
  margin: 42px auto;
  display: block;
  overflow: visible;
  border: 0;
  border-radius: 0;
  background: transparent;
}
.login-atmosphere { display: none; }
.login-card {
  min-height: 560px;
  padding: 46px 42px 36px;
  justify-content: center;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface-panel);
}
.login-heading { text-align: center; }
.login-heading h2 { font-size: 30px; }
.login-benefits { justify-content: center; }

/* TikTok-like profile header: avatar, identity, actions and stats in one plane. */
.profile-banner { display: none; }
.profile-hero { overflow: visible; background: transparent; }
.profile-info {
  min-height: auto;
  padding: 30px 28px 16px;
  display: grid;
  grid-template-columns: 104px minmax(0,1fr) auto;
  align-items: center;
  gap: 22px;
}
.profile-avatar {
  position: static;
  padding: 0;
  border: 0;
  background: transparent;
}
.profile-copy h1 { font-size: 28px; }
.profile-bio { margin-top: 8px; }
.profile-stats { padding: 2px 28px 26px 154px; gap: 28px; }
.profile-content { border-top: 1px solid var(--border); background: transparent; }
.profile-tabs { justify-content: center; gap: 44px; }
.content-toolbar { padding-top: 18px; }
.video-grid { grid-template-columns: repeat(5,minmax(0,1fr)); }
@media (max-width: 900px) {
  .profile-info { grid-template-columns: 90px minmax(0,1fr); }
  .profile-actions { grid-column: 2; }
  .profile-stats { padding-left: 140px; }
  .video-grid { grid-template-columns: repeat(3,minmax(0,1fr)); }
}
@media (max-width: 760px) {
  .login-wrap { margin: 12px auto; }
  .login-card { min-height: calc(100dvh - 110px); padding: 36px 24px; }
  .profile-info { padding: 24px 18px 14px; grid-template-columns: 76px minmax(0,1fr); gap: 14px; }
  .profile-actions { grid-column: 1/-1; }
  .profile-stats { padding: 4px 18px 20px; }
}
</style>
