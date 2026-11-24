import { expect, test } from "@playwright/test";

import { logIn, recordExpense, unique } from "../api";

test("records an expense with a known vendor dated today and sees it in the list", async ({ page }) => {
  await page.clock.setFixedTime(new Date("2026-03-02T09:30:00+01:00"));
  await logIn(page);
  const vendor = unique("Pekara");
  await recordExpense(page, { amount: 12000, vendorName: vendor, entryDate: "2026-02-20" });

  await page.goto("/entries");
  await page.getByRole("link", { name: "Add entry" }).filter({ visible: true }).click();

  const keypad = page.getByRole("group", { name: "Keypad" });
  for (const key of "450") await keypad.getByRole("button", { name: key, exact: true }).click();
  await page.getByRole("button", { name: /^Vendor/ }).click();
  const sheet = page.getByRole("dialog", { name: "Vendor" });
  await sheet.getByRole("searchbox", { name: "Search" }).fill(vendor);
  await sheet.getByRole("button", { name: vendor }).click();
  await expect(page.getByRole("button", { name: /^Category/ })).toHaveText(/Other/);
  await expect(page.getByRole("button", { name: /^Date/ })).toHaveText(/Today/);
  await page.getByRole("button", { name: "Save", exact: true }).click();

  await expect(page.getByRole("heading", { level: 1, name: "Entries" })).toBeVisible();
  const today = page.getByRole("region", { name: "Today" });
  await expect(today.getByRole("link", { name: new RegExp(vendor) })).toContainText("450.00 din");
});
