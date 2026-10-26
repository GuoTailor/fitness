import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 后端本地地址：go run ./cmd/server（默认 :8080）
// 生产部署在 /admin/ 子路径下，构建时资源前缀必须同步，否则 assets 404 白屏
export default defineConfig(({ command }) => ({
  base: command === 'build' ? '/admin/' : '/',
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/admin/api': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true },
    },
  },
}))
