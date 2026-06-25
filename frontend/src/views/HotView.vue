<script setup lang="ts">
import { computed, onMounted, reactive } from 'vue'

import { ApiError } from '../api/client'
import * as feedApi from '../api/feed'
import * as likeApi from '../api/like'
import type { FeedVideoItem } from '../api/types'
import AppShell from '../components/AppShell.vue'
import FeedVideoCard from '../components/FeedVideoCard.vue'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'

const auth = useAuthStore()
const toast = useToastStore()
const canLike = computed(() => auth.isLoggedIn)

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
          <svg viewBox="0 0 24 24" fill="none"><path d="M20 7v5h-5M4 17v-5h5"/><path d="M18.5 9A7 7 0 0 0 6.2 6.2L4 8m2 7a7 7 0 0 0 11.8 2.8L20 16"/></svg>
          {{ state.loading ? '更新中' : '刷新榜单' }}
        </button>
      </header>

      <section v-if="state.items.length >= 3" class="podium">
        <RouterLink v-for="(item, idx) in state.items.slice(0, 3)" :key="`top-${item.id}`" class="podium-card" :class="`place-${idx + 1}`" :to="`/video/${item.id}`">
          <img :src="item.cover_url" :alt="item.title" />
          <div class="podium-shade" />
          <span class="podium-rank">{{ idx + 1 }}</span>
          <div class="podium-copy">
            <strong>{{ item.title }}</strong>
            <span>@{{ item.author.username }} · {{ item.likes_count }} 赞</span>
          </div>
        </RouterLink>
      </section>

      <section class="ranking-panel">
        <div class="ranking-head">
          <div>
            <h2>完整榜单</h2>
            <span>当前展示 {{ state.items.length }} 条热视频</span>
          </div>
          <span class="ranking-rule">按综合互动热度排序</span>
        </div>

        <div v-if="state.error" class="state-hint error">{{ state.error }}</div>
        <div v-else-if="state.loading && state.items.length===0" class="state-hint">正在生成热榜...</div>
        <div v-else-if="state.items.length === 0" class="state-hint">暂无热视频</div>

        <div v-if="state.items.length" class="ranking-list">
          <article v-for="(item,idx) in state.items" :key="item.id" class="rank-row">
            <div class="rank-num" :class="{ top3: idx<3 }"><span>TOP</span>{{ idx+1 }}</div>
            <div class="rank-content">
              <FeedVideoCard :item="item" :can-like="canLike" :busy="!!likeBusy[String(item.id)]" @toggle-like="toggleLike" />
            </div>
          </article>
        </div>

        <button v-if="state.items.length" class="more-button" type="button" :disabled="state.loading || !state.hasMore" @click="loadHot(false)">
          {{ state.hasMore ? (state.loading ? '加载中...' : '查看更多热视频') : '已经到底了' }}
        </button>
      </section>
    </div>
  </AppShell>
</template>

<style scoped>
.hot-page { padding-bottom: 40px; }
.hot-header { min-height: 150px; padding: 28px 34px; border: 1px solid var(--border); border-radius: 14px; display: flex; align-items: center; justify-content: space-between; gap: 24px; background: radial-gradient(circle at 80% 20%, rgba(254,44,85,.16), transparent 32%), linear-gradient(135deg, var(--surface-raised), var(--surface-panel)); }
.hot-kicker { color: #fe2c55; font-size: 10px; font-weight: 900; letter-spacing: .18em; }
.hot-header h1 { margin-top: 10px; font-size: 32px; letter-spacing: -.05em; }
.hot-header p { margin-top: 7px; color: #858585; font-size: 12px; }
.refresh-button { height: 40px; padding: 0 16px; border: 1px solid rgba(255,255,255,.12); border-radius: 6px; display: inline-flex; align-items: center; gap: 8px; background: var(--surface-hover); color: #ddd; font-size: 12px; }
.refresh-button svg { width: 16px; stroke: currentColor; stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round; }
.podium { height: 310px; margin-top: 16px; display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; }
.podium-card { position: relative; overflow: hidden; border: 1px solid var(--border); border-radius: 12px; color: #fff; }
.podium-card.place-1 { grid-column: 2; grid-row: 1; transform: translateY(-7px); }
.podium-card.place-2 { grid-column: 1; grid-row: 1; }
.podium-card.place-3 { grid-column: 3; grid-row: 1; }
.podium-card img { width: 100%; height: 100%; object-fit: cover; transition: transform .3s ease; }
.podium-card:hover img { transform: scale(1.03); }
.podium-shade { position: absolute; inset: 0; background: linear-gradient(to top, rgba(0,0,0,.9), transparent 65%); }
.podium-rank { position: absolute; top: 14px; left: 14px; width: 34px; height: 34px; border-radius: 7px; display: grid; place-items: center; background: #fe2c55; color: #fff; font-size: 16px; font-weight: 900; box-shadow: 0 10px 25px rgba(254,44,85,.32); }
.place-2 .podium-rank, .place-3 .podium-rank { background: rgba(0,0,0,.68); box-shadow: none; }
.podium-copy { position: absolute; right: 16px; bottom: 15px; left: 16px; display: grid; gap: 5px; }
.podium-copy strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 14px; }
.podium-copy span { color: rgba(255,255,255,.65); font-size: 10px; }
.ranking-panel { margin-top: 18px; padding: 24px; border: 1px solid var(--border); border-radius: 14px; background: var(--surface-panel); }
.ranking-head { display: flex; align-items: center; justify-content: space-between; gap: 20px; padding-bottom: 20px; border-bottom: 1px solid var(--border); }
.ranking-head h2 { font-size: 18px; }
.ranking-head span { color: #777; font-size: 10px; }
.ranking-rule { padding: 6px 10px; border-radius: 999px; background: var(--surface-raised); }
.ranking-list { margin-top: 18px; display: grid; gap: 10px; }
.rank-row { display: grid; grid-template-columns: 50px 1fr; gap: 12px; align-items: center; }
.rank-num {
  height: 50px; width: 50px; border-radius: 8px;
  display: grid; place-items: center;
  align-content: center;
  font-weight: 900; font-size: 17px;
  background: var(--surface-raised); border: 1px solid var(--border); color: #85858b;
}
.rank-num span { display: block; color: #555; font-size: 7px; letter-spacing: .13em; }
.rank-num.top3 { background: rgba(254,44,85,0.1); color: var(--accent); border-color: rgba(254,44,85,0.28); }
.rank-content { min-width: 0; }
.more-button { width: 100%; height: 42px; margin-top: 18px; border-radius: 7px; background: var(--surface-raised); color: #aaa; font-size: 12px; }
.state-hint { padding: 70px 0; color: #777; text-align: center; font-size: 12px; }
.state-hint.error { color: #fe2c55; }
@media (max-width: 760px) { .hot-header { align-items: flex-start; flex-direction: column; padding: 24px; } .podium { height: auto; grid-template-columns: 1fr; } .podium-card, .podium-card.place-1, .podium-card.place-2, .podium-card.place-3 { grid-column: 1; grid-row: auto; height: 220px; transform: none; } .ranking-panel { padding: 16px; } .rank-row { grid-template-columns: 36px 1fr; } .rank-num { width: 36px; height: 42px; font-size: 13px; } }
</style>
