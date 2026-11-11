import { expect, test } from "@playwright/test";

// The operator creates this account with `spendulum user create` before the tests run (Taskfile e2e).
const email = process.env.E2E_EMAIL ?? "";
const password = process.env.E2E_PASSWORD ?? "";

test("logs in, sees the Dashboard and logs out", async ({ page, isMobile }) => {
  expect(email, "E2E_EMAIL is set").not.toBe("");

  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1, name: "Log in" })).toBeVisible();

  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Log in" }).click();

  await expect(page.getByRole("heading", { level: 1, name: "Dashboard" })).toBeVisible();
  await expect(page).toHaveURL("/");

  const tabs = page.getByRole("navigation", { name: "Tabs" });
  const sidebar = page.getByRole("navigation", { name: "Sidebar" });
  const nav = isMobile ? tabs : sidebar;
  await expect(nav).toBeVisible();
  await expect(isMobile ? sidebar : tabs).toBeHidden();

  await nav.getByRole("link", { name: "More" }).click();
  await page.getByRole("button", { name: "Log out" }).click();

  await expect(page.getByRole("heading", { level: 1, name: "Log in" })).toBeVisible();

  await page.goto("/entries");
  await expect(page.getByRole("heading", { level: 1, name: "Log in" })).toBeVisible();
});
