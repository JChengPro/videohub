<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import * as accountApi from '../api/account'
import { ApiError } from '../api/client'
import * as messageApi from '../api/message'
import type { ChatMessage, Conversation, SendMessageResponse } from '../api/message'
import AppShell from '../components/AppShell.vue'
import UserAvatar from '../components/UserAvatar.vue'
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

const state = reactive({
  loadingConversations: false,
  loadingMessages: false,
  error: '',
  conversations: [] as Conversation[],
  messages: [] as ChatMessage[],
  hasMore: false,
  nextBeforeId: 0,
  peerUsername: '',
  sendBusy: false,
  actionBusy: false,
  peerReadMessageId: 0,
})
const content = ref('')
const messageList = ref<HTMLElement | null>(null)
const peerId = computed(() => Number(route.params.peerId || 0))
const myId = computed(() => auth.claims?.account_id ?? 0)
const activeConversation = computed(() => state.conversations.find((item) => item.peer_id === peerId.value) ?? null)

const statusText = computed(() => {
  const conversation = activeConversation.value
  if (!conversation) return '发送第一条消息后创建会话'
  if (conversation.status === 'blocked') return '你或对方已拉黑，当前无法发送消息'
  if (conversation.status === 'rejected') return '对方已拒绝这条消息请求'
  if (conversation.status === 'mutual') return '你们已互相关注'
  if (conversation.status === 'accepted') return '对方已接受消息，可以正常聊天'
  if (conversation.request_sender_id === myId.value) {
    return `对方回复前还可以发送 ${conversation.remaining_request_messages} 条`
  }
  return '回复后将自动接受这条消息请求'
})

function randomClientMessageId() {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID()
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function formatTime(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString('zh-CN', { hour12: false })
}

function mergeMessages(incoming: ChatMessage[], prepend = false) {
  const map = new Map(state.messages.map((item) => [item.id, item]))
  incoming.forEach((item) => map.set(item.id, item))
  const sorted = [...map.values()].sort((a, b) => a.id - b.id)
  state.messages = sorted
  if (!prepend) void scrollToBottom()
}

async function scrollToBottom() {
  await nextTick()
  if (messageList.value) messageList.value.scrollTop = messageList.value.scrollHeight
}

async function loadConversations() {
  if (!auth.isLoggedIn) return
  state.loadingConversations = true
  try {
    const response = await messageApi.listConversations()
    state.conversations = response.conversations
    const firstConversation = response.conversations[0]
    if (!peerId.value && firstConversation) {
      await router.replace(`/messages/chat/${firstConversation.peer_id}`)
    }
  } catch (cause) {
    state.error = cause instanceof Error ? cause.message : String(cause)
  } finally {
    state.loadingConversations = false
  }
}

async function loadPeer() {
  state.peerUsername = activeConversation.value?.peer_username ?? ''
  if (state.peerUsername || !peerId.value) return
  try {
    const peer = await accountApi.findById(peerId.value)
    state.peerUsername = peer.username
  } catch {
    state.peerUsername = `用户 #${peerId.value}`
  }
}

async function loadMessages(reset: boolean) {
  const conversation = activeConversation.value
  if (!conversation) {
    state.messages = []
    state.hasMore = false
    state.nextBeforeId = 0
    await loadPeer()
    return
  }
  if (!reset && (!state.hasMore || state.loadingMessages)) return
  state.loadingMessages = true
  try {
    const response = await messageApi.listMessages(
      conversation.id,
      reset ? 0 : state.nextBeforeId,
    )
    if (reset) state.messages = response.messages
    else mergeMessages(response.messages, true)
    state.hasMore = response.has_more
    state.nextBeforeId = response.next_before_id
    await loadPeer()
    if (reset) await scrollToBottom()
    if (conversation.unread_count > 0) await markCurrentRead()
  } catch (cause) {
    state.error = cause instanceof Error ? cause.message : String(cause)
  } finally {
    state.loadingMessages = false
  }
}

async function markCurrentRead() {
  const conversation = activeConversation.value
  if (!conversation) return
  const lastMessage = state.messages[state.messages.length - 1]
  try {
    await messageApi.markRead(conversation.id, lastMessage?.id ?? 0)
    conversation.unread_count = 0
    await chatStore.refreshUnread()
  } catch {
    // 已读失败不影响继续浏览。
  }
}

async function send() {
  const text = content.value.trim()
  if (!text || !peerId.value || state.sendBusy) return
  state.sendBusy = true
  const request = {
    receiver_id: peerId.value,
    client_message_id: randomClientMessageId(),
    content: text,
  }
  try {
    let response: SendMessageResponse
    if (realtime.connected) {
      response = await realtime.send<SendMessageResponse>('chat.send', request)
    } else {
      response = await messageApi.sendMessage(request.receiver_id, request.client_message_id, request.content)
    }
    content.value = ''
    mergeMessages([response.message])
    await loadConversations()
  } catch (cause) {
    toast.error(cause instanceof ApiError || cause instanceof Error ? cause.message : String(cause))
  } finally {
    state.sendBusy = false
  }
}

async function acceptRequest() {
  const conversation = activeConversation.value
  if (!conversation || state.actionBusy) return
  state.actionBusy = true
  try {
    await messageApi.accept(conversation.id)
    toast.success('已接受消息请求')
    await loadConversations()
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : String(cause))
  } finally {
    state.actionBusy = false
  }
}

async function rejectRequest() {
  const conversation = activeConversation.value
  if (!conversation || state.actionBusy) return
  state.actionBusy = true
  try {
    await messageApi.reject(conversation.id)
    toast.info('已拒绝消息请求')
    await loadConversations()
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : String(cause))
  } finally {
    state.actionBusy = false
  }
}

async function toggleBlock() {
  const conversation = activeConversation.value
  if (!conversation || state.actionBusy) return
  state.actionBusy = true
  try {
    if (conversation.blocked_by_me) {
      await messageApi.unblock(conversation.peer_id)
      toast.success('已解除拉黑')
    } else {
      await messageApi.block(conversation.peer_id)
      toast.info('已拉黑该用户')
    }
    await loadConversations()
  } catch (cause) {
    toast.error(cause instanceof Error ? cause.message : String(cause))
  } finally {
    state.actionBusy = false
  }
}

function onRealtime(raw: Event) {
  const event = (raw as CustomEvent<RealtimeEvent>).detail
  if (event.type === 'chat.new_message') {
    const response = event.data as SendMessageResponse
    if (response?.message?.conversation_id === activeConversation.value?.id) {
      mergeMessages([response.message])
      void markCurrentRead()
    }
    void loadConversations()
  } else if (event.type === 'chat.conversation_changed') {
    void loadConversations()
  } else if (event.type === 'chat.read_receipt') {
    const data = event.data as { conversation_id: number; message_id: number }
    if (data.conversation_id === activeConversation.value?.id) state.peerReadMessageId = data.message_id
  } else if (event.type === 'notification.new') {
    const data = event.data as { type?: string } | undefined
    if (data?.type === 'follow') void loadConversations()
  }
}

watch(peerId, async (id) => {
  chatStore.setActivePeer(id)
  state.error = ''
  state.messages = []
  state.peerReadMessageId = 0
  await loadPeer()
  await loadMessages(true)
})

watch(() => auth.isLoggedIn, async (loggedIn) => {
  if (loggedIn) await loadConversations()
  else await router.push('/account')
})

onMounted(async () => {
  if (!auth.isLoggedIn) {
    await router.push('/account')
    return
  }
  window.addEventListener('videohub:realtime', onRealtime)
  await loadConversations()
  await loadPeer()
  await loadMessages(true)
})

onBeforeUnmount(() => {
  chatStore.setActivePeer(0)
  window.removeEventListener('videohub:realtime', onRealtime)
})
</script>

<template>
  <AppShell>
    <section class="chat-page">
      <header class="chat-heading">
        <div>
          <span>MESSAGES</span>
          <h1>私信</h1>
        </div>
        <nav>
          <RouterLink to="/messages">互动通知</RouterLink>
          <RouterLink class="active" to="/messages/chat">私信</RouterLink>
        </nav>
      </header>

      <div class="chat-layout">
        <aside class="conversation-panel">
          <p v-if="state.loadingConversations">正在读取会话…</p>
          <p v-else-if="state.conversations.length === 0">暂时没有私信</p>
          <button
            v-for="item in state.conversations"
            :key="item.id"
            type="button"
            :class="{ active: item.peer_id === peerId }"
            @click="router.push(`/messages/chat/${item.peer_id}`)"
          >
            <UserAvatar :username="item.peer_username" :id="item.peer_id" :size="42" />
            <span>
              <strong>{{ item.peer_username }}</strong>
              <small>{{ item.last_message_content || '开始聊天' }}</small>
            </span>
            <i v-if="item.unread_count">{{ item.unread_count > 99 ? '99+' : item.unread_count }}</i>
          </button>
        </aside>

        <section v-if="peerId" class="dialog-panel">
          <header class="dialog-header">
            <div>
              <strong>{{ state.peerUsername || activeConversation?.peer_username }}</strong>
              <small>{{ statusText }}</small>
            </div>
            <div class="dialog-actions">
              <button
                v-if="activeConversation?.status === 'pending' && activeConversation.request_sender_id !== myId"
                type="button"
                :disabled="state.actionBusy"
                @click="acceptRequest"
              >接受</button>
              <button
                v-if="activeConversation?.status === 'pending' && activeConversation.request_sender_id !== myId"
                type="button"
                :disabled="state.actionBusy"
                @click="rejectRequest"
              >拒绝</button>
              <button
                v-if="activeConversation && (!activeConversation.blocked_by_peer || activeConversation.blocked_by_me)"
                type="button"
                :disabled="state.actionBusy"
                @click="toggleBlock"
              >
                {{ activeConversation.blocked_by_me ? '解除拉黑' : '拉黑' }}
              </button>
            </div>
          </header>

          <div ref="messageList" class="message-history">
            <button v-if="state.hasMore" type="button" :disabled="state.loadingMessages" @click="loadMessages(false)">
              {{ state.loadingMessages ? '加载中…' : '查看更早消息' }}
            </button>
            <p v-if="!state.loadingMessages && state.messages.length === 0">还没有消息，打个招呼吧</p>
            <article v-for="item in state.messages" :key="item.id" :class="{ mine: item.sender_id === myId }">
              <span>{{ item.content }}</span>
              <small>{{ formatTime(item.created_at) }}<template v-if="item.sender_id === myId && item.id <= state.peerReadMessageId"> · 已读</template></small>
            </article>
          </div>

          <form class="composer" @submit.prevent="send">
            <textarea
              v-model="content"
              maxlength="1000"
              :disabled="activeConversation ? !activeConversation.can_send : false"
              placeholder="输入消息，Enter 发送"
              @keydown.enter.exact.prevent="send"
            />
            <button type="submit" :disabled="state.sendBusy || !content.trim() || (activeConversation ? !activeConversation.can_send : false)">
              {{ state.sendBusy ? '发送中' : '发送' }}
            </button>
          </form>
        </section>

        <section v-else class="dialog-empty">选择一个会话，或者从用户主页发起私信</section>
      </div>
    </section>
  </AppShell>
</template>

<style scoped>
.chat-page { max-width: 1100px; margin: 0 auto; }
.chat-heading { margin-bottom: 14px; padding: 22px 26px; border: 1px solid var(--border); border-radius: 14px; display: flex; align-items: center; justify-content: space-between; background: var(--surface-panel); }
.chat-heading span { color: var(--accent-cyan); font-size: 10px; font-weight: 900; letter-spacing: .2em; }
.chat-heading h1 { margin-top: 5px; font-size: 28px; }
.chat-heading nav { display: flex; gap: 6px; }.chat-heading nav a { padding: 8px 13px; border-radius: 999px; color: var(--text-secondary); font-size: 12px; }.chat-heading nav a.active { background: #fff; color: #111; font-weight: 800; }
.chat-layout { min-height: 610px; border: 1px solid var(--border); border-radius: 14px; display: grid; grid-template-columns: 300px minmax(0,1fr); overflow: hidden; background: var(--surface-panel); }
.conversation-panel { border-right: 1px solid var(--border); overflow-y: auto; }.conversation-panel > p { padding: 24px; color: var(--text-secondary); font-size: 12px; }.conversation-panel > button { width: 100%; padding: 14px; border-radius: 0; display: grid; grid-template-columns: 42px minmax(0,1fr) auto; align-items: center; gap: 10px; background: transparent; text-align: left; }.conversation-panel > button.active,.conversation-panel > button:hover { background: var(--surface-raised); }.conversation-panel span { min-width: 0; display: grid; gap: 4px; }.conversation-panel strong,.conversation-panel small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.conversation-panel strong { font-size: 12px; }.conversation-panel small { color: var(--text-muted); font-size: 10px; }.conversation-panel i { min-width: 19px; height: 19px; padding: 0 5px; border-radius: 10px; display: grid; place-items: center; background: var(--accent); color: #fff; font-size: 9px; font-style: normal; }
.dialog-panel { min-width: 0; display: grid; grid-template-rows: auto minmax(0,1fr) auto; }.dialog-header { min-height: 70px; padding: 13px 18px; border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; gap: 12px; }.dialog-header > div:first-child { display: grid; gap: 5px; }.dialog-header strong { font-size: 14px; }.dialog-header small { color: var(--text-muted); font-size: 10px; }.dialog-actions { display: flex; gap: 5px; }.dialog-actions button { padding: 7px 10px; background: var(--surface-raised); color: var(--text-secondary); font-size: 10px; }
.message-history { padding: 18px; overflow-y: auto; display: flex; flex-direction: column; gap: 10px; }.message-history > button { align-self: center; background: transparent; color: var(--text-muted); font-size: 10px; }.message-history > p { margin: auto; color: var(--text-muted); font-size: 12px; }.message-history article { max-width: 72%; align-self: flex-start; display: grid; gap: 4px; }.message-history article.mine { align-self: flex-end; justify-items: end; }.message-history article span { padding: 10px 13px; border-radius: 13px 13px 13px 3px; background: var(--surface-strong); color: #eee; font-size: 13px; line-height: 1.55; white-space: pre-wrap; word-break: break-word; }.message-history article.mine span { border-radius: 13px 13px 3px 13px; background: var(--accent); color: #fff; }.message-history article small { color: var(--text-muted); font-size: 9px; }
.composer { padding: 12px; border-top: 1px solid var(--border); display: grid; grid-template-columns: minmax(0,1fr) auto; gap: 9px; }.composer textarea { min-height: 52px; max-height: 130px; padding: 11px 13px; border: 1px solid var(--border); border-radius: 11px; resize: vertical; background: var(--surface-raised); color: #eee; }.composer button { min-width: 74px; background: var(--accent); color: #fff; font-weight: 800; }.dialog-empty { display: grid; place-items: center; color: var(--text-muted); font-size: 12px; }
@media (max-width: 800px) { .chat-layout { grid-template-columns: 120px minmax(0,1fr); }.conversation-panel > button { grid-template-columns: 1fr; justify-items: center; }.conversation-panel span { width: 100%; text-align: center; }.conversation-panel i { position: absolute; }.chat-heading { align-items: flex-start; flex-direction: column; } }

.chat-heading {
  margin-bottom: 12px;
  padding: 6px 2px 18px;
  border: 0;
  border-bottom: 1px solid var(--border);
  border-radius: 0;
  background: transparent;
}
.chat-heading h1 { font-size: 32px; font-weight: 850; }
.chat-heading nav a { border-radius: 6px; }
.chat-heading nav a.active { background: var(--surface-raised); color: #fff; }
.chat-layout { min-height: calc(100dvh - 180px); border-radius: 8px; background: #111113; }
.conversation-panel > button { border-left: 2px solid transparent; }
.conversation-panel > button.active { border-left-color: var(--accent); background: #1d1d20; }
.message-history article span { border-radius: 9px 9px 9px 2px; }
.message-history article.mine span { border-radius: 9px 9px 2px 9px; }
.composer textarea,
.composer button { border-radius: 7px; }
</style>
