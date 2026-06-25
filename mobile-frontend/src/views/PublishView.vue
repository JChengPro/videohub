<script setup lang="ts">
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api, isSupportedVideo, VIDEO_ACCEPT, videoFileExtension } from '../api'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'
import AppIcon from '../components/AppIcon.vue'

const auth = useAuthStore()
const toast = useToastStore()
const router = useRouter()
const busy = ref(false)
const stage = ref('')
const uploadProgress = ref(0)
const previewError = ref(false)
const videoInput = ref<HTMLInputElement | null>(null)
const form = reactive({ title: '', description: '', video: null as File | null, cover: null as File | null })
const videoPreview = ref('')
const coverPreview = ref('')
const canPublish = computed(() => form.title.trim() && form.video && form.cover && !busy.value)
const videoMeta = computed(() => {
  if (!form.video) return ''
  const size = form.video.size / 1024 / 1024
  return `${videoFileExtension(form.video).slice(1).toUpperCase()} · ${size.toFixed(size >= 10 ? 0 : 1)} MB`
})

watch(() => form.video, (file) => {
  if (videoPreview.value) URL.revokeObjectURL(videoPreview.value)
  previewError.value = false
  videoPreview.value = file ? URL.createObjectURL(file) : ''
})
watch(() => form.cover, (file) => { if (coverPreview.value) URL.revokeObjectURL(coverPreview.value); coverPreview.value = file ? URL.createObjectURL(file) : '' })
onUnmounted(() => { if (videoPreview.value) URL.revokeObjectURL(videoPreview.value); if (coverPreview.value) URL.revokeObjectURL(coverPreview.value) })

function choose(event: Event, type: 'video' | 'cover') {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0] ?? null
  input.value = ''
  if (type === 'video' && file && (!isSupportedVideo(file) || file.size > 200 * 1024 * 1024)) return toast.error('支持 MP4、MOV、M4V、WebM、3GP，最大 200MB')
  if (type === 'cover' && file && file.size > 10 * 1024 * 1024) return toast.error('封面不能超过 10MB')
  form[type] = file
}

async function publish() {
  if (!auth.isLoggedIn) return router.push('/me')
  if (!canPublish.value || !form.video || !form.cover) return
  busy.value = true
  uploadProgress.value = 0
  try {
    stage.value = '上传封面'
    const cover = await api.uploadCover(form.cover)
    stage.value = '上传视频'
    uploadProgress.value = 0
    const video = await api.uploadVideo(form.video, (progress) => { uploadProgress.value = progress })
    const playURL = video.url || video.play_url || ''
    const coverURL = cover.url || cover.cover_url || ''
    if (!playURL || !coverURL || !video.object_key || !cover.object_key) throw new Error('上传成功，但文件地址不完整，请重试')
    stage.value = '发布作品'
    uploadProgress.value = 100
    await api.publish({
      title: form.title.trim(),
      description: form.description.trim(),
      play_url: playURL,
      cover_url: coverURL,
      play_object_key: video.object_key,
      cover_object_key: cover.object_key,
    })
    toast.success('作品已发布')
    await router.push('/')
  } catch (cause) { toast.error(cause instanceof Error ? cause.message : String(cause)) }
  finally { busy.value = false; stage.value = ''; uploadProgress.value = 0 }
}
</script>

<template>
  <main class="page publish-page">
    <header class="publish-header">
      <button class="cancel" :disabled="busy" @click="router.back()">取消</button>
      <b>发布作品</b>
      <button class="submit" :disabled="!canPublish" @click="publish">{{ busy ? stage : '发布' }}</button>
    </header>
    <section v-if="!auth.isLoggedIn" class="login-required"><h2>登录后发布作品</h2><p>分享你的视角，让更多人看见。</p><button @click="router.push('/me')">立即登录</button></section>
    <section v-else class="creator">
      <div class="video-picker" :class="{ selected: videoPreview }">
        <video v-if="videoPreview && !previewError" :src="videoPreview" controls playsinline preload="metadata" @error="previewError = true" />
        <div v-else-if="previewError" class="preview-error">
          <AppIcon name="play" :size="32" />
          <b>当前浏览器无法预览此视频</b>
          <p>部分手机拍摄格式可正常上传，但需要发布后转码才能跨设备播放。</p>
        </div>
        <div v-else class="empty-video">
          <span><AppIcon name="plus" :size="30" /></span>
          <b>选择要发布的视频</b>
          <p>支持手机拍摄视频，最大 200MB</p>
          <div class="format-strip"><i>MP4</i><i>MOV</i><i>M4V</i><i>WebM</i><i>3GP</i></div>
        </div>
        <button v-if="form.video" class="replace-video" type="button" @click="videoInput?.click()"><AppIcon name="plus" :size="14" /> 更换视频</button>
        <button v-else class="choose-overlay" type="button" aria-label="选择视频" @click="videoInput?.click()" />
        <input ref="videoInput" class="hidden-input" type="file" :accept="VIDEO_ACCEPT" @change="choose($event, 'video')" />
      </div>
      <p v-if="videoMeta" class="video-meta">{{ form.video?.name }}<span>{{ videoMeta }}</span></p>

      <section v-if="busy" class="upload-progress" aria-live="polite">
        <div><b>{{ stage }}</b><span>{{ stage === '上传视频' ? `${uploadProgress}%` : stage === '发布作品' ? '即将完成' : '准备中' }}</span></div>
        <i><span :style="{ width: `${stage === '上传封面' ? 8 : stage === '发布作品' ? 100 : uploadProgress}%` }" /></i>
        <p>请保持页面打开，上传过程中不要退出。</p>
      </section>

      <section class="form-card">
        <label class="field">
          <span>标题</span>
          <input v-model="form.title" maxlength="80" placeholder="添加作品标题" />
          <small>{{ form.title.length }}/80</small>
        </label>
        <label class="field description">
          <span>描述</span>
          <textarea v-model="form.description" maxlength="500" rows="3" placeholder="分享这一刻的故事..." />
          <small>{{ form.description.length }}/500</small>
        </label>
        <label class="cover-picker">
          <div class="cover-copy"><span>选择封面</span><p>清晰封面更容易被看见</p></div>
          <div class="cover-preview"><img v-if="coverPreview" :src="coverPreview" /><span v-else><AppIcon name="plus" :size="20" /></span></div>
          <input type="file" accept="image/jpeg,image/png,image/webp" @change="choose($event, 'cover')" />
        </label>
      </section>
      <p class="publish-note">发布即表示你确认拥有该内容的使用权</p>
    </section>
  </main>
</template>

<style scoped>
.publish-page { min-height: 100dvh; padding-bottom: calc(82px + env(safe-area-inset-bottom)); background: radial-gradient(circle at 50% -10%, #29292f 0, #17171a 30%, #121214 62%); }
.publish-header { position: sticky; z-index: 10; top: 0; min-height: calc(56px + env(safe-area-inset-top)); padding: env(safe-area-inset-top) 15px 0; display: grid; grid-template-columns: 64px 1fr 64px; align-items: center; border-bottom: 1px solid rgba(255,255,255,.06); background: rgba(18,18,20,.9); backdrop-filter: blur(20px); }.publish-header b { text-align: center; font-size: 15px; }.publish-header button { height: 34px; border-radius: 7px; font-size: 13px; font-weight: 700; }.cancel { color: #aaa; text-align: left; }.submit { background: #fe2c55; color: #fff; }.submit:disabled { background: #343439; color: #717176; }
.creator { padding: 18px 14px 0; }
.video-picker { position: relative; width: min(66vw, 270px); aspect-ratio: 9/14; margin: 0 auto; overflow: hidden; display: grid; place-items: center; border: 1px dashed rgba(255,255,255,.19); border-radius: 13px; background: linear-gradient(145deg, #29292e, #0d0d0f 72%); box-shadow: 0 22px 54px rgba(0,0,0,.35); }.video-picker.selected { border-style: solid; border-color: rgba(255,255,255,.12); }.hidden-input,.cover-picker input { position: absolute; width: 1px; height: 1px; opacity: 0; pointer-events: none; }.choose-overlay { position: absolute; z-index: 3; inset: 0; width: 100%; }.video-picker video { width: 100%; height: 100%; object-fit: contain; background: #09090b; }.empty-video,.preview-error { padding: 20px; display: grid; justify-items: center; gap: 9px; text-align: center; }.empty-video > span { width: 54px; height: 54px; display: grid; place-items: center; border-radius: 50%; background: #fff; color: #111; }.empty-video b,.preview-error b { margin-top: 5px; font-size: 14px; }.empty-video p,.preview-error p { color: #85858c; font-size: 10px; line-height: 1.6; }.preview-error { color: #aaa; }.format-strip { margin-top: 8px; display: flex; flex-wrap: wrap; justify-content: center; gap: 5px; }.format-strip i { padding: 3px 6px; border-radius: 4px; background: rgba(255,255,255,.07); color: #8d8d94; font-size: 8px; font-style: normal; }.replace-video { position: absolute; z-index: 3; right: 8px; bottom: 8px; padding: 6px 9px; display: flex; align-items: center; gap: 3px; border-radius: 14px; background: rgba(10,10,12,.72); color: #fff; font-size: 10px; backdrop-filter: blur(10px); }.video-meta { max-width: 300px; margin: 10px auto 20px; overflow: hidden; display: flex; justify-content: space-between; gap: 12px; color: #8b8b91; font-size: 9px; white-space: nowrap; text-overflow: ellipsis; }.video-meta span { flex: 0 0 auto; color: #b9b9be; }
.upload-progress { margin: 0 0 14px; padding: 13px 14px; border: 1px solid rgba(255,255,255,.07); border-radius: 11px; background: #202024; }.upload-progress div { display: flex; justify-content: space-between; color: #eee; font-size: 11px; }.upload-progress div span { color: #a2a2a9; }.upload-progress > i { height: 3px; margin-top: 10px; overflow: hidden; display: block; border-radius: 3px; background: #36363b; }.upload-progress > i span { height: 100%; display: block; border-radius: inherit; background: #fe2c55; transition: width .18s ease; }.upload-progress p { margin-top: 8px; color: #696970; font-size: 8px; }
.form-card { overflow: hidden; border: 1px solid rgba(255,255,255,.06); border-radius: 13px; background: #202024; }.field { position: relative; padding: 16px 15px 13px; display: grid; gap: 7px; border-bottom: 1px solid rgba(255,255,255,.06); }.field span,.cover-copy span { color: #ededf0; font-size: 12px; font-weight: 700; }.field input,.field textarea { width: 100%; padding-right: 34px; border: 0; background: transparent; color: #fff; font-size: 13px; line-height: 1.55; resize: none; }.field small { position: absolute; right: 15px; bottom: 14px; color: #66666d; font-size: 8px; }.description small { bottom: 15px; }
.cover-picker { position: relative; min-height: 84px; padding: 12px 15px; display: flex; align-items: center; justify-content: space-between; }.cover-copy p { margin-top: 5px; color: #73737a; font-size: 9px; }.cover-preview { width: 48px; height: 62px; overflow: hidden; display: grid; place-items: center; border-radius: 5px; background: #303035; color: #8c8c93; }.cover-preview img { width: 100%; height: 100%; object-fit: cover; }
.publish-note { padding: 13px 2px 0; color: #606067; font-size: 9px; text-align: center; }
.login-required { min-height: 70vh; display: grid; place-content: center; justify-items: center; gap: 9px; }.login-required p { color: #777; font-size: 12px; }.login-required button { margin-top: 10px; padding: 11px 24px; border-radius: 22px; background: #fff; color: #111; font-weight: 700; }
</style>
