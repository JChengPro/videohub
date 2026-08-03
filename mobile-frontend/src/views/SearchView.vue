<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import type { Account, FeedVideo } from '../api/types'
import AppIcon from '../components/AppIcon.vue'
import Avatar from '../components/Avatar.vue'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const input = ref(typeof route.query.q === 'string' ? route.query.q : '')
const query = computed(() => typeof route.query.q === 'string' ? route.query.q.trim() : '')
const state = reactive({ loading: false, error: '', users: [] as Account[], videos: [] as FeedVideo[] })
let requestId = 0

function matches(item: FeedVideo, keyword: string) {
  const needle = keyword.toLocaleLowerCase()
  return item.title.toLocaleLowerCase().includes(needle)
    || item.description?.toLocaleLowerCase().includes(needle)
    || item.author.username.toLocaleLowerCase().includes(needle)
}

async function load() {
  const keyword = query.value
  input.value = keyword
  const current = ++requestId
  state.error = ''
  state.users = []
  state.videos = []
  if (!keyword) return
  state.loading = true
  try {
    const [users, feed] = await Promise.all([api.searchUsers(keyword), api.latest(0)])
    if (current !== requestId) return
    state.users = users.users ?? []
    state.videos = feed.video_list.filter((item) => matches(item, keyword))
  } catch (cause) {
    if (current === requestId) state.error = cause instanceof Error ? cause.message : String(cause)
  } finally {
    if (current === requestId) state.loading = false
  }
}

function submit() {
  const keyword = input.value.trim()
  if (keyword) void router.replace({ path: '/search', query: { q: keyword } })
}

function message(user: Account) {
  void router.push(auth.isLoggedIn ? `/chat/${user.id}` : '/me')
}

watch(query, load, { immediate: true })
</script>

<template>
  <main class="page search-page">
    <header class="search-bar">
      <button type="button" aria-label="返回上一页" @click="router.back()"><AppIcon name="back" /></button>
      <form role="search" @submit.prevent="submit">
        <AppIcon name="search" :size="17" />
        <input v-model="input" autofocus autocomplete="off" aria-label="搜索用户或视频" placeholder="搜索用户名、作者或视频" />
      </form>
      <button class="submit" type="button" @click="submit">搜索</button>
    </header>

    <section v-if="!query" class="search-empty"><AppIcon name="search" :size="34" /><h1>找到想认识的人</h1><p>搜索用户后可查看主页、关注或发送私信。</p></section>
    <section v-else-if="state.loading" class="search-empty" role="status">正在搜索“{{ query }}”…</section>
    <section v-else-if="state.error" class="search-empty" role="alert"><AppIcon name="warning" :size="32" /><h1>搜索失败</h1><p>{{ state.error }}</p><button type="button" @click="load">重试</button></section>
    <template v-else>
      <section class="result-block">
        <header><div><small>PEOPLE</small><h2>用户</h2></div><span>{{ state.users.length }}</span></header>
        <article v-for="user in state.users" :key="user.id" class="user-result">
          <button class="identity" type="button" @click="router.push(`/user/${user.id}`)">
            <Avatar :name="user.username" :id="user.id" :size="48" /><span><b>{{ user.username }}</b><small>@{{ user.account_name }}</small></span>
          </button>
          <button v-if="user.id !== auth.claims?.account_id" class="message" type="button" @click="message(user)"><AppIcon name="message" :size="16" />私信</button>
        </article>
        <p v-if="!state.users.length" class="no-result">没有匹配的用户</p>
      </section>

      <section class="result-block videos">
        <header><div><small>VIDEOS</small><h2>视频</h2></div><span>{{ state.videos.length }}</span></header>
        <div class="video-grid">
          <button v-for="item in state.videos" :key="item.id" type="button" @click="router.push(`/video/${item.id}`)">
            <img :src="item.cover_url" :alt="item.title" loading="lazy" />
            <span><b>{{ item.title }}</b><small>{{ item.author.username }} · 评论 {{ item.comments_count }}</small></span>
          </button>
        </div>
        <p v-if="!state.videos.length" class="no-result">没有匹配的视频</p>
      </section>
    </template>
  </main>
</template>

<style scoped>
.search-page { min-height: 100dvh; background: var(--mobile-surface); }
.search-bar { position: sticky; z-index: 20; top: 0; min-height: calc(62px + env(safe-area-inset-top)); padding: env(safe-area-inset-top) 8px 0; display: grid; grid-template-columns: 42px 1fr 48px; align-items: center; gap: 5px; border-bottom: 1px solid var(--mobile-border); background: rgba(20,20,23,.95); backdrop-filter: blur(18px); }
.search-bar > button { min-height: 44px; display: grid; place-items: center; color: var(--mobile-text-secondary); }.search-bar .submit { color: var(--mobile-accent); font-size: 11px; font-weight: 800; }
form { height: 40px; padding: 0 12px; display: flex; align-items: center; gap: 8px; border: 1px solid var(--mobile-border); border-radius: 999px; background: var(--mobile-surface-raised); color: var(--mobile-text-muted); }
input { min-width: 0; width: 100%; border: 0; outline: 0; background: transparent; color: var(--mobile-text); font-size: 12px; }
.search-empty { min-height: 62dvh; padding: 30px; display: grid; place-content: center; justify-items: center; gap: 8px; color: var(--mobile-text-muted); text-align: center; }.search-empty h1 { color: var(--mobile-text); font-size: 17px; }.search-empty p { max-width: 270px; font-size: 11px; line-height: 1.6; }.search-empty button { min-height: 42px; margin-top: 5px; padding: 0 18px; border-radius: 999px; background: var(--mobile-accent); color: #fff; }
.result-block { padding: 18px 14px 6px; }.result-block + .result-block { margin-top: 10px; padding-bottom: 90px; border-top: 1px solid var(--mobile-border); }
.result-block > header { margin-bottom: 10px; display: flex; align-items: end; justify-content: space-between; }.result-block header small { color: var(--mobile-accent); font-size: 7px; font-weight: 900; letter-spacing: .16em; }.result-block h2 { font-size: 17px; }.result-block header > span { min-width: 26px; height: 26px; display: grid; place-items: center; border-radius: 999px; background: var(--mobile-accent-dim); color: var(--mobile-accent); font-size: 10px; }
.user-result { min-height: 70px; padding: 10px; display: flex; align-items: center; gap: 8px; border: 1px solid var(--mobile-border); border-radius: 16px; background: var(--mobile-surface-raised); }.user-result + .user-result { margin-top: 8px; }
.identity { min-width: 0; flex: 1; display: flex; align-items: center; gap: 10px; text-align: left; }.identity > span { min-width: 0; }.identity b,.identity small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.identity b { font-size: 13px; }.identity small { margin-top: 3px; color: var(--mobile-text-muted); font-size: 9px; }
.message { min-height: 38px; padding: 0 12px; display: flex; align-items: center; gap: 5px; border-radius: 999px; background: var(--mobile-accent); color: #fff; font-size: 10px; font-weight: 800; }
.video-grid { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 8px; }.video-grid button { overflow: hidden; border: 1px solid var(--mobile-border); border-radius: 14px; background: var(--mobile-surface-raised); text-align: left; }.video-grid img { width: 100%; aspect-ratio: 16/10; display: block; object-fit: cover; }.video-grid button > span { padding: 9px; display: block; }.video-grid b,.video-grid small { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.video-grid b { font-size: 11px; }.video-grid small { margin-top: 3px; color: var(--mobile-text-muted); font-size: 8px; }
.no-result { padding: 28px 10px; border: 1px dashed var(--mobile-border); border-radius: 14px; color: var(--mobile-text-muted); text-align: center; font-size: 10px; }

.search-bar { min-height: calc(56px + env(safe-area-inset-top)); background: rgba(15,15,17,.96); }
form { height: 38px; border: 0; border-radius: 7px; background: var(--mobile-surface-raised); }
.search-empty button,
.message { border-radius: 7px; }
.result-block { padding: 16px 12px 6px; }
.user-result {
  min-height: 66px;
  padding: 9px 4px;
  border: 0;
  border-bottom: 1px solid var(--mobile-border);
  border-radius: 0;
  background: transparent;
}
.user-result + .user-result { margin-top: 0; }
.video-grid { grid-template-columns: repeat(2,minmax(0,1fr)); gap: 10px 6px; }
.video-grid button { border: 0; border-radius: 6px; background: var(--mobile-surface-raised); }
.video-grid img { aspect-ratio: 3/4; }
.video-grid button > span { padding: 7px 6px; }
.no-result { border-radius: 7px; }
</style>
