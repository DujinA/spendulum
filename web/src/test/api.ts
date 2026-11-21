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
type Kind = components["schemas"]["Kind"];
type Vendor = components["schemas"]["Vendor"];
type VendorMatches = components["schemas"]["VendorMatches"];

export const groceries: Category = { id: "0195f3a0-0000-7000-8000-000000000001", kind: "expense", name: "Groceries" };
export const eatingOut: Category = { id: "0195f3a0-0000-7000-8000-000000000002", kind: "expense", name: "Eating out" };
export const otherExpense: Category = { id: "0195f3a0-0000-7000-8000-000000000003", kind: "expense", name: "Other" };
export const salary: Category = { id: "0195f3a0-0000-7000-8000-000000000004", kind: "income", name: "Salary" };
export const otherIncome: Category = { id: "0195f3a0-0000-7000-8000-000000000005", kind: "income", name: "Other" };

export interface FakeEntriesOptions {
  settings?: Settings;
  categories?: Category[];
  entries?: Entry[];
  pageSize?: number;
  // Each kind's vendors as the server ranks them, with that kind's use.
  vendors?: Partial<Record<Kind, Vendor[]>>;
  // The server's near-matches for a typed name; the fake does not compute them.
  nearMatches?: Record<string, Vendor[]>;
}

let nextVendorId = 1;

export function fakeVendor(fields: Partial<Vendor> & Pick<Vendor, "name">): Vendor {
  const id = `0195f3a2-0000-7000-8000-${String(nextVendorId++).padStart(12, "0")}`;
  return { id, archived: false, useCount: 0, ...fields };
}

// A stand-in for the server's name key: lowercase, no diacritics, letters and digits only.
function nameKey(name: string) {
  return name
    .normalize("NFD")
    .replace(/\p{M}/gu, "")
    .replace(/[đĐ]/g, "d")
    .toLowerCase()
    .replace(/[^\p{L}\p{N}]/gu, "");
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
  vendors = {},
  nearMatches = {},
}: FakeEntriesOptions = {}) {
  const state = {
    entries: [...entries],
    creates: [] as CreateRequest[],
    vendorSearches: [] as { kind: string | null; name: string | null }[],
    // Holds every vendor search response until the returned function is called.
    holdVendorSearches() {
      let release: () => void = () => undefined;
      vendorsHeld = new Promise<void>((resolve) => {
        release = resolve;
      });
      return () => {
        vendorsHeld = undefined;
        release();
      };
    },
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
  let vendorsHeld: Promise<void> | undefined;
  const byKey = new Map<string, Entry>();

  const sorted = () =>
    [...state.entries].sort(
      (a, b) => b.entryDate.localeCompare(a.entryDate) || b.createdAt.localeCompare(a.createdAt),
    );

  server.use(
    http.get("*/api/settings", () => HttpResponse.json<Settings>(settings)),
    http.get("*/api/categories", () => HttpResponse.json<Category[]>(categories)),
    http.get("*/api/vendors", async ({ request }) => {
      await vendorsHeld;
      const params = new URL(request.url).searchParams;
      const kind = params.get("kind");
      const name = params.get("name");
      state.vendorSearches.push({ kind, name });

      const key = nameKey(name ?? "");
      const all = kind === "expense" || kind === "income" ? (vendors[kind] ?? []) : [];
      const match = key ? all.find((v) => nameKey(v.name) === key) : undefined;
      const body: VendorMatches = {
        vendors: all.filter((v) => v !== match && !v.archived && nameKey(v.name).includes(key)),
        nearMatches: match ? [] : (nearMatches[name?.trim() ?? ""] ?? []),
      };
      if (match) body.match = match;
      return HttpResponse.json<VendorMatches>(body);
    }),
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
      const vendorName = body.vendorName?.trim();
      if (vendorName) {
        const known = Object.values(vendors)
          .flat()
          .find((v) => nameKey(v.name) === nameKey(vendorName));
        const vendor = known ?? fakeVendor({ name: vendorName });
        entry.vendorId = vendor.id;
        entry.vendorName = vendor.name;
      }
      state.entries.push(entry);
      if (idempotencyKey !== null) byKey.set(idempotencyKey, entry);
      return HttpResponse.json<Entry>(entry, { status: 201 });
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
