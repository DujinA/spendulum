import { expect, test } from "@playwright/test";

test("/healthz answers 200", async ({ request }) => {
  const res = await request.get("/healthz");

  expect(res.status()).toBe(200);
  expect(await res.json()).toEqual({ status: "ok" });
});

test("/api answers JSON", async ({ request }) => {
  const res = await request.get("/api");

  expect(res.status()).toBe(404);
  expect(res.headers()["content-type"]).toBe("application/problem+json");
  expect(await res.json()).toMatchObject({ status: 404, title: "Not Found" });
});

test("/ serves the SPA with the navigation for the screen size", async ({ page, isMobile }) => {
  await page.goto("/");

  await expect(page).toHaveTitle("Spendulum");
  await expect(page.getByRole("heading", { level: 1, name: "Dashboard" })).toBeVisible();

  const tabs = page.getByRole("navigation", { name: "Tabs" });
  const sidebar = page.getByRole("navigation", { name: "Sidebar" });
  if (isMobile) {
    await expect(tabs).toBeVisible();
    await expect(sidebar).toBeHidden();
  } else {
    await expect(sidebar).toBeVisible();
    await expect(tabs).toBeHidden();
  }
});

test("a client-side route loads on a direct visit", async ({ page }) => {
  await page.goto("/entries");

  await expect(page.getByRole("heading", { level: 1, name: "Entries" })).toBeVisible();
});
