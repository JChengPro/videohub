<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'
import type { Notification } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'
import Avatar from '../components/Avatar.vue'
import AppIcon from '../components/AppIcon.vue'

const props = defineProps<{ unread: number }>()
const emit = defineEmits<{ 'update:unread': [value: number] }>()
const auth = useAuthStore()
const toast = useToastStore()
const router = useRouter()
const items = ref<Notification[]>([])
const loading = ref(false)
const opening = ref(new Set<number>())
const markAllBusy = ref(false)

function text(item: Notification) {
  if (item.type === 'like') return '赞了你的视频'
  if (item.type === 'comment') return `评论了你的视频：${item.content}`
  return '关注了你'
}

async function load() {
  if (!auth.isLoggedIn) return
  loading.value = true
  try { items.value = (await api.notifications()).notifications } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) } finally { loading.value = false }
}

async function open(item: Notification) {
  if (opening.value.has(item.id)) return
  opening.value.add(item.id)
  try {
    if (!item.is_read) {
      await api.markRead(item.id)
      item.is_read = true
      emit('update:unread', Math.max(0, props.unread - 1))
    }
    if (item.target_type === 'video') await router.push(`/video/${item.target_id}`)
    else await router.push(`/user/${item.actor_id}`)
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
  finally { opening.value.delete(item.id) }
}

function openActor(item: Notification) {
  router.push(`/user/${item.actor_id}`)
}

async function markAll() {
  if (markAllBusy.value) return
  markAllBusy.value = true
  try {
    await api.markAllRead()
    items.value.forEach((item) => (item.is_read = true))
    emit('update:unread', 0)
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
  finally { markAllBusy.value = false }
}

onMounted(load)
</script>

<template>
  <main class="page">
    <header class="page-header"><div><small>ACTIVITY</small><h1>消息</h1></div><button v-if="auth.isLoggedIn && items.some(i => !i.is_read)" class="text-action" :disabled="markAllBusy" @click="markAll">全部已读</button></header>
    <section v-if="!auth.isLoggedIn" class="empty"><AppIcon name="message" :size="38" /><h2>登录后查看消息</h2><button @click="router.push('/me')">立即登录</button></section>
    <section v-else-if="loading" class="empty">正在加载消息...</section>
    <section v-else-if="!items.length" class="empty"><AppIcon name="message" :size="38" /><h2>暂时没有新消息</h2><p>互动通知会出现在这里</p></section>
    <section v-else class="message-list">
      <article v-for="item in items" :key="item.id" :class="{ unread: !item.is_read }">
        <button class="actor" @click="openActor(item)"><Avatar :name="item.actor_username" :size="46" /></button>
        <button class="message-copy" :disabled="opening.has(item.id)" @click="open(item)"><b>{{ item.actor_username }}</b><p>{{ text(item) }}</p><small>{{ new Date(item.create_time).toLocaleString() }}</small></button>
        <i v-if="!item.is_read" />
      </article>
    </section>
  </main>
</template>

<style scoped>
.message-list { padding: 5px 14px 90px; }.message-list article { width: 100%; padding: 14px 2px; display: grid; grid-template-columns: 46px 1fr auto; gap: 12px; text-align: left; border-bottom: 1px solid rgba(255,255,255,.07); }.message-copy { min-width: 0; text-align: left; }.message-list b { font-size: 13px; }.message-list p { margin-top: 4px; overflow: hidden; color: #999; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }.message-list small { color: #555; font-size: 9px; }.message-list i { width: 7px; height: 7px; margin-top: 8px; border-radius: 50%; background: #fe2c55; }.message-list .unread { background: linear-gradient(90deg, rgba(254,44,85,.05), transparent 70%); }
</style>
