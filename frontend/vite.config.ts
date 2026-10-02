import { fileURLToPath, URL } from 'node:url'

import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'
import { seoPlugin } from './scripts/seo-build.ts'

// https://vite.dev/config/
export default defineConfig(({ mode }) => ({
  // 保留已有构建产物；不在构建过程中自动删除文件。
  build: { emptyOutDir: false },
  plugins: [
    vue(),
    vueDevTools(),
    seoPlugin({ ...loadEnv(mode, process.cwd(), ''), ...process.env } as Record<string, string>, mode),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    // Keep the browser same-origin while forwarding API calls to the Go service.
    proxy: {
      '/api/v1/music/streams': { target: 'http://127.0.0.1:8082', changeOrigin: false, timeout: 0, proxyTimeout: 0 },
      '/api': { target: 'http://127.0.0.1:8081', changeOrigin: false },
    },
  },
}))
