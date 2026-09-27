import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

// https://vite.dev/config/
export default defineConfig({
  // 保留已有构建产物；不在构建过程中自动删除文件。
  build: { emptyOutDir: false },
  plugins: [
    vue(),
    vueDevTools(),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    // Keep the browser same-origin while forwarding API calls to the Go service.
    proxy: {
      '/api': { target: 'http://127.0.0.1:8081', changeOrigin: false },
    },
  },
})
