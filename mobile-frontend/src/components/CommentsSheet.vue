<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref } from 'vue'
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

const sheetElement = ref<HTMLElement | null>(null)
const inputElement = ref<HTMLInputElement | null>(null)
const comments = ref<Comment[]>([])
const content = ref('')
const loading = ref(false)
const submitting = ref(false)
const deletingId = ref(0)
const error = ref('')

let active = true
let requestId = 0
let previousFocus: HTMLElement | null = null
let previousBodyOverflow = ''

function errorMessage(value: unknown) {
  return value instanceof Error ? value.message : String(value)
}

function close() {
  emit('close')
}

async function load() {
  const request = ++requestId
  loading.value = true
  error.value = ''
  try {
    const nextComments = await api.comments(props.video.id)
    if (active && request === requestId) comments.value = nextComments
  } catch (cause) {
    if (active && request === requestId) error.value = errorMessage(cause)
  } finally {
    if (active && request === requestId) loading.value = false
  }
}

async function submit() {
  const value = content.value.trim()
  if (!value || submitting.value) return
  if (!auth.isLoggedIn) {
    toast.error('登录后才能评论')
    return
  }
  submitting.value = true
  try {
    await api.comment(props.video.id, value)
    content.value = ''
    await load()
    await nextTick()
    inputElement.value?.focus()
    toast.success('评论已发布')
  } catch (cause) {
    toast.error(errorMessage(cause))
  } finally {
    submitting.value = false
  }
}

async function remove(item: Comment) {
  if (deletingId.value || submitting.value) return
  if (!window.confirm('确认删除这条评论？')) return
  deletingId.value = item.id
  try {
    await api.deleteComment(item.id)
    comments.value = comments.value.filter((comment) => comment.id !== item.id)
    toast.info('评论已删除')
  } catch (cause) {
    toast.error(errorMessage(cause))
  } finally {
    deletingId.value = 0
  }
}

async function openAuthor(authorId: number) {
  close()
  await router.push(`/user/${authorId}`)
}

function formatDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString()
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    close()
    return
  }
  if (event.key !== 'Tab' || !sheetElement.value) return
  const focusable = Array.from(
    sheetElement.value.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), [tabindex]:not([tabindex="-1"])'),
  )
  if (focusable.length === 0) return
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (!first || !last) return
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

onMounted(async () => {
  previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
  previousBodyOverflow = document.body.style.overflow
  document.body.style.overflow = 'hidden'
  document.addEventListener('keydown', onKeydown)
  await load()
  await nextTick()
  sheetElement.value?.focus()
})

onUnmounted(() => {
  active = false
  requestId += 1
  document.body.style.overflow = previousBodyOverflow
  document.removeEventListener('keydown', onKeydown)
  previousFocus?.focus()
})
</script>

<template>
  <div class="sheet-mask" @click.self="close">
    <section
      ref="sheetElement"
      class="sheet"
      role="dialog"
      aria-modal="true"
      aria-labelledby="comments-sheet-title"
      :aria-busy="loading"
      tabindex="-1"
    >
      <div class="drag-handle" aria-hidden="true"><span /></div>
      <header>
        <div>
          <strong id="comments-sheet-title">评论</strong>
          <small v-if="!loading">{{ comments.length }} 条</small>
        </div>
        <button type="button" aria-label="关闭评论" @click="close"><AppIcon name="close" /></button>
      </header>

      <div class="comments">
        <div v-if="loading" class="sheet-state" role="status">
          <span class="sheet-loader" />
          <p>正在加载评论</p>
        </div>
        <div v-else-if="error" class="sheet-state" role="alert">
          <AppIcon name="warning" :size="28" />
          <p>{{ error }}</p>
          <button type="button" @click="load">重新加载</button>
        </div>
        <div v-else-if="!comments.length" class="sheet-state">
          <AppIcon name="comment" :size="30" />
          <strong>还没有评论</strong>
          <p>来留下第一句话</p>
        </div>

        <article v-for="item in comments" v-else :key="item.id">
          <button class="comment-avatar" type="button" :aria-label="`查看 ${item.username} 的主页`" @click="openAuthor(item.author_id)">
            <Avatar :name="item.username" :size="38" />
          </button>
          <div class="comment-copy">
            <button type="button" class="comment-author" @click="openAuthor(item.author_id)">{{ item.username }}</button>
            <p>{{ item.content }}</p>
            <small>{{ formatDate(item.created_at) }}</small>
          </div>
          <button
            v-if="item.author_id === auth.claims?.account_id"
            class="delete-comment"
            type="button"
            :disabled="deletingId === item.id"
            aria-label="删除评论"
            @click="remove(item)"
          >
            <AppIcon name="trash" :size="17" />
          </button>
        </article>
      </div>

      <form @submit.prevent="submit">
        <Avatar :name="auth.claims?.username ?? '游客'" :size="34" />
        <input
          ref="inputElement"
          v-model="content"
          maxlength="300"
          aria-label="评论内容"
          autocomplete="off"
          placeholder="留下你的评论..."
        />
        <button type="submit" :disabled="submitting || !content.trim()" aria-label="发送评论">
          <AppIcon name="send" :size="20" />
        </button>
      </form>
    </section>
  </div>
</template>

<style scoped>
.sheet-mask {
  position: fixed;
  z-index: 120;
  inset: 0;
  display: flex;
  align-items: flex-end;
  background: rgba(0, 0, 0, .58);
  backdrop-filter: blur(3px);
  overscroll-behavior: contain;
  touch-action: auto;
  animation: mask-in var(--mobile-duration) ease-out;
}

.sheet {
  width: 100%;
  height: min(76dvh, 680px);
  max-height: calc(100dvh - env(safe-area-inset-top) - 12px);
  padding-bottom: env(safe-area-inset-bottom);
  display: grid;
  grid-template-rows: 18px 50px minmax(0, 1fr) 64px;
  overflow: hidden;
  border: 1px solid var(--mobile-border);
  border-bottom: 0;
  border-radius: 22px 22px 0 0;
  background: var(--mobile-overlay);
  box-shadow: var(--mobile-shadow);
  touch-action: pan-y;
  animation: sheet-in 260ms cubic-bezier(.22, 1, .36, 1);
}

.drag-handle {
  display: grid;
  place-items: center;
}

.drag-handle span {
  width: 38px;
  height: 4px;
  border-radius: 999px;
  background: rgba(255, 255, 255, .18);
}

header {
  padding: 0 12px 0 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--mobile-border);
}

header > div {
  display: flex;
  align-items: baseline;
  gap: 7px;
}

header strong {
  font-size: 14px;
}

header small {
  color: var(--mobile-text-muted);
  font-size: 10px;
}

header > button {
  width: 44px;
  height: 44px;
  display: grid;
  place-items: center;
  color: var(--mobile-text-secondary);
}

.comments {
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: 8px 16px 18px;
}

.sheet-state {
  min-height: 220px;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 9px;
  color: var(--mobile-text-muted);
  text-align: center;
  font-size: 12px;
}

.sheet-state strong {
  color: var(--mobile-text);
  font-size: 14px;
}

.sheet-state > button {
  min-height: 40px;
  margin-top: 4px;
  padding: 0 14px;
  border-radius: 999px;
  background: var(--mobile-surface-strong);
  color: var(--mobile-text);
}

.sheet-loader {
  width: 24px;
  height: 24px;
  border: 2px solid rgba(255, 255, 255, .14);
  border-top-color: var(--mobile-text);
  border-radius: 50%;
  animation: sheet-spin .8s linear infinite;
}

article {
  padding: 11px 0;
  display: grid;
  grid-template-columns: 38px minmax(0, 1fr) 40px;
  gap: 10px;
  border-bottom: 1px solid rgba(255, 255, 255, .055);
}

.comment-avatar {
  align-self: start;
}

.comment-copy {
  min-width: 0;
}

.comment-author {
  min-height: 22px;
  color: var(--mobile-text-secondary);
  font-size: 11px;
  font-weight: 750;
}

.comment-copy p {
  margin-top: 3px;
  color: var(--mobile-text);
  font-size: 13px;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-word;
}

.comment-copy small {
  display: block;
  margin-top: 5px;
  color: var(--mobile-text-muted);
  font-size: 9px;
}

.delete-comment {
  width: 40px;
  height: 40px;
  display: grid;
  place-items: center;
  color: var(--mobile-text-muted);
}

form {
  padding: 10px 12px;
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr) 42px;
  align-items: center;
  gap: 8px;
  border-top: 1px solid var(--mobile-border);
  background: var(--mobile-surface);
}

input {
  min-width: 0;
  height: 42px;
  padding: 0 14px;
  border: 1px solid transparent;
  border-radius: 22px;
  outline: 0;
  background: var(--mobile-surface-raised);
  color: var(--mobile-text);
}

input:focus {
  border-color: var(--mobile-accent);
  box-shadow: 0 0 0 3px var(--mobile-accent-dim);
}

form button {
  width: 42px;
  height: 42px;
  display: grid;
  place-items: center;
  color: var(--mobile-accent);
}

@keyframes sheet-in {
  from { transform: translateY(100%); }
}

@keyframes mask-in {
  from { opacity: 0; }
}

@keyframes sheet-spin {
  to { transform: rotate(360deg); }
}
</style>
