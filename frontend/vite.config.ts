import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { resolve } from 'path'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'

// https://vite.dev/config/
// 后端默认 HTTP。后端以 TLS_ENABLED=true 启动时，用 RABBIT_DEV_BACKEND=https://localhost:3958 覆盖。
const backendTarget = process.env.RABBIT_DEV_BACKEND ?? 'http://localhost:3958'
const wsTarget = backendTarget.replace(/^http/, 'ws')

export default defineConfig({
  plugins: [
    vue(),
    tailwindcss(),
    AutoImport({
      imports: ['vue', 'vue-router', 'pinia'],
      dts: 'src/auto-imports.d.ts',
    }),
    Components({
      dts: 'src/components.d.ts',
    }),
  ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: backendTarget,
        changeOrigin: true,
        secure: false,
      },
      '/ws': {
        target: wsTarget,
        ws: true,
        secure: false,
      },
    },
  },
  build: {
    // 输出到 backend/dist 目录，供 Go 后端嵌入
    outDir: '../backend/.dist',
    emptyOutDir: true,
    // 优化打包体积
    rollupOptions: {
      output: {
        // 分离第三方库，优化缓存
        manualChunks: {
          'vue-vendor': ['vue', 'vue-router', 'pinia'],
          'ui-vendor': ['reka-ui', 'lucide-vue-next', 'vue-sonner'],
          'xterm': ['@xterm/xterm', '@xterm/addon-fit', '@xterm/addon-web-links'],
        },
        // 优化资源文件命名
        chunkFileNames: 'assets/[name]-[hash].js',
        entryFileNames: 'assets/[name]-[hash].js',
        assetFileNames: 'assets/[name]-[hash].[ext]',
      },
    },
    // 启用 CSS 代码分割
    cssCodeSplit: true,
    // 设置 chunk 大小警告阈值 (500KB)
    chunkSizeWarningLimit: 500,
    // 压缩选项
    minify: 'esbuild',
    // 生成 sourcemap 用于调试（生产环境可设为 false）
    sourcemap: false,
    // 目标浏览器
    target: 'es2015',
  },
})

