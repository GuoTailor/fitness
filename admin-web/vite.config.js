import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 后端本地地址：go run ./cmd/server（默认 :8080）
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/admin/api': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true },
    },
  },
})
