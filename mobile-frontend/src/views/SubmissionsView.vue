<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { submissions, statusLabel, type Submission } from "../api/submissions";
import { useAuthStore } from "../stores/auth";
import { useDialogStore } from "../stores/dialog";
import AppIcon from "../components/AppIcon.vue";
const route = useRoute(),
  router = useRouter(),
  auth = useAuthStore(),
  dialog = useDialogStore();
const items = ref<Submission[]>([]),
  selected = ref<Submission | null>(null),
  offset = ref(0),
  total = ref(0),
  loading = ref(false),
  busy = ref(false),
  error = ref("");
let request = 0,
  previewRefreshedAt = 0,
  timer: ReturnType<typeof setInterval> | undefined;
async function load() {
  const seq = ++request;
  if (!auth.isLoggedIn) {
    items.value = [];
    selected.value = null;
    return;
  }
  loading.value = true;
  error.value = "";
  try {
    const list = await submissions.list(offset.value);
    const detail = route.params.id
      ? await submissions.detail(Number(route.params.id))
      : null;
    if (seq !== request) return;
    items.value = list.items;
    total.value = list.total;
    if (
      detail &&
      selected.value?.id === detail.id &&
      selected.value.status === detail.status &&
      Date.now() - previewRefreshedAt < 240000
    ) {
      const { play_url: _, cover_url: __, ...metadata } = detail;
      Object.assign(selected.value, metadata);
    } else {
      selected.value = detail;
      previewRefreshedAt = Date.now();
    }
  } catch (e) {
    if (seq === request) {
      error.value = e instanceof Error ? e.message : String(e);
      selected.value = null;
    }
  } finally {
    if (seq === request) loading.value = false;
  }
}
async function remove(item: Submission) {
  if (
    busy.value ||
    !(await dialog.ask({
      title: "撤回这条投稿？",
      message: "撤回后视频将无法观看。",
      confirmLabel: "撤回",
      tone: "danger",
    }))
  )
    return;
  busy.value = true;
  try {
    await submissions.remove(item.id);
    if (selected.value?.id === item.id) await router.replace("/submissions");
    await load();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}
async function page(delta: number) {
  offset.value += delta * 20;
  await load();
}
function refresh() {
  selected.value = null;
  void load();
}
watch(
  () => route.params.id,
  () => void load(),
);
watch(
  () => auth.token,
  () => {
    request++;
    items.value = [];
    selected.value = null;
    void load();
  },
);
onMounted(() => {
  void load();
  timer = setInterval(() => {
    if (
      !loading.value &&
      !busy.value &&
      !document.hidden &&
      (items.value.some(
        (v) => v.status === "processing" || v.status === "pending_review",
      ) ||
        (selected.value && Date.now() - previewRefreshedAt >= 240000))
    )
      void load();
  }, 10000);
});
onUnmounted(() => {
  request++;
  clearInterval(timer);
});
</script>
<template>
  <main class="submissions-page">
    <header>
      <RouterLink to="/" class="back" aria-label="返回社区" title="返回社区"
        ><AppIcon name="back"
      /></RouterLink>
      <h1>我的投稿</h1>
      <button
        aria-label="刷新投稿"
        title="刷新投稿"
        :disabled="loading || busy"
        @click="refresh"
      >
        <AppIcon name="refresh" :size="20" />
      </button>
    </header>
    <p v-if="!auth.isLoggedIn">请先登录社区账号</p>
    <p v-if="error" class="submission-error" role="alert">{{ error }}</p>
    <section v-if="selected" class="submission-detail">
      <div class="detail-heading">
        <h2>{{ selected.title }}</h2>
        <span class="state" :class="selected.status">{{
          statusLabel(selected.status)
        }}</span>
      </div>
      <p v-if="selected.review_reason" class="submission-error">
        审核意见：{{ selected.review_reason }}
      </p>
      <p v-if="selected.processing_error" class="submission-error">
        视频处理失败，请重新上传。
      </p>
      <video
        v-if="selected.play_url"
        :key="selected.id"
        :src="selected.play_url"
        :poster="selected.cover_url"
        controls
        playsinline
        preload="metadata"
      />
      <p v-else class="waiting">视频处理中</p>
      <p>{{ selected.description }}</p>
      <small
        >提交时间 {{ new Date(selected.create_time).toLocaleString() }}</small
      >
      <RouterLink
        v-if="selected.status === 'published'"
        :to="`/video/${selected.id}`"
        >查看社区作品</RouterLink
      >
    </section>
    <div class="list-heading">
      <h2>
        全部投稿 <small>{{ total }}</small>
      </h2>
      <span v-if="loading">正在更新…</span>
    </div>
    <div class="submission-list">
      <article v-for="item in items" :key="item.id">
        <button class="open" @click="router.push(`/submissions/${item.id}`)">
          <img
            v-if="item.cover_url"
            :src="item.cover_url"
            :alt="item.title"
          /><span v-else class="cover-placeholder"><AppIcon name="play" /></span
          ><span class="content"
            ><strong>{{ item.title }}</strong
            ><span class="state" :class="item.status">{{
              statusLabel(item.status)
            }}</span
            ><small>{{ new Date(item.create_time).toLocaleString() }}</small
            ><span v-if="item.review_reason" class="reason">{{
              item.review_reason
            }}</span></span
          >
        </button>
        <button
          class="remove"
          title="撤回投稿"
          aria-label="撤回投稿"
          :disabled="busy"
          @click="remove(item)"
        >
          <AppIcon name="trash" :size="18" />
        </button>
      </article>
      <p v-if="!loading && !items.length">暂无投稿</p>
    </div>
    <footer>
      <button :disabled="offset === 0 || loading" @click="page(-1)">
        上一页</button
      ><span>{{ Math.floor(offset / 20) + 1 }}</span
      ><button :disabled="offset + 20 >= total || loading" @click="page(1)">
        下一页
      </button>
    </footer>
  </main>
</template>
<style scoped>
.submissions-page {
  max-width: 900px;
  margin: auto;
  padding: 25px 20px 110px;
  color: #eceef0;
  min-height: 100dvh;
  background: #17181b;
  letter-spacing: 0;
}
.submissions-page header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 28px;
}
.submissions-page h1 {
  font-size: 23px;
  margin: 0;
  flex: 1;
}
.submissions-page h2 {
  font-size: 18px;
  margin: 0;
  overflow-wrap: anywhere;
}
.submissions-page button,
.back {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #3a3d42;
  border-radius: 5px;
  padding: 9px;
  background: #25272c;
  color: #e6e8eb;
  cursor: pointer;
}
.submissions-page button:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.submissions-page a {
  color: #83d7b6;
}
.submission-detail {
  padding-bottom: 24px;
  border-bottom: 1px solid #34373c;
  margin-bottom: 24px;
}
.detail-heading {
  display: flex;
  gap: 14px;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}
.submission-detail video {
  width: 100%;
  aspect-ratio: 16/9;
  max-height: 440px;
  background: #090a0b;
  object-fit: contain;
  border-radius: 5px;
}
.submission-detail p {
  line-height: 1.7;
  overflow-wrap: anywhere;
}
.submission-detail > a {
  display: block;
  margin-top: 16px;
}
.submissions-page small {
  color: #989da6;
  font-size: 12px;
}
.state {
  font-size: 12px;
  color: #c8ccd2;
  white-space: nowrap;
}
.state.pending_review {
  color: #e8c76c;
}
.state.published {
  color: #80d5ab;
}
.state.rejected,
.state.failed {
  color: #f29399;
}
.submission-error {
  background: #3a2026;
  color: #f5abb0;
  border-radius: 5px;
  padding: 12px;
  overflow-wrap: anywhere;
  line-height: 1.6;
}
.list-heading {
  display: flex;
  justify-content: space-between;
  margin: 18px 0;
}
.list-heading > span {
  font-size: 12px;
  color: #9ba4ad;
}
.submission-list article {
  display: flex;
  gap: 14px;
  align-items: center;
  border-bottom: 1px solid #33363b;
  padding: 16px 0;
}
.submission-list .open {
  display: flex;
  flex: 1;
  min-width: 0;
  background: none;
  border: 0;
  padding: 0;
  gap: 15px;
  text-align: left;
  justify-content: flex-start;
}
.submission-list img,
.cover-placeholder {
  width: 110px;
  height: 75px;
  object-fit: cover;
  border-radius: 4px;
  flex-shrink: 0;
  background: #292c32;
  display: grid;
  place-items: center;
}
.content {
  display: grid;
  gap: 7px;
  min-width: 0;
}
.content strong,
.reason {
  overflow-wrap: anywhere;
}
.reason {
  font-size: 12px;
  color: #e89ea4;
}
.remove {
  flex-shrink: 0;
}
.submissions-page footer {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 20px;
  margin-top: 25px;
}
.waiting {
  text-align: center;
  padding: 70px 10px;
  background: #22252a;
}
@media (max-width: 600px) {
  .submissions-page {
    padding: 20px 14px 100px;
  }
  .submission-list img,
  .cover-placeholder {
    width: 80px;
    height: 65px;
  }
  .detail-heading {
    align-items: flex-start;
  }
  .submission-list .open {
    gap: 10px;
  }
  .content small {
    font-size: 11px;
  }
}
</style>
