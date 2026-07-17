<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'
import type { Account, Video } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'
import Avatar from '../components/Avatar.vue'
import AppIcon from '../components/AppIcon.vue'
import { PASSWORD_MAX_LENGTH, passwordError, USERNAME_MAX_LENGTH, usernameError } from '../utils/accountValidation'

const auth = useAuthStore()
const toast = useToastStore()
const router = useRouter()
const mode = ref<'login' | 'register'>('login')
const form = reactive({ username: '', password: '', confirm: '' })
const busy = ref(false)
const authDialog = reactive({ open: false, title: '', message: '' })
const tab = ref<'works' | 'liked' | 'following' | 'followers'>('works')
const videos = ref<Video[]>([])
const accounts = ref<Account[]>([])
const profileLoading = ref(false)
const tabLoading = ref(false)
const deletingId = ref(0)
const stats = reactive({ works: 0, liked: 0, following: 0, followers: 0 })
const username = computed(() => auth.claims?.username ?? '')
const settingsOpen = ref(false)
const settings = reactive({ newUsername: '', oldPassword: '', newPassword: '' })
let tabRequest = 0
let profileRequest = 0

function showAuthDialog(title: string, message: string) {
  authDialog.title = title
  authDialog.message = message
  authDialog.open = true
}

function authErrorMessage(cause: unknown) {
  const message = cause instanceof Error ? cause.message : String(cause)
  if (message.includes('username or password is wrong')) return '账号或密码错误，请检查后重新输入。'
  if (message.includes('username is required')) return '请输入账号。'
  if (message.includes('password is required')) return '请输入密码。'
  return message
}

function switchMode(next: typeof mode.value) {
  mode.value = next
  form.confirm = ''
}

async function submitAuth() {
  if (!form.username.trim()) return showAuthDialog('账号未填写', '请输入账号后继续。')
  if (!form.password) return showAuthDialog('密码未填写', '请输入密码后继续。')
  if (mode.value === 'register' && !form.confirm) return showAuthDialog('请确认密码', '请再次输入密码。')
  if (mode.value === 'register' && form.password !== form.confirm) return showAuthDialog('两次密码不一致', '请确认两次输入的密码完全相同。')
  if (mode.value === 'register') {
    const invalidUsername = usernameError(form.username)
    if (invalidUsername) return showAuthDialog('账号格式不正确', invalidUsername)
    const invalidPassword = passwordError(form.password)
    if (invalidPassword) return showAuthDialog('密码格式不正确', invalidPassword)
  }
  busy.value = true
  try {
    if (mode.value === 'register') {
      await api.register(form.username.trim(), form.password)
      mode.value = 'login'
      form.password = ''
      form.confirm = ''
      showAuthDialog('注册成功', '账号已创建，请使用新账号登录。')
    } else {
      auth.setToken((await api.login(form.username.trim(), form.password)).token)
      toast.success('登录成功')
    }
  } catch (cause) {
    showAuthDialog(mode.value === 'login' ? '登录失败' : '注册失败', authErrorMessage(cause))
  } finally { busy.value = false }
}

async function loadProfile() {
  const id = auth.claims?.account_id
  if (!id) return
  const request = ++profileRequest
  profileLoading.value = true
  try {
    const [works, liked, following, followers] = await Promise.all([api.videosByAuthor(id), api.likedVideos(), api.following(), api.followers()])
    if (request !== profileRequest) return
    stats.works = works.length; stats.liked = liked.length; stats.following = following.vloggers?.length ?? 0; stats.followers = followers.followers?.length ?? 0
    videos.value = works
  } catch (cause) {
    if (request === profileRequest) toast.error(cause instanceof Error ? cause.message : String(cause))
  } finally {
    if (request === profileRequest) profileLoading.value = false
  }
}

async function selectTab(next: typeof tab.value) {
  const request = ++tabRequest
  tab.value = next
  const id = auth.claims?.account_id
  if (!id) return
  if (next === 'works' || next === 'liked') videos.value = []
  else accounts.value = []
  tabLoading.value = true
  try {
    if (next === 'works') {
      const value = await api.videosByAuthor(id)
      if (request === tabRequest) videos.value = value
    }
    if (next === 'liked') {
      const value = await api.likedVideos()
      if (request === tabRequest) videos.value = value
    }
    if (next === 'following') {
      const value = (await api.following()).vloggers
      if (request === tabRequest) accounts.value = value
    }
    if (next === 'followers') {
      const value = (await api.followers()).followers
      if (request === tabRequest) accounts.value = value
    }
  } catch (cause) { if (request === tabRequest) toast.error(cause instanceof Error ? cause.message : String(cause)) }
  finally { if (request === tabRequest) tabLoading.value = false }
}

async function removeVideo(item: Video) {
  if (deletingId.value) return
  if (!confirm('确认删除这个视频？')) return
  deletingId.value = item.id
  try { await api.deleteVideo(item.id); videos.value = videos.value.filter((video) => video.id !== item.id); stats.works = Math.max(0, stats.works - 1); toast.info('视频已删除') } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
  finally { deletingId.value = 0 }
}

async function logout() {
  try { await api.logout() } catch { /* Clear local token even if server is unavailable. */ }
  auth.clearToken()
  toast.info('已退出登录')
}

function resetProfile() {
  profileRequest += 1
  tabRequest += 1
  profileLoading.value = false
  tabLoading.value = false
  deletingId.value = 0
  tab.value = 'works'
  videos.value = []
  accounts.value = []
  stats.works = 0
  stats.liked = 0
  stats.following = 0
  stats.followers = 0
  settingsOpen.value = false
}

async function rename() {
  const value = settings.newUsername.trim()
  if (!value) return toast.error('请输入新用户名')
  const invalidUsername = usernameError(value)
  if (invalidUsername) return toast.error(invalidUsername)
  if (busy.value) return
  busy.value = true
  try {
    auth.setToken((await api.rename(value)).token)
    settings.newUsername = ''
    toast.success('用户名已更新')
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
  finally { busy.value = false }
}

async function changePassword() {
  if (!settings.oldPassword || !settings.newPassword) return toast.error('请填写原密码和新密码')
  const invalidPassword = passwordError(settings.newPassword)
  if (invalidPassword) return toast.error(invalidPassword)
  if (busy.value) return
  busy.value = true
  try {
    await api.changePassword(username.value, settings.oldPassword, settings.newPassword)
    settings.oldPassword = ''
    settings.newPassword = ''
    auth.clearToken()
    settingsOpen.value = false
    toast.success('密码已修改，请重新登录')
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
  finally { busy.value = false }
}

function onKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape') return
  if (authDialog.open) authDialog.open = false
  else if (settingsOpen.value) settingsOpen.value = false
}

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
})
onUnmounted(() => {
  profileRequest += 1
  tabRequest += 1
  window.removeEventListener('keydown', onKeydown)
})

watch(
  () => auth.isLoggedIn ? (auth.claims?.account_id ?? 0) : 0,
  (accountId, previousAccountId) => {
    if (!accountId) {
      resetProfile()
      return
    }
    if (accountId !== previousAccountId) {
      resetProfile()
      void loadProfile()
    }
  },
  { immediate: true },
)
</script>

<template>
  <main class="page me-page">
    <template v-if="!auth.isLoggedIn">
      <section class="auth-page">
        <div class="auth-brand"><span>VH</span><b>VideoHub</b></div>
        <form class="auth-form" @submit.prevent="submitAuth">
          <header>
            <h1>{{ mode === 'login' ? '登录 VideoHub' : '注册 VideoHub' }}</h1>
            <p>{{ mode === 'login' ? '登录后，记录喜欢并与创作者互动' : '创建账号，开始分享你的作品' }}</p>
          </header>
          <label>
            <span>账号</span>
            <input v-model="form.username" autocomplete="username" :maxlength="USERNAME_MAX_LENGTH" placeholder="请输入账号" />
          </label>
          <label>
            <span>密码</span>
            <input v-model="form.password" type="password" :maxlength="PASSWORD_MAX_LENGTH" :autocomplete="mode === 'login' ? 'current-password' : 'new-password'" placeholder="请输入密码" />
          </label>
          <label v-if="mode === 'register'">
            <span>确认密码</span>
            <input v-model="form.confirm" type="password" :maxlength="PASSWORD_MAX_LENGTH" autocomplete="new-password" placeholder="请再次输入密码" />
          </label>
          <p v-if="mode === 'register'" class="auth-rule">账号为 3-24 位中文、字母、数字或下划线；密码为 8-64 位，需包含字母和数字。</p>
          <button class="auth-submit" type="submit" :disabled="busy">{{ busy ? '请稍候...' : mode === 'login' ? '登录' : '注册' }}</button>
          <p class="auth-agreement">继续操作即表示你同意 VideoHub 用户协议与隐私政策</p>
        </form>
        <div class="auth-change">
          <span>{{ mode === 'login' ? '还没有账号？' : '已有账号？' }}</span>
          <button type="button" @click="switchMode(mode === 'login' ? 'register' : 'login')">{{ mode === 'login' ? '注册账号' : '返回登录' }}</button>
        </div>
      </section>
      <Transition name="dialog">
        <div v-if="authDialog.open" class="auth-dialog-mask" @click.self="authDialog.open = false">
          <section class="auth-dialog" role="alertdialog" aria-modal="true" :aria-label="authDialog.title">
            <span class="dialog-mark">!</span>
            <h2>{{ authDialog.title }}</h2>
            <p>{{ authDialog.message }}</p>
            <button type="button" autofocus @click="authDialog.open = false">知道了</button>
          </section>
        </div>
      </Transition>
    </template>
    <template v-else>
      <header class="profile-head"><button class="settings" type="button" aria-label="账号设置" @click="settingsOpen = true"><AppIcon name="settings" /></button><Avatar :name="username" :size="82" /><h1>@{{ username }}</h1><small>VideoHub ID · {{ auth.claims?.account_id }}</small><button class="logout" type="button" @click="logout">退出登录</button></header>
      <div v-if="profileLoading" class="profile-loading" role="status">正在加载个人资料...</div>
      <nav class="stats">
        <button type="button" @click="selectTab('following')"><b>{{ stats.following }}</b><span>关注</span></button>
        <button type="button" @click="selectTab('followers')"><b>{{ stats.followers }}</b><span>粉丝</span></button>
        <button type="button" @click="selectTab('liked')"><b>{{ stats.liked }}</b><span>喜欢</span></button>
      </nav>
      <nav class="profile-tabs" aria-label="个人中心分类"><button type="button" :class="{ active: tab === 'works' }" @click="selectTab('works')">作品 {{ stats.works }}</button><button type="button" :class="{ active: tab === 'liked' }" @click="selectTab('liked')">喜欢</button><button type="button" :class="{ active: ['following','followers'].includes(tab) }" @click="selectTab('following')">关系</button></nav>
      <div v-if="tabLoading" class="tab-loading" role="status">正在加载...</div>
      <section v-if="tab === 'works' || tab === 'liked'" class="video-grid">
        <article v-for="item in videos" :key="item.id"><button class="work-open" type="button" :aria-label="`播放 ${item.title}`" @click="router.push(`/video/${item.id}`)"><img :src="item.cover_url" :alt="item.title" loading="lazy" /><span><AppIcon name="play" :size="13" filled />{{ Math.max(0, item.likes_count) }}</span></button><button v-if="tab === 'works'" class="work-delete" type="button" :disabled="deletingId === item.id" aria-label="删除视频" @click="removeVideo(item)"><AppIcon name="trash" :size="16" /></button></article>
        <p v-if="!tabLoading && !videos.length" class="grid-empty">暂时没有内容</p>
      </section>
      <section v-else class="account-list"><button v-for="item in accounts" :key="item.id" type="button" @click="router.push(`/user/${item.id}`)"><Avatar :name="item.username" :size="44" /><b>{{ item.username }}</b></button><p v-if="!tabLoading && !accounts.length" class="grid-empty">暂时没有用户</p></section>
      <div v-if="settingsOpen" class="settings-mask" @click.self="settingsOpen = false">
        <section class="settings-sheet" role="dialog" aria-modal="true" aria-labelledby="mobile-settings-title">
          <header><h2 id="mobile-settings-title">账号设置</h2><button type="button" aria-label="关闭设置" @click="settingsOpen = false"><AppIcon name="close" /></button></header>
          <div class="setting-block"><b>修改用户名</b><p>改名后会刷新登录 Token。</p><input v-model="settings.newUsername" :maxlength="USERNAME_MAX_LENGTH" aria-label="新用户名" placeholder="输入新用户名" /><button type="button" :disabled="busy" @click="rename">确认改名</button></div>
          <div class="setting-block"><b>修改密码</b><p>修改成功后需要重新登录。</p><input v-model="settings.oldPassword" type="password" :maxlength="PASSWORD_MAX_LENGTH" autocomplete="current-password" aria-label="原密码" placeholder="原密码" /><input v-model="settings.newPassword" type="password" :maxlength="PASSWORD_MAX_LENGTH" autocomplete="new-password" aria-label="新密码" placeholder="新密码：8-64 位，包含字母和数字" /><button type="button" :disabled="busy" @click="changePassword">确认修改</button></div>
        </section>
      </div>
    </template>
  </main>
</template>

<style scoped>
.me-page { background: #151518; }
.auth-page { min-height: calc(100dvh - 70px - env(safe-area-inset-bottom)); padding: calc(18px + env(safe-area-inset-top)) 30px 24px; display: flex; flex-direction: column; background: radial-gradient(circle at 50% -15%, #303036 0, #18181b 33%, #151518 58%); }
.auth-brand { display: flex; align-items: center; gap: 9px; }.auth-brand span { width: 32px; height: 32px; display: grid; place-items: center; border-radius: 8px; background: #fff; box-shadow: -3px 0 #25d7e8, 3px 0 #fe2c55; color: #111; font-size: 10px; font-weight: 900; }.auth-brand b { font-size: 14px; letter-spacing: -.03em; }
.auth-form { width: 100%; margin: auto 0; padding: 36px 0 18px; }.auth-form header { margin-bottom: 35px; }.auth-form h1 { font-size: 26px; letter-spacing: -.055em; }.auth-form header p { margin-top: 9px; color: #77777f; font-size: 11px; }
.auth-form label { position: relative; height: 58px; display: grid; grid-template-columns: 68px 1fr; align-items: center; border-bottom: 1px solid rgba(255,255,255,.12); }.auth-form label:focus-within { border-bottom-color: rgba(255,255,255,.55); }.auth-form label span { color: #d6d6da; font-size: 12px; font-weight: 700; }.auth-form input { min-width: 0; height: 100%; border: 0; outline: 0; background: transparent; color: #fff; font-size: 14px; }.auth-form input::placeholder { color: #55555d; }
.auth-rule { margin-top: 14px; color: #66666e; font-size: 9px; line-height: 1.65; }.auth-submit { width: 100%; height: 48px; margin-top: 28px; border-radius: 4px; background: #fe2c55; color: #fff; font-size: 14px; font-weight: 800; transition: background .2s ease, opacity .2s ease; }.auth-submit:disabled { opacity: .45; }.auth-agreement { margin-top: 13px; color: #55555c; font-size: 8px; line-height: 1.6; text-align: center; }
.auth-change { padding: 16px 0 7px; display: flex; justify-content: center; gap: 5px; color: #6f6f77; font-size: 11px; }.auth-change button { color: #eeeef0; font-weight: 700; }
.auth-dialog-mask { position: fixed; z-index: 220; inset: 0; padding: 24px; display: grid; place-items: center; background: rgba(0,0,0,.68); backdrop-filter: blur(5px); }.auth-dialog { width: min(290px, 100%); padding: 24px 22px 18px; display: grid; justify-items: center; border: 1px solid rgba(255,255,255,.1); border-radius: 14px; background: #29292e; box-shadow: 0 24px 70px rgba(0,0,0,.55); text-align: center; }.dialog-mark { width: 34px; height: 34px; display: grid; place-items: center; border-radius: 50%; background: rgba(254,44,85,.13); color: #fe5575; font-size: 18px; font-weight: 900; }.auth-dialog h2 { margin-top: 13px; font-size: 16px; }.auth-dialog p { margin-top: 8px; color: #9999a0; font-size: 11px; line-height: 1.65; }.auth-dialog button { width: 100%; height: 42px; margin-top: 20px; border-radius: 5px; background: #fe2c55; color: #fff; font-size: 12px; font-weight: 800; }.dialog-enter-active,.dialog-leave-active { transition: opacity .18s ease; }.dialog-enter-active .auth-dialog,.dialog-leave-active .auth-dialog { transition: transform .18s ease, opacity .18s ease; }.dialog-enter-from,.dialog-leave-to { opacity: 0; }.dialog-enter-from .auth-dialog,.dialog-leave-to .auth-dialog { opacity: 0; transform: scale(.94); }
.profile-head { position: relative; padding: calc(38px + env(safe-area-inset-top)) 20px 20px; display: grid; justify-items: center; }.profile-head h1 { margin-top: 12px; font-size: 20px; }.profile-head small { margin-top: 4px; color: var(--mobile-text-muted); }.settings { position: absolute; top: calc(14px + env(safe-area-inset-top)); right: 14px; width: 44px; height: 44px; display: grid; place-items: center; color: var(--mobile-text-secondary); }.logout { min-height: 38px; margin-top: 13px; padding: 7px 16px; border: 1px solid var(--mobile-border); border-radius: 999px; color: var(--mobile-text-secondary); font-size: 11px; }
.profile-loading,.tab-loading { min-height: 48px; display: grid; place-items: center; color: var(--mobile-text-muted); font-size: 11px; }
.stats { display: grid; grid-template-columns: repeat(3,1fr); padding: 5px 44px 18px; }.stats button { min-height: 44px; display: grid; place-content: center; gap: 2px; color: var(--mobile-text); }.stats b { font-size: 16px; }.stats span { color: var(--mobile-text-muted); font-size: 10px; }.profile-tabs { display: grid; grid-template-columns: repeat(3,1fr); border-top: 1px solid var(--mobile-border); border-bottom: 1px solid var(--mobile-border); }.profile-tabs button { position: relative; min-height: 44px; padding: 12px; color: var(--mobile-text-muted); font-size: 11px; font-weight: 700; }.profile-tabs button.active { color: var(--mobile-text); }.profile-tabs button.active::after { content: ''; position: absolute; right: 30%; bottom: 0; left: 30%; height: 2px; border-radius: 2px; background: var(--mobile-text); }
.video-grid { padding-bottom: calc(80px + env(safe-area-inset-bottom)); display: grid; grid-template-columns: repeat(3,1fr); gap: 2px; }.video-grid article { position: relative; aspect-ratio: .75; overflow: hidden; background: var(--mobile-surface-raised); }.work-open { width: 100%; height: 100%; display: block; padding: 0; border-radius: 0; }.video-grid img { width: 100%; height: 100%; object-fit: cover; }.work-open span { position: absolute; right: 5px; bottom: 5px; left: 5px; display: flex; align-items: center; gap: 3px; color: #fff; font-size: 9px; text-shadow: 0 2px 6px #000; }.work-delete { position: absolute; z-index: 2; top: 6px; right: 6px; width: 36px; height: 36px; display: grid; place-items: center; border: 1px solid rgba(255,255,255,.14); border-radius: 50%; background: rgba(12,12,14,.72); color: #fff; backdrop-filter: blur(8px); }.work-delete:disabled { opacity: .45; }.grid-empty { grid-column: 1/-1; padding: 60px 0; color: var(--mobile-text-muted); text-align: center; font-size: 12px; }
.account-list { padding: 12px 16px 90px; }.account-list > button { width: 100%; min-height: 56px; padding: 8px 0; display: flex; align-items: center; gap: 12px; border-bottom: 1px solid var(--mobile-border); }.account-list b { font-size: 13px; }
.settings-mask { position: fixed; z-index: 120; inset: 0; display: flex; align-items: flex-end; background: rgba(0,0,0,.64); backdrop-filter: blur(4px); }.settings-sheet { width: 100%; max-height: 82dvh; overflow-y: auto; padding: 18px 16px calc(20px + env(safe-area-inset-bottom)); border: 1px solid var(--mobile-border); border-bottom: 0; border-radius: 20px 20px 0 0; background: var(--mobile-surface-raised); }.settings-sheet header { display: flex; align-items: center; justify-content: space-between; }.settings-sheet h2 { font-size: 18px; }.settings-sheet header button { width: 44px; height: 44px; display: grid; place-items: center; color: var(--mobile-text-muted); }.setting-block { margin-top: 18px; padding: 16px; display: grid; gap: 10px; border: 1px solid var(--mobile-border); border-radius: 14px; background: var(--mobile-surface-strong); }.setting-block b { font-size: 13px; }.setting-block p { color: var(--mobile-text-muted); font-size: 10px; }.setting-block input { height: 44px; padding: 0 12px; border: 1px solid var(--mobile-border); border-radius: 9px; background: var(--mobile-bg); color: var(--mobile-text); }.setting-block button { min-height: 44px; border-radius: 9px; background: var(--mobile-text); color: var(--mobile-bg); font-size: 12px; font-weight: 800; }
</style>
