import { randomUUID } from "node:crypto";

import { expect, type Page } from "@playwright/test";

// The operator creates this account with `spendulum user create` before the tests run (Taskfile e2e).
export const email = process.env.E2E_EMAIL ?? "";
export const password = process.env.E2E_PASSWORD ?? "";

// Both browsers run at once against one account, so every name a test makes is its own.
export function unique(name: string) {
  return `${name} ${randomUUID().slice(0, 8)}`;
}

// The API shares the page's cookies, so the journey starts logged in.
export async function logIn(page: Page) {
  expect(email, "E2E_EMAIL is set").not.toBe("");
  const res = await page.request.post("/api/session", { data: { email, password } });
  expect(res.status()).toBe(204);
}

export async function categoryId(page: Page, kind: "expense" | "income", name: string) {
  const res = await page.request.get("/api/categories");
  expect(res.ok()).toBe(true);
  const categories = (await res.json()) as { id: string; kind: string; name: string }[];
  const found = categories.find((c) => c.kind === kind && c.name === name);
  expect(found, `the ${kind} category ${name}`).toBeDefined();
  return found?.id ?? "";
}

export async function recordExpense(page: Page, entry: { amount: number; vendorName: string; entryDate: string }) {
  const res = await page.request.post("/api/entries", {
    headers: { "Idempotency-Key": randomUUID() },
    data: { kind: "expense", categoryId: await categoryId(page, "expense", "Other"), ...entry },
  });
  expect(res.status()).toBe(201);
}
