<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import BottomNav from './components/BottomNav.vue'
import ToastHost from './components/ToastHost.vue'

const route = useRoute()
const unread = ref(0)
const showBottomNav = computed(() => !route.path.startsWith('/video/') && !route.path.startsWith('/user/'))
</script>

<template>
  <RouterView v-slot="{ Component }"><component :is="Component" v-model:unread="unread" /></RouterView>
  <BottomNav v-if="showBottomNav" v-model:unread="unread" />
  <ToastHost />
</template>
