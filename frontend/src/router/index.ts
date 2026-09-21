import { createRouter, createWebHistory } from 'vue-router'

import HomeView from '../views/HomeView.vue'
import HotView from '../views/HotView.vue'
import VideoView from '../views/VideoView.vue'
import VideoDetailView from '../views/VideoDetailView.vue'
import AccountView from '../views/AccountView.vue'
import ChangePasswordView from '../views/ChangePasswordView.vue'
import RegisterView from '../views/RegisterView.vue'
import SettingsView from '../views/SettingsView.vue'
import UserProfileView from '../views/UserProfileView.vue'
import MessagesView from '../views/MessagesView.vue'
import ChatView from '../views/ChatView.vue'
import SearchView from '../views/SearchView.vue'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/submissions/:id?', component: () => import('../views/SubmissionsView.vue'), meta: { requiresAuth: true } },
    { path: '/following', name: 'following', component: HomeView },
    { path: '/feed', redirect: '/' },
    { path: '/hot', name: 'hot', component: HotView },
    { path: '/messages', name: 'messages', component: MessagesView },
    { path: '/messages/chat/:peerId?', name: 'chat', component: ChatView },
    { path: '/search', name: 'search', component: SearchView },
    { path: '/video', name: 'video', component: VideoView },
    { path: '/video/:id', name: 'video-detail', component: VideoDetailView, props: true },
    { path: '/account', name: 'account', component: AccountView },
    { path: '/account/register', name: 'account-register', component: RegisterView },
    { path: '/account/change-password', name: 'account-change-password', component: ChangePasswordView, meta: { requiresAuth: true } },
    { path: '/settings', name: 'settings', component: SettingsView },
    { path: '/u/:id', name: 'user-profile', component: UserProfileView, props: true },
  ],
})

router.beforeEach((to) => {
  if (!to.meta.requiresAuth) return true

  const auth = useAuthStore()
  auth.syncFromStorage()
  if (auth.isLoggedIn) return true

  return { name: 'account', query: { reason: 'login-required' } }
})

export default router
