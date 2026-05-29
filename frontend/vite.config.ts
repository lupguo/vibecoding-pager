import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import wailsPlugin from '@wailsio/runtime/plugins/vite'

export default defineConfig({
  plugins: [react(), wailsPlugin('./bindings')],
  server: {
    host: '127.0.0.1',
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  build: {
    outDir: 'dist',
  },
})
