<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import { ApiError } from '../api/client'
import * as accountApi from '../api/account'
import { useToastStore } from '../stores/toast'
import { validatePassword } from '../utils/accountValidation'

const router = useRouter()
const toast = useToastStore()

const busy = ref(false)
const form = reactive({ oldPassword: '', newPassword: '' })

async function submit() {
  if (busy.value) return
  const oldPassword = form.oldPassword
  const newPassword = form.newPassword
  if (!oldPassword || !newPassword) {
    toast.error('请把信息填完整')
    return
  }
  const passwordError = validatePassword(newPassword)
  if (passwordError) {
    toast.error(passwordError)
    return
  }
  if (oldPassword === newPassword) {
    toast.error('新密码不能与原密码相同')
    return
  }

  busy.value = true
  try {
    await accountApi.changePassword(oldPassword, newPassword)
    toast.success('密码已修改，请重新登录')
    await router.push('/account')
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <AppShell>
    <div class="security-page">
      <section class="security-copy">
        <span>ACCOUNT SECURITY</span>
        <h1>保护你的<br />创作与账号</h1>
        <p>更新密码后，当前账号已签发的旧 Token 会立即失效，需要使用新密码重新登录。</p>
        <div class="security-note">
          <svg viewBox="0 0 24 24" fill="none"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10Z"/><path d="m9 12 2 2 4-4"/></svg>
          <div><b>安全提醒</b><span>请使用与其他平台不同的密码</span></div>
        </div>
      </section>

      <section class="security-form-panel">
        <div class="security-form">
          <div>
            <span class="form-kicker">修改密码</span>
            <h2>验证当前密码</h2>
            <p>当前登录账号验证通过后即可设置新密码</p>
          </div>

          <form @submit.prevent="submit">
            <label><span>原密码</span><input v-model="form.oldPassword" type="password" autocomplete="current-password" placeholder="输入当前密码" /></label>
            <label><span>新密码</span><input v-model="form.newPassword" type="password" autocomplete="new-password" placeholder="8-64 位，必须包含字母和数字" @keydown.enter="submit" /></label>
            <button type="submit" :disabled="busy">{{ busy ? '正在修改...' : '确认修改密码' }}</button>
          </form>

          <button class="back-login" type="button" @click="router.push('/account')">返回登录</button>
        </div>
      </section>
    </div>
  </AppShell>
</template>

<style scoped>
.security-page { min-height: calc(100vh - 96px); display: grid; grid-template-columns: 1fr 460px; overflow: hidden; border: 1px solid var(--border); border-radius: 16px; background: #202023; }
.security-copy { position: relative; padding: 64px; display: flex; flex-direction: column; justify-content: center; background: radial-gradient(circle at 20% 20%, rgba(32,213,236,.12), transparent 30%), radial-gradient(circle at 85% 80%, rgba(254,44,85,.17), transparent 32%), #202023; }
.security-copy::after { content: ''; position: absolute; inset: 0; opacity: .12; background-image: linear-gradient(rgba(255,255,255,.18) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,.18) 1px, transparent 1px); background-size: 48px 48px; }
.security-copy > * { position: relative; z-index: 1; }
.security-copy > span, .form-kicker { color: #fe2c55; font-size: 10px; font-weight: 900; letter-spacing: .18em; }
.security-copy h1 { margin-top: 17px; font-size: clamp(42px, 5vw, 68px); line-height: 1.06; letter-spacing: -.07em; }
.security-copy > p { max-width: 450px; margin-top: 20px; color: #858585; font-size: 13px; line-height: 1.8; }
.security-note { width: fit-content; margin-top: 38px; padding: 14px 16px; border: 1px solid var(--border); border-radius: 9px; display: flex; align-items: center; gap: 12px; background: rgba(255,255,255,.035); }
.security-note svg { width: 25px; stroke: #fe2c55; stroke-width: 1.6; stroke-linecap: round; stroke-linejoin: round; }
.security-note div { display: grid; gap: 3px; }
.security-note b { font-size: 11px; }
.security-note span { color: #777; font-size: 9px; }
.security-form-panel { padding: 54px 44px; display: grid; place-items: center; background: #242428; }
.security-form { width: 100%; }
.security-form h2 { margin-top: 9px; font-size: 27px; letter-spacing: -.04em; }
.security-form > div > p { margin-top: 7px; color: #777; font-size: 11px; }
.security-form form { margin-top: 30px; display: grid; gap: 17px; }
.security-form label { margin: 0; }
.security-form label span { display: block; margin-bottom: 8px; color: #bbb; font-size: 11px; font-weight: 600; }
.security-form input { height: 46px; border-radius: 7px; background: var(--surface-raised); }
.security-form form button { height: 46px; margin-top: 5px; border-radius: 7px; background: #fe2c55; color: #fff; font-weight: 700; }
.back-login { width: 100%; margin-top: 12px; background: transparent; color: #777; font-size: 11px; }
.back-login:hover { background: transparent; color: #fff; }
@media (max-width: 960px) { .security-page { grid-template-columns: 1fr; } .security-copy { display: none; } .security-form-panel { min-height: calc(100vh - 100px); padding: 42px 28px; } }

.security-page { min-height: calc(100dvh - 110px); border-radius: 12px; background: #151517; }
.security-copy { background: #101012; }
.security-copy::after { opacity: .06; background-size: 52px 52px; }
.security-note { border-radius: 8px; }
.security-form-panel { background: #19191c; }

.security-page {
  width: min(460px,100%);
  min-height: auto;
  margin: 42px auto;
  display: block;
  overflow: visible;
  border: 0;
  border-radius: 0;
  background: transparent;
}
.security-copy { display: none; }
.security-form-panel {
  min-height: 560px;
  padding: 44px 38px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface-panel);
}
.security-form > div:first-child { text-align: center; }
</style>
