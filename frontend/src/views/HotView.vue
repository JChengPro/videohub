<script setup lang="ts">
import { onMounted, reactive } from 'vue'

import { ApiError } from '../api/client'
import * as feedApi from '../api/feed'
import * as likeApi from '../api/like'
import type { FeedVideoItem } from '../api/types'
import AppShell from '../components/AppShell.vue'
import AppIcon from '../components/AppIcon.vue'
import UserAvatar from '../components/UserAvatar.vue'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'

const auth = useAuthStore()
const toast = useToastStore()
const state = reactive({
  loading: false, error: '',
  items: [] as FeedVideoItem[], hasMore: false,
  limit: 10, asOf: 0, nextOffset: 0,
})

const likeBusy = reactive<Record<string, boolean>>({})

async function loadHot(reset: boolean) {
  if (state.loading) return
  state.loading = true; state.error = ''
  try {
    const res = await feedApi.listByPopularity({ limit: state.limit, as_of: reset ? 0 : state.asOf, offset: reset ? 0 : state.nextOffset })
    state.hasMore = res.has_more; state.asOf = res.as_of; state.nextOffset = res.next_offset
    state.items = reset ? res.video_list : state.items.concat(res.video_list)
  } catch (e) { state.error = e instanceof ApiError ? e.message : String(e) }
  finally { state.loading = false }
}

async function toggleLike(item: FeedVideoItem) {
  if (!auth.isLoggedIn) { toast.error('请先登录'); return }
  const key = String(item.id)
  if (likeBusy[key]) return
  likeBusy[key] = true
  try {
    const state = item.is_liked ? await likeApi.unlike(item.id) : await likeApi.like(item.id)
    item.is_liked = state.is_liked
    item.likes_count = Math.max(0, state.likes_count)
  } catch (e) { toast.error(e instanceof ApiError ? e.message : String(e)) }
  finally { likeBusy[key] = false }
}

onMounted(async () => { await loadHot(true) })
</script>

<template>
  <AppShell>
    <div class="hot-page">
      <header class="hot-header">
        <div>
          <span class="hot-kicker">VIDEOHUB TRENDING</span>
          <h1>视频热榜</h1>
          <p>根据点赞、评论等互动热度实时排序</p>
        </div>
        <button class="refresh-button" type="button" :disabled="state.loading" @click="loadHot(true)">
          <AppIcon name="refresh" :size="16" />
          {{ state.loading ? '更新中' : '刷新榜单' }}
        </button>
      </header>

      <div class="explore-heading">
        <div><h2>热门作品</h2><span>按当前互动热度排序</span></div>
        <b>{{ state.items.length }}</b>
      </div>

      <div v-if="state.error" class="state-hint error">{{ state.error }}</div>
      <div v-else-if="state.loading && state.items.length===0" class="state-hint">正在加载热门作品...</div>
      <div v-else-if="state.items.length === 0" class="state-hint">暂无热视频</div>

      <section v-if="state.items.length" class="explore-grid">
        <article v-for="(item, idx) in state.items" :key="item.id" class="explore-card">
          <RouterLink class="cover" :to="`/video/${item.id}`">
            <img :src="item.cover_url" :alt="item.title" loading="lazy" />
            <span v-if="idx < 3" class="rank">TOP {{ idx + 1 }}</span>
            <span class="cover-stats"><AppIcon name="heart" :size="13" filled />{{ item.likes_count }}</span>
          </RouterLink>
          <RouterLink class="video-title" :to="`/video/${item.id}`">{{ item.title }}</RouterLink>
          <div class="card-foot">
            <RouterLink class="creator" :to="`/u/${item.author.id}`">
              <UserAvatar :username="item.author.username" :id="item.author.id" :size="28" />
              <span>{{ item.author.username }}</span>
            </RouterLink>
            <button
              type="button"
              :class="{ liked: item.is_liked }"
              :disabled="!!likeBusy[String(item.id)]"
              :aria-label="item.is_liked ? '取消点赞' : '点赞'"
              @click="toggleLike(item)"
            >
              <AppIcon name="heart" :size="17" :filled="item.is_liked" />
              {{ item.likes_count }}
            </button>
          </div>
        </article>
      </section>

      <button v-if="state.items.length" class="more-button" type="button" :disabled="state.loading || !state.hasMore" @click="loadHot(false)">
        {{ state.hasMore ? (state.loading ? '加载中...' : '加载更多') : '已经到底了' }}
      </button>
    </div>
  </AppShell>
</template>

<style scoped>
.hot-page { max-width: 1180px; margin: 0 auto; padding-bottom: 42px; }
.hot-header { min-height: 118px; padding: 16px 2px 22px; border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; gap: 20px; }
.hot-kicker { color: var(--accent); font-size: 9px; font-weight: 900; letter-spacing: .18em; }
.hot-header h1 { margin-top: 6px; font-size: 36px; font-weight: 850; letter-spacing: -.05em; }
.hot-header p { margin-top: 5px; color: var(--text-muted); font-size: 12px; }
.refresh-button { height: 38px; padding: 0 14px; border: 1px solid var(--border); border-radius: 5px; display: inline-flex; align-items: center; gap: 7px; background: transparent; color: var(--text-secondary); font-size: 11px; font-weight: 700; }
.refresh-button:hover { background: var(--surface-raised); color: #fff; }
.explore-heading { padding: 22px 0 14px; display: flex; align-items: flex-end; justify-content: space-between; }
.explore-heading h2 { font-size: 19px; }.explore-heading span { color: var(--text-muted); font-size: 10px; }.explore-heading b { color: var(--text-muted); font-size: 11px; }
.explore-grid { display: grid; grid-template-columns: repeat(5,minmax(0,1fr)); gap: 22px 10px; }
.explore-card { min-width: 0; }
.cover { position: relative; aspect-ratio: 3/4; overflow: hidden; display: block; border-radius: 6px; background: #08080a; }
.cover img { width: 100%; height: 100%; display: block; object-fit: cover; transition: transform .2s ease; }.cover:hover img { transform: scale(1.02); }
.cover::after { position: absolute; inset: auto 0 0; height: 34%; background: linear-gradient(transparent,rgba(0,0,0,.72)); content: ''; }
.rank { position: absolute; z-index: 2; top: 8px; left: 8px; padding: 4px 7px; border-radius: 4px; background: var(--accent); color: #fff; font-size: 8px; font-weight: 900; }
.cover-stats { position: absolute; z-index: 2; bottom: 8px; left: 8px; display: flex; align-items: center; gap: 4px; color: #fff; font-size: 10px; font-weight: 750; }
.video-title { margin-top: 8px; overflow: hidden; display: -webkit-box; color: #eee; font-size: 13px; font-weight: 700; line-height: 1.4; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.video-title:hover { color: #fff; }
.card-foot { margin-top: 8px; display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.creator { min-width: 0; display: flex; align-items: center; gap: 6px; color: var(--text-muted); font-size: 10px; }.creator span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.card-foot > button { padding: 4px; display: inline-flex; align-items: center; gap: 4px; background: transparent; color: var(--text-muted); font-size: 9px; }.card-foot > button:hover,.card-foot > button.liked { color: var(--accent); }
.more-button { width: 100%; height: 42px; margin-top: 24px; border-radius: 5px; background: var(--surface-raised); color: var(--text-secondary); font-size: 11px; }
.state-hint { padding: 100px 0; color: var(--text-muted); text-align: center; font-size: 12px; }.state-hint.error { color: var(--accent); }
@media (max-width: 1100px) { .explore-grid { grid-template-columns: repeat(4,minmax(0,1fr)); } }
@media (max-width: 760px) { .hot-header { align-items: flex-start; flex-direction: column; }.explore-grid { grid-template-columns: repeat(2,minmax(0,1fr)); } }
</style>
