import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

import { pwaOptions } from "@/pwa";

const root = resolve(import.meta.dirname, "..");

describe("PWA config", () => {
  const manifest = pwaOptions.manifest;
  if (!manifest) throw new Error("pwaOptions has no manifest");

  it("names the app Spendulum and opens standalone", () => {
    expect(manifest.name).toBe("Spendulum");
    expect(manifest.display).toBe("standalone");
  });

  it("has 192 and 512 px icons, including a maskable one, that exist in public/", () => {
    const icons = manifest.icons ?? [];

    expect(icons.map((i) => i.sizes)).toEqual(expect.arrayContaining(["192x192", "512x512"]));
    expect(icons.some((i) => i.purpose === "maskable")).toBe(true);
    for (const icon of icons) expect(existsSync(resolve(root, "public", icon.src))).toBe(true);
  });

  it("links an apple-touch-icon that exists in public/", () => {
    const html = readFileSync(resolve(root, "index.html"), "utf8");
    const href = /<link rel="apple-touch-icon"[^>]*href="\/([^"]+)"/.exec(html)?.[1];

    expect(href).toBeDefined();
    expect(existsSync(resolve(root, "public", href ?? ""))).toBe(true);
  });

  it("waits for the user to accept an update", () => {
    expect(pwaOptions.registerType).toBe("prompt");
  });

  it("precaches only the app shell and never serves /api from the navigation fallback", () => {
    const workbox = pwaOptions.workbox ?? {};
    const denylist = workbox.navigateFallbackDenylist ?? [];

    expect(workbox.globPatterns).toEqual(["**/*.{js,css,html,svg,png}"]);
    expect(workbox.runtimeCaching).toBeUndefined();
    expect(denylist.some((re) => re.test("/api/entries"))).toBe(true);
    expect(denylist.some((re) => re.test("/api"))).toBe(true);
    expect(denylist.some((re) => re.test("/api?page=2"))).toBe(true);
    expect(denylist.some((re) => re.test("/healthz?probe=1"))).toBe(true);
    expect(denylist.some((re) => re.test("/entries"))).toBe(false);
  });
});
