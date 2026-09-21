<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue';
import { UserPlus, RefreshCw, Search, ChevronLeft, ChevronRight, Pencil, KeyRound, Link2, Link2Off, UserX, UserCheck, Copy, X, Users, Trash2 } from 'lucide-vue-next';
import { post } from './api';
const props = defineProps<{auditMode: boolean; myId: number}>();
const emit = defineEmits<{selfChanged: []}>();
type Member = {id: number; account_name: string; username: string; role: string; status: string; activated: boolean; last_login_at?: string; creator_name: string; link_kind?: string; link_expires_at?: string};
type Audit = {id: number; actor_name: string; target_name: string; action: string; before: string; after: string; created_at: string};
type LinkResult = {activation_path: string; expires_at: string};
const members = ref<Member[]>([]), logs = ref<Audit[]>([]), total = ref(0), offset = ref(0), query = ref(''), status = ref('');
const loading = ref(false), busy = ref(false), error = ref(''), dialogError = ref(''), notice = ref('');
const dialog = ref<HTMLDialogElement | null>(null), mode = ref(''), selected = ref<Member | null>(null);
const name = ref(''), account = ref(''), role = ref('reviewer'), link = ref(''), expires = ref(''), copied = ref(false);
const deleteAccount = ref('');
const date = (value?: string) => value ? new Date(value).toLocaleString('zh-CN', {hour12: false}) : '未登录';
const roleName = (value: string) => value === 'owner' ? '超级管理员' : '审核员';
const statusName = (value: string) => ({pending:'待激活', active:'正常', disabled:'已停用'}[value] || value);
const actions: Record<string, string> = {invite:'添加成员', update:'修改成员', delete:'删除成员', invite_link:'重新邀请', reset_link:'发起密码重置', revoke_link:'撤销链接', accept_invite:'激活账号', accept_reset:'完成密码重置', change_password:'修改密码', setup:'初始化管理员', migration:'迁移管理员', recovery:'运维恢复'};
const stateLabel = (value: string) => value ? value.split('/').map(x => ({owner:'超级管理员', reviewer:'审核员', admin:'管理员', active:'正常', pending:'待激活', disabled:'已停用', deleted:'已删除'}[x] || x)).join(' / ') : '-';
function invitation(member: Member) {
  if (!member.link_kind) return member.status === 'pending' ? '未发送或已撤销' : '-';
  return (member.link_kind === 'invite' ? '邀请' : '重置') + (new Date(member.link_expires_at!).getTime() > Date.now() ? '有效至 ' : '已过期于 ') + date(member.link_expires_at);
}
async function load() {
  loading.value = true; error.value = '';
  try {
    if(props.auditMode) { const data = await post<{items: Audit[]; total: number}>('audit', {offset: offset.value}); logs.value = data.items; total.value = data.total; }
    else {
      const data = await post<{items: Member[]; total: number}>('members', {query: query.value.trim(), status: status.value, offset: offset.value});
      if (offset.value > 0 && offset.value >= data.total) { offset.value = Math.max(0, Math.floor((data.total - 1) / 20) * 20); await load(); return; }
      members.value = data.items; total.value = data.total;
    }
  } catch(e) { error.value = (e as Error).message; }
  finally { loading.value = false; }
}
async function search() { offset.value = 0; await load(); }
async function page(delta: number) { offset.value += delta*20; await load(); }
async function open(action: string, member: Member | null = null) {
  mode.value = action; selected.value = member; dialogError.value = ''; link.value = ''; copied.value = false;
  name.value = ''; account.value = ''; role.value = member?.role ?? 'reviewer';
  deleteAccount.value = '';
  await nextTick(); dialog.value?.showModal();
}
function close() { if (!busy.value) { dialog.value?.close(); link.value = ''; selected.value = null; mode.value = ''; } }
async function copy() {
  try { await navigator.clipboard.writeText(link.value); copied.value = true; }
  catch { dialogError.value = '自动复制失败，请选中链接后复制'; }
}
async function submit() {
  if(busy.value) return;
  busy.value = true; dialogError.value = '';
  try {
    let result: LinkResult | undefined;
    const member = selected.value;
    if(mode.value === 'add') result = await post<LinkResult>('members/invite', {username: name.value, account_name: account.value, role: role.value});
    else if(member && ['invite','reset','revoke'].includes(mode.value)) {
      result = await post<LinkResult>('members/link', {id: member.id, action: mode.value});
    } else if(member && mode.value === 'delete') {
      await post('members/delete', {id: member.id, account_name: deleteAccount.value});
    } else if(member) {
      const newStatus = mode.value === 'disable' ? 'disabled' : mode.value === 'enable' ? (member.activated ? 'active' : 'pending') : member.status;
      await post('members/update', {id: member.id, role: role.value, status: newStatus});
    }
    if(result?.activation_path) {
      link.value = new URL(result.activation_path, location.origin).href; expires.value = result.expires_at; mode.value = 'result';
    } else { dialog.value?.close(); notice.value = mode.value === 'delete' ? '成员已删除，历史记录已保留' : '操作已完成'; }
    if(member?.id === props.myId && ['edit','disable','enable','reset'].includes(mode.value)) { emit('selfChanged'); return; }
    await load();
  } catch(e) { dialogError.value = (e as Error).message; }
  finally { busy.value = false; }
}
onMounted(load);
</script>
<template>
  <main class="workspace members-workspace">
    <div class="heading"><h1>{{ auditMode ? '操作日志' : '成员管理' }}</h1><div class="toolbar-actions">
      <button class="icon" title="刷新" aria-label="刷新" :disabled="loading || busy" @click="load"><RefreshCw :size="18" /></button>
      <button v-if="!auditMode" class="primary" :disabled="busy" @click="open('add')"><UserPlus :size="18" />添加成员</button>
    </div></div>
    <form v-if="!auditMode" class="member-search" @submit.prevent="search">
      <div class="search-field"><Search :size="18" /><input v-model="query" aria-label="搜索成员" placeholder="搜索姓名或登录账号" maxlength="100" /></div>
      <select v-model="status" aria-label="成员状态" @change="search"><option value="">全部状态</option><option value="pending">待激活</option><option value="active">正常</option><option value="disabled">已停用</option></select>
      <button :disabled="loading">搜索</button>
    </form>
    <p v-if="error" role="alert" class="error">{{ error }}</p>
    <p v-if="notice" role="status" class="notice">{{ notice }}<button class="icon" aria-label="关闭提示" @click="notice = ''"><X :size="16" /></button></p>
    <div v-if="loading" class="empty"><RefreshCw :size="24" class="spin" />正在加载</div>
    <div v-else-if="!(auditMode ? logs.length : members.length)" class="empty"><Users :size="36" />{{ auditMode ? '暂无操作日志' : '没有符合条件的成员' }}</div>
    <div v-else class="table-scroll" tabindex="0" :aria-label="auditMode ? '操作日志列表' : '成员列表'">
      <table v-if="auditMode" class="data-table audit-table"><thead><tr><th>时间</th><th>操作人</th><th>操作</th><th>目标账号</th><th>变更前</th><th>变更后</th></tr></thead>
        <tbody><tr v-for="item in logs" :key="item.id"><td>{{ date(item.created_at) }}</td><td>{{ item.actor_name }}</td><td>{{ actions[item.action] || item.action }}</td><td>{{ item.target_name }}</td><td>{{ stateLabel(item.before) }}</td><td>{{ stateLabel(item.after) }}</td></tr></tbody>
      </table>
      <table v-else class="data-table"><thead><tr><th>成员</th><th>角色</th><th>状态</th><th>邀请 / 重置</th><th>最近登录</th><th>添加人</th><th>操作</th></tr></thead>
        <tbody><tr v-for="member in members" :key="member.id" :data-member="member.account_name">
          <td><strong>{{ member.username }}<span v-if="member.id === myId" class="muted">（我）</span></strong><small>{{ member.account_name }}</small></td>
          <td>{{ roleName(member.role) }}</td><td><span class="member-badge" :class="member.status">{{ statusName(member.status) }}</span></td>
          <td>{{ invitation(member) }}</td><td>{{ date(member.last_login_at) }}</td><td>{{ member.creator_name || '系统' }}</td>
          <td><div class="row-actions">
            <button class="icon" title="修改角色" aria-label="修改角色" :disabled="busy" @click="open('edit', member)"><Pencil :size="16" /></button>
            <button v-if="member.status === 'pending'" class="icon" title="重新邀请" aria-label="重新邀请" :disabled="busy" @click="open('invite', member)"><Link2 :size="16" /></button>
            <button v-if="member.status === 'active' && member.id !== myId" class="icon" title="重置密码" aria-label="重置密码" :disabled="busy" @click="open('reset', member)"><KeyRound :size="16" /></button>
            <button v-if="member.link_kind" class="icon" title="撤销链接" aria-label="撤销链接" :disabled="busy" @click="open('revoke', member)"><Link2Off :size="16" /></button>
            <button v-if="member.status !== 'disabled'" class="icon danger" title="停用成员" aria-label="停用成员" :disabled="busy" @click="open('disable', member)"><UserX :size="16" /></button>
            <button v-else class="icon" title="启用成员" aria-label="启用成员" :disabled="busy" @click="open('enable', member)"><UserCheck :size="16" /></button>
            <button v-if="member.id !== myId" class="icon danger" title="删除成员" aria-label="删除成员" :disabled="busy" @click="open('delete', member)"><Trash2 :size="16" /></button>
          </div></td>
        </tr></tbody>
      </table>
    </div>
    <footer class="pagination"><span>共 {{ total }} 条 · 第 {{ Math.floor(offset / 20) + 1 }} 页</span><div>
      <button class="icon" title="上一页" aria-label="上一页" :disabled="!offset || loading || busy" @click="page(-1)"><ChevronLeft :size="18" /></button>
      <button class="icon" title="下一页" aria-label="下一页" :disabled="offset + 20 >= total || loading || busy" @click="page(1)"><ChevronRight :size="18" /></button>
    </div></footer>
    <dialog ref="dialog" @cancel.prevent="close">
      <form @submit.prevent="submit"><div class="dialog-heading"><h2>{{ {add:'添加成员', edit:'修改角色', invite:'重新邀请', reset:'重置密码', revoke:'撤销链接', disable:'停用成员', enable:'启用成员', delete:'删除成员', result:'链接已生成'}[mode] }}</h2><button type="button" class="icon" title="关闭" aria-label="关闭" :disabled="busy" @click="close"><X :size="18" /></button></div>
        <template v-if="mode === 'result'">
          <label>激活或重置链接<textarea :value="link" readonly rows="4" @focus="($event.target as HTMLTextAreaElement).select()" /></label>
          <p class="muted">有效期至 {{ date(expires) }}</p>
          <p v-if="dialogError" class="error" role="alert">{{ dialogError }}</p>
          <div class="actions"><button type="button" @click="close">完成</button><button type="button" class="primary" @click="copy"><Copy :size="18" />{{ copied ? '已复制' : '复制链接' }}</button></div>
        </template>
        <template v-else>
          <p v-if="selected" class="dialog-title">{{ selected.username }} · {{ selected.account_name }}</p>
          <template v-if="mode === 'add'"><label>姓名<input v-model="name" maxlength="24" required /></label><label>登录账号<input v-model="account" minlength="3" maxlength="64" pattern="[a-zA-Z0-9][a-zA-Z0-9._@+\-]{2,63}" autocomplete="off" required /></label></template>
          <label v-if="['add','edit'].includes(mode)">角色<select v-model="role" aria-label="角色"><option value="reviewer">审核员</option><option value="owner">超级管理员</option></select></label>
          <p v-if="mode === 'edit'" class="dialog-title">保存后该成员需重新登录，尚未使用的邀请和重置链接将失效。</p>
          <p v-if="mode === 'disable'" class="dialog-title">停用后该成员将无法登录，已有登录和邀请链接立即失效。</p>
          <template v-if="mode === 'delete'">
            <p class="dialog-title">删除后无法恢复，登录、邀请和重置链接立即失效。历史审核和操作日志保留，登录账号不可重复使用。</p>
            <label>确认登录账号<input v-model="deleteAccount" autocomplete="off" maxlength="64" required /></label>
          </template>
          <p v-if="mode === 'reset'" class="dialog-title">将生成新的密码重置链接，并使已有登录失效。</p>
          <p v-if="mode === 'revoke'" class="dialog-title">尚未使用的邀请或重置链接将立即失效。</p>
          <p v-if="mode === 'invite'" class="dialog-title">将生成新的邀请链接，旧链接随即失效。</p>
          <p v-if="dialogError" class="error" role="alert">{{ dialogError }}</p>
          <div class="actions"><button type="button" :disabled="busy" @click="close">取消</button><button :class="['disable','revoke','delete'].includes(mode) ? 'danger' : 'primary'" :disabled="busy || (mode === 'delete' && deleteAccount.trim().toLowerCase() !== selected?.account_name)">{{ busy ? '正在保存' : mode === 'add' ? '生成邀请' : mode === 'delete' ? '确认删除' : '确认' }}</button></div>
        </template>
      </form>
    </dialog>
  </main>
</template>
