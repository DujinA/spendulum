import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";

import type { components } from "@/api/schema.gen";

type Problem = components["schemas"]["Problem"];
type Session = components["schemas"]["Session"];
type Credentials = components["schemas"]["Credentials"];

export const server = setupServer();

export const testEmail = "ana@example.test";
export const testPassword = "correct horse battery";

function problem(status: number, title: string, detail: string, headers: Record<string, string> = {}) {
  return HttpResponse.json<Problem>(
    { type: "about:blank", status, title, detail },
    { status, headers: { "Content-Type": "application/problem+json", ...headers } },
  );
}

// A fake session API: one account, logged out until it logs in.
export function fakeSessionApi({ loggedIn }: { loggedIn: boolean }) {
  const state = { loggedIn, logins: 0, logouts: 0 };

  server.use(
    http.get<never, never, Session | Problem>("*/api/session", () =>
      state.loggedIn
        ? HttpResponse.json<Session>({ email: testEmail })
        : problem(401, "Unauthorized", "Log in to continue."),
    ),
    http.post<never, Credentials>("*/api/session", async ({ request }) => {
      const { email, password } = await request.json();
      if (email !== testEmail || password !== testPassword) {
        return problem(401, "Unauthorized", "Email or password is incorrect.");
      }
      state.loggedIn = true;
      state.logins++;
      return new HttpResponse(null, { status: 204 });
    }),
    http.delete("*/api/session", () => {
      state.loggedIn = false;
      state.logouts++;
      return new HttpResponse(null, { status: 204 });
    }),
  );

  return state;
}

type Settings = components["schemas"]["Settings"];
type Category = components["schemas"]["Category"];
type Entry = components["schemas"]["Entry"];
type NewEntry = components["schemas"]["NewEntry"];
type EntryPage = components["schemas"]["EntryPage"];

function fakeCategory(id: number, kind: Category["kind"], name: string, fields: Partial<Category> = {}): Category {
  return { id: `0195f3a0-0000-7000-8000-${String(id).padStart(12, "0")}`, kind, name, note: "", archived: false, ...fields };
}

export const groceries = fakeCategory(1, "expense", "Groceries", { note: "Food and drugstore" });
export const eatingOut = fakeCategory(2, "expense", "Eating out");
export const otherExpense = fakeCategory(3, "expense", "Other");
export const salary = fakeCategory(4, "income", "Salary");
export const otherIncome = fakeCategory(5, "income", "Other");
export const oldHobby = fakeCategory(6, "expense", "Old hobby", { archived: true });

export interface FakeEntriesOptions {
  settings?: Settings;
  categories?: Category[];
  entries?: Entry[];
  pageSize?: number;
}

export interface CreateRequest {
  idempotencyKey: string | null;
  body: NewEntry;
}

let nextId = 1;

export function fakeEntry(fields: Partial<Entry> & Pick<Entry, "entryDate">): Entry {
  const id = `0195f3a1-0000-7000-8000-${String(nextId++).padStart(12, "0")}`;
  return {
    id,
    kind: "expense",
    amount: 100,
    currency: "RSD",
    categoryId: groceries.id,
    categoryName: groceries.name,
    note: "",
    createdAt: `${fields.entryDate}T12:00:00Z`,
    ...fields,
  };
}

// A fake entries API: lists newest entry date first in pages, and records each create request.
export function fakeEntriesApi({
  settings = { primaryCurrency: "RSD", timeZone: "Europe/Belgrade" },
  categories = [groceries, eatingOut, otherExpense, salary, otherIncome],
  entries = [],
  pageSize = 50,
}: FakeEntriesOptions = {}) {
  const state = {
    entries: [...entries],
    creates: [] as CreateRequest[],
    listRequests: 0,
    failLists: 0,
    failCreates: 0,
    // Holds every list response until the returned function is called.
    holdLists() {
      let release: () => void = () => undefined;
      held = new Promise<void>((resolve) => {
        release = resolve;
      });
      return () => {
        held = undefined;
        release();
      };
    },
  };
  let held: Promise<void> | undefined;
  const byKey = new Map<string, Entry>();

  const sorted = () =>
    [...state.entries].sort(
      (a, b) => b.entryDate.localeCompare(a.entryDate) || b.createdAt.localeCompare(a.createdAt),
    );

  server.use(
    http.get("*/api/settings", () => HttpResponse.json<Settings>(settings)),
    http.get("*/api/categories", () => HttpResponse.json<Category[]>(categories)),
    http.get<never, never, EntryPage | Problem>("*/api/entries", async ({ request }) => {
      state.listRequests++;
      await held;
      if (state.failLists > 0) {
        state.failLists--;
        return problem(503, "Service Unavailable", "");
      }
      const start = Number(new URL(request.url).searchParams.get("cursor") ?? "0");
      const all = sorted();
      const page: EntryPage = { entries: all.slice(start, start + pageSize) };
      if (start + pageSize < all.length) page.nextCursor = String(start + pageSize);
      return HttpResponse.json<EntryPage>(page);
    }),
    http.post<never, NewEntry>("*/api/entries", async ({ request }) => {
      const body = await request.json();
      const idempotencyKey = request.headers.get("Idempotency-Key");
      state.creates.push({ idempotencyKey, body });

      if (state.failCreates > 0) {
        state.failCreates--;
        return problem(503, "Service Unavailable", "");
      }

      const replay = idempotencyKey === null ? undefined : byKey.get(idempotencyKey);
      if (replay) return HttpResponse.json<Entry>(replay, { status: 201 });

      const category = categories.find((c) => c.id === body.categoryId);
      const entry = fakeEntry({
        kind: body.kind,
        amount: body.amount,
        currency: settings.primaryCurrency,
        categoryId: body.categoryId,
        categoryName: category?.name ?? "",
        entryDate: body.entryDate,
        note: body.note?.trim() ?? "",
        createdAt: new Date().toISOString(),
      });
      state.entries.push(entry);
      if (idempotencyKey !== null) byKey.set(idempotencyKey, entry);
      return HttpResponse.json<Entry>(entry, { status: 201 });
    }),
  );

  return state;
}

type NewCategory = components["schemas"]["NewCategory"];
type CategoryChange = components["schemas"]["CategoryChange"];
type CategoryOrder = components["schemas"]["CategoryOrder"];

// The same key as the server's namekey.Of, so the fake refuses the same names.
function nameKey(name: string) {
  return name.normalize("NFD").replace(/[đĐ]/g, "d").replace(/[^\p{L}\p{N}]/gu, "").toLowerCase();
}

export interface FakeCategoriesOptions {
  categories?: Category[];
  // Ids of categories that have entries, so deleting them answers 409.
  inUse?: string[];
}

// A fake categories API that keeps its own state: expense categories first, each kind in manual order.
export function fakeCategoriesApi({
  categories = [groceries, eatingOut, otherExpense, oldHobby, salary, otherIncome],
  inUse = [],
}: FakeCategoriesOptions = {}) {
  const state = {
    categories: categories.map((c) => ({ ...c })),
    creates: [] as { idempotencyKey: string | null; body: NewCategory }[],
    changes: [] as { id: string; body: CategoryChange }[],
    orders: [] as CategoryOrder[],
    deletes: [] as string[],
    failOrders: 0,
  };
  let nextCategory = 100;

  const listed = (withArchived: boolean) =>
    [...state.categories.filter((c) => c.kind === "expense"), ...state.categories.filter((c) => c.kind === "income")].filter(
      (c) => withArchived || !c.archived,
    );
  const nameTaken = (kind: Category["kind"], name: string, exceptId?: string) => {
    const existing = state.categories.find((c) => c.kind === kind && c.id !== exceptId && nameKey(c.name) === nameKey(name));
    return existing && problem(422, "Unprocessable Content", `An ${kind} category named “${existing.name}” already exists.`);
  };

  server.use(
    http.get("*/api/categories", ({ request }) =>
      HttpResponse.json<Category[]>(listed(new URL(request.url).searchParams.get("includeArchived") === "true")),
    ),
    http.post<never, NewCategory>("*/api/categories", async ({ request }) => {
      const body = await request.json();
      state.creates.push({ idempotencyKey: request.headers.get("Idempotency-Key"), body });
      const taken = nameTaken(body.kind, body.name);
      if (taken) return taken;
      const created = fakeCategory(nextCategory++, body.kind, body.name.trim(), { note: body.note?.trim() ?? "" });
      state.categories.push(created);
      return HttpResponse.json<Category>(created, { status: 201 });
    }),
    http.patch<{ id: string }, CategoryChange>("*/api/categories/:id", async ({ request, params }) => {
      const body = await request.json();
      state.changes.push({ id: params.id, body });
      const category = state.categories.find((c) => c.id === params.id);
      if (!category) return problem(404, "Not Found", "");
      const taken = body.name === undefined ? undefined : nameTaken(category.kind, body.name, category.id);
      if (taken) return taken;
      if (body.name !== undefined) category.name = body.name.trim();
      if (body.note !== undefined) category.note = body.note.trim();
      if (body.archived !== undefined) category.archived = body.archived;
      return HttpResponse.json<Category>(category);
    }),
    http.delete<{ id: string }>("*/api/categories/:id", ({ params }) => {
      state.deletes.push(params.id);
      if (inUse.includes(params.id)) {
        return problem(409, "Conflict", "This category has entries, so it cannot be deleted. Archive it instead.");
      }
      state.categories = state.categories.filter((c) => c.id !== params.id);
      return new HttpResponse(null, { status: 204 });
    }),
    http.put<never, CategoryOrder>("*/api/categories/order", async ({ request }) => {
      const body = await request.json();
      state.orders.push(body);
      if (state.failOrders > 0) {
        state.failOrders--;
        return problem(503, "Service Unavailable", "");
      }
      const ordered = body.ids.map((id) => state.categories.find((c) => c.id === id)).filter((c) => c !== undefined);
      state.categories = [...state.categories.filter((c) => c.kind !== body.kind), ...ordered];
      return new HttpResponse(null, { status: 204 });
    }),
  );

  return state;
}

export function tooManyLogins(retryAfterSeconds: number) {
  server.use(
    http.post("*/api/session", () =>
      problem(429, "Too Many Requests", "Too many failed logins. Try again later.", {
        "Retry-After": String(retryAfterSeconds),
      }),
    ),
  );
}
