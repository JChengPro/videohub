<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'
import type { Comment, FeedVideo } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'
import AppIcon from './AppIcon.vue'
import Avatar from './Avatar.vue'

const props = defineProps<{ video: FeedVideo }>()
const emit = defineEmits<{ close: [] }>()
const auth = useAuthStore()
const toast = useToastStore()
const router = useRouter()
const comments = ref<Comment[]>([])
const content = ref('')
const busy = ref(false)

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error)
}

async function load() {
  try { comments.value = await api.comments(props.video.id) } catch (error) { toast.error(errorMessage(error)) }
}

async function submit() {
  const value = content.value.trim()
  if (!value) return
  if (!auth.isLoggedIn) return toast.error('登录后才能评论')
  busy.value = true
  try {
    await api.comment(props.video.id, value)
    content.value = ''
    await load()
  } catch (error) { toast.error(errorMessage(error)) } finally { busy.value = false }
}

async function remove(item: Comment) {
  if (busy.value) return
  if (!confirm('确认删除这条评论？')) return
  busy.value = true
  try { await api.deleteComment(item.id); comments.value = comments.value.filter((comment) => comment.id !== item.id) }
  catch (error) { toast.error(errorMessage(error)) }
  finally { busy.value = false }
}

onMounted(load)
</script>

<template>
  <div class="sheet-mask" @click.self="emit('close')">
    <section class="sheet" aria-modal="true">
      <header><strong>{{ comments.length }} 条评论</strong><button aria-label="关闭评论" @click="emit('close')"><AppIcon name="close" /></button></header>
      <div class="comments">
        <p v-if="!comments.length" class="empty">还没有评论，来留下第一句话</p>
        <article v-for="item in comments" :key="item.id">
          <button class="comment-avatar" @click="router.push(`/user/${item.author_id}`); emit('close')"><Avatar :name="item.username" :size="36" /></button>
          <div><b>{{ item.username }}</b><p>{{ item.content }}</p><small>{{ new Date(item.created_at).toLocaleString() }}</small></div>
          <button v-if="item.author_id === auth.claims?.account_id" :disabled="busy" aria-label="删除评论" @click="remove(item)"><AppIcon name="trash" :size="17" /></button>
        </article>
      </div>
      <form @submit.prevent="submit">
        <Avatar :name="auth.claims?.username ?? '游客'" :size="34" />
        <input v-model="content" maxlength="300" placeholder="留下你的评论..." />
        <button :disabled="busy || !content.trim()" aria-label="发送评论"><AppIcon name="send" :size="20" /></button>
      </form>
    </section>
  </div>
</template>

<style scoped>
.sheet-mask { position: fixed; z-index: 120; inset: 0; display: flex; align-items: flex-end; background: rgba(0,0,0,.52); }
.sheet { width: 100%; height: min(72vh, 650px); padding-bottom: env(safe-area-inset-bottom); display: grid; grid-template-rows: 52px 1fr 62px; border-radius: 18px 18px 0 0; background: #202024; animation: rise .22s ease-out; }
header { padding: 0 16px; display: flex; align-items: center; justify-content: center; border-bottom: 1px solid rgba(255,255,255,.07); } header strong { font-size: 13px; } header button { position: absolute; right: 14px; color: #aaa; }
.comments { overflow-y: auto; padding: 14px 16px; }.empty { padding-top: 70px; color: #777; text-align: center; font-size: 13px; }
article { padding: 9px 0; display: grid; grid-template-columns: 36px 1fr auto; gap: 10px; } article b { color: #8f8f98; font-size: 11px; } article p { margin-top: 3px; color: #eee; font-size: 13px; line-height: 1.5; } article small { color: #666; font-size: 9px; } article button { color: #666; }
form { padding: 9px 12px; display: grid; grid-template-columns: 34px 1fr 34px; align-items: center; gap: 8px; border-top: 1px solid rgba(255,255,255,.07); background: #18181b; } input { height: 40px; padding: 0 14px; border: 0; border-radius: 20px; background: #29292e; color: #fff; } form button { color: #fe2c55; }
@keyframes rise { from { transform: translateY(100%); } }
</style>
