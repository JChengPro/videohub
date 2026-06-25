<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { ApiError } from '../api/client'
import * as notificationApi from '../api/notification'
import type { Notification, NotificationType } from '../api/notification'
import AppShell from '../components/AppShell.vue'
import { useAuthStore } from '../stores/auth'
import { useNotificationStore } from '../stores/notification'
import { useToastStore } from '../stores/toast'

type Filter = 'all' | NotificationType

const auth = useAuthStore()
const notificationStore = useNotificationStore()
const router = useRouter()
const toast = useToastStore()
const filter = ref<Filter>('all')

const state = reactive({
  loading: false,
  error: '',
  items: [] as Notification[],
  hasMore: false,
  nextBeforeId: 0,
})

const filters: Array<{ key: Filter; label: string }> = [
  { key: 'all', label: '全部消息' },
  { key: 'like', label: '点赞' },
  { key: 'comment', label: '评论' },
  { key: 'follow', label: '关注' },
]

const emptyText = computed(() => {
  if (filter.value === 'all') return '暂时还没有互动消息'
  return `暂时还没有${filters.find((item) => item.key === filter.value)?.label ?? ''}消息`
})

function messageTitle(item: Notification) {
  const actor = item.actor_username || `用户 #${item.actor_id}`
  if (item.type === 'like') return `${actor} 点赞了你的视频`
  if (item.type === 'comment') return `${actor} 评论了你的视频`
  return `${actor} 关注了你`
}

function messageDetail(item: Notification) {
  if (item.type === 'comment') return item.content
  if (item.type === 'like') return '你的作品获得了新的喜欢'
  return '你有了一位新的关注者'
}

function typeLabel(type: NotificationType) {
  if (type === 'like') return '赞'
  if (type === 'comment') return '评'
  return '关'
}

function formatTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

async function load(reset: boolean) {
  if (!auth.isLoggedIn || state.loading) return
  state.loading = true
  state.error = ''
  try {
    const res = await notificationApi.list({
      type: filter.value === 'all' ? undefined : filter.value,
      limit: 20,
      before_id: reset ? undefined : state.nextBeforeId,
    })
    state.items = reset ? res.notifications : state.items.concat(res.notifications)
    state.hasMore = res.has_more
    state.nextBeforeId = res.next_before_id
  } catch (e) {
    state.error = e instanceof ApiError ? e.message : String(e)
  } finally {
    state.loading = false
  }
}

async function selectFilter(next: Filter) {
  filter.value = next
  await load(true)
}

async function openNotification(item: Notification) {
  if (!item.is_read) {
    try {
      await notificationApi.markRead(item.id)
      item.is_read = true
      notificationStore.readOne()
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : String(e))
      return
    }
  }

  if (item.target_type === 'video') await router.push(`/video/${item.target_id}`)
  else await router.push(`/u/${item.actor_id}`)
}

async function markAllRead() {
  try {
    await notificationApi.markAllRead()
    state.items.forEach((item) => { item.is_read = true })
    notificationStore.readAll()
    toast.success('全部消息已读')
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e))
  }
}

onMounted(async () => {
  if (auth.isLoggedIn) {
    await Promise.all([load(true), notificationStore.refreshUnread()])
  }
})
</script>

<template>
  <AppShell>
    <section class="messages-page">
      <header class="messages-header">
        <div>
          <span class="eyebrow">ACTIVITY</span>
          <h1>消息中心</h1>
          <p>查看点赞、评论和关注动态</p>
        </div>
        <button v-if="auth.isLoggedIn" type="button" :disabled="notificationStore.unread === 0" @click="markAllRead">
          全部已读
        </button>
      </header>

      <div v-if="!auth.isLoggedIn" class="empty-panel">
        <strong>登录后查看消息</strong>
        <p>互动通知只对当前账号可见。</p>
        <RouterLink class="login-link" to="/account">去登录</RouterLink>
      </div>

      <template v-else>
        <nav class="filters">
          <button
            v-for="item in filters"
            :key="item.key"
            type="button"
            :class="{ active: filter === item.key }"
            @click="selectFilter(item.key)"
          >
            {{ item.label }}
          </button>
        </nav>

        <div class="message-panel">
          <div v-if="state.error" class="empty-panel error">{{ state.error }}</div>
          <div v-else-if="state.loading && state.items.length === 0" class="empty-panel">正在读取消息...</div>
          <div v-else-if="state.items.length === 0" class="empty-panel">{{ emptyText }}</div>

          <button
            v-for="item in state.items"
            v-else
            :key="item.id"
            class="message-row"
            :class="{ unread: !item.is_read }"
            type="button"
            @click="openNotification(item)"
          >
            <span class="type-icon" :class="item.type">{{ typeLabel(item.type) }}</span>
            <span class="message-copy">
              <strong>{{ messageTitle(item) }}</strong>
              <span>{{ messageDetail(item) }}</span>
            </span>
            <span class="message-meta">
              <i v-if="!item.is_read" />
              {{ formatTime(item.create_time) }}
            </span>
          </button>

          <button
            v-if="state.items.length > 0"
            class="load-more"
            type="button"
            :disabled="state.loading || !state.hasMore"
            @click="load(false)"
          >
            {{ state.hasMore ? (state.loading ? '加载中...' : '查看更多') : '已经到底了' }}
          </button>
        </div>
      </template>
    </section>
  </AppShell>
</template>

<style scoped>
.messages-page { max-width: 900px; margin: 0 auto; padding-bottom: 40px; }
.messages-header { min-height: 142px; padding: 28px 32px; border: 1px solid var(--border); border-radius: 14px; display: flex; align-items: center; justify-content: space-between; gap: 20px; background: radial-gradient(circle at 85% 20%, rgba(32,213,236,.1), transparent 30%), linear-gradient(135deg, var(--surface-raised), var(--surface-panel)); }
.eyebrow { color: var(--accent-cyan); font-size: 10px; font-weight: 900; letter-spacing: .2em; }
.messages-header h1 { margin-top: 9px; font-size: 30px; letter-spacing: -.045em; }
.messages-header p { margin-top: 6px; color: var(--text-secondary); font-size: 12px; }
.messages-header button { border: 1px solid var(--border); background: var(--surface-hover); font-size: 12px; }
.filters { margin: 16px 0 10px; display: flex; gap: 6px; }
.filters button { padding: 8px 14px; border-radius: 999px; background: transparent; color: var(--text-secondary); font-size: 12px; }
.filters button.active { background: #fff; color: #111; font-weight: 700; }
.message-panel { overflow: hidden; border: 1px solid var(--border); border-radius: 14px; background: var(--surface-panel); }
.message-row { width: 100%; min-height: 88px; padding: 16px 20px; border-bottom: 1px solid var(--border); border-radius: 0; display: grid; grid-template-columns: 46px minmax(0, 1fr) auto; align-items: center; gap: 14px; background: transparent; text-align: left; }
.message-row:hover { background: var(--surface-raised); }
.message-row.unread { background: linear-gradient(90deg, rgba(254,44,85,.08), transparent 55%); }
.type-icon { width: 42px; height: 42px; border-radius: 50%; display: grid; place-items: center; background: var(--surface-strong); color: #fff; font-size: 12px; font-weight: 800; }
.type-icon.like { background: rgba(254,44,85,.18); color: var(--accent); }
.type-icon.comment { background: rgba(32,213,236,.14); color: var(--accent-cyan); }
.type-icon.follow { background: rgba(255,255,255,.1); }
.message-copy { min-width: 0; display: grid; gap: 5px; }
.message-copy strong { overflow: hidden; color: #eee; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.message-copy span { overflow: hidden; color: var(--text-secondary); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.message-meta { display: flex; align-items: center; gap: 7px; color: var(--text-muted); font-size: 10px; white-space: nowrap; }
.message-meta i { width: 6px; height: 6px; border-radius: 50%; background: var(--accent); box-shadow: 0 0 10px rgba(254,44,85,.6); }
.load-more { width: 100%; height: 44px; border-radius: 0; background: transparent; color: var(--text-secondary); font-size: 11px; }
.empty-panel { min-height: 220px; padding: 50px 20px; display: grid; place-items: center; align-content: center; gap: 8px; color: var(--text-secondary); text-align: center; }
.empty-panel strong { color: #eee; font-size: 16px; }
.empty-panel p { font-size: 12px; }
.empty-panel.error { color: var(--accent); }
.login-link { margin-top: 8px; padding: 8px 18px; border-radius: 6px; background: var(--accent); color: #fff; font-size: 12px; }
@media (max-width: 700px) { .messages-header { padding: 22px; align-items: flex-start; flex-direction: column; } .message-row { grid-template-columns: 42px minmax(0,1fr); padding: 14px; } .message-meta { grid-column: 2; } }
</style>
