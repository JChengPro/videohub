import { createRouter, createWebHistory } from 'vue-router'
import FeedView from '../views/FeedView.vue'
import PublishView from '../views/PublishView.vue'
import MessagesView from '../views/MessagesView.vue'
import MeView from '../views/MeView.vue'
import UserProfileView from '../views/UserProfileView.vue'
import VideoDetailView from '../views/VideoDetailView.vue'

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: FeedView },
    { path: '/following', component: FeedView },
    { path: '/hot', component: FeedView },
    { path: '/publish', component: PublishView },
    { path: '/messages', component: MessagesView },
    { path: '/me', component: MeView },
    { path: '/user/:id', component: UserProfileView },
    { path: '/video/:id', component: VideoDetailView },
  ],
})
