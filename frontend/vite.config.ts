import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { writeFileSync } from "node:fs";

// In development the browser talks only to Vite (localhost:5173), which
// forwards /api and /auth to the Go backend. Cookies and the Steam callback
// therefore all live on one origin; set PUBLIC_URL=http://localhost:5173 for
// the backend.
//
// Both servers listen on this machine only unless LISTEN_HOST says otherwise
// (see "Remote access" in the README). 127.0.0.1 rather than "localhost" so
// Node does not try IPv6 (::1), where the backend is not listening.
const backend = process.env.BACKEND_URL ?? "http://127.0.0.1:8080";
const host = process.env.LISTEN_HOST || "localhost";

// Vite answers only requests addressed to localhost or an IP; also accept the
// host name in PUBLIC_URL, e.g. http://my-pc.lan:5173.
const allowedHosts: string[] = [];
try {
  if (process.env.PUBLIC_URL) allowedHosts.push(new URL(process.env.PUBLIC_URL).hostname);
} catch {
  // A bad PUBLIC_URL is reported by the backend.
}

// `npm run build` writes straight into the Go backend, which embeds the build
// and serves it, so one binary is the whole app. Emptying the folder deletes
// the .gitkeep that lets the backend compile before the first build, so put
// it back afterwards.
const outDir = "../backend/internal/webui/dist";
const keepGitkeep = {
  name: "keep-gitkeep",
  closeBundle() {
    writeFileSync(`${outDir}/.gitkeep`, "");
  },
};

export default defineConfig({
  plugins: [react(), tailwindcss(), keepGitkeep],
  build: {
    outDir,
    emptyOutDir: true,
  },
  server: {
    host,
    port: 5173,
    allowedHosts,
    strictPort: true,
    proxy: {
      "/api": backend,
      "/auth": backend,
    },
  },
});
