import { defineConfig, minimal2023Preset } from "@vite-pwa/assets-generator/config";

// Regenerate the icons in public/ with: pnpm exec pwa-assets-generator
const background = "#0f766e";

export default defineConfig({
  preset: {
    ...minimal2023Preset,
    maskable: { ...minimal2023Preset.maskable, resizeOptions: { background } },
    apple: { ...minimal2023Preset.apple, resizeOptions: { background } },
  },
  images: ["public/favicon.svg"],
});
