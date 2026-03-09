import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // v32 fix: allowedHosts: true allows the Vite dev server to accept requests
    // from any hostname. Without this, Vite 5 blocks requests arriving via a
    // public URL (Render, Vercel preview, reverse-proxy) with "Invalid Host header",
    // making the portal completely inaccessible when deployed for testing.
    // Type: true | string[] — 'all' (string) is NOT valid in Vite 5 TypeScript types.
    allowedHosts: true,
    host: true,   // bind to 0.0.0.0 so the container/VM port is reachable
    port: 5173,
  },
  preview: {
    // Same fix for `vite preview` (used to test the production build locally)
    allowedHosts: true,
    host: true,
    port: 4173,
  },
})
