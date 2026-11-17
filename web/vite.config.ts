import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import UnoCSS from 'unocss/vite'

export default defineConfig({
  base: '/',
  plugins: [
    vue(),
    UnoCSS({ configFile: fileURLToPath(new URL('./uno.config.ts', import.meta.url)) }),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8090',
    },
  },
  build: {
    outDir: '../internal/httpapi/dist',
    emptyOutDir: true,
  },
})
