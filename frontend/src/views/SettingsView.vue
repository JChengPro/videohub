<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import AppIcon from '../components/AppIcon.vue'
import UserAvatar from '../components/UserAvatar.vue'
import { ApiError } from '../api/client'
import type { Account } from '../api/types'
import * as accountApi from '../api/account'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'
import { useDialogStore } from '../stores/dialog'
import { validateUsername } from '../utils/accountValidation'

const router = useRouter()
const auth = useAuthStore()
const toast = useToastStore()
const dialog = useDialogStore()

const busy = ref(false)
const profile = ref<Account | null>(null)
const avatarInput = ref<HTMLInputElement | null>(null)
const avatarFile = ref<File | null>(null)
const avatarPreview = ref('')
const avatarVersion = ref(0)

const me = computed(() => ({
  id: profile.value?.id ?? auth.claims?.account_id ?? 0,
  accountName: profile.value?.account_name ?? auth.claims?.account_name ?? '',
  username: profile.value?.username ?? auth.claims?.username ?? '',
}))

const rename = reactive({
  open: false,
  newUsername: '',
})

async function openRename() {
  if (!auth.isLoggedIn) return
  rename.open = true
  rename.newUsername = me.value.username
  await nextTick()
}

async function loadMe() {
  if (!auth.isLoggedIn) return
  try {
    profile.value = await accountApi.me()
  } catch {
    profile.value = null
  }
}

function selectAvatar() {
  avatarInput.value?.click()
}

function clearAvatarSelection() {
  avatarFile.value = null
  if (avatarPreview.value) URL.revokeObjectURL(avatarPreview.value)
  avatarPreview.value = ''
  if (avatarInput.value) avatarInput.value.value = ''
}

function onAvatarSelected(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
    toast.error('头像仅支持 JPG、PNG 或 WebP 图片')
    input.value = ''
    return
  }
  if (file.size > 5 * 1024 * 1024) {
    toast.error('头像文件不能超过 5MB')
    input.value = ''
    return
  }
  clearAvatarSelection()
  avatarFile.value = file
  avatarPreview.value = URL.createObjectURL(file)
}

async function saveAvatar() {
  if (!avatarFile.value || busy.value || !me.value.id) return
  busy.value = true
  try {
    await accountApi.uploadAvatar(avatarFile.value)
    avatarVersion.value = Date.now()
    window.dispatchEvent(new CustomEvent('videohub:avatar-updated', { detail: { accountId: me.value.id } }))
    clearAvatarSelection()
    toast.success('头像已更新')
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e))
  } finally {
    busy.value = false
  }
}

async function submitRename() {
  if (!auth.isLoggedIn) return
  if (busy.value) return
  const newUsername = rename.newUsername.trim()
  if (!newUsername) {
    toast.error('请输入新用户名')
    return
  }
  const usernameError = validateUsername(newUsername)
  if (usernameError) {
    toast.error(usernameError)
    return
  }

  busy.value = true
  try {
    const res = await accountApi.rename(newUsername)
    auth.setToken(res.token)
    if (profile.value) profile.value.username = newUsername
    rename.open = false
    toast.success('改名成功（已刷新 token）')
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
  } finally {
    busy.value = false
  }
}

async function goLogin() {
  await router.push('/account')
}

async function goChangePassword() {
  await router.push('/account/change-password')
}

async function onLogout() {
  if (!auth.isLoggedIn) return
  if (busy.value) return
  if (!await dialog.ask({
    title: '退出当前账号？',
    message: '退出后本机登录状态会被清除，需要重新输入账号和密码才能继续互动。',
    confirmLabel: '退出登录',
    tone: 'danger',
  })) return

  busy.value = true
  try {
    await accountApi.logout()
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(`登出失败：${msg}`)
  } finally {
    auth.clearToken()
    rename.open = false
    toast.info('已退出登录')
    busy.value = false
    await router.push('/')
  }
}

onMounted(() => { void loadMe() })
onUnmounted(clearAvatarSelection)
</script>

<template>
  <AppShell>
    <div class="settings-page">
      <header class="settings-heading">
        <span>SETTINGS</span>
        <h1>设置</h1>
        <p>管理你的账号资料与登录安全</p>
      </header>

      <section v-if="!auth.isLoggedIn" class="login-required">
        <AppIcon name="user" :size="34" />
        <h2>登录后管理账号</h2>
        <p>登录后可以修改用户名、密码以及退出当前账号。</p>
        <button class="primary" type="button" @click="goLogin">登录 VideoHub</button>
      </section>

      <div v-else class="settings-layout">
        <aside class="settings-side">
          <div class="account-summary">
            <UserAvatar :username="me.username" :id="me.id" :size="58" :avatar-url="avatarPreview || undefined" :version="avatarVersion" />
            <div><strong>{{ me.username }}</strong><span>@{{ me.accountName || 'loading' }}</span></div>
          </div>
          <nav aria-label="设置分类">
            <a href="#account-profile" class="active"><AppIcon name="user" :size="18" />账号资料</a>
            <a href="#account-security"><AppIcon name="settings" :size="18" />账号安全</a>
          </nav>
        </aside>

        <div class="settings-content">
          <section id="account-profile" class="setting-section">
            <header><div><h2>账号资料</h2><p>管理展示给其他用户的账号信息</p></div></header>
            <div class="setting-row avatar-row">
              <div class="avatar-setting">
                <UserAvatar :username="me.username" :id="me.id" :size="72" :avatar-url="avatarPreview || undefined" :version="avatarVersion" />
                <div><strong>头像</strong><span>{{ avatarFile ? '已选择新头像，保存后对所有用户可见' : 'JPG、PNG 或 WebP，最大 5MB' }}</span></div>
              </div>
              <div class="avatar-actions">
                <input ref="avatarInput" type="file" accept="image/jpeg,image/png,image/webp" @change="onAvatarSelected" />
                <button v-if="avatarFile" type="button" :disabled="busy" @click="clearAvatarSelection">取消</button>
                <button v-if="avatarFile" class="primary" type="button" :disabled="busy" @click="saveAvatar">{{ busy ? '上传中…' : '保存头像' }}</button>
                <button v-else class="setting-action" type="button" :disabled="busy" @click="selectAvatar">更换头像</button>
              </div>
            </div>
            <div class="setting-row">
              <div><strong>账号名</strong><span>@{{ me.accountName }} · 登录使用，当前不可修改</span></div>
              <span class="locked-badge">唯一账号</span>
            </div>
            <div class="setting-row">
              <div><strong>公开昵称</strong><span>当前昵称：{{ me.username }} · 支持中文</span></div>
              <button class="setting-action" type="button" :disabled="busy" @click="openRename">编辑</button>
            </div>
            <form v-if="rename.open" class="rename-panel" @submit.prevent="submitRename">
              <label for="settings-username">新昵称</label>
              <input id="settings-username" v-model.trim="rename.newUsername" />
              <p>3–24 个字符，仅支持中文、字母、数字和下划线。</p>
              <div>
                <button type="button" :disabled="busy" @click="rename.open = false">取消</button>
                <button class="primary" type="submit" :disabled="busy">{{ busy ? '保存中…' : '保存' }}</button>
              </div>
            </form>
          </section>

          <section id="account-security" class="setting-section">
            <header><div><h2>账号安全</h2><p>管理密码和当前登录状态</p></div></header>
            <div class="setting-row">
              <div><strong>登录密码</strong><span>修改密码后，原有登录凭证会失效</span></div>
              <button class="setting-action" type="button" :disabled="busy" @click="goChangePassword">修改</button>
            </div>
            <div class="setting-row danger-row">
              <div><strong>退出登录</strong><span>清除当前浏览器保存的登录状态</span></div>
              <button class="logout-action" type="button" :disabled="busy" @click="onLogout">退出</button>
            </div>
          </section>

          <p class="security-note"><AppIcon name="check" :size="16" />账号资料或密码变更后，旧 Token 会立即失效。</p>
        </div>
      </div>
    </div>
  </AppShell>
</template>

<style scoped>
.settings-page { max-width: 1080px; margin: 0 auto; padding-bottom: 44px; }
.settings-heading { padding: 8px 0 24px; border-bottom: 1px solid var(--border); }
.settings-heading > span { color: var(--accent); font-size: 9px; font-weight: 900; letter-spacing: .18em; }
.settings-heading h1 { margin-top: 5px; font-size: 34px; font-weight: 850; letter-spacing: -.045em; }
.settings-heading p { margin-top: 5px; color: var(--text-muted); font-size: 11px; }
.settings-layout { min-height: 620px; display: grid; grid-template-columns: 250px minmax(0,1fr); }
.settings-side { padding: 24px 20px 24px 0; border-right: 1px solid var(--border); }
.account-summary { padding: 4px 8px 22px; display: flex; align-items: center; gap: 12px; border-bottom: 1px solid var(--border); }
.account-summary > div { min-width: 0; }.account-summary strong,.account-summary span { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.account-summary strong { font-size: 13px; }.account-summary span { margin-top: 4px; color: var(--text-muted); font-size: 9px; }
.settings-side nav { margin-top: 14px; display: grid; gap: 3px; }
.settings-side nav a { min-height: 44px; padding: 0 12px; display: flex; align-items: center; gap: 11px; border-radius: 6px; color: var(--text-secondary); font-size: 12px; font-weight: 700; }
.settings-side nav a:hover,.settings-side nav a.active { background: var(--surface-raised); color: #fff; }
.settings-side nav a.active { border-left: 2px solid var(--accent); }
.settings-content { min-width: 0; padding: 8px 0 40px 34px; }
.setting-section { scroll-margin-top: 80px; }
.setting-section + .setting-section { margin-top: 10px; }
.setting-section > header { padding: 22px 0 14px; border-bottom: 1px solid var(--border); }
.setting-section h2 { font-size: 19px; }.setting-section header p { margin-top: 4px; color: var(--text-muted); font-size: 10px; }
.setting-row { min-height: 88px; padding: 18px 0; display: flex; align-items: center; justify-content: space-between; gap: 24px; border-bottom: 1px solid var(--border); }
.setting-row > div { min-width: 0; }.setting-row strong,.setting-row span { display: block; }.setting-row strong { font-size: 13px; }.setting-row span { margin-top: 5px; color: var(--text-muted); font-size: 10px; line-height: 1.5; }
.setting-action,.logout-action { min-width: 66px; height: 36px; padding: 0 14px; border: 1px solid var(--border); border-radius: 5px; background: transparent; color: var(--text-secondary); font-size: 11px; font-weight: 750; }
.setting-action:hover { background: var(--surface-raised); color: #fff; }
.avatar-setting { display: flex; align-items: center; gap: 14px; }.avatar-setting > div { min-width: 0; }
.avatar-actions { display: flex; align-items: center; justify-content: flex-end; gap: 7px; }.avatar-actions input { display: none; }.avatar-actions button { min-width: 70px; height: 36px; padding: 0 13px; border-radius: 5px; font-size: 10px; }
.locked-badge { flex: 0 0 auto; margin: 0 !important; padding: 6px 9px; border-radius: 999px; background: rgba(37,244,238,.07); color: var(--accent-cyan) !important; font-size: 9px !important; font-weight: 750; }
.logout-action { border-color: rgba(254,44,85,.3); color: var(--accent); }.logout-action:hover { background: var(--accent-dim); }
.danger-row strong { color: #f1f1f3; }
.rename-panel { margin: 14px 0 6px; padding: 18px; border-radius: 7px; background: var(--surface-panel); }
.rename-panel label { margin-bottom: 8px; }.rename-panel input { height: 44px; }.rename-panel > p { margin-top: 7px; color: var(--text-muted); font-size: 9px; }
.rename-panel > div { margin-top: 14px; display: flex; justify-content: flex-end; gap: 7px; }
.rename-panel button { min-width: 70px; border-radius: 5px; }
.security-note { margin-top: 20px; padding: 12px 14px; display: flex; align-items: center; gap: 8px; border-radius: 6px; background: rgba(37,244,238,.06); color: #9babae; font-size: 10px; }
.security-note :deep(svg) { color: var(--accent-cyan); }
.login-required { min-height: 500px; display: grid; place-content: center; justify-items: center; gap: 9px; color: var(--text-muted); text-align: center; }
.login-required h2 { color: #fff; font-size: 18px; }.login-required p { max-width: 330px; font-size: 11px; }.login-required button { margin-top: 10px; border-radius: 5px; }
@media (max-width: 820px) {
  .settings-layout { grid-template-columns: 1fr; }
  .settings-side { padding: 16px 0; border-right: 0; border-bottom: 1px solid var(--border); }
  .settings-side nav { display: flex; }.settings-side nav a { flex: 1; }
  .settings-content { padding-left: 0; }
  .avatar-row { align-items: flex-start; flex-direction: column; }.avatar-actions { width: 100%; justify-content: flex-start; }
}
</style>
