<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppShell from '../components/AppShell.vue'
import AppIcon from '../components/AppIcon.vue'
import UserAvatar from '../components/UserAvatar.vue'
import * as accountApi from '../api/account'
import * as feedApi from '../api/feed'
import { ApiError } from '../api/client'
import type { Account, FeedVideoItem } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const toast = useToastStore()
const query = computed(() => typeof route.query.q === 'string' ? route.query.q.trim() : '')
const myId = computed(() => auth.claims?.account_id ?? 0)

const state = reactive({
  loading: false,
  loadingMore: false,
  error: '',
  users: [] as Account[],
  videos: [] as FeedVideoItem[],
  hasMoreUsers: false,
  nextOffset: 0,
})
let searchRequest = 0

function matchesVideo(item: FeedVideoItem, keyword: string) {
  const needle = keyword.toLocaleLowerCase()
  return item.title.toLocaleLowerCase().includes(needle)
    || item.description?.toLocaleLowerCase().includes(needle)
    || item.author.username.toLocaleLowerCase().includes(needle)
}

async function search() {
  const keyword = query.value
  const request = ++searchRequest
  state.users = []
  state.videos = []
  state.error = ''
  state.hasMoreUsers = false
  state.nextOffset = 0
  if (!keyword) return

  state.loading = true
  try {
    const [users, feed] = await Promise.all([
      accountApi.searchUsers(keyword, 20, 0),
      feedApi.listLatest({ limit: 50, latest_time: 0 }),
    ])
    if (request !== searchRequest) return
    state.users = users.users ?? []
    state.hasMoreUsers = users.has_more
    state.nextOffset = users.next_offset
    state.videos = feed.video_list.filter((item) => matchesVideo(item, keyword))
  } catch (cause) {
    if (request === searchRequest) state.error = cause instanceof ApiError ? cause.message : String(cause)
  } finally {
    if (request === searchRequest) state.loading = false
  }
}

async function loadMoreUsers() {
  if (!query.value || !state.hasMoreUsers || state.loadingMore) return
  state.loadingMore = true
  try {
    const response = await accountApi.searchUsers(query.value, 20, state.nextOffset)
    state.users.push(...response.users)
    state.hasMoreUsers = response.has_more
    state.nextOffset = response.next_offset
  } catch (cause) {
    toast.error(cause instanceof ApiError ? cause.message : String(cause))
  } finally {
    state.loadingMore = false
  }
}

async function startChat(user: Account) {
  if (!auth.isLoggedIn) {
    toast.info('登录后才能发送私信')
    await router.push('/account')
    return
  }
  await router.push(`/messages/chat/${user.id}`)
}

watch(query, search, { immediate: true })
</script>

<template>
  <AppShell>
    <section class="search-head">
      <div>
        <p>SEARCH</p>
        <h1>搜索结果</h1>
      </div>
      <span v-if="query">“{{ query }}”</span>
    </section>

    <div v-if="!query" class="search-state">
      <AppIcon name="search" :size="42" />
      <h2>输入用户名、作者或视频标题</h2>
      <p>用户结果可以直接进入主页，登录后也可以发起私信。</p>
    </div>
    <div v-else-if="state.loading" class="search-state" role="status"><span class="spinner" />正在搜索…</div>
    <div v-else-if="state.error" class="search-state error" role="alert">
      <h2>搜索失败</h2><p>{{ state.error }}</p><button type="button" @click="search">重新搜索</button>
    </div>
    <template v-else>
      <section class="result-section">
        <header><div><h2>用户</h2><p>找到你想关注或私信的人</p></div><b>{{ state.users.length }}</b></header>
        <div v-if="state.users.length" class="user-grid">
          <article v-for="user in state.users" :key="user.id" class="user-card">
            <RouterLink class="user-link" :to="`/u/${user.id}`">
              <UserAvatar :username="user.username" :id="user.id" :size="54" />
              <div><strong>{{ user.username }}</strong><span>@{{ user.account_name }}</span></div>
            </RouterLink>
            <div class="user-actions">
              <RouterLink :to="`/u/${user.id}`">主页</RouterLink>
              <button v-if="user.id !== myId" type="button" @click="startChat(user)">
                <AppIcon name="message" :size="14" />
                私信
              </button>
            </div>
          </article>
        </div>
        <div v-else class="inline-empty">没有匹配的用户，试试更短的用户名关键词。</div>
        <button v-if="state.hasMoreUsers" class="load-more" type="button" :disabled="state.loadingMore" @click="loadMoreUsers">{{ state.loadingMore ? '加载中…' : '查看更多用户' }}</button>
      </section>

      <section class="result-section">
        <header><div><h2>视频</h2><p>标题、简介或作者匹配的作品</p></div><b>{{ state.videos.length }}</b></header>
        <div v-if="state.videos.length" class="video-grid">
          <article v-for="item in state.videos" :key="item.id" class="video-card">
            <RouterLink class="cover-link" :to="`/video/${item.id}`">
              <img :src="item.cover_url" :alt="item.title" loading="lazy" />
              <span>播放</span>
            </RouterLink>
            <div class="video-copy">
              <RouterLink :to="`/video/${item.id}`"><strong>{{ item.title }}</strong></RouterLink>
              <RouterLink class="author" :to="`/u/${item.author.id}`">{{ item.author.username }}</RouterLink>
              <div><span>♥ {{ item.likes_count }}</span><span>评论 {{ item.comments_count }}</span></div>
            </div>
          </article>
        </div>
        <div v-else class="inline-empty">没有匹配的视频。</div>
      </section>
    </template>
  </AppShell>
</template>

<style scoped>
.search-head { margin-bottom: 20px; display: flex; align-items: end; justify-content: space-between; }
.search-head p { color: var(--accent); font-size: 10px; font-weight: 900; letter-spacing: .18em; }
.search-head h1 { margin-top: 3px; font-size: 28px; letter-spacing: -.04em; }
.search-head > span { max-width: 55%; overflow: hidden; color: var(--text-secondary); font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }
.search-state { min-height: 55vh; display: grid; place-content: center; justify-items: center; gap: 10px; color: var(--text-secondary); text-align: center; }
.search-state svg { width: 42px; stroke: var(--accent); stroke-width: 1.5; stroke-linecap: round; }
.search-state h2 { color: var(--text); font-size: 18px; }.search-state p { font-size: 13px; }.search-state.error button { margin-top: 6px; background: var(--accent); color: #fff; }
.spinner { width: 22px; height: 22px; border: 2px solid var(--border); border-top-color: var(--accent); border-radius: 50%; animation: spin .7s linear infinite; }
.result-section { padding: 20px; border: 1px solid var(--border); border-radius: 20px; background: var(--surface-panel); }
.result-section + .result-section { margin-top: 16px; }
.result-section > header { margin-bottom: 15px; display: flex; align-items: center; justify-content: space-between; }
.result-section h2 { font-size: 18px; }.result-section header p { color: var(--text-muted); font-size: 12px; }.result-section header b { min-width: 30px; height: 30px; display: grid; place-items: center; border-radius: 999px; background: var(--accent-dim); color: var(--accent); font-size: 12px; }
.user-grid { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 10px; }
.user-card { padding: 14px; display: flex; align-items: center; gap: 12px; border: 1px solid var(--border); border-radius: 16px; background: var(--surface-raised); transition: border-color var(--duration-fast) ease, transform var(--duration-fast) ease; }
.user-card:hover { border-color: var(--border-hover); transform: translateY(-1px); }
.user-link { min-width: 0; flex: 1; display: flex; align-items: center; gap: 12px; }
.user-link div { min-width: 0; }.user-link strong { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.user-link span { display: block; margin-top: 3px; color: var(--text-muted); font-size: 10px; }
.user-actions { display: flex; align-items: center; gap: 6px; }.user-actions a,.user-actions button { min-height: 36px; padding: 0 12px; display: inline-flex; align-items: center; gap: 5px; border-radius: 10px; background: var(--surface-hover); color: var(--text-secondary); font-size: 12px; }.user-actions button { background: var(--accent); color: #fff; }.user-actions svg { width: 14px; stroke: currentColor; stroke-width: 1.8; }
.video-grid { display: grid; grid-template-columns: repeat(4,minmax(0,1fr)); gap: 12px; }.video-card { overflow: hidden; border: 1px solid var(--border); border-radius: 15px; background: var(--surface-raised); }.cover-link { position: relative; aspect-ratio: 16/10; display: block; overflow: hidden; background: #09090a; }.cover-link img { width: 100%; height: 100%; object-fit: cover; transition: transform var(--duration) ease; }.cover-link:hover img { transform: scale(1.025); }.cover-link > span { position: absolute; right: 8px; bottom: 8px; padding: 3px 7px; border-radius: 999px; background: rgba(0,0,0,.7); font-size: 9px; }
.video-copy { padding: 11px; }.video-copy > a strong { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 13px; }.video-copy .author { margin-top: 4px; display: block; color: var(--text-secondary); font-size: 11px; }.video-copy > div { margin-top: 8px; display: flex; gap: 12px; color: var(--text-muted); font-size: 10px; }
.inline-empty { padding: 30px 15px; border: 1px dashed var(--border); border-radius: 14px; color: var(--text-muted); font-size: 12px; text-align: center; }
.load-more { margin: 14px auto 0; display: block; border: 1px solid var(--border); }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 1000px) { .user-grid { grid-template-columns: 1fr; }.video-grid { grid-template-columns: repeat(2,minmax(0,1fr)); } }

.search-head { margin-bottom: 24px; padding: 4px 0 22px; border-bottom: 1px solid var(--border); }
.search-head h1 { font-size: 34px; font-weight: 850; }
.result-section {
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
}
.result-section + .result-section { margin-top: 34px; }
.result-section > header { padding-bottom: 12px; border-bottom: 1px solid var(--border); }
.user-grid { max-width: 820px; grid-template-columns: 1fr; gap: 2px; }
.user-card {
  padding: 12px;
  border-color: transparent;
  border-bottom: 1px solid var(--border);
  border-radius: 0;
  background: transparent;
}
.user-card:hover { transform: none; background: var(--surface-raised); }
.user-actions a,
.user-actions button { border-radius: 6px; }
.video-grid { grid-template-columns: repeat(5,minmax(0,1fr)); gap: 18px 8px; }
.video-card { border: 0; border-radius: 6px; background: transparent; }
.cover-link { aspect-ratio: 3/4; }
.inline-empty { border-style: solid; border-radius: 8px; background: var(--surface-panel); }
@media (max-width: 1100px) { .video-grid { grid-template-columns: repeat(4,minmax(0,1fr)); } }
@media (max-width: 760px) { .video-grid { grid-template-columns: repeat(2,minmax(0,1fr)); } }
</style>
