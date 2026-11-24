import { expect, test } from "@playwright/test";

import { logIn, recordExpense, unique } from "../api";

test("edits an amount, deletes the entry, taps Undo, and sees the entry back", async ({ page }) => {
  await page.clock.setFixedTime(new Date("2026-03-02T09:30:00+01:00"));
  await logIn(page);
  const vendor = unique("Apoteka");
  await recordExpense(page, { amount: 50000, vendorName: vendor, entryDate: "2026-03-01" });

  await page.goto("/entries");
  const row = page.getByRole("link", { name: new RegExp(vendor) });
  await expect(row).toContainText("500.00 din");

  await row.click();
  await page.getByRole("textbox", { name: "Amount" }).fill("999.50");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(row).toContainText("999.50 din");

  await row.click();
  await page.getByRole("button", { name: "Delete" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Entries" })).toBeVisible();
  await expect(row).toBeHidden();
  await expect(page.getByText("Deleted", { exact: true })).toBeVisible();

  await page.getByRole("button", { name: "Undo" }).click();

  await expect(row).toContainText("999.50 din");
});
