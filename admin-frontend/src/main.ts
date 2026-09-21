import { createApp } from "vue";
import { createRouter, createWebHistory } from "vue-router";
import App from "./App.vue";
import "./style.css";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", redirect: "/reviews" },
    { path: "/login", component: App },
    { path: "/activate", component: App },
    { path: "/setup", component: App },
    { path: "/members", component: App },
    { path: "/audit", component: App },
    { path: "/settings", component: App },
    { path: "/reviews/:id?", component: App },
    { path: "/:pathMatch(.*)*", redirect: "/reviews" },
  ],
});
createApp(App).use(router).mount("#app");
