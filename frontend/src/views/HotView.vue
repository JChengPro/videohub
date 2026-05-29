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
    if (item.is_liked) await likeApi.unlike(item.id)
    else await likeApi.like(item.id)
    item.is_liked = !item.is_liked
    item.likes_count = Math.max(0, item.likes_count + (item.is_liked ? 1 : -1))
  } catch (e) { toast.error(e instanceof ApiError ? e.message : String(e)) }
  finally { likeBusy[key] = false }
}

onMounted(async () => { await loadHot(true) })
</script>

<template>
  <AppShell>
    <div class="card">
      <div class="row" style="justify-content:space-between;align-items:baseline">
        <div><p class="title" style="margin:0">热榜</p><p class="subtle" style="margin:4px 0 0">按互动热度排序</p></div>
        <div class="row">
          <input v-model.number="state.limit" type="number" min="1" max="50" style="width:80px" :disabled="state.loading" />
          <button class="primary" type="button" :disabled="state.loading" @click="loadHot(true)">刷新</button>
          <button type="button" :disabled="state.loading || !state.hasMore" @click="loadHot(false)">更多</button>
        </div>
      </div>
      <div v-if="state.error" style="margin-top:12px;color:var(--danger)">{{ state.error }}</div>
      <div v-else-if="state.loading && state.items.length===0" class="subtle" style="margin-top:12px">加载中…</div>
      <div v-if="state.items.length" style="margin-top:14px;display:grid;gap:12px">
        <div v-for="(item,idx) in state.items" :key="item.id" style="display:grid;grid-template-columns:40px 1fr;gap:10px;align-items:start">
          <div class="rank-num" :class="{ top3: idx<3 }">{{ idx+1 }}</div>
          <FeedVideoCard :item="item" :can-like="canLike" :busy="!!likeBusy[String(item.id)]" @toggle-like="toggleLike" />
        </div>
      </div>
    </div>
  </AppShell>
</template>

<style scoped>
.rank-num {
  height: 40px; width: 40px; border-radius: 10px;
  display: grid; place-items: center;
  font-weight: 900; font-size: 16px;
  background: var(--bg-card); border: 1px solid var(--border); color: var(--text-muted);
}
.rank-num.top3 { background: rgba(254,44,85,0.15); color: var(--accent); border-color: rgba(254,44,85,0.35); }
</style>
