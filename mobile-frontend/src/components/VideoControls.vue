<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = defineProps<{ video: HTMLVideoElement | null }>()
const emit = defineEmits<{ seekingChange: [value: boolean] }>()
const root = ref<HTMLElement | null>(null)
const currentTime = ref(0)
const duration = ref(0)
const playbackRate = ref(1)
const quality = ref('检测中')
const chromeVisible = ref(false)
const menuOpen = ref(false)
const seeking = ref(false)
let boundVideo: HTMLVideoElement | null = null
let hideTimer = 0

const progress = computed(() => duration.value > 0 ? Math.min(100, currentTime.value / duration.value * 100) : 0)

function formatTime(seconds: number) {
  if (!Number.isFinite(seconds) || seconds < 0) return '0:00'
  const total = Math.floor(seconds)
  return `${Math.floor(total / 60)}:${String(total % 60).padStart(2, '0')}`
}

function qualityLabel(video: HTMLVideoElement) {
  const side = Math.min(video.videoWidth, video.videoHeight)
  if (!side) return '检测中'
  if (side >= 2160) return '4K'
  if (side >= 1440) return '2K'
  if (side >= 1080) return '1080P'
  if (side >= 720) return '720P'
  if (side >= 480) return '480P'
  if (side >= 360) return '360P'
  return `${side}P`
}

function syncTime() {
  if (!boundVideo) return
  currentTime.value = Number.isFinite(boundVideo.currentTime) ? boundVideo.currentTime : 0
  duration.value = Number.isFinite(boundVideo.duration) ? boundVideo.duration : 0
  playbackRate.value = boundVideo.playbackRate
}

function syncMetadata() {
  syncTime()
  if (boundVideo) quality.value = qualityLabel(boundVideo)
}

function unbind() {
  if (!boundVideo) return
  boundVideo.removeEventListener('timeupdate', syncTime)
  boundVideo.removeEventListener('durationchange', syncTime)
  boundVideo.removeEventListener('loadedmetadata', syncMetadata)
  boundVideo.removeEventListener('ratechange', syncTime)
  boundVideo.removeEventListener('emptied', syncMetadata)
  boundVideo = null
}

function bind(video: HTMLVideoElement | null) {
  unbind()
  boundVideo = video
  currentTime.value = 0
  duration.value = 0
  playbackRate.value = 1
  quality.value = '检测中'
  menuOpen.value = false
  if (!video) return
  video.addEventListener('timeupdate', syncTime)
  video.addEventListener('durationchange', syncTime)
  video.addEventListener('loadedmetadata', syncMetadata)
  video.addEventListener('ratechange', syncTime)
  video.addEventListener('emptied', syncMetadata)
  syncMetadata()
}

function reveal() {
  window.clearTimeout(hideTimer)
  chromeVisible.value = true
  hideTimer = window.setTimeout(() => {
    if (!menuOpen.value && !seeking.value) chromeVisible.value = false
  }, 2200)
}

function seek(event: Event) {
  const video = props.video
  if (!video || !Number.isFinite(video.duration)) return
  video.currentTime = Math.max(0, Math.min(video.duration, Number((event.target as HTMLInputElement).value)))
  currentTime.value = video.currentTime
}

function startSeeking() {
  seeking.value = true
  reveal()
  emit('seekingChange', true)
}

function stopSeeking() {
  if (!seeking.value) return
  seeking.value = false
  emit('seekingChange', false)
  reveal()
}

function selectRate(rate: number) {
  if (!props.video) return
  props.video.playbackRate = rate
  playbackRate.value = rate
  menuOpen.value = false
  reveal()
}

function toggleMenu() {
  menuOpen.value = !menuOpen.value
  reveal()
}

function onOutside(event: PointerEvent) {
  if (root.value?.contains(event.target as Node)) return
  menuOpen.value = false
}

watch(() => props.video, bind, { immediate: true })
onMounted(() => {
  document.addEventListener('pointerdown', onOutside)
  window.addEventListener('pointerup', stopSeeking)
})
onBeforeUnmount(() => {
  unbind()
  window.clearTimeout(hideTimer)
  document.removeEventListener('pointerdown', onOutside)
  window.removeEventListener('pointerup', stopSeeking)
  if (seeking.value) emit('seekingChange', false)
})
</script>

<template>
  <div ref="root" class="player-controls" :class="{ visible: chromeVisible || menuOpen || seeking, seeking }" @pointerdown="reveal" @click.stop @dblclick.stop>
    <div v-if="seeking" class="seek-time">{{ formatTime(currentTime) }} <span>/ {{ formatTime(duration) }}</span></div>
    <div class="control-row">
      <span>{{ formatTime(currentTime) }} / {{ formatTime(duration) }}</span>
      <div class="settings">
        <button class="speed-trigger" type="button" aria-haspopup="menu" :aria-expanded="menuOpen" @click="toggleMenu">{{ playbackRate }}×</button>
        <div v-if="menuOpen" class="settings-menu" role="menu">
          <div class="quality-row"><span>画质</span><b>{{ quality }}</b></div>
          <div class="menu-divider" />
          <span class="menu-label">播放速度</span>
          <button v-for="rate in [0.5, 1, 1.5, 2]" :key="rate" type="button" role="menuitemradio" :aria-checked="playbackRate === rate" @click="selectRate(rate)"><i :class="{ selected: playbackRate === rate }" />{{ rate === 1 ? '正常' : `${rate}×` }}</button>
        </div>
      </div>
    </div>
    <input class="seek" type="range" min="0" :max="duration || 0" step="0.05" :value="currentTime" :disabled="!duration" :style="{ '--seek-progress': `${progress}%` }" aria-label="视频播放进度" @input="seek" @pointerdown="startSeeking" />
  </div>
</template>

<style scoped>
.player-controls { position:absolute; z-index:6; right:0; bottom:0; left:0; height:48px; color:#fff; }
.control-row { position:absolute; right:9px; bottom:14px; left:9px; display:flex; align-items:center; justify-content:space-between; color:rgba(255,255,255,.78); font-size:9px; font-weight:700; font-variant-numeric:tabular-nums; opacity:0; transform:translateY(4px); transition:opacity 140ms ease,transform 140ms ease; pointer-events:none; text-shadow:0 1px 4px #000; }
.visible .control-row { opacity:1; transform:none; pointer-events:auto; }
.settings { position:relative; }
.speed-trigger { min-width:34px; height:26px; padding:0 6px; border:0; border-radius:4px; background:rgba(0,0,0,.46); color:#fff; font-size:10px; font-weight:850; backdrop-filter:blur(8px); }
.settings-menu { position:absolute; right:0; bottom:32px; width:140px; padding:7px; border:1px solid rgba(255,255,255,.1); border-radius:8px; background:rgba(27,27,30,.97); box-shadow:0 12px 34px rgba(0,0,0,.5); backdrop-filter:blur(16px); }
.settings-menu button,.quality-row { width:100%; min-height:34px; padding:0 8px; display:flex; align-items:center; gap:8px; border:0; border-radius:5px; background:transparent; color:rgba(255,255,255,.88); font-size:11px; text-align:left; }
.settings-menu i { width:6px; height:6px; border-radius:50%; }
.settings-menu i.selected { background:#fe2c55; box-shadow:0 0 0 3px rgba(254,44,85,.15); }
.quality-row { justify-content:space-between; color:rgba(255,255,255,.55); }
.quality-row b { color:#fff; font-size:10px; }
.menu-label { display:block; padding:6px 8px 3px; color:rgba(255,255,255,.42); font-size:8px; font-weight:850; letter-spacing:.08em; }
.menu-divider { height:1px; margin:4px 2px; background:rgba(255,255,255,.08); }
.seek { position:absolute; right:0; bottom:0; left:0; width:100%; height:16px; margin:0; appearance:none; background:transparent; cursor:pointer; pointer-events:auto; touch-action:pan-x; }
.seek::-webkit-slider-runnable-track { height:2px; background:linear-gradient(to right,#fff var(--seek-progress),rgba(255,255,255,.3) var(--seek-progress)); transition:height 120ms ease; }
.seek::-moz-range-track { height:2px; background:rgba(255,255,255,.3); }
.seek::-moz-range-progress { height:2px; background:#fff; }
.seek::-webkit-slider-thumb { width:0; height:0; margin-top:1px; appearance:none; border:0; border-radius:50%; background:#fff; }
.seek::-moz-range-thumb { width:0; height:0; border:0; border-radius:50%; background:#fff; }
.visible .seek::-webkit-slider-runnable-track,.seeking .seek::-webkit-slider-runnable-track { height:4px; }
.visible .seek::-webkit-slider-thumb,.seeking .seek::-webkit-slider-thumb { width:12px; height:12px; margin-top:-4px; box-shadow:0 1px 5px #000; }
.visible .seek::-moz-range-track,.visible .seek::-moz-range-progress { height:4px; }
.visible .seek::-moz-range-thumb { width:10px; height:10px; }
.seek-time { position:absolute; right:50%; bottom:30px; padding:7px 10px; transform:translateX(50%); border-radius:6px; background:rgba(0,0,0,.7); font-size:13px; font-weight:850; font-variant-numeric:tabular-nums; text-shadow:0 1px 3px #000; pointer-events:none; }
.seek-time span { color:rgba(255,255,255,.58); font-weight:650; }
.seek:disabled { opacity:.35; }
</style>
