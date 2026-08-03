<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { api } from '../api'
import type { Notification } from '../api/types'
import AppIcon from '../components/AppIcon.vue'
import Avatar from '../components/Avatar.vue'
import { useAuthStore } from '../stores/auth'
import { useNotificationStore } from '../stores/notification'
import { useToastStore } from '../stores/toast'
import type { RealtimeEvent } from '../stores/realtime'

const auth = useAuthStore()
const notifications = useNotificationStore()
const toast = useToastStore()
const router = useRouter()

const items = ref<Notification[]>([])
const loading = ref(false)
const loadingMore = ref(false)
const error = ref('')
const hasMore = ref(false)
const nextBeforeId = ref(0)
const opening = ref(new Set<number>())
const markAllBusy = ref(false)
let requestId = 0

function text(item: Notification) {
  if (item.type === 'like') return '赞了你的视频'
  if (item.type === 'comment') return `评论了你的视频：${item.content}`
  return '关注了你'
}

function formatDate(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString()
}

function mergeUnique(current: Notification[], incoming: Notification[]) {
  const seen = new Set(current.map((item) => item.id))
  return current.concat(incoming.filter((item) => !seen.has(item.id)))
}

async function load(reset: boolean) {
  if (!auth.isLoggedIn) {
    requestId += 1
    items.value = []
    loading.value = false
    loadingMore.value = false
    error.value = ''
    hasMore.value = false
    nextBeforeId.value = 0
    return
  }
  if (!reset && (loadingMore.value || !hasMore.value)) return

  const request = reset ? ++requestId : requestId
  if (reset) {
    loading.value = true
    error.value = ''
    nextBeforeId.value = 0
  } else {
    loadingMore.value = true
    error.value = ''
  }

  try {
    const response = await api.notifications(reset ? 0 : nextBeforeId.value)
    if (request !== requestId) return
    items.value = reset ? response.notifications : mergeUnique(items.value, response.notifications)
    hasMore.value = response.has_more
    nextBeforeId.value = response.next_before_id
  } catch (cause) {
    if (request === requestId) error.value = cause instanceof Error ? cause.message : String(cause)
  } finally {
    if (request === requestId) {
      loading.value = false
      loadingMore.value = false
    }
  }
}

async function open(item: Notification) {
  if (opening.value.has(item.id)) return
  opening.value.add(item.id)
  try {
    if (!item.is_read) {
      await api.markRead(item.id)
      item.is_read = true
      notifications.readOne()
    }
    if (item.target_type === 'video') await router.push(`/video/${item.target_id}`)
    else await router.push(`/user/${item.actor_id}`)
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : String(cause))
  } finally {
    opening.value.delete(item.id)
  }
}

async function openActor(item: Notification) {
  await router.push(`/user/${item.actor_id}`)
}

async function markAll() {
  if (markAllBusy.value) return
  markAllBusy.value = true
  try {
    await api.markAllRead()
    items.value.forEach((item) => (item.is_read = true))
    notifications.readAll()
    toast.success('已全部标记为已读')
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : String(cause))
  } finally {
    markAllBusy.value = false
  }
}

function onRealtime(raw: Event) {
  const event = (raw as CustomEvent<RealtimeEvent>).detail
  if (event.type === 'notification.new') void load(true)
}

watch(() => auth.isLoggedIn, () => void load(true))

onMounted(() => {
  window.addEventListener('videohub:realtime', onRealtime)
  void load(true)
  void notifications.refresh()
})

onBeforeUnmount(() => window.removeEventListener('videohub:realtime', onRealtime))
</script>

<template>
  <main class="page messages-page">
    <header class="page-header">
      <div><small>ACTIVITY</small><h1>消息</h1></div>
      <button
        v-if="auth.isLoggedIn && items.some((item) => !item.is_read)"
        class="text-action"
        type="button"
        :disabled="markAllBusy"
        @click="markAll"
      >
        {{ markAllBusy ? '处理中' : '全部已读' }}
      </button>
    </header>

    <nav class="message-tabs">
      <button class="active" type="button">互动通知</button>
      <button type="button" @click="router.push('/chat')">私信</button>
    </nav>

    <section v-if="!auth.isLoggedIn" class="empty">
      <AppIcon name="message" :size="38" />
      <h2>登录后查看消息</h2>
      <p>点赞、评论和关注通知会集中显示在这里</p>
      <button type="button" @click="router.push('/me')">立即登录</button>
    </section>

    <section v-else-if="loading" class="empty" role="status">
      <span class="message-loader" />
      <p>正在加载消息</p>
    </section>

    <section v-else-if="error && !items.length" class="empty" role="alert">
      <AppIcon name="warning" :size="36" />
      <h2>消息加载失败</h2>
      <p>{{ error }}</p>
      <button type="button" @click="load(true)">重新加载</button>
    </section>

    <section v-else-if="!items.length" class="empty">
      <AppIcon name="message" :size="38" />
      <h2>暂时没有新消息</h2>
      <p>有新的互动时，我们会在这里提醒你</p>
    </section>

    <section v-else class="message-list" aria-label="互动消息">
      <article v-for="item in items" :key="item.id" :class="{ unread: !item.is_read }">
        <button class="actor" type="button" :aria-label="`查看 ${item.actor_username} 的主页`" @click="openActor(item)">
          <Avatar :name="item.actor_username" :id="item.actor_id" :size="46" />
        </button>
        <button class="message-copy" type="button" :disabled="opening.has(item.id)" @click="open(item)">
          <b>{{ item.actor_username }}</b>
          <p>{{ text(item) }}</p>
          <small>{{ formatDate(item.create_time) }}</small>
        </button>
        <i v-if="!item.is_read" aria-label="未读" />
      </article>

      <div v-if="error" class="inline-error" role="alert">{{ error }}</div>
      <button v-if="hasMore" class="load-more" type="button" :disabled="loadingMore" @click="load(false)">
        {{ loadingMore ? '加载中...' : '查看更多消息' }}
      </button>
      <p v-else class="list-end">已经到底了</p>
    </section>
  </main>
</template>

<style scoped>
.messages-page {
  background: var(--mobile-surface);
}

.message-tabs { padding: 8px 14px; display: grid; grid-template-columns: 1fr 1fr; gap: 7px; }
.message-tabs button { min-height: 38px; border-radius: 999px; background: var(--mobile-surface-raised); color: var(--mobile-text-muted); font-size: 11px; }
.message-tabs button.active { background: var(--mobile-text); color: var(--mobile-bg); font-weight: 800; }

.message-list {
  padding: 6px 14px calc(88px + env(safe-area-inset-bottom));
}

.message-list article {
  position: relative;
  width: 100%;
  min-height: 78px;
  padding: 13px 4px;
  display: grid;
  grid-template-columns: 46px minmax(0, 1fr) 12px;
  align-items: center;
  gap: 12px;
  text-align: left;
  border-bottom: 1px solid var(--mobile-border);
}

.message-list article.unread {
  background: linear-gradient(90deg, var(--mobile-accent-dim), transparent 70%);
}

.actor {
  width: 46px;
  height: 46px;
}

.message-copy {
  min-width: 0;
  min-height: 52px;
  display: block;
  text-align: left;
}

.message-list b {
  font-size: 13px;
}

.message-list p {
  margin-top: 4px;
  overflow: hidden;
  color: var(--mobile-text-secondary);
  font-size: 12px;
  line-height: 1.45;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.message-list small {
  display: block;
  margin-top: 4px;
  color: var(--mobile-text-muted);
  font-size: 9px;
}

.message-list i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--mobile-accent);
}

.load-more {
  width: 100%;
  min-height: 44px;
  margin-top: 14px;
  border-radius: var(--mobile-radius);
  background: var(--mobile-surface-raised);
  color: var(--mobile-text-secondary);
  font-size: 11px;
}

.list-end,
.inline-error {
  padding: 18px;
  color: var(--mobile-text-muted);
  text-align: center;
  font-size: 10px;
}

.inline-error {
  color: var(--mobile-danger);
}

.message-loader {
  width: 28px;
  height: 28px;
  border: 3px solid rgba(255, 255, 255, .14);
  border-top-color: var(--mobile-text);
  border-radius: 50%;
  animation: spin .8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.message-tabs {
  padding: 0 14px;
  gap: 18px;
  border-bottom: 1px solid var(--mobile-border);
}
.message-tabs button {
  position: relative;
  min-height: 45px;
  border-radius: 0;
  background: transparent;
}
.message-tabs button.active { background: transparent; color: var(--mobile-text); }
.message-tabs button.active::after {
  position: absolute;
  right: 28%;
  bottom: 0;
  left: 28%;
  height: 2px;
  background: var(--mobile-text);
  content: '';
}
.message-list { padding-top: 2px; }
.message-list article { min-height: 74px; padding: 11px 2px; }
.message-list article.unread { background: transparent; }
.message-list article.unread::before {
  position: absolute;
  top: 15px;
  left: -8px;
  width: 2px;
  height: 36px;
  background: var(--mobile-accent);
  content: '';
}
</style>
