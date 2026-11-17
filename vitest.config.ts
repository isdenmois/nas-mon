import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./web/src', import.meta.url)),
    },
  },
  test: {
    environment: 'jsdom',
    include: ['web/src/**/*.test.ts'],
    setupFiles: ['web/src/test/setup.ts'],
    clearMocks: true,
    restoreMocks: true,
  },
})
