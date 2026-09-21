<script setup lang="ts">
import { ref } from 'vue';
import { KeyRound } from 'lucide-vue-next';
import { post } from './api';
const emit = defineEmits<{changed: []}>();
const oldPassword = ref(''), password = ref(''), confirm = ref(''), busy = ref(false), error = ref('');
async function submit() {
  if (busy.value) return;
  error.value = '';
  if (password.value !== confirm.value) { error.value = '两次密码不一致'; return; }
  busy.value = true;
  try { await post('password', {old_password: oldPassword.value, new_password: password.value}); emit('changed'); }
  catch(e) { error.value = (e as Error).message; }
  finally { busy.value = false; }
}
</script>
<template>
  <main class="workspace"><h1>个人设置</h1>
    <form class="password-form" @submit.prevent="submit">
      <h2>修改密码</h2>
      <label>原密码<input v-model="oldPassword" type="password" autocomplete="current-password" required /></label>
      <label>新密码<input v-model="password" type="password" minlength="12" maxlength="72" placeholder="至少 12 位" autocomplete="new-password" required /></label>
      <label>确认密码<input v-model="confirm" type="password" minlength="12" maxlength="72" autocomplete="new-password" required /></label>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <button class="primary" :disabled="busy"><KeyRound :size="18" />{{ busy ? '正在保存' : '修改并重新登录' }}</button>
    </form>
  </main>
</template>
