<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import MembersView from "./MembersView.vue";
import AccessView from "./AccessView.vue";
import SettingsView from "./SettingsView.vue";
import {
  ShieldCheck,
  LogOut,
  RefreshCw,
  Search,
  ChevronLeft,
  ChevronRight,
  Check,
  X,
  Film,
  Inbox,
  ArrowLeft,
  Clock3,
} from "lucide-vue-next";
import {
  ApiError,
  getToken,
  post,
  setToken,
  type Video,
  type Detail,
} from "./api";

const route = useRoute(),
  router = useRouter();
const role = ref(""), myID = ref(0), setupAvailable = ref(false);
const publicAccess = computed(() => ["/activate", "/setup"].includes(route.path));
const ready = ref(false),
  username = ref(""),
  account = ref(""),
  password = ref(""),
  busy = ref(false),
  loading = ref(false),
  detailLoading = ref(false);
const error = ref(""),
  detailError = ref(""),
  notice = ref(""),
  status = ref("pending_review"),
  query = ref(""),
  offset = ref(0),
  total = ref(0),
  items = ref<Video[]>([]),
  detail = ref<Detail | null>(null);
const decision = ref<"approve" | "reject" | null>(null),
  reason = ref(""),
  modal = ref<HTMLDialogElement | null>(null),
  reasonInput = ref<HTMLTextAreaElement | null>(null);
const filters = [
  ["pending_review", "待审核"],
  ["published", "已发布"],
  ["rejected", "已驳回"],
  ["processing", "处理中"],
  ["failed", "处理失败"],
  ["deleted", "已撤回"],
];
const label = (value: string) =>
  filters.find((x) => x[0] === value)?.[1] ?? value;
const date = (value: string) =>
  new Date(value).toLocaleString("zh-CN", { hour12: false });
const isLogin = computed(() => route.path === "/login");
let listRequest = 0,
  detailRequest = 0,
  refreshTimer: ReturnType<typeof setInterval> | undefined;
function message(e: unknown) {
  return e instanceof Error ? e.message : String(e);
}
async function ended() {
  listRequest++;
  detailRequest++;
  ready.value = false;
  detail.value = null;
  items.value = [];
  decision.value = null;
  modal.value?.close();
  password.value = "";
  await router.replace("/login");
}
async function login() {
  if (busy.value) return;
  busy.value = true;
  error.value = "";
  try {
    const data = await post<{ token: string; username: string; role: string; account_id: number }>("login", {
      account_name: account.value,
      password: password.value,
    });
    setToken(data.token);
    username.value = data.username;
    role.value = data.role;
    myID.value = data.account_id;
    password.value = "";
    ready.value = true;
    await router.replace("/reviews");
    await load();
  } catch (e) {
    error.value = message(e);
  } finally {
    busy.value = false;
  }
}
async function logout() {
  busy.value = true;
  try {
    await post("logout");
    setToken("");
    await ended();
  } catch (e) {
    error.value = message(e);
  } finally {
    busy.value = false;
  }
}
async function load() {
  if (!ready.value) return;
  const id = ++listRequest;
  loading.value = true;
  error.value = "";
  try {
    const data = await post<{ items: Video[]; total: number }>("videos", {
      status: status.value,
      query: query.value,
      offset: offset.value,
      limit: 15,
    });
    if (id !== listRequest) return;
    items.value = data.items;
    total.value = data.total;
  } catch (e) {
    if (id === listRequest) error.value = message(e);
  } finally {
    if (id === listRequest) loading.value = false;
  }
}
async function select(id: number, quiet = false) {
  const req = ++detailRequest;
  if (!quiet) {
    detailLoading.value = true;
    detail.value = null;
  }
  detailError.value = "";
  try {
    const data = await post<Detail>("detail", { id });
    if (req !== detailRequest) return;
    detail.value = data;
    if (String(route.params.id) !== String(id))
      await router.replace(`/reviews/${id}`);
  } catch (e) {
    if (req === detailRequest) detailError.value = message(e);
  } finally {
    if (req === detailRequest) detailLoading.value = false;
  }
}
async function filter(value: string) {
  if (busy.value) return;
  status.value = value;
  offset.value = 0;
  detailRequest++;
  detail.value = null;
  await router.replace("/reviews");
  await load();
}
async function search() {
  offset.value = 0;
  await load();
}
async function page(delta: number) {
  offset.value += delta * 15;
  await load();
}
async function ask(value: "approve" | "reject") {
  decision.value = value;
  reason.value = "";
  detailError.value = "";
  await nextTick();
  modal.value?.showModal();
  if (value === "reject") reasonInput.value?.focus();
}
function cancel() {
  if (!busy.value) {
    modal.value?.close();
    decision.value = null;
  }
}
async function decide() {
  if (!detail.value || !decision.value || busy.value) return;
  busy.value = true;
  detailError.value = "";
  const id = detail.value.video.id,
    choice = decision.value;
  try {
    await post("decide", { id, decision: choice, reason: reason.value.trim() });
    modal.value?.close();
    decision.value = null;
    notice.value =
      choice === "approve"
        ? "已通过审核，作品已发布"
        : "已驳回，作者可查看原因";
    await select(id);
    await load();
  } catch (e) {
    detailError.value = message(e);
    if (e instanceof ApiError && e.status === 409) {
      modal.value?.close();
      decision.value = null;
      await select(id);
      await load();
    }
  } finally {
    busy.value = false;
  }
}
watch(
  () => route.params.id,
  (id) => {
    if (ready.value && id && detail.value?.video.id !== Number(id))
      void select(Number(id));
  },
);
onMounted(async () => {
  window.addEventListener("review-session-ended", ended);
  refreshTimer = setInterval(
    () => {
      if (route.path.startsWith('/reviews') && detail.value && !decision.value && !busy.value)
        void select(detail.value.video.id, true);
    },
    4 * 60 * 1000,
  );
  try { setupAvailable.value = (await post<{available: boolean}>("setup/status")).available; } catch { /* Login remains available during a status lookup failure. */ }
  if (publicAccess.value) return;
  if (!getToken()) {
    await ended();
    return;
  }
  try {
    const me = await post<{ username: string; role: string; id: number }>("me");
    username.value = me.username;
    role.value = me.role;
    myID.value = me.id;
    ready.value = true;
    if (isLogin.value) await router.replace("/reviews");
    await load();
    if (route.params.id) await select(Number(route.params.id));
  } catch (e) {
    error.value = message(e);
  }
});
onUnmounted(() => {
  window.removeEventListener("review-session-ended", ended);
  clearInterval(refreshTimer);
  listRequest++;
  detailRequest++;
});
</script>

<template>
  <div class="app">
    <header class="topbar">
      <div class="brand">
        <ShieldCheck :size="25" /><strong>VideoHub</strong><span>审核中心</span>
      </div>
      <div v-if="ready" class="operator">
        <span>{{ username }}</span
        ><button
          class="icon"
          title="退出登录"
          aria-label="退出登录"
          :disabled="busy"
          @click="logout"
        >
          <LogOut :size="18" />
        </button>
      </div>
    </header>
    <nav v-if="ready && !publicAccess && !isLogin" class="site-nav" aria-label="主导航">
      <RouterLink to="/reviews">视频审核</RouterLink>
      <RouterLink v-if="role === 'owner'" to="/members">成员管理</RouterLink>
      <RouterLink v-if="role === 'owner'" to="/audit">操作日志</RouterLink>
      <RouterLink to="/settings">个人设置</RouterLink>
    </nav>
    <AccessView v-if="publicAccess" :key="route.path" />
    <main v-else-if="isLogin" class="login">
      <ShieldCheck :size="38" class="login-mark" />
      <h1>管理员登录</h1>
      <form @submit.prevent="login">
        <label
          >账号<input
            v-model="account"
            autocomplete="username"
            maxlength="64"
            required
            autofocus /></label
        ><label
          >密码<input
            v-model="password"
            type="password"
            autocomplete="current-password"
            required
        /></label>
        <p v-if="error" class="error" role="alert">{{ error }}</p>
        <button class="primary" :disabled="busy">
          {{ busy ? "正在登录…" : "登录审核中心" }}
        </button>
      </form>
      <p class="login-help">忘记密码请联系超级管理员。</p>
      <RouterLink v-if="setupAvailable" to="/setup">创建首位超级管理员</RouterLink>
    </main>
    <SettingsView v-else-if="ready && route.path === '/settings'" @changed="setToken(''); ended()" />
    <MembersView v-else-if="ready && ['/members', '/audit'].includes(route.path) && role === 'owner'" :key="route.path" :audit-mode="route.path === '/audit'" :my-id="myID" @self-changed="setToken(''); ended()" />
    <main v-else-if="ready && ['/members', '/audit'].includes(route.path)" class="empty tall"><h1>无访问权限</h1><RouterLink to="/reviews">返回视频审核</RouterLink></main>
    <main v-else-if="ready" class="workspace">
      <div class="heading">
        <div>
          <p class="eyebrow">CONTENT REVIEW</p>
          <h1>视频审核</h1>
        </div>
        <button
          class="icon"
          title="刷新列表"
          aria-label="刷新列表"
          :disabled="loading || busy"
          @click="load"
        >
          <RefreshCw :size="19" :class="{ spin: loading }" />
        </button>
      </div>
      <p v-if="notice" class="notice" role="status">
        {{ notice
        }}<button
          class="icon"
          title="关闭提示"
          aria-label="关闭提示"
          @click="notice = ''"
        >
          <X :size="16" />
        </button>
      </p>
      <nav class="tabs" aria-label="审核状态">
        <button
          v-for="[value, text] in filters"
          :key="value"
          :class="{ active: status === value }"
          :disabled="busy"
          @click="filter(value!)"
        >
          {{ text }}<span v-if="status === value">{{ total }}</span>
        </button>
      </nav>
      <div class="review-layout">
        <section class="queue">
          <form class="search" @submit.prevent="search">
            <Search :size="18" /><input
              v-model="query"
              placeholder="搜索标题或作者"
              aria-label="搜索标题或作者"
              maxlength="100"
            /><button
              type="submit"
              class="icon"
              title="搜索"
              aria-label="搜索"
              :disabled="loading"
            >
              <ChevronRight :size="18" />
            </button>
          </form>
          <p v-if="error" class="error" role="alert">
            {{ error }} <button @click="load">重试</button>
          </p>
          <div v-if="loading" class="empty">
            <RefreshCw :size="24" class="spin" />正在加载
          </div>
          <div v-else-if="!items.length" class="empty">
            <Inbox :size="36" /><strong>暂无{{ label(status) }}视频</strong>
          </div>
          <div v-else class="queue-items">
            <button
              v-for="item in items"
              :key="item.id"
              class="queue-item"
              :class="{ selected: detail?.video.id === item.id }"
              :disabled="busy"
              @click="select(item.id)"
            >
              <span class="item-symbol"><Film :size="20" /></span
              ><span class="item-content"
                ><strong>{{ item.title }}</strong
                ><span>{{ item.username }} <i>·</i> #{{ item.id }}</span
                ><small>{{ date(item.create_time) }}</small></span
              ><ChevronRight :size="16" />
            </button>
          </div>
          <footer class="pagination">
            <span
              >共 {{ total }} 条 · 第 {{ Math.floor(offset / 15) + 1 }} 页</span
            >
            <div>
              <button
                class="icon"
                title="上一页"
                aria-label="上一页"
                :disabled="offset === 0 || loading || busy"
                @click="page(-1)"
              >
                <ChevronLeft :size="18" /></button
              ><button
                class="icon"
                title="下一页"
                aria-label="下一页"
                :disabled="offset + 15 >= total || loading || busy"
                @click="page(1)"
              >
                <ChevronRight :size="18" />
              </button>
            </div>
          </footer>
        </section>
        <section class="inspection" aria-label="审核详情">
          <div v-if="detailLoading" class="empty tall">
            <RefreshCw :size="28" class="spin" />正在加载视频
          </div>
          <div v-else-if="!detail" class="empty tall">
            <ShieldCheck :size="48" />
            <h2>待审内容</h2>
            <p v-if="detailError" class="error">{{ detailError }}</p>
            <p v-else>尚未选择视频</p>
          </div>
          <template v-else>
            <div class="detail-header">
              <span class="badge" :class="detail.video.status">{{
                label(detail.video.status)
              }}</span
              ><span class="muted">视频 #{{ detail.video.id }}</span>
            </div>
            <h2 class="video-title">{{ detail.video.title }}</h2>
            <p class="metadata">
              {{ detail.video.username }} · 作者 #{{ detail.video.author_id }} ·
              {{ date(detail.video.create_time) }}
            </p>
            <video
              v-if="detail.video.play_url"
              :key="detail.video.id"
              class="player"
              :src="detail.video.play_url"
              controls
              playsinline
              preload="metadata"
              @error="detailError = '预览加载失败，请刷新预览'"
            />
            <div v-else class="player unavailable">
              <Film :size="32" /><span>{{
                detail.video.status === "processing"
                  ? "视频处理中"
                  : "暂无可播放文件"
              }}</span>
            </div>
            <div class="media-caption">
              <span
                >{{ detail.video.width }} × {{ detail.video.height }} ·
                {{ (detail.video.duration_millis / 1000).toFixed(1) }} 秒</span
              ><button
                class="icon"
                title="刷新预览"
                aria-label="刷新预览"
                :disabled="busy"
                @click="select(detail.video.id)"
              >
                <RefreshCw :size="16" />
              </button>
            </div>
            <div class="submission-content">
              <div>
                <h3>投稿简介</h3>
                <p>{{ detail.video.description || "未填写简介" }}</p>
                <p v-if="detail.video.processing_error" class="error">
                  {{ detail.video.processing_error }}
                </p>
              </div>
              <div v-if="detail.video.cover_url" class="cover">
                <h3>投稿封面</h3>
                <a
                  :href="detail.video.cover_url"
                  target="_blank"
                  rel="noopener noreferrer"
                  ><img :src="detail.video.cover_url" alt="投稿封面"
                /></a>
              </div>
            </div>
            <div v-if="detail.reviews.length" class="history">
              <h3>审核记录</h3>
              <div v-for="review in detail.reviews" :key="review.id">
                <Clock3 :size="16" />
                <div>
                  <strong>{{
                    review.decision === "approve" ? "审核通过" : "已驳回"
                  }}</strong>
                  <p>
                    {{ review.reviewer_name }} · {{ date(review.create_time) }}
                  </p>
                  <p v-if="review.reason">{{ review.reason }}</p>
                </div>
              </div>
            </div>
            <p v-if="detailError && !decision" class="error" role="alert">
              {{ detailError }}
            </p>
            <div
              v-if="detail.video.status === 'pending_review'"
              class="actions"
            >
              <button class="danger" :disabled="busy" @click="ask('reject')">
                <X :size="18" />驳回</button
              ><button class="primary" :disabled="busy" @click="ask('approve')">
                <Check :size="18" />通过并发布
              </button>
            </div>
          </template>
        </section>
      </div>
    </main>
    <main v-else class="empty tall">
      <p v-if="error" class="error">{{ error }}</p>
      <span v-else>正在验证登录</span
      ><button v-if="error" @click="router.replace('/login')">返回登录</button>
    </main>
    <dialog ref="modal" @cancel.prevent="cancel">
      <form @submit.prevent="decide">
        <div class="dialog-heading">
          <h2>
            {{ decision === "approve" ? "通过审核并发布？" : "驳回投稿" }}
          </h2>
          <button
            class="icon"
            type="button"
            title="关闭"
            aria-label="关闭"
            :disabled="busy"
            @click="cancel"
          >
            <X :size="20" />
          </button>
        </div>
        <p class="dialog-title">{{ detail?.video.title }}</p>
        <label
          >{{ decision === "reject" ? "驳回原因" : "审核备注（选填）"
          }}<textarea
            ref="reasonInput"
            v-model="reason"
            :required="decision === 'reject'"
            maxlength="500"
            rows="4"
          />
        </label>
        <p v-if="detailError" class="error" role="alert">{{ detailError }}</p>
        <div class="actions">
          <button type="button" :disabled="busy" @click="cancel">
            <ArrowLeft :size="16" />取消</button
          ><button
            :class="decision === 'approve' ? 'primary' : 'danger'"
            :disabled="busy || (decision === 'reject' && !reason.trim())"
          >
            {{
              busy
                ? "正在保存…"
                : "确认" + (decision === "approve" ? "发布" : "驳回")
            }}
          </button>
        </div>
      </form>
    </dialog>
  </div>
</template>
