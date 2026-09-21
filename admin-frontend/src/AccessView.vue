<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ShieldCheck } from 'lucide-vue-next';
import { post, setToken } from './api';
const route = useRoute(), router = useRouter();
const setup = route.path === '/setup';
const token = computed(() => route.hash.slice(1));
const loading = ref(true), busy = ref(false), available = ref(false), done = ref(false), error = ref('');
const password = ref(''), confirm = ref(''), account = ref(''), username = ref(''), setupToken = ref('');
const info = ref<{username: string; account_name: string; kind: string; role: string} | null>(null);
let request = 0;
watch(() => route.hash, async () => {
  if (done.value && !route.hash) return;
  const id = ++request;
  loading.value = true; done.value = false; info.value = null; error.value = '';
  password.value = ''; confirm.value = '';
  try {
    if (setup) available.value = (await post<{available: boolean}>('setup/status')).available;
    else {
      const result = await post<{username: string; account_name: string; kind: string; role: string}>('link/info', {token: token.value});
      if (id === request) info.value = result;
    }
  } catch(e) { if(id === request) error.value = (e as Error).message; }
  finally { if(id === request) loading.value = false; }
}, {immediate: true});
async function submit() {
  if (busy.value) return;
  error.value = '';
  if (password.value !== confirm.value) { error.value = '两次密码不一致'; return; }
  busy.value = true;
  try {
    if (setup) await post('setup', {setup_token: setupToken.value, account_name: account.value, username: username.value, password: password.value});
    else await post('link/accept', {token: token.value, password: password.value});
    password.value = ''; confirm.value = ''; setupToken.value = '';
    setToken(''); done.value = true;
    await router.replace({path: route.path, hash: ''});
  } catch(e) { error.value = (e as Error).message; }
  finally { busy.value = false; }
}
</script>
<template>
  <main class="login access-form">
    <ShieldCheck :size="38" class="login-mark" />
    <h1>{{ setup ? '创建超级管理员' : info?.kind === 'reset' ? '重置密码' : '激活审核账号' }}</h1>
    <p v-if="loading" role="status">正在验证</p>
    <template v-else-if="done"><p class="notice" role="status">{{ setup ? '超级管理员已创建' : '密码已设置' }}</p><a href="/login">返回登录</a></template>
    <template v-else>
      <p v-if="setup && !available && !error" class="error">初始化未开放或已完成</p>
      <form v-if="(setup && available) || info" @submit.prevent="submit">
        <template v-if="setup">
          <label>安装凭证<input v-model="setupToken" type="password" autocomplete="off" required /></label>
          <label>姓名<input v-model="username" maxlength="24" autocomplete="name" required /></label>
          <label>登录账号<input v-model="account" minlength="3" maxlength="64" pattern="[a-zA-Z0-9][a-zA-Z0-9._@+\-]{2,63}" autocomplete="username" required /></label>
        </template>
        <dl v-else-if="info" class="identity"><dt>姓名</dt><dd>{{ info.username }}</dd><dt>登录账号</dt><dd>{{ info.account_name }}</dd><dt>角色</dt><dd>{{ info.role === 'owner' ? '超级管理员' : '审核员' }}</dd></dl>
        <label>新密码<input v-model="password" type="password" minlength="12" maxlength="72" placeholder="至少 12 位" autocomplete="new-password" required /></label>
        <label>确认密码<input v-model="confirm" type="password" minlength="12" maxlength="72" autocomplete="new-password" required /></label>
        <p v-if="error" class="error" role="alert">{{ error }}</p>
        <button class="primary" :disabled="busy">{{ busy ? '正在保存' : setup ? '创建超级管理员' : '确认设置密码' }}</button>
      </form>
      <p v-else-if="error" class="error" role="alert">{{ error }}</p>
      <a class="login-help" href="/login">返回登录</a>
    </template>
  </main>
</template>
