import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// In development the browser talks only to Vite (localhost:5173), which
// forwards /api and /auth to the Go backend. Cookies and the Steam callback
// therefore all live on one origin; set PUBLIC_URL=http://localhost:5173 for
// the backend.
const backend = process.env.BACKEND_URL ?? "http://localhost:8080";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      "/api": backend,
      "/auth": backend,
    },
  },
});
