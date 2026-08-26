<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const title = computed(() => auth.sessionEndReason === 'replaced' ? '账号已在其他设备登录' : '登录状态已失效')
const message = computed(() => {
  if (auth.sessionEndReason === 'replaced') return '当前设备已被安全退出。如非本人操作，请重新登录后立即修改密码。'
  if (auth.sessionEndReason === 'revoked') return '账号已退出或密码已经变更，请使用当前密码重新登录。'
  return '登录凭证已过期，请重新登录后继续使用。'
})

async function acknowledge() {
  auth.dismissSessionNotice()
  await router.push('/me')
}
</script>

<template>
  <Transition name="session-dialog">
    <div v-if="auth.sessionEndReason" class="session-mask">
      <section class="session-dialog" role="alertdialog" aria-modal="true" :aria-label="title">
        <span class="session-mark">!</span>
        <h2>{{ title }}</h2>
        <p>{{ message }}</p>
        <button type="button" autofocus @click="acknowledge">重新登录</button>
      </section>
    </div>
  </Transition>
</template>

<style scoped>
.session-mask { position:fixed; z-index:1000; inset:0; padding:22px; display:grid; place-items:center; background:rgba(0,0,0,.76); backdrop-filter:blur(7px); }
.session-dialog { width:min(320px,100%); padding:28px 23px 20px; display:grid; justify-items:center; border:1px solid rgba(255,255,255,.11); border-radius:12px; background:#242427; box-shadow:0 24px 70px rgba(0,0,0,.62); color:#fff; text-align:center; }
.session-mark { width:40px; height:40px; display:grid; place-items:center; border-radius:50%; background:rgba(254,44,85,.14); color:#fe2c55; font-size:20px; font-weight:900; }
.session-dialog h2 { margin-top:16px; font-size:18px; }
.session-dialog p { margin-top:9px; color:#a1a1a7; font-size:11px; line-height:1.7; }
.session-dialog button { width:100%; height:44px; margin-top:22px; border-radius:6px; background:#fe2c55; color:#fff; font-size:13px; font-weight:800; }
.session-dialog-enter-active,.session-dialog-leave-active { transition:opacity .18s ease; }
.session-dialog-enter-from,.session-dialog-leave-to { opacity:0; }
</style>
