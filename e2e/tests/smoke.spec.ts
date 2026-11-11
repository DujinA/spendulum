import { expect, test } from "@playwright/test";

test("/healthz answers 200", async ({ request }) => {
  const res = await request.get("/healthz");

  expect(res.status()).toBe(200);
  expect(await res.json()).toEqual({ status: "ok" });
});

test("/api answers problem JSON", async ({ request }) => {
  const res = await request.get("/api");

  expect(res.status()).toBe(401);
  expect(res.headers()["content-type"]).toBe("application/problem+json");
  expect(await res.json()).toMatchObject({ status: 401, title: "Unauthorized" });
});

test("/ serves the SPA", async ({ page }) => {
  await page.goto("/");

  await expect(page).toHaveTitle("Spendulum");
  await expect(page.getByRole("heading", { level: 1, name: "Log in" })).toBeVisible();
});

test("a client-side route loads on a direct visit", async ({ page }) => {
  await page.goto("/entries");

  await expect(page.getByRole("heading", { level: 1, name: "Log in" })).toBeVisible();
  await expect(page).toHaveURL(/\/login\?redirect=%2Fentries$/);
});
