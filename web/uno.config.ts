import { defineConfig, presetWind3 } from 'unocss'

export default defineConfig({
  presets: [presetWind3()],
  theme: {
    colors: {
      bg: 'var(--bg)',
      panel: 'var(--panel)',
      'panel-alt': 'var(--panel-alt)',
      border: 'var(--border)',
      'border-hover': '#3b4a68',
      text: 'var(--text)',
      muted: 'var(--muted)',
      accent: 'var(--accent)',
      ok: 'var(--ok)',
      warn: 'var(--warn)',
      danger: 'var(--danger)',
    },
  },
  shortcuts: {
    // Shared by the history range picker and the container All/None buttons.
    'range-btn':
      'px-[9px] py-[5px] border border-border rounded-[7px] bg-transparent text-muted text-[0.78rem] cursor-pointer hover:border-border-hover hover:text-text aria-pressed:border-accent aria-pressed:bg-accent aria-pressed:text-bg aria-pressed:font-semibold',
  },
})
