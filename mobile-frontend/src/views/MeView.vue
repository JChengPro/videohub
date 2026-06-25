<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'
import type { Account, Video } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'
import Avatar from '../components/Avatar.vue'
import AppIcon from '../components/AppIcon.vue'

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
const stats = reactive({ works: 0, liked: 0, following: 0, followers: 0 })
const username = computed(() => auth.claims?.username ?? '')
const settingsOpen = ref(false)
const settings = reactive({ newUsername: '', oldPassword: '', newPassword: '' })
let tabRequest = 0

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
      await loadProfile()
    }
  } catch (cause) {
    showAuthDialog(mode.value === 'login' ? '登录失败' : '注册失败', authErrorMessage(cause))
  } finally { busy.value = false }
}

async function loadProfile() {
  const id = auth.claims?.account_id
  if (!id) return
  try {
    const [works, liked, following, followers] = await Promise.all([api.videosByAuthor(id), api.likedVideos(), api.following(), api.followers()])
    stats.works = works.length; stats.liked = liked.length; stats.following = following.vloggers?.length ?? 0; stats.followers = followers.followers?.length ?? 0
    videos.value = works
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
}

async function selectTab(next: typeof tab.value) {
  const request = ++tabRequest
  tab.value = next
  const id = auth.claims?.account_id
  if (!id) return
  if (next === 'works' || next === 'liked') videos.value = []
  else accounts.value = []
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
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
}

async function removeVideo(item: Video) {
  if (!confirm('确认删除这个视频？')) return
  try { await api.deleteVideo(item.id); videos.value = videos.value.filter((video) => video.id !== item.id); stats.works = Math.max(0, stats.works - 1) } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
}

async function logout() {
  try { await api.logout() } catch { /* Clear local token even if server is unavailable. */ }
  auth.clearToken()
  videos.value = []
  toast.info('已退出登录')
}

async function rename() {
  const value = settings.newUsername.trim()
  if (!value) return toast.error('请输入新用户名')
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

onMounted(() => { if (auth.isLoggedIn) loadProfile() })
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
            <input v-model="form.username" autocomplete="username" maxlength="24" placeholder="请输入账号" />
          </label>
          <label>
            <span>密码</span>
            <input v-model="form.password" type="password" :autocomplete="mode === 'login' ? 'current-password' : 'new-password'" placeholder="请输入密码" />
          </label>
          <label v-if="mode === 'register'">
            <span>确认密码</span>
            <input v-model="form.confirm" type="password" autocomplete="new-password" placeholder="请再次输入密码" />
          </label>
          <p v-if="mode === 'register'" class="auth-rule">账号为 3-24 位中文、字母、数字或下划线；密码为 8-64 位，需包含字母和数字。</p>
          <button class="auth-submit" :disabled="busy">{{ busy ? '请稍候...' : mode === 'login' ? '登录' : '注册' }}</button>
          <p class="auth-agreement">继续操作即表示你同意 VideoHub 用户协议与隐私政策</p>
        </form>
        <div class="auth-change">
          <span>{{ mode === 'login' ? '还没有账号？' : '已有账号？' }}</span>
          <button @click="switchMode(mode === 'login' ? 'register' : 'login')">{{ mode === 'login' ? '注册账号' : '返回登录' }}</button>
        </div>
      </section>
      <Transition name="dialog">
        <div v-if="authDialog.open" class="auth-dialog-mask" @click.self="authDialog.open = false">
          <section class="auth-dialog" role="alertdialog" aria-modal="true" :aria-label="authDialog.title">
            <span class="dialog-mark">!</span>
            <h2>{{ authDialog.title }}</h2>
            <p>{{ authDialog.message }}</p>
            <button autofocus @click="authDialog.open = false">知道了</button>
          </section>
        </div>
      </Transition>
    </template>
    <template v-else>
      <header class="profile-head"><button class="settings" aria-label="账号设置" @click="settingsOpen = true"><AppIcon name="settings" /></button><Avatar :name="username" :size="82" /><h1>@{{ username }}</h1><small>VideoHub ID · {{ auth.claims?.account_id }}</small><button class="logout" @click="logout">退出登录</button></header>
      <nav class="stats">
        <button @click="selectTab('following')"><b>{{ stats.following }}</b><span>关注</span></button>
        <button @click="selectTab('followers')"><b>{{ stats.followers }}</b><span>粉丝</span></button>
        <button @click="selectTab('liked')"><b>{{ stats.liked }}</b><span>喜欢</span></button>
      </nav>
      <nav class="profile-tabs"><button :class="{ active: tab === 'works' }" @click="selectTab('works')">作品 {{ stats.works }}</button><button :class="{ active: tab === 'liked' }" @click="selectTab('liked')">喜欢</button><button :class="{ active: ['following','followers'].includes(tab) }" @click="selectTab('following')">关系</button></nav>
      <section v-if="tab === 'works' || tab === 'liked'" class="video-grid">
        <article v-for="item in videos" :key="item.id" @click="router.push(`/video/${item.id}`)"><img :src="item.cover_url" :alt="item.title" /><span><AppIcon name="play" :size="13" filled />{{ Math.max(0, item.likes_count) }}</span><button v-if="tab === 'works'" aria-label="删除视频" @click.stop="removeVideo(item)"><AppIcon name="trash" :size="16" /></button></article>
        <p v-if="!videos.length" class="grid-empty">暂时没有内容</p>
      </section>
      <section v-else class="account-list"><button v-for="item in accounts" :key="item.id" @click="router.push(`/user/${item.id}`)"><Avatar :name="item.username" :size="44" /><b>{{ item.username }}</b></button><p v-if="!accounts.length" class="grid-empty">暂时没有用户</p></section>
      <div v-if="settingsOpen" class="settings-mask" @click.self="settingsOpen = false">
        <section class="settings-sheet">
          <header><h2>账号设置</h2><button aria-label="关闭设置" @click="settingsOpen = false"><AppIcon name="close" /></button></header>
          <div class="setting-block"><b>修改用户名</b><p>改名后会刷新登录 Token。</p><input v-model="settings.newUsername" placeholder="输入新用户名" /><button :disabled="busy" @click="rename">确认改名</button></div>
          <div class="setting-block"><b>修改密码</b><p>修改成功后需要重新登录。</p><input v-model="settings.oldPassword" type="password" placeholder="原密码" /><input v-model="settings.newPassword" type="password" placeholder="新密码：8-64 位，包含字母和数字" /><button :disabled="busy" @click="changePassword">确认修改</button></div>
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
.profile-head { position: relative; padding: calc(38px + env(safe-area-inset-top)) 20px 20px; display: grid; justify-items: center; }.profile-head h1 { margin-top: 12px; font-size: 20px; }.profile-head small { margin-top: 4px; color: #666; }.settings { position: absolute; top: calc(18px + env(safe-area-inset-top)); right: 18px; color: #aaa; }.logout { margin-top: 13px; padding: 7px 14px; border: 1px solid rgba(255,255,255,.12); border-radius: 18px; color: #bbb; font-size: 10px; }
.stats { display: grid; grid-template-columns: repeat(3,1fr); padding: 5px 44px 18px; }.stats button { display: grid; gap: 2px; color: #fff; }.stats b { font-size: 16px; }.stats span { color: #777; font-size: 10px; }.profile-tabs { display: grid; grid-template-columns: repeat(3,1fr); border-top: 1px solid rgba(255,255,255,.07); border-bottom: 1px solid rgba(255,255,255,.07); }.profile-tabs button { position: relative; padding: 13px; color: #777; font-size: 11px; font-weight: 700; }.profile-tabs button.active { color: #fff; }.profile-tabs button.active::after { content: ''; position: absolute; right: 30%; bottom: 0; left: 30%; height: 2px; background: #fff; }
.video-grid { padding-bottom: 80px; display: grid; grid-template-columns: repeat(3,1fr); gap: 2px; }.video-grid article { position: relative; aspect-ratio: .75; overflow: hidden; background: #222; }.video-grid img { width: 100%; height: 100%; object-fit: cover; }.video-grid article span { position: absolute; bottom: 5px; left: 5px; display: flex; align-items: center; gap: 3px; font-size: 9px; }.video-grid article button { position: absolute; top: 5px; right: 5px; width: 25px; height: 25px; display: grid; place-items: center; border-radius: 50%; background: rgba(0,0,0,.55); color: #fff; }.grid-empty { grid-column: 1/-1; padding: 60px 0; color: #666; text-align: center; font-size: 12px; }
.account-list { padding: 12px 16px 90px; }.account-list > button { width: 100%; padding: 10px 0; display: flex; align-items: center; gap: 12px; border-bottom: 1px solid rgba(255,255,255,.07); }.account-list b { font-size: 13px; }
.settings-mask { position: fixed; z-index: 120; inset: 0; display: flex; align-items: flex-end; background: rgba(0,0,0,.55); }.settings-sheet { width: 100%; max-height: 82vh; overflow-y: auto; padding: 18px 16px calc(20px + env(safe-area-inset-bottom)); border-radius: 20px 20px 0 0; background: #202024; }.settings-sheet header { display: flex; align-items: center; justify-content: space-between; }.settings-sheet h2 { font-size: 18px; }.settings-sheet header button { color: #888; }.setting-block { margin-top: 20px; padding: 16px; display: grid; gap: 10px; border-radius: 14px; background: #29292e; }.setting-block b { font-size: 13px; }.setting-block p { color: #777; font-size: 10px; }.setting-block input { height: 42px; padding: 0 12px; border: 1px solid rgba(255,255,255,.08); border-radius: 9px; background: #1c1c20; color: #fff; }.setting-block button { height: 40px; border-radius: 9px; background: #fff; color: #111; font-size: 12px; font-weight: 800; }
</style>
