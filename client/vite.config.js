import { defineConfig } from 'vite'
import uni from '@dcloudio/vite-plugin-uni'

// H5 开发服务器：5174 端口 + /api、/uploads 代理到本地后端（免 CORS，与 admin-web 同模式）
export default defineConfig({
  plugins: [uni()],
  server: {
    port: 5174,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/uploads': { target: 'http://127.0.0.1:8080', changeOrigin: true }
    }
  }
})
