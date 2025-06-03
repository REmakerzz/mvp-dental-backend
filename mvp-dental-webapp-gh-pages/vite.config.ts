import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  base: '/mvp-dental-webapp/', // Базовый путь для GitHub Pages
  build: {
    assetsDir: 'assets',
    rollupOptions: {
      output: {
        // Используем фиксированные имена файлов без хешей
        assetFileNames: 'assets/[name][extname]',
        chunkFileNames: 'assets/[name].js',
        entryFileNames: 'assets/[name].js',
      },
    },
  },
}) 