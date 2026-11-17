import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [vue()],
  test: {
    environment: 'jsdom',
    include: ['web/src/**/*.test.ts'],
    setupFiles: ['web/src/test/setup.ts'],
    clearMocks: true,
    restoreMocks: true,
  },
})
