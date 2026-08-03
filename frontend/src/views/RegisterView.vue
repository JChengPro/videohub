<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import { ApiError } from '../api/client'
import * as accountApi from '../api/account'
import { useToastStore } from '../stores/toast'
import { accountNameRules, passwordRules, usernameRules, validateAccountName, validatePassword, validateUsername } from '../utils/accountValidation'

const router = useRouter()
const toast = useToastStore()

const busy = ref(false)
const showPassword = ref(false)
const accountNameChecking = ref(false)
const accountNameAvailable = ref<boolean | null>(null)
const form = reactive({ accountName: '', username: '', password: '', confirmPassword: '' })
const currentAccountNameRules = computed(() => accountNameRules(form.accountName))
const currentUsernameRules = computed(() => usernameRules(form.username))
const currentPasswordRules = computed(() => passwordRules(form.password))
const passwordsMatch = computed(() => form.confirmPassword.length > 0 && form.password === form.confirmPassword)

async function submit() {
  if (busy.value) return
  const accountName = form.accountName.trim().toLowerCase()
  const username = form.username.trim()
  const password = form.password
  if (!accountName || !username || !password) {
    toast.error('请把账号名、昵称和密码填写完整')
    return
  }
  const accountNameError = validateAccountName(accountName)
  if (accountNameError) {
    toast.error(accountNameError)
    return
  }
  const usernameError = validateUsername(username)
  if (usernameError) {
    toast.error(usernameError)
    return
  }
  const passwordError = validatePassword(password)
  if (passwordError) {
    toast.error(passwordError)
    return
  }
  if (password !== form.confirmPassword) {
    toast.error('两次输入的密码不一致')
    return
  }

  busy.value = true
  try {
    await accountApi.register(accountName, username, password)
    toast.success('注册成功，请登录')
    await router.push('/account')
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
  } finally {
    busy.value = false
  }
}

async function checkAccountName() {
  const accountName = form.accountName.trim().toLowerCase()
  accountNameAvailable.value = null
  if (validateAccountName(accountName)) return
  accountNameChecking.value = true
  try {
    const result = await accountApi.checkAccountName(accountName)
    accountNameAvailable.value = result.available
  } catch {
    accountNameAvailable.value = null
  } finally {
    accountNameChecking.value = false
  }
}
</script>

<template>
  <AppShell>
    <div class="auth-page">
      <section class="auth-visual">
        <div class="visual-glow cyan" />
        <div class="visual-glow red" />
        <div class="visual-copy">
          <span class="visual-brand">VIDEOHUB</span>
          <h1>记录热爱，<br />分享你的视角。</h1>
          <p>加入视频社区，发现有趣创作者，也让你的作品被更多人看见。</p>
          <div class="feature-list" aria-label="VideoHub 主要功能">
            <div class="feature-card">
              <span class="feature-icon cyan">
                <svg viewBox="0 0 24 24" fill="none"><path d="m9 6 10 6-10 6V6Z"/><circle cx="12" cy="12" r="10"/></svg>
              </span>
              <div><small>发现</small><b>浏览社区热门视频</b></div>
              <em>01</em>
            </div>
            <div class="feature-card">
              <span class="feature-icon red">
                <svg viewBox="0 0 24 24" fill="none"><circle cx="9" cy="8" r="4"/><path d="M2 21a7 7 0 0 1 14 0M19 8v6m3-3h-6"/></svg>
              </span>
              <div><small>连接</small><b>关注并私信创作者</b></div>
              <em>02</em>
            </div>
            <div class="feature-card">
              <span class="feature-icon violet">
                <svg viewBox="0 0 24 24" fill="none"><rect x="3" y="3" width="18" height="18" rx="5"/><path d="M12 7v10M7 12h10"/></svg>
              </span>
              <div><small>表达</small><b>发布你的原创作品</b></div>
              <em>03</em>
            </div>
          </div>
        </div>
      </section>

      <section class="auth-panel">
        <div class="auth-box">
          <div class="auth-heading">
            <span>创建账号</span>
            <h2>欢迎来到 VideoHub</h2>
            <p>注册后即可点赞、评论、关注和发布视频</p>
          </div>

          <form class="auth-form" @submit.prevent="submit">
            <label>
              <span>账号名</span>
              <div class="input-shell">
                <svg viewBox="0 0 24 24" fill="none"><circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0 1 16 0"/></svg>
                <input v-model.trim="form.accountName" autocomplete="username" maxlength="24" placeholder="例如 jcheng2026" @input="accountNameAvailable = null" @blur="checkAccountName" />
              </div>
              <ul class="rule-list">
                <li v-for="rule in currentAccountNameRules" :key="rule.text" :class="{ valid: rule.valid }">{{ rule.text }}</li>
              </ul>
              <p v-if="accountNameChecking" class="availability-tip">正在检查账号名…</p>
              <p v-else-if="accountNameAvailable === true" class="availability-tip available">账号名可以使用</p>
              <p v-else-if="accountNameAvailable === false" class="availability-tip unavailable">账号名已被使用，请更换一个</p>
            </label>

            <label>
              <span>公开昵称</span>
              <div class="input-shell">
                <svg viewBox="0 0 24 24" fill="none"><path d="M4 19h16M7 16l7-7 3 3-7 7H7v-3Z"/></svg>
                <input v-model.trim="form.username" maxlength="24" placeholder="支持中文，例如 成都小王" />
              </div>
              <ul class="rule-list">
                <li v-for="rule in currentUsernameRules" :key="rule.text" :class="{ valid: rule.valid }">{{ rule.text }}</li>
              </ul>
            </label>

            <label>
              <span>密码</span>
              <div class="input-shell">
                <svg viewBox="0 0 24 24" fill="none"><rect x="4" y="10" width="16" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/></svg>
                <input v-model="form.password" :type="showPassword ? 'text' : 'password'" autocomplete="new-password" placeholder="设置登录密码" />
                <button type="button" class="show-password" @click="showPassword = !showPassword">{{ showPassword ? '隐藏' : '显示' }}</button>
              </div>
              <ul class="rule-list">
                <li v-for="rule in currentPasswordRules" :key="rule.text" :class="{ valid: rule.valid }">{{ rule.text }}</li>
              </ul>
            </label>

            <label>
              <span>确认密码</span>
              <div class="input-shell">
                <svg viewBox="0 0 24 24" fill="none"><path d="m5 12 4 4L19 6"/></svg>
                <input v-model="form.confirmPassword" :type="showPassword ? 'text' : 'password'" autocomplete="new-password" placeholder="再次输入密码" @keydown.enter="submit" />
              </div>
              <p v-if="form.confirmPassword" class="match-tip" :class="{ valid: passwordsMatch }">
                {{ passwordsMatch ? '两次密码输入一致' : '两次密码输入不一致' }}
              </p>
            </label>

            <button class="submit-button" type="submit" :disabled="busy">
              {{ busy ? '正在创建...' : '注册并加入 VideoHub' }}
            </button>
          </form>

          <p class="auth-switch">已经有账号？ <RouterLink to="/account">立即登录</RouterLink></p>
        </div>
      </section>
    </div>
  </AppShell>
</template>

<style scoped>
.auth-page { min-height: calc(100vh - 96px); display: grid; grid-template-columns: 1.08fr .92fr; overflow: hidden; border: 1px solid var(--border); border-radius: 16px; background: #202023; }
.auth-visual { position: relative; min-height: 660px; overflow: hidden; display: flex; align-items: center; padding: 64px; background: radial-gradient(circle at 50% 30%, #37373c 0, #29292d 52%, #202023 100%); }
.auth-visual::after { content: ''; position: absolute; inset: 0; opacity: .16; background-image: linear-gradient(rgba(255,255,255,.2) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,.2) 1px, transparent 1px); background-size: 42px 42px; mask-image: linear-gradient(to bottom, black, transparent 90%); }
.visual-glow { position: absolute; width: 280px; height: 280px; border-radius: 50%; filter: blur(100px); opacity: .18; }
.visual-glow.cyan { top: 8%; left: 12%; background: #20d5ec; }
.visual-glow.red { right: 6%; bottom: 14%; background: #fe2c55; }
.visual-copy { position: relative; z-index: 1; max-width: 500px; }
.visual-brand { color: #fe2c55; font-size: 11px; font-weight: 900; letter-spacing: .22em; }
.visual-copy h1 { margin-top: 18px; font-size: clamp(42px, 5vw, 66px); line-height: 1.08; letter-spacing: -.065em; }
.visual-copy > p { max-width: 430px; margin-top: 22px; color: #919191; font-size: 14px; line-height: 1.8; }
.feature-list { margin-top: 38px; display: grid; gap: 9px; }
.feature-card { min-height: 72px; padding: 12px 14px; display: grid; grid-template-columns: 44px minmax(0,1fr) auto; align-items: center; gap: 12px; border: 1px solid rgba(255,255,255,.1); border-radius: 15px; background: linear-gradient(100deg, rgba(255,255,255,.075), rgba(255,255,255,.025)); box-shadow: 0 18px 40px rgba(0,0,0,.18); transition: border-color var(--duration-fast) ease, transform var(--duration-fast) ease; }
.feature-card:hover { border-color: rgba(255,255,255,.2); transform: translateX(3px); }
.feature-icon { width: 44px; height: 44px; display: grid; place-items: center; border-radius: 13px; }
.feature-icon.cyan { background: rgba(114,217,230,.13); color: var(--accent-cyan); }
.feature-icon.red { background: var(--accent-dim); color: var(--accent); }
.feature-icon.violet { background: rgba(171,126,255,.13); color: #ad83ff; }
.feature-icon svg { width: 22px; stroke: currentColor; stroke-width: 1.7; stroke-linecap: round; stroke-linejoin: round; }
.feature-card small,.feature-card b { display: block; }.feature-card small { color: #85858c; font-size: 9px; letter-spacing: .12em; }.feature-card b { margin-top: 2px; font-size: 13px; }
.feature-card em { color: rgba(255,255,255,.16); font-size: 18px; font-style: normal; font-weight: 900; letter-spacing: -.04em; }
.auth-panel { display: grid; place-items: center; padding: 54px 42px; background: #242428; }
.auth-box { width: min(390px, 100%); }
.auth-heading > span { color: #fe2c55; font-size: 11px; font-weight: 800; letter-spacing: .12em; }
.auth-heading h2 { margin-top: 10px; font-size: 28px; letter-spacing: -.04em; }
.auth-heading p { margin-top: 8px; color: #777; font-size: 12px; }
.auth-form { margin-top: 32px; display: grid; gap: 18px; }
.auth-form label { margin: 0; }
.auth-form label > span { display: block; margin-bottom: 8px; color: #bbb; font-size: 12px; font-weight: 600; }
.input-shell { position: relative; }
.input-shell svg { position: absolute; top: 50%; left: 13px; width: 18px; transform: translateY(-50%); stroke: #666; stroke-width: 1.7; stroke-linecap: round; stroke-linejoin: round; }
.input-shell input { height: 46px; padding: 0 58px 0 42px; border-radius: 7px; background: var(--surface-raised); }
.show-password { position: absolute; top: 50%; right: 7px; padding: 6px 8px; transform: translateY(-50%); background: transparent; color: #777; font-size: 11px; }
.rule-list { margin-top: 8px; display: grid; grid-template-columns: 1fr 1fr; gap: 4px 10px; list-style: none; color: #777; font-size: 10px; }
.rule-list li::before { content: '○'; margin-right: 5px; }
.rule-list li.valid { color: #9a9a9a; }
.rule-list li.valid::before { content: '✓'; color: var(--ok); }
.match-tip { margin-top: 7px; color: var(--accent); font-size: 10px; }
.match-tip.valid { color: var(--ok); }
.availability-tip { margin-top: 7px; color: #777; font-size: 10px; }
.availability-tip.available { color: var(--ok); }
.availability-tip.unavailable { color: var(--accent); }
.submit-button { height: 46px; margin-top: 6px; border-radius: 7px; background: #fe2c55; color: #fff; font-weight: 700; }
.submit-button:hover { background: #ff405e; }
.auth-switch { margin-top: 24px; color: #777; text-align: center; font-size: 12px; }
.auth-switch a { color: #fff; font-weight: 700; }
@media (max-width: 980px) { .auth-page { grid-template-columns: 1fr; } .auth-visual { display: none; } .auth-panel { min-height: calc(100vh - 100px); } }

.auth-page { min-height: calc(100dvh - 110px); border-radius: 12px; background: #151517; }
.auth-visual { min-height: 620px; padding: 56px; background: #101012; }
.auth-visual::after { opacity: .06; background-size: 52px 52px; }
.visual-glow { opacity: .1; }
.visual-copy h1 { font-size: clamp(40px, 5vw, 62px); }
.feature-list { gap: 2px; }
.feature-card {
  min-height: 66px;
  border: 0;
  border-bottom: 1px solid var(--border);
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}
.feature-card:hover { transform: none; background: rgba(255,255,255,.025); }
.feature-icon { border-radius: 8px; }
.auth-panel { background: #19191c; }

.auth-page {
  width: min(480px, 100%);
  min-height: auto;
  margin: 36px auto;
  display: block;
  overflow: visible;
  border: 0;
  border-radius: 0;
  background: transparent;
}
.auth-visual { display: none; }
.auth-panel {
  min-height: 640px;
  padding: 42px 38px 32px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface-panel);
}
.auth-heading { text-align: center; }
.auth-heading h2 { font-size: 29px; }
@media (max-width: 600px) {
  .auth-page { margin: 8px auto; }
  .auth-panel { min-height: calc(100dvh - 110px); padding: 34px 22px; }
}
</style>
