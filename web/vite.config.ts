import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// Saat development, semua request /api diteruskan ke server Go.
// Dengan begitu browser menganggapnya satu origin, jadi cookie refresh_token
// (SameSite=Strict, path /api/v1/auth) ikut terkirim. xfwd mengirim alamat perangkat (HP atau komputer kasir) lewat
// X-Forwarded-For, supaya log server mencatat perangkat mana yang mengakses, bukan selalu 127.0.0.1.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      '/api': { target: 'http://localhost:8080', xfwd: true },
    },
  },
})
