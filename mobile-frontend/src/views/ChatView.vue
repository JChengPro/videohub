<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { api } from '../api'
import type { ChatMessage, Conversation, SendMessageResponse } from '../api/types'
import AppIcon from '../components/AppIcon.vue'
import Avatar from '../components/Avatar.vue'
import { useAuthStore } from '../stores/auth'
import { useChatStore } from '../stores/chat'
import { useRealtimeStore, type RealtimeEvent } from '../stores/realtime'
import { useToastStore } from '../stores/toast'

const auth = useAuthStore()
const chatStore = useChatStore()
const realtime = useRealtimeStore()
const toast = useToastStore()
const route = useRoute()
const router = useRouter()

const conversations = ref<Conversation[]>([])
const messages = ref<ChatMessage[]>([])
const peerUsername = ref('')
const content = ref('')
const loading = ref(false)
const sending = ref(false)
const hasMore = ref(false)
const nextBeforeId = ref(0)
const readMessageId = ref(0)
const history = ref<HTMLElement | null>(null)
const peerId = computed(() => Number(route.params.peerId || 0))
const myId = computed(() => auth.claims?.account_id ?? 0)
const conversation = computed(() => conversations.value.find((item) => item.peer_id === peerId.value) ?? null)

const statusText = computed(() => {
  const item = conversation.value
  if (!item) return '发送第一条消息后创建会话'
  if (item.status === 'blocked') return '当前用户已被拉黑'
  if (item.status === 'rejected') return '消息请求已被拒绝'
  if (item.status === 'mutual') return '已互相关注'
  if (item.status === 'accepted') return '消息请求已接受'
  if (item.request_sender_id === myId.value) return `对方回复前还可发送 ${item.remaining_request_messages} 条`
  return '回复后将自动接受消息请求'
})

function clientMessageId() {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID()
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function formatTime(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString()
}

async function loadConversations() {
  if (!auth.isLoggedIn) return
  try {
    conversations.value = (await api.conversations()).conversations
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : String(cause))
  }
}

async function loadPeer() {
  peerUsername.value = conversation.value?.peer_username ?? ''
  if (!peerUsername.value && peerId.value) {
    try { peerUsername.value = (await api.accountById(peerId.value)).username }
    catch { peerUsername.value = `用户 #${peerId.value}` }
  }
}

async function loadMessages(reset: boolean) {
  const item = conversation.value
  if (!item) {
    messages.value = []
    hasMore.value = false
    nextBeforeId.value = 0
    await loadPeer()
    return
  }
  if (!reset && (!hasMore.value || loading.value)) return
  loading.value = true
  try {
    const response = await api.chatMessages(item.id, reset ? 0 : nextBeforeId.value)
    const current = reset ? [] : messages.value
    const map = new Map(current.map((message) => [message.id, message]))
    response.messages.forEach((message) => map.set(message.id, message))
    messages.value = [...map.values()].sort((a, b) => a.id - b.id)
    hasMore.value = response.has_more
    nextBeforeId.value = response.next_before_id
    if (reset) await scrollBottom()
    if (item.unread_count) await markRead()
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : String(cause))
  } finally {
    loading.value = false
  }
}

async function scrollBottom() {
  await nextTick()
  if (history.value) history.value.scrollTop = history.value.scrollHeight
}

async function markRead() {
  const item = conversation.value
  if (!item) return
  try {
    await api.markChatRead(item.id, messages.value[messages.value.length - 1]?.id ?? 0)
    item.unread_count = 0
    await chatStore.refresh()
  } catch {
    // 不阻塞阅读和后续发送。
  }
}

async function send() {
  const text = content.value.trim()
  if (!text || !peerId.value || sending.value) return
  sending.value = true
  const request = { receiver_id: peerId.value, client_message_id: clientMessageId(), content: text }
  try {
    const response = realtime.connected
      ? await realtime.send<SendMessageResponse>('chat.send', request)
      : await api.sendChatMessage(request.receiver_id, request.client_message_id, request.content)
    const map = new Map(messages.value.map((message) => [message.id, message]))
    map.set(response.message.id, response.message)
    messages.value = [...map.values()].sort((a, b) => a.id - b.id)
    content.value = ''
    await Promise.all([loadConversations(), scrollBottom()])
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : String(cause))
  } finally {
    sending.value = false
  }
}

async function accept() {
  if (!conversation.value) return
  try {
    await api.acceptConversation(conversation.value.id)
    toast.success('已接受消息请求')
    await loadConversations()
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
}

async function reject() {
  if (!conversation.value) return
  try {
    await api.rejectConversation(conversation.value.id)
    toast.info('已拒绝消息请求')
    await loadConversations()
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
}

async function toggleBlock() {
  if (!conversation.value) return
  try {
    if (conversation.value.blocked_by_me) await api.unblockUser(conversation.value.peer_id)
    else await api.blockUser(conversation.value.peer_id)
    await loadConversations()
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
}

function onRealtime(raw: Event) {
  const event = (raw as CustomEvent<RealtimeEvent>).detail
  if (event.type === 'chat.new_message') {
    const response = event.data as SendMessageResponse
    if (response?.message?.conversation_id === conversation.value?.id) {
      if (!messages.value.some((item) => item.id === response.message.id)) messages.value.push(response.message)
      void Promise.all([scrollBottom(), markRead()])
    }
    void loadConversations()
  } else if (event.type === 'chat.conversation_changed') {
    void loadConversations()
  } else if (event.type === 'chat.read_receipt') {
    const data = event.data as { conversation_id: number; message_id: number }
    if (data.conversation_id === conversation.value?.id) readMessageId.value = data.message_id
  } else if (event.type === 'notification.new') {
    const data = event.data as { type?: string } | undefined
    if (data?.type === 'follow') void loadConversations()
  }
}

watch(peerId, async () => {
  messages.value = []
  readMessageId.value = 0
  await loadPeer()
  await loadMessages(true)
})

watch(() => auth.isLoggedIn, async (loggedIn) => {
  if (!loggedIn) await router.push('/me')
  else await loadConversations()
})

onMounted(async () => {
  if (!auth.isLoggedIn) {
    await router.push('/me')
    return
  }
  window.addEventListener('videohub:realtime', onRealtime)
  await loadConversations()
  await loadPeer()
  await loadMessages(true)
})

onBeforeUnmount(() => window.removeEventListener('videohub:realtime', onRealtime))
</script>

<template>
  <main class="page chat-page">
    <header class="topbar">
      <button type="button" aria-label="返回" @click="peerId ? router.push('/chat') : router.push('/messages')"><AppIcon name="back" /></button>
      <b>{{ peerId ? `@${peerUsername}` : '私信' }}</b>
      <button
        v-if="conversation && (!conversation.blocked_by_peer || conversation.blocked_by_me)"
        type="button"
        aria-label="拉黑设置"
        @click="toggleBlock"
      >
        {{ conversation.blocked_by_me ? '解除' : '拉黑' }}
      </button>
      <span v-else />
    </header>

    <template v-if="!peerId">
      <nav class="message-tabs">
        <button type="button" @click="router.push('/messages')">互动通知</button>
        <button class="active" type="button">私信</button>
      </nav>
      <section v-if="!conversations.length" class="empty">
        <AppIcon name="message" :size="38" />
        <h2>暂时没有私信</h2>
        <p>可以从其他用户的主页发起聊天</p>
      </section>
      <section v-else class="conversation-list">
        <button v-for="item in conversations" :key="item.id" type="button" @click="router.push(`/chat/${item.peer_id}`)">
          <Avatar :name="item.peer_username" :id="item.peer_id" :size="48" />
          <span><b>{{ item.peer_username }}</b><small>{{ item.last_message_content }}</small></span>
          <i v-if="item.unread_count">{{ item.unread_count > 99 ? '99+' : item.unread_count }}</i>
        </button>
      </section>
    </template>

    <template v-else>
      <section class="policy">
        <span>{{ statusText }}</span>
        <div v-if="conversation?.status === 'pending' && conversation.request_sender_id !== myId">
          <button type="button" @click="accept">接受</button>
          <button type="button" @click="reject">拒绝</button>
        </div>
      </section>

      <section ref="history" class="history">
        <button v-if="hasMore" type="button" :disabled="loading" @click="loadMessages(false)">{{ loading ? '加载中' : '更早消息' }}</button>
        <p v-if="!messages.length">还没有消息，打个招呼吧</p>
        <article v-for="item in messages" :key="item.id" :class="{ mine: item.sender_id === myId }">
          <span>{{ item.content }}</span>
          <small>{{ formatTime(item.created_at) }}<template v-if="item.sender_id === myId && item.id <= readMessageId"> · 已读</template></small>
        </article>
      </section>

      <form class="composer" @submit.prevent="send">
        <textarea v-model="content" maxlength="1000" :disabled="conversation ? !conversation.can_send : false" placeholder="输入消息" @keydown.enter.exact.prevent="send" />
        <button type="submit" :disabled="sending || !content.trim() || (conversation ? !conversation.can_send : false)">发送</button>
      </form>
    </template>
  </main>
</template>

<style scoped>
.chat-page { min-height: 100dvh; background: var(--mobile-surface); }.topbar { position: sticky; z-index: 20; top: 0; min-height: calc(54px + env(safe-area-inset-top)); padding: env(safe-area-inset-top) 8px 0; display: grid; grid-template-columns: 52px 1fr 52px; align-items: center; border-bottom: 1px solid var(--mobile-border); background: rgba(20,20,23,.94); }.topbar button { min-height: 44px; display: grid; place-items: center; color: var(--mobile-text-secondary); font-size: 10px; }.topbar b { overflow: hidden; text-align: center; text-overflow: ellipsis; white-space: nowrap; font-size: 14px; }
.message-tabs { padding: 10px 14px; display: grid; grid-template-columns: 1fr 1fr; gap: 7px; }.message-tabs button { min-height: 38px; border-radius: 999px; background: var(--mobile-surface-raised); color: var(--mobile-text-muted); font-size: 11px; }.message-tabs button.active { background: var(--mobile-text); color: var(--mobile-bg); font-weight: 800; }
.conversation-list { padding: 4px 14px calc(80px + env(safe-area-inset-bottom)); }.conversation-list > button { width: 100%; min-height: 76px; padding: 12px 3px; border-bottom: 1px solid var(--mobile-border); display: grid; grid-template-columns: 48px minmax(0,1fr) auto; align-items: center; gap: 12px; text-align: left; }.conversation-list span { min-width: 0; display: grid; gap: 5px; }.conversation-list b,.conversation-list small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.conversation-list b { font-size: 13px; }.conversation-list small { color: var(--mobile-text-muted); font-size: 10px; }.conversation-list i { min-width: 19px; height: 19px; padding: 0 5px; border-radius: 10px; display: grid; place-items: center; background: var(--mobile-accent); color: #fff; font-size: 8px; font-style: normal; }
.policy { min-height: 45px; padding: 8px 13px; display: flex; align-items: center; justify-content: space-between; gap: 8px; background: var(--mobile-surface-raised); color: var(--mobile-text-muted); font-size: 10px; }.policy div { display: flex; gap: 5px; }.policy button { padding: 7px 10px; border-radius: 999px; background: var(--mobile-surface-strong); color: var(--mobile-text-secondary); font-size: 10px; }
.history { position: fixed; inset: calc(99px + env(safe-area-inset-top)) 0 calc(72px + env(safe-area-inset-bottom)); padding: 14px; overflow-y: auto; display: flex; flex-direction: column; gap: 9px; }.history > button { align-self: center; color: var(--mobile-text-muted); font-size: 10px; }.history > p { margin: auto; color: var(--mobile-text-muted); font-size: 11px; }.history article { max-width: 80%; align-self: flex-start; display: grid; gap: 3px; }.history article.mine { align-self: flex-end; justify-items: end; }.history article span { padding: 9px 12px; border-radius: 14px 14px 14px 3px; background: var(--mobile-surface-strong); font-size: 13px; line-height: 1.5; white-space: pre-wrap; word-break: break-word; }.history article.mine span { border-radius: 14px 14px 3px 14px; background: var(--mobile-accent); color: #fff; }.history small { color: var(--mobile-text-muted); font-size: 8px; }
.composer { position: fixed; z-index: 20; right: 0; bottom: 0; left: 0; min-height: calc(72px + env(safe-area-inset-bottom)); padding: 10px 10px env(safe-area-inset-bottom); border-top: 1px solid var(--mobile-border); display: grid; grid-template-columns: minmax(0,1fr) 62px; gap: 8px; background: rgba(20,20,23,.96); }.composer textarea { height: 48px; padding: 12px; border: 1px solid var(--mobile-border); border-radius: 12px; resize: none; background: var(--mobile-surface-raised); color: var(--mobile-text); }.composer button { border-radius: 12px; background: var(--mobile-accent); color: #fff; font-weight: 800; }

.topbar { min-height: calc(52px + env(safe-area-inset-top)); background: rgba(15,15,17,.96); }
.message-tabs {
  padding: 0 14px;
  gap: 18px;
  border-bottom: 1px solid var(--mobile-border);
}
.message-tabs button { min-height: 44px; border-radius: 0; background: transparent; }
.message-tabs button.active { background: transparent; color: var(--mobile-text); }
.conversation-list { padding-top: 2px; }
.policy { background: #171719; }
.policy button { border-radius: 6px; }
.history article span { border-radius: 9px 9px 9px 2px; }
.history article.mine span { border-radius: 9px 9px 2px 9px; }
.composer { min-height: calc(66px + env(safe-area-inset-bottom)); background: rgba(15,15,17,.97); }
.composer textarea,
.composer button { border-radius: 7px; }
</style>
