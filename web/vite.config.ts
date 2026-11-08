import { fileURLToPath, URL } from "node:url";

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { VitePWA } from "vite-plugin-pwa";
import { defineConfig } from "vitest/config";

import { pwaOptions } from "./src/pwa.ts";

const backend = "http://localhost:8080";

export default defineConfig({
  plugins: [react(), tailwindcss(), VitePWA(pwaOptions)],
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  server: {
    proxy: {
      "/api": backend,
      "/healthz": backend,
      "/readyz": backend,
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
  },
});
