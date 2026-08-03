import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api': {
        // Force IPv4 to avoid Windows resolving `localhost` -> `::1` (IPv6) and causing ECONNREFUSED
        target: 'http://127.0.0.1:8080',
        // WebSocket 握手会校验同源，开发代理需要保留浏览器访问时的 Host。
        changeOrigin: false,
        ws: true,
        rewrite: (path) => path.replace(/^\/api/, ''),
      },
      '/static': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
})
