<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave, RouterLink, useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import { ApiError } from '../api/client'
import * as videoApi from '../api/video'
import type { Video } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'

const router = useRouter()
const auth = useAuthStore()
const toast = useToastStore()

const busy = ref(false)
const stage = ref('')
const published = ref<Video | null>(null)
const maxVideoBytes = 200 * 1024 * 1024
const maxCoverBytes = 10 * 1024 * 1024
const videoProgress = ref(0)
const draggingVideo = ref(false)

const videoInput = ref<HTMLInputElement | null>(null)
const coverInput = ref<HTMLInputElement | null>(null)

const publishForm = reactive({
  title: '',
  description: '',
  video: null as File | null,
  cover: null as File | null,
})

const preview = reactive({
  videoUrl: '',
  coverUrl: '',
})

const titleCount = computed(() => publishForm.title.length)
const descriptionCount = computed(() => publishForm.description.length)
const overallProgress = computed(() => {
  if (stage.value === '上传封面') return 8
  if (stage.value === '上传视频') return 8 + Math.round(videoProgress.value * 0.84)
  if (stage.value === '发布视频') return 96
  return 0
})

function formatMiB(bytes: number) {
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function setPreviewVideo(file: File | null) {
  if (preview.videoUrl) URL.revokeObjectURL(preview.videoUrl)
  preview.videoUrl = file ? URL.createObjectURL(file) : ''
}

function setPreviewCover(file: File | null) {
  if (preview.coverUrl) URL.revokeObjectURL(preview.coverUrl)
  preview.coverUrl = file ? URL.createObjectURL(file) : ''
}

watch(
  () => publishForm.video,
  (f) => setPreviewVideo(f),
)

watch(
  () => publishForm.cover,
  (f) => setPreviewCover(f),
)

function onBeforeUnload(event: BeforeUnloadEvent) {
  if (!busy.value) return
  event.preventDefault()
  event.returnValue = ''
}

onMounted(() => window.addEventListener('beforeunload', onBeforeUnload))

onBeforeRouteLeave(() => {
  if (!busy.value) return true
  toast.info('视频正在上传，请等待发布完成后再离开')
  return false
})

onUnmounted(() => {
  window.removeEventListener('beforeunload', onBeforeUnload)
  setPreviewVideo(null)
  setPreviewCover(null)
})

function selectVideo(file: File | null) {
  if (file && !videoApi.isSupportedVideo(file)) {
    toast.error('支持 MP4、MOV、M4V、WebM、3GP 视频')
    return
  }
  if (file && file.size > maxVideoBytes) {
    toast.error(`视频文件过大：${formatMiB(file.size)}，最大支持 ${formatMiB(maxVideoBytes)}`)
    publishForm.video = null
    return
  }
  publishForm.video = file
}

function pickVideo(e: Event) {
  const input = e.target as HTMLInputElement
  selectVideo(input.files?.[0] ?? null)
  if (!publishForm.video) input.value = ''
}

function dropVideo(e: DragEvent) {
  draggingVideo.value = false
  if (busy.value) return
  selectVideo(e.dataTransfer?.files?.[0] ?? null)
}

function pickCover(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0] ?? null
  if (file && !['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
    toast.error('封面仅支持 JPG、PNG 或 WebP')
    input.value = ''
    publishForm.cover = null
    return
  }
  if (file && file.size > maxCoverBytes) {
    toast.error(`封面文件过大：${formatMiB(file.size)}，最大支持 ${formatMiB(maxCoverBytes)}`)
    input.value = ''
    publishForm.cover = null
    return
  }
  publishForm.cover = file
}

function openVideoPicker() {
  videoInput.value?.click()
}

function openCoverPicker() {
  coverInput.value?.click()
}

function clearVideo() {
  publishForm.video = null
  if (videoInput.value) videoInput.value.value = ''
}

function clearCover() {
  publishForm.cover = null
  if (coverInput.value) coverInput.value.value = ''
}

async function onPublish() {
  if (busy.value) return
  if (!auth.isLoggedIn) {
    toast.error('请先登录')
    await router.push('/account')
    return
  }

  const title = publishForm.title.trim()
  const description = publishForm.description.trim()
  if (!title) {
    toast.error('请输入 title')
    return
  }
  if (!publishForm.video) {
    toast.error('请选择视频文件')
    return
  }
  if (publishForm.video.size > maxVideoBytes) {
    toast.error(`视频文件过大：${formatMiB(publishForm.video.size)}，最大支持 ${formatMiB(maxVideoBytes)}`)
    return
  }
  if (!publishForm.cover) {
    toast.error('请选择封面图片（jpg/png/webp）')
    return
  }
  if (publishForm.cover.size > maxCoverBytes) {
    toast.error(`封面文件过大：${formatMiB(publishForm.cover.size)}，最大支持 ${formatMiB(maxCoverBytes)}`)
    return
  }

  busy.value = true
  stage.value = ''
  published.value = null
  try {
    stage.value = '上传封面'
    const coverRes = await videoApi.uploadCover(publishForm.cover!)

    stage.value = '上传视频'
    videoProgress.value = 0
    const videoRes = await videoApi.uploadVideoSmart(publishForm.video!, (pct) => { videoProgress.value = pct })

    const coverUrl = coverRes.url || coverRes.cover_url || ''
    const playUrl = videoRes.url || videoRes.play_url || ''
    const coverObjectKey = coverRes.object_key || ''
    const playObjectKey = videoRes.object_key || ''
    if (!coverUrl || !playUrl || !coverObjectKey || !playObjectKey) {
      toast.error('上传成功但缺少文件地址或 object key')
      return
    }

    stage.value = '发布视频'
    const res = await videoApi.publishVideo({
      title,
      description,
      play_url: playUrl,
      cover_url: coverUrl,
      play_object_key: playObjectKey,
      cover_object_key: coverObjectKey,
    })

    published.value = res
    toast.success('已发布')

    publishForm.title = ''
    publishForm.description = ''
    clearVideo()
    clearCover()
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
  } finally {
    busy.value = false
    stage.value = ''
  }
}
</script>

<template>
  <AppShell>
    <div class="creator-page">
      <header class="creator-header">
        <div>
          <p class="eyebrow">CREATOR CENTER</p>
          <h1>发布视频</h1>
          <p>上传你的作品，让更多人看见。</p>
        </div>
        <div class="header-status">
          <span class="status-dot" :class="{ active: busy }" />
          {{ busy ? stage || '正在处理' : '完成视频、封面和标题后即可发布' }}
        </div>
      </header>

      <main class="publish-studio">
        <section class="studio-main">
          <input
            ref="videoInput"
            class="file-native"
            type="file"
            :accept="videoApi.VIDEO_ACCEPT"
            :disabled="busy"
            @change="pickVideo"
          />

          <button
            v-if="!publishForm.video"
            class="video-dropzone"
            :class="{ dragging: draggingVideo }"
            type="button"
            :disabled="busy"
            @click="openVideoPicker"
            @dragenter.prevent="draggingVideo = true"
            @dragover.prevent="draggingVideo = true"
            @dragleave.prevent="draggingVideo = false"
            @drop.prevent="dropVideo"
          >
            <span class="upload-icon" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="none">
                <path d="M12 16V4m0 0L7.5 8.5M12 4l4.5 4.5M5 14v4a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-4" />
              </svg>
            </span>
            <strong>拖拽视频到这里上传</strong>
            <span>或点击选择视频文件</span>
            <span class="select-video">选择视频</span>
            <span class="upload-rules">
              <b>支持手机常见格式</b>
              <i />
              <b>最大 200 MB</b>
              <i />
              <b>支持分片上传</b>
            </span>
          </button>

          <div v-else class="video-workspace">
            <div class="video-stage">
              <video class="video-preview" :src="preview.videoUrl" controls playsinline preload="metadata" />
              <div class="video-filebar">
                <div class="file-symbol" aria-hidden="true">
                  <svg viewBox="0 0 24 24" fill="none">
                    <path d="M15 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-5-5Z" />
                    <path d="M14 3v6h6M9 13l6 3-6 3v-6Z" />
                  </svg>
                </div>
                <div class="file-copy">
                  <strong>{{ publishForm.video.name }}</strong>
                  <span>{{ formatMiB(publishForm.video.size) }} · {{ videoApi.videoFileExtension(publishForm.video).slice(1).toUpperCase() }}</span>
                </div>
                <button class="text-button" type="button" :disabled="busy" @click="openVideoPicker">更换</button>
                <button class="text-button danger-text" type="button" :disabled="busy" @click="clearVideo">删除</button>
              </div>
            </div>
          </div>

          <section class="form-section">
            <div class="section-heading">
              <div>
                <h2>作品信息</h2>
                <p>清晰的标题和描述能帮助观众快速了解内容。</p>
              </div>
              <span>必填</span>
            </div>

            <label class="field">
              <span class="field-title"><b>作品标题</b><em>{{ titleCount }}/80</em></span>
              <input
                v-model="publishForm.title"
                maxlength="80"
                placeholder="填写作品标题，让更多人发现你的视频"
                :disabled="busy"
              />
            </label>

            <label class="field">
              <span class="field-title"><b>作品描述</b><em>{{ descriptionCount }}/500</em></span>
              <textarea
                v-model="publishForm.description"
                maxlength="500"
                placeholder="介绍一下视频内容，也可以分享创作灵感..."
                :disabled="busy"
              />
            </label>
          </section>

          <section class="form-section">
            <div class="section-heading">
              <div>
                <h2>视频封面</h2>
                <p>建议使用清晰、有辨识度的竖版图片。</p>
              </div>
            </div>

            <input
              ref="coverInput"
              class="file-native"
              type="file"
              accept="image/jpeg,image/png,image/webp"
              :disabled="busy"
              @change="pickCover"
            />

            <div class="cover-row">
              <button class="cover-picker" type="button" :disabled="busy" @click="openCoverPicker">
                <img v-if="preview.coverUrl" :src="preview.coverUrl" alt="封面预览" />
                <span v-else class="cover-placeholder">
                  <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                    <rect x="3" y="4" width="18" height="16" rx="2" />
                    <circle cx="9" cy="10" r="2" />
                    <path d="m5 18 5-5 3 3 2-2 4 4" />
                  </svg>
                  <b>上传封面</b>
                </span>
                <span v-if="preview.coverUrl" class="cover-change">更换封面</span>
              </button>
              <div class="cover-tips">
                <strong>{{ publishForm.cover ? publishForm.cover.name : '选择一张作品封面' }}</strong>
                <p>{{ publishForm.cover ? formatMiB(publishForm.cover.size) : '支持 JPG、PNG、WEBP，最大 10 MB' }}</p>
                <button v-if="publishForm.cover" class="text-button danger-text" type="button" :disabled="busy" @click="clearCover">
                  移除封面
                </button>
              </div>
            </div>
          </section>

          <section class="form-section publish-settings">
            <div class="section-heading">
              <div>
                <h2>发布设置</h2>
                <p>视频发布后将展示在 VideoHub 视频流中。</p>
              </div>
            </div>
            <div class="setting-row">
              <span class="setting-icon">
                <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <circle cx="12" cy="12" r="9" />
                  <path d="M3 12h18M12 3c2.2 2.5 3.3 5.5 3.3 9S14.2 18.5 12 21c-2.2-2.5-3.3-5.5-3.3-9S9.8 5.5 12 3Z" />
                </svg>
              </span>
              <div>
                <strong>公开可见</strong>
                <p>所有用户都可以浏览和互动</p>
              </div>
              <span class="setting-value">公开</span>
            </div>
          </section>
        </section>

        <aside class="studio-side">
          <div class="side-card">
            <span class="side-kicker">发布检查</span>
            <h3>准备好了吗？</h3>
            <ul class="check-list">
              <li :class="{ ready: publishForm.video }">
                <span>{{ publishForm.video ? '✓' : '1' }}</span>
                <div><b>上传视频</b><small>MP4 / MOV / M4V / WebM / 3GP，最大 200 MB</small></div>
              </li>
              <li :class="{ ready: publishForm.cover }">
                <span>{{ publishForm.cover ? '✓' : '2' }}</span>
                <div><b>选择封面</b><small>清晰封面更容易被发现</small></div>
              </li>
              <li :class="{ ready: publishForm.title.trim() }">
                <span>{{ publishForm.title.trim() ? '✓' : '3' }}</span>
                <div><b>填写标题</b><small>概括作品的主要内容</small></div>
              </li>
            </ul>
          </div>

          <div v-if="busy" class="side-card progress-card" role="progressbar" aria-label="视频发布进度" :aria-valuenow="overallProgress" aria-valuemin="0" aria-valuemax="100">
            <div class="progress-heading">
              <div>
                <span>正在处理</span>
                <strong>{{ stage || '准备中' }}</strong>
              </div>
              <b>{{ overallProgress }}%</b>
            </div>
            <div class="progress-bar">
              <div class="progress-fill" :style="{ width: `${overallProgress}%` }" />
            </div>
            <p>发布期间请不要关闭页面</p>
          </div>

          <div v-if="published" class="side-card success-card">
            <span class="success-mark">✓</span>
            <div>
              <strong>视频发布成功</strong>
              <p>{{ published.title }}</p>
            </div>
            <RouterLink :to="`/video/${published.id}`">查看作品</RouterLink>
          </div>
        </aside>
      </main>

      <footer class="publish-actions">
        <div>
          <strong>发布前请确认作品信息</strong>
          <span>视频、封面和标题均为必填项</span>
        </div>
        <div class="action-buttons">
          <button class="secondary-action" type="button" :disabled="busy" @click="router.push('/')">取消</button>
          <button class="publish-action" type="button" :disabled="busy" @click="onPublish">
            <span v-if="busy" class="spinner" />
            {{ busy ? stage || '发布中' : '发布视频' }}
          </button>
        </div>
      </footer>
    </div>
  </AppShell>
</template>

<style scoped>
.creator-page {
  width: min(1240px, 100%);
  margin: 0 auto;
  padding: 14px 0 110px;
  color: #f6f6f6;
}

.creator-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  padding: 18px 4px 24px;
}

.eyebrow {
  margin-bottom: 8px;
  color: #fe2c55;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: .18em;
}

.creator-header h1 {
  font-size: clamp(28px, 3vw, 38px);
  line-height: 1.1;
  letter-spacing: -.04em;
}

.creator-header > div > p:last-child {
  margin-top: 10px;
  color: #8d8d8d;
  font-size: 14px;
}

.header-status {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #858585;
  font-size: 12px;
}

.status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #5b5b5b;
}

.status-dot.active {
  background: #fe2c55;
  box-shadow: 0 0 0 5px rgba(254, 44, 85, .12);
}

.publish-studio {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 290px;
  gap: 18px;
  align-items: start;
}

.studio-main,
.side-card {
  border: 1px solid rgba(255, 255, 255, .08);
  background: var(--surface-panel);
  border-radius: 12px;
}

.studio-main {
  overflow: hidden;
}

.video-dropzone {
  width: calc(100% - 48px);
  min-height: 390px;
  margin: 24px;
  border: 1px dashed rgba(255, 255, 255, .19);
  border-radius: 10px;
  background: var(--surface-raised);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: #fff;
  transition: border-color 180ms ease, background 180ms ease;
}

.video-dropzone:hover,
.video-dropzone.dragging {
  border-color: #fe2c55;
  background: rgba(254, 44, 85, .045);
}

.upload-icon {
  width: 56px;
  height: 56px;
  margin-bottom: 20px;
  border-radius: 50%;
  background: var(--surface-hover);
  display: grid;
  place-items: center;
}

.upload-icon svg {
  width: 25px;
  stroke: #fff;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.video-dropzone strong {
  font-size: 18px;
}

.video-dropzone > span:nth-of-type(2) {
  margin-top: 7px;
  color: #858585;
  font-size: 13px;
}

.select-video {
  min-width: 124px;
  margin-top: 22px;
  padding: 10px 24px;
  border-radius: 6px;
  background: #fe2c55;
  font-size: 14px;
  font-weight: 700;
}

.upload-rules {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 24px;
  color: #777;
}

.upload-rules b {
  font-size: 11px;
  font-weight: 500;
}

.upload-rules i {
  width: 2px;
  height: 2px;
  border-radius: 50%;
  background: #555;
}

.video-workspace {
  padding: 24px;
}

.video-stage {
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, .08);
  border-radius: 10px;
  background: #202023;
}

.video-preview {
  display: block;
  width: 100%;
  max-height: 520px;
  aspect-ratio: 16 / 9;
  background: #202023;
}

.video-filebar {
  min-height: 70px;
  padding: 12px 14px;
  display: flex;
  align-items: center;
  gap: 12px;
  background: var(--surface-raised);
}

.file-symbol,
.setting-icon {
  flex: 0 0 auto;
  display: grid;
  place-items: center;
  border-radius: 8px;
  background: var(--surface-hover);
}

.file-symbol {
  width: 42px;
  height: 42px;
}

.file-symbol svg,
.setting-icon svg {
  width: 21px;
  stroke: #bbb;
  stroke-width: 1.6;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.file-copy {
  flex: 1;
  min-width: 0;
  display: grid;
  gap: 3px;
}

.file-copy strong,
.file-copy span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-copy strong {
  font-size: 13px;
}

.file-copy span {
  color: #7e7e7e;
  font-size: 11px;
}

.text-button {
  padding: 7px 8px;
  background: transparent;
  color: #aaa;
  font-size: 12px;
}

.text-button:hover {
  background: rgba(255, 255, 255, .06);
  color: #fff;
}

.danger-text,
.danger-text:hover {
  color: #fe2c55;
}

.form-section {
  padding: 26px 28px;
  border-top: 1px solid rgba(255, 255, 255, .07);
}

.section-heading {
  margin-bottom: 22px;
  display: flex;
  justify-content: space-between;
  gap: 20px;
}

.section-heading h2 {
  font-size: 17px;
  letter-spacing: -.02em;
}

.section-heading p {
  margin-top: 5px;
  color: #777;
  font-size: 12px;
}

.section-heading > span {
  color: #fe2c55;
  font-size: 11px;
}

.field {
  margin: 0 0 20px;
}

.field:last-child {
  margin-bottom: 0;
}

.field-title {
  margin-bottom: 9px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.field-title b {
  color: #d5d5d5;
  font-size: 13px;
}

.field-title em {
  color: #666;
  font-size: 11px;
  font-style: normal;
}

.field input,
.field textarea {
  border-radius: 7px;
  background: var(--surface-raised);
  border-color: rgba(255, 255, 255, .08);
}

.field input {
  height: 44px;
}

.field textarea {
  min-height: 112px;
}

.field input:focus,
.field textarea:focus {
  border-color: rgba(254, 44, 85, .85);
}

.cover-row {
  display: flex;
  align-items: center;
  gap: 18px;
}

.cover-picker {
  position: relative;
  width: 150px;
  height: 200px;
  flex: 0 0 auto;
  overflow: hidden;
  padding: 0;
  border: 1px dashed rgba(255, 255, 255, .17);
  border-radius: 8px;
  background: var(--surface-raised);
}

.cover-picker:hover {
  border-color: #fe2c55;
  background: var(--surface-hover);
}

.cover-picker img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.cover-placeholder {
  height: 100%;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 10px;
  color: #929292;
}

.cover-placeholder svg {
  width: 28px;
  stroke: #777;
  stroke-width: 1.5;
}

.cover-placeholder b {
  font-size: 12px;
}

.cover-change {
  position: absolute;
  inset: auto 8px 8px;
  padding: 7px;
  border-radius: 5px;
  background: rgba(0, 0, 0, .72);
  font-size: 11px;
}

.cover-tips {
  min-width: 0;
}

.cover-tips strong {
  display: block;
  max-width: 340px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
}

.cover-tips p {
  margin: 7px 0 9px;
  color: #777;
  font-size: 11px;
}

.setting-row {
  padding: 14px;
  display: flex;
  align-items: center;
  gap: 12px;
  border: 1px solid rgba(255, 255, 255, .07);
  border-radius: 8px;
  background: var(--surface-raised);
}

.setting-icon {
  width: 40px;
  height: 40px;
}

.setting-row > div {
  flex: 1;
}

.setting-row strong {
  font-size: 13px;
}

.setting-row p {
  margin-top: 3px;
  color: #777;
  font-size: 11px;
}

.setting-value {
  color: #aaa;
  font-size: 12px;
}

.studio-side {
  position: sticky;
  top: 20px;
  display: grid;
  gap: 12px;
}

.side-card {
  padding: 20px;
}

.side-kicker {
  color: #fe2c55;
  font-size: 10px;
  font-weight: 800;
  letter-spacing: .13em;
}

.side-card h3 {
  margin-top: 7px;
  font-size: 18px;
}

.check-list {
  margin-top: 20px;
  display: grid;
  gap: 17px;
  list-style: none;
}

.check-list li {
  display: flex;
  gap: 11px;
  color: #777;
}

.check-list li > span {
  width: 24px;
  height: 24px;
  flex: 0 0 auto;
  border: 1px solid rgba(255, 255, 255, .12);
  border-radius: 50%;
  display: grid;
  place-items: center;
  color: #777;
  font-size: 10px;
}

.check-list li div {
  display: grid;
  gap: 3px;
}

.check-list b {
  color: #aaa;
  font-size: 12px;
}

.check-list small {
  font-size: 10px;
}

.check-list li.ready > span {
  border-color: rgba(254, 44, 85, .32);
  background: rgba(254, 44, 85, .12);
  color: #fe2c55;
}

.check-list li.ready b {
  color: #eee;
}

.progress-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.progress-heading div {
  display: grid;
  gap: 3px;
}

.progress-heading span,
.progress-card p {
  color: #777;
  font-size: 10px;
}

.progress-heading strong {
  font-size: 13px;
}

.progress-heading > b {
  color: #fe2c55;
  font-size: 12px;
}

.progress-bar {
  height: 4px;
  margin-top: 16px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--surface-hover);
}

.progress-fill {
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, #20d5ec, #fe2c55);
  transition: width 180ms ease;
}

.progress-card p {
  margin-top: 9px;
}

.success-card {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 10px;
}

.success-mark {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: rgba(46, 213, 115, .12);
  color: #2ed573;
  font-size: 12px;
}

.success-card strong {
  font-size: 12px;
}

.success-card p {
  max-width: 170px;
  margin-top: 3px;
  overflow: hidden;
  color: #777;
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.success-card a {
  grid-column: 2;
  color: #fe2c55;
  font-size: 11px;
}

.publish-actions {
  position: fixed;
  z-index: 5;
  right: 0;
  bottom: 0;
  left: 208px;
  min-height: 74px;
  padding: 12px max(24px, calc((100vw - 208px - 1240px) / 2));
  border-top: 1px solid rgba(255, 255, 255, .08);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  background: var(--surface-overlay);
  backdrop-filter: blur(18px);
}

.publish-actions > div:first-child {
  display: grid;
  gap: 3px;
}

.publish-actions strong {
  font-size: 12px;
}

.publish-actions span {
  color: #777;
  font-size: 10px;
}

.action-buttons {
  display: flex;
  gap: 10px;
}

.secondary-action,
.publish-action {
  min-width: 100px;
  height: 40px;
  border-radius: 6px;
  font-weight: 700;
}

.secondary-action {
  background: var(--surface-hover);
  color: #bbb;
}

.publish-action {
  background: #fe2c55;
  color: #fff;
}

.publish-action:hover {
  background: #ff405e;
}

.spinner {
  width: 13px;
  height: 13px;
  margin-right: 7px;
  border: 2px solid rgba(255, 255, 255, .35);
  border-top-color: #fff;
  border-radius: 50%;
  display: inline-block;
  vertical-align: -2px;
  animation: spin .8s linear infinite;
}

.file-native {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

@media (max-width: 1050px) {
  .publish-studio {
    grid-template-columns: 1fr;
  }

  .studio-side {
    position: static;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .creator-page {
    padding-top: 0;
  }

  .creator-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .video-dropzone {
    width: calc(100% - 28px);
    min-height: 320px;
    margin: 14px;
  }

  .upload-rules {
    gap: 7px;
  }

  .form-section {
    padding: 22px 18px;
  }

  .studio-side {
    grid-template-columns: 1fr;
  }

  .publish-actions {
    left: 0;
    padding: 12px 16px;
  }

  .publish-actions > div:first-child {
    display: none;
  }

  .action-buttons {
    width: 100%;
  }

  .action-buttons button {
    flex: 1;
  }
}

@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    transition-duration: .01ms !important;
    animation-duration: .01ms !important;
  }
}

.creator-page { width: min(1100px, 100%); }
.creator-header { padding: 8px 2px 24px; border-bottom: 1px solid var(--border); }
.creator-header h1 { font-weight: 850; }
.publish-studio { margin-top: 20px; grid-template-columns: minmax(0,1fr) 280px; gap: 12px; }
.studio-main,
.side-card { border-color: transparent; border-radius: 8px; }
.video-dropzone { border-radius: 8px; background: #1b1b1e; }
.studio-side { gap: 8px; }
.side-card { padding: 18px; }
.publish-actions {
  left: 240px;
  min-height: 66px;
  padding: 10px max(24px, calc((100vw - 240px - 1100px) / 2));
  background: rgba(11,11,13,.96);
}
@media (max-width: 1180px) {
  .publish-actions { left: 76px; }
}
@media (max-width: 768px) {
  .publish-actions { left: 0; }
}
</style>
