import type { VitePWAOptions } from "vite-plugin-pwa";

// The installed web app on iOS (issue #8): updates wait for the user (prompt mode),
// the service worker precaches only the app shell, and /api is never served from it.
export const pwaOptions: Partial<VitePWAOptions> = {
  registerType: "prompt",
  injectRegister: false,
  // The glob below already precaches the svg and png icons.
  includeAssets: ["favicon.ico"],
  includeManifestIcons: false,
  manifest: {
    name: "Spendulum",
    short_name: "Spendulum",
    description: "Record expenses and income and compare spending with budget targets.",
    display: "standalone",
    start_url: "/",
    scope: "/",
    theme_color: "#0f766e",
    background_color: "#0f766e",
    icons: [
      { src: "pwa-64x64.png", sizes: "64x64", type: "image/png" },
      { src: "pwa-192x192.png", sizes: "192x192", type: "image/png" },
      { src: "pwa-512x512.png", sizes: "512x512", type: "image/png" },
      { src: "maskable-icon-512x512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
    ],
  },
  workbox: {
    globPatterns: ["**/*.{js,css,html,svg,png}"],
    navigateFallback: "/index.html",
    navigateFallbackDenylist: [/^\/api(\/|$)/, /^\/(healthz|readyz)$/],
    cleanupOutdatedCaches: true,
  },
};
