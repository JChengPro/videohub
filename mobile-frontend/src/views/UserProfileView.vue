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
const followed = ref(false)
const followBusy = ref(false)
const userId = computed(() => Number(route.params.id))

async function load() {
  user.value = null
  videos.value = []
  followed.value = false
  if (!Number.isInteger(userId.value) || userId.value <= 0) {
    toast.error('无效的用户 ID')
    return
  }
  try {
    [user.value, videos.value] = await Promise.all([api.accountById(userId.value), api.videosByAuthor(userId.value)])
    followed.value = auth.isLoggedIn ? (await api.following()).vloggers?.some((item) => item.id === userId.value) ?? false : false
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
}

async function toggleFollow() {
  if (!auth.isLoggedIn) return router.push('/me')
  if (!user.value || user.value.id === auth.claims?.account_id || followBusy.value) return
  followBusy.value = true
  try {
    followed.value ? await api.unfollow(userId.value) : await api.follow(userId.value)
    followed.value = !followed.value
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
  finally { followBusy.value = false }
}

watch(userId, load, { immediate: true })
</script>

<template>
  <main class="page profile-page">
    <header class="topbar"><button @click="router.back()"><AppIcon name="back" /></button><b>用户主页</b><span /></header>
    <section v-if="user" class="hero">
      <Avatar :name="user.username" :size="82" />
      <h1>@{{ user.username }}</h1>
      <p>VideoHub ID · {{ user.id }}</p>
      <button v-if="user.id !== auth.claims?.account_id" :class="{ followed }" :disabled="followBusy" @click="toggleFollow">{{ followed ? '已关注' : '关注' }}</button>
    </section>
    <div class="section-title"><b>作品</b><span>{{ videos.length }}</span></div>
    <section class="video-grid">
      <button v-for="item in videos" :key="item.id" @click="router.push(`/video/${item.id}`)"><img :src="item.cover_url" :alt="item.title" /><span><AppIcon name="play" :size="12" filled />{{ Math.max(0, item.likes_count) }}</span></button>
      <p v-if="!videos.length">暂时没有作品</p>
    </section>
  </main>
</template>

<style scoped>
.topbar { min-height: calc(54px + env(safe-area-inset-top)); padding: env(safe-area-inset-top) 15px 0; display: grid; grid-template-columns: 40px 1fr 40px; align-items: center; border-bottom: 1px solid rgba(255,255,255,.07); }.topbar b { text-align: center; font-size: 14px; }
.hero { padding: 32px 18px 26px; display: grid; justify-items: center; background: radial-gradient(circle at 50% 0, rgba(254,44,85,.15), transparent 58%); }.hero h1 { margin-top: 12px; font-size: 21px; }.hero p { margin-top: 4px; color: #666; font-size: 10px; }.hero button { min-width: 116px; margin-top: 18px; padding: 10px 22px; border-radius: 8px; background: #fe2c55; color: #fff; font-weight: 800; }.hero button.followed { background: #303035; color: #aaa; }
.section-title { padding: 13px 15px; display: flex; gap: 7px; border-top: 1px solid rgba(255,255,255,.07); }.section-title b { font-size: 13px; }.section-title span { color: #666; font-size: 11px; }
.video-grid { padding-bottom: 75px; display: grid; grid-template-columns: repeat(3,1fr); gap: 2px; }.video-grid button { position: relative; aspect-ratio: .75; overflow: hidden; background: #222; }.video-grid img { width: 100%; height: 100%; object-fit: cover; }.video-grid span { position: absolute; bottom: 5px; left: 5px; display: flex; align-items: center; gap: 3px; font-size: 9px; }.video-grid p { grid-column: 1/-1; padding: 55px 0; color: #666; text-align: center; font-size: 12px; }
</style>
