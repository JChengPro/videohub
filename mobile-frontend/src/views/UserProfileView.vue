<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import type { Account, Video } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'
import AppIcon from '../components/AppIcon.vue'
import Avatar from '../components/Avatar.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const toast = useToastStore()
const user = ref<Account | null>(null)
const videos = ref<Video[]>([])
const loading = ref(false)
const error = ref('')
const followed = ref(false)
const followBusy = ref(false)
const userId = computed(() => Number(route.params.id))
let requestId = 0

async function load() {
  const request = ++requestId
  user.value = null
  videos.value = []
  followed.value = false
  error.value = ''
  if (!Number.isInteger(userId.value) || userId.value <= 0) {
    error.value = '无效的用户 ID'
    return
  }
  loading.value = true
  try {
    const [nextUser, nextVideos] = await Promise.all([api.accountById(userId.value), api.videosByAuthor(userId.value)])
    if (request !== requestId) return
    user.value = nextUser
    videos.value = nextVideos
    if (auth.isLoggedIn) {
      const following = await api.following()
      if (request === requestId) followed.value = following.vloggers.some((item) => item.id === userId.value)
    }
  } catch (cause) {
    if (request === requestId) error.value = cause instanceof Error ? cause.message : String(cause)
  } finally {
    if (request === requestId) loading.value = false
  }
}

async function toggleFollow() {
  if (!auth.isLoggedIn) return router.push('/me')
  if (!user.value || user.value.id === auth.claims?.account_id || followBusy.value) return
  followBusy.value = true
  try {
    followed.value ? await api.unfollow(userId.value) : await api.follow(userId.value)
    followed.value = !followed.value
    toast.success(followed.value ? '已关注' : '已取消关注')
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
  finally { followBusy.value = false }
}

watch(userId, load, { immediate: true })
watch(() => auth.isLoggedIn, load)
</script>

<template>
  <main class="page profile-page">
    <header class="topbar"><button type="button" aria-label="返回上一页" @click="router.back()"><AppIcon name="back" /></button><b>用户主页</b><span /></header>
    <section v-if="loading" class="profile-state" role="status">正在加载用户主页...</section>
    <section v-else-if="error" class="profile-state" role="alert"><AppIcon name="warning" :size="32" /><b>无法打开用户主页</b><p>{{ error }}</p><button type="button" @click="load">重新加载</button></section>
    <section v-else-if="user" class="hero">
      <Avatar :name="user.username" :id="user.id" :size="82" />
      <h1>{{ user.username }}</h1>
      <p>@{{ user.account_name }}</p>
      <div v-if="user.id !== auth.claims?.account_id" class="profile-actions">
        <button type="button" :disabled="!auth.isLoggedIn" @click="router.push(`/chat/${user.id}`)">私信</button>
        <button type="button" :class="{ followed }" :disabled="followBusy" @click="toggleFollow">{{ followed ? '已关注' : '关注' }}</button>
      </div>
    </section>
    <div v-if="user" class="section-title"><b>作品</b><span>{{ videos.length }}</span></div>
    <section v-if="user" class="video-grid">
      <button v-for="item in videos" :key="item.id" type="button" :aria-label="`播放 ${item.title}`" @click="router.push(`/video/${item.id}`)"><img :src="item.cover_url" :alt="item.title" loading="lazy" /><span><AppIcon name="play" :size="12" filled />{{ Math.max(0, item.likes_count) }}</span></button>
      <p v-if="!videos.length">暂时没有作品</p>
    </section>
  </main>
</template>

<style scoped>
.profile-page { background: var(--mobile-surface); }
.topbar { position: sticky; z-index: 10; top: 0; min-height: calc(54px + env(safe-area-inset-top)); padding: env(safe-area-inset-top) 8px 0; display: grid; grid-template-columns: 44px 1fr 44px; align-items: center; border-bottom: 1px solid var(--mobile-border); background: rgba(20,20,23,.92); backdrop-filter: blur(18px); }
.topbar button { width: 44px; height: 44px; display: grid; place-items: center; color: var(--mobile-text-secondary); }.topbar b { text-align: center; font-size: 14px; }
.hero { padding: 34px 18px 28px; display: grid; justify-items: center; background: linear-gradient(180deg, rgba(255,59,92,.08), transparent); }.hero h1 { margin-top: 13px; font-size: 21px; letter-spacing: -.03em; }.hero p { margin-top: 4px; color: var(--mobile-text-muted); font-size: 10px; }.profile-actions { margin-top: 18px; display: flex; gap: 8px; }.profile-actions button { min-width: 104px; min-height: 44px; padding: 10px 18px; border-radius: 999px; background: var(--mobile-accent); color: #fff; font-weight: 800; }.profile-actions button:first-child { border: 1px solid var(--mobile-border); background: var(--mobile-surface-strong); color: var(--mobile-text-secondary); }.profile-actions button.followed { border: 1px solid var(--mobile-border); background: var(--mobile-surface-strong); color: var(--mobile-text-secondary); }
.section-title { padding: 14px 15px; display: flex; gap: 7px; border-top: 1px solid var(--mobile-border); border-bottom: 1px solid var(--mobile-border); }.section-title b { font-size: 13px; }.section-title span { color: var(--mobile-text-muted); font-size: 11px; }
.video-grid { padding-bottom: calc(75px + env(safe-area-inset-bottom)); display: grid; grid-template-columns: repeat(3,1fr); gap: 2px; }.video-grid button { position: relative; aspect-ratio: .75; overflow: hidden; background: var(--mobile-surface-raised); }.video-grid img { width: 100%; height: 100%; object-fit: cover; }.video-grid span { position: absolute; bottom: 5px; left: 5px; display: flex; align-items: center; gap: 3px; color: #fff; font-size: 9px; text-shadow: 0 2px 5px #000; }.video-grid p { grid-column: 1/-1; padding: 55px 0; color: var(--mobile-text-muted); text-align: center; font-size: 12px; }
.profile-state { min-height: 65dvh; padding: 28px; display: grid; place-content: center; justify-items: center; gap: 10px; color: var(--mobile-text-muted); text-align: center; font-size: 12px; }.profile-state b { color: var(--mobile-text); font-size: 15px; }.profile-state p { max-width: 280px; line-height: 1.6; }.profile-state button { min-height: 44px; margin-top: 4px; padding: 0 16px; border-radius: 999px; background: var(--mobile-text); color: var(--mobile-bg); font-weight: 750; }

.topbar { background: rgba(15,15,17,.96); }
.hero { padding: 28px 18px 24px; background: transparent; }
.profile-actions button { border-radius: 7px; }
.section-title { padding-block: 12px; }
.profile-state button { border-radius: var(--mobile-radius); }
</style>
