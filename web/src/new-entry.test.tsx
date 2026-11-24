import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { eatingOut, fakeEntriesApi, fakeEntry, fakeSessionApi, fakeVendor, groceries, oldHobby, salary } from "@/test/api";
import { renderApp } from "@/test/render";

// 09:30 in Belgrade on 2 March 2026.
const now = new Date("2026-03-02T08:30:00Z");

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"], now });
});

afterEach(() => {
  vi.useRealTimers();
});

const keyNames: Record<string, string> = { ".": "Decimal point", "<": "Delete" };

async function tapKeypad(keys: string) {
  const keypad = await screen.findByRole("group", { name: "Keypad" });
  for (const key of keys) {
    await userEvent.click(within(keypad).getByRole("button", { name: keyNames[key] ?? key }));
  }
}

async function openSheet(tile: string) {
  await userEvent.click(screen.getByRole("button", { name: new RegExp(`^${tile}`) }));
  return screen.findByRole("dialog", { name: tile });
}

// An open sheet hides the rest of the screen from assistive technology.
async function closed(sheet: HTMLElement) {
  await waitFor(() => {
    expect(sheet).not.toBeInTheDocument();
  });
}

async function pickCategory(name: string) {
  const sheet = await openSheet("Category");
  await userEvent.click(within(sheet).getByRole("button", { name }));
  await closed(sheet);
}

function amount() {
  return screen.getByRole("status", { name: "Amount" });
}

async function openKeypad() {
  fakeSessionApi({ loggedIn: true });
  const fake = fakeEntriesApi();
  const { router } = renderApp("/new");
  await screen.findByRole("group", { name: "Keypad" });
  return { fake, router };
}

describe("keypad screen", () => {
  it("saves 1780.80 as an expense of 178080 dated today and shows it first under Today on the entry list", async () => {
    fakeSessionApi({ loggedIn: true });
    const api = fakeEntriesApi({ entries: [fakeEntry({ entryDate: "2026-03-01", note: "older" })] });
    const { router } = renderApp("/entries");
    const tabs = await screen.findByRole("navigation", { name: "Tabs" });

    await userEvent.click(within(tabs).getByRole("link", { name: "Add entry" }));
    await tapKeypad("1780.80");
    await pickCategory("Groceries");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByRole("heading", { level: 1, name: "Entries" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/entries");
    expect(api.creates.map((c) => c.body)).toEqual([
      { kind: "expense", amount: 178080, categoryId: groceries.id, entryDate: "2026-03-02", note: "" },
    ]);

    const days = await screen.findAllByRole("region");
    expect(days[0]).toHaveAccessibleName("Today");
    const [first] = within(screen.getByRole("region", { name: "Today" })).getAllByRole("listitem");
    expect(first).toHaveTextContent("Groceries");
    expect(first).toHaveTextContent("1,780.80 din");
  });

  it("shows the amount as typed with the primary currency", async () => {
    await openKeypad();

    await tapKeypad("1234.5<6");

    expect(amount()).toHaveTextContent(/^1,234\.6\s*din$/);
  });

  it.each([
    ["JPY", "12.5", /^125\s*JPY$/, 125, "125 JPY"],
    ["KWD", "1.2345", /^1\.234\s*KWD$/, 1234, "1.234 KWD"],
  ])(
    "in %s takes the currency's minor units: typing %s shows and saves them",
    async (primaryCurrency, keys, shown, saved, listed) => {
      fakeSessionApi({ loggedIn: true });
      const fake = fakeEntriesApi({ settings: { primaryCurrency, timeZone: "Europe/Belgrade" } });
      renderApp("/new");

      await tapKeypad(keys);
      expect(amount()).toHaveTextContent(shown);

      await pickCategory("Groceries");
      await userEvent.click(screen.getByRole("button", { name: "Save" }));

      const [row] = within(await screen.findByRole("region", { name: "Today" })).getAllByRole("listitem");
      expect(fake.creates.map((c) => c.body.amount)).toEqual([saved]);
      expect(row).toHaveTextContent(listed);
    },
  );

  it("hides Refund and clears the chosen category when switched to Income", async () => {
    await openKeypad();
    await tapKeypad("5");
    await pickCategory("Groceries");
    await userEvent.click(screen.getByRole("button", { name: "Refund" }));

    await userEvent.click(screen.getByRole("radio", { name: "Income" }));

    expect(screen.getByRole("radio", { name: "Income" })).toBeChecked();
    expect(screen.queryByRole("button", { name: "Refund" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Category/ })).toHaveTextContent("Pick one");
    expect(amount()).toHaveTextContent(/^5\s*din$/);

    await userEvent.click(screen.getByRole("radio", { name: "Expense" }));
    expect(screen.getByRole("button", { name: /^Category/ })).toHaveTextContent("Pick one");
    expect(screen.getByRole("button", { name: "Refund" })).toHaveAttribute("aria-pressed", "false");
  });

  it("saves an expense with Refund on as a negative amount, and the list shows it as a refund", async () => {
    const { fake } = await openKeypad();

    await tapKeypad("1990");
    await userEvent.click(screen.getByRole("button", { name: "Refund" }));
    await pickCategory("Groceries");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    const [row] = within(await screen.findByRole("region", { name: "Today" })).getAllByRole("listitem");
    expect(fake.creates.map((c) => c.body.amount)).toEqual([-199000]);
    expect(row).toHaveTextContent("Refund");
    expect(row).toHaveTextContent("−1,990.00 din");
  });

  it("keeps Save disabled until the amount is not 0 and a category is picked", async () => {
    await openKeypad();
    const save = screen.getByRole("button", { name: "Save" });

    expect(save).toBeDisabled();
    await pickCategory("Groceries");
    expect(save).toBeDisabled();
    await tapKeypad("0.00");
    expect(save).toBeDisabled();
    await tapKeypad("<1");
    expect(save).toBeEnabled();
  });

  it("lists only the current kind's categories, in manual order", async () => {
    await openKeypad();

    const expense = await openSheet("Category");
    expect(within(expense).getAllByRole("button", { name: /^(?!Done)/ }).map((b) => b.textContent)).toEqual([
      "Groceries",
      "Eating out",
      "Other",
    ]);
    await userEvent.click(within(expense).getByRole("button", { name: "Done" }));
    await closed(expense);

    await userEvent.click(screen.getByRole("radio", { name: "Income" }));
    const income = await openSheet("Category");
    expect(within(income).getAllByRole("button", { name: /^(?!Done)/ }).map((b) => b.textContent)).toEqual([
      "Salary",
      "Other",
    ]);
  });

  it("offers Today, Yesterday, the day before and a date picker", async () => {
    const { fake } = await openKeypad();
    await tapKeypad("1");
    await pickCategory("Groceries");

    expect(screen.getByRole("button", { name: /^Date/ })).toHaveTextContent("Today");
    let sheet = await openSheet("Date");
    expect(within(sheet).getByRole("button", { name: "Today" })).toBeInTheDocument();
    expect(within(sheet).getByRole("button", { name: "Yesterday" })).toBeInTheDocument();
    await userEvent.click(within(sheet).getByRole("button", { name: "Sat 28 Feb" }));
    await closed(sheet);
    expect(screen.getByRole("button", { name: /^Date/ })).toHaveTextContent("Sat 28 Feb");

    sheet = await openSheet("Date");
    fireEvent.change(within(sheet).getByLabelText("Pick a date"), { target: { value: "2026-01-15" } });
    await userEvent.click(within(sheet).getByRole("button", { name: "Done" }));
    await closed(sheet);
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    await screen.findByRole("heading", { level: 1, name: "Entries" });
    expect(fake.creates.map((c) => c.body.entryDate)).toEqual(["2026-01-15"]);
  });

  it("takes today from the time-zone setting when the device's zone is on another date", async () => {
    vi.stubEnv("TZ", "America/Los_Angeles");
    vi.setSystemTime(new Date("2026-03-01T23:30:00Z"));
    expect(new Date().getDate()).toBe(1);

    const { fake } = await openKeypad();
    await tapKeypad("1");
    await pickCategory("Groceries");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    await screen.findByRole("heading", { level: 1, name: "Entries" });
    expect(fake.creates.map((c) => c.body.entryDate)).toEqual(["2026-03-02"]);
    vi.unstubAllEnvs();
  });

  it("saves a typed note trimmed and shows it on the Note tile", async () => {
    const { fake } = await openKeypad();
    await tapKeypad("1");
    await pickCategory("Groceries");

    const sheet = await openSheet("Note");
    await userEvent.type(within(sheet).getByRole("textbox", { name: "Note" }), "  coffee beans  {Enter}");
    await closed(sheet);
    expect(screen.getByRole("button", { name: /^Note/ })).toHaveTextContent("coffee beans");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    await screen.findByRole("heading", { level: 1, name: "Entries" });
    expect(fake.creates.map((c) => c.body.note)).toEqual(["coffee beans"]);
  });

  it("sends an Idempotency-Key made when the form opened, and the same key on a retry", async () => {
    const { fake, router } = await openKeypad();
    fake.failCreates = 1;
    await tapKeypad("1");
    await pickCategory("Groceries");

    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Could not save");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await screen.findByRole("heading", { level: 1, name: "Entries" });

    await router.navigate({ to: "/new" });
    await tapKeypad("2");
    await pickCategory("Groceries");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await screen.findByRole("heading", { level: 1, name: "Entries" });

    const [first, retry, next] = fake.creates.map((c) => c.idempotencyKey);
    expect(first).toMatch(/^[0-9a-f-]{36}$/);
    expect(retry).toBe(first);
    expect(next).not.toBe(first);
  });

  it("edits the amount with digits, . , and Backspace, toggles Refund with - and saves with Enter, without focusing a field", async () => {
    const { fake } = await openKeypad();
    await pickCategory("Groceries");
    await userEvent.click(amount());
    expect(document.activeElement).toBe(document.body);

    await userEvent.keyboard("1780,89{Backspace}0");
    expect(amount()).toHaveTextContent(/^1,780\.80\s*din$/);
    await userEvent.keyboard("-");
    expect(screen.getByRole("button", { name: "Refund" })).toHaveAttribute("aria-pressed", "true");
    await userEvent.keyboard("{Enter}");

    await screen.findByRole("heading", { level: 1, name: "Entries" });
    expect(fake.creates.map((c) => c.body)).toEqual([
      { kind: "expense", amount: -178080, categoryId: groceries.id, entryDate: "2026-03-02", note: "" },
    ]);
  });

  it("saves with Enter after Refund was clicked and the amount typed on the keyboard", async () => {
    const { fake } = await openKeypad();
    await pickCategory("Groceries");
    await userEvent.click(screen.getByRole("button", { name: "Refund" }));

    await userEvent.keyboard("25{Enter}");

    await screen.findByRole("heading", { level: 1, name: "Entries" });
    expect(fake.creates.map((c) => c.body.amount)).toEqual([-2500]);
  });

  it("leaves Enter to a control reached with Tab", async () => {
    const { fake, router } = await openKeypad();
    await pickCategory("Groceries");
    await userEvent.click(amount());
    await userEvent.keyboard("25");

    await userEvent.tab();
    expect(document.activeElement).toHaveAccessibleName("Dashboard");
    await userEvent.keyboard("{Enter}");

    expect(await screen.findByRole("heading", { level: 1, name: "Dashboard" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
    expect(fake.creates).toEqual([]);
  });
});

function tile(label: string) {
  return screen.getByRole("button", { name: new RegExp(`^${label}`) });
}

async function pickVendor(name: string) {
  const sheet = await openSheet("Vendor");
  await userEvent.click(await within(sheet).findByRole("button", { name }));
  await closed(sheet);
}

async function typeNote(text: string) {
  const sheet = await openSheet("Note");
  await userEvent.type(within(sheet).getByRole("textbox", { name: "Note" }), `${text}{Enter}`);
  await closed(sheet);
}

async function pickDate(label: string) {
  const sheet = await openSheet("Date");
  await userEvent.click(within(sheet).getByRole("button", { name: label }));
  await closed(sheet);
}

const maxi = fakeVendor({
  name: "Maxi",
  useCount: 12,
  latest: { categoryId: groceries.id, categoryName: groceries.name, currency: "RSD" },
});

describe("after Save", () => {
  it("shows a toast with Undo, and Undo deletes the saved entry", async () => {
    fakeSessionApi({ loggedIn: true });
    const fake = fakeEntriesApi({ entries: [fakeEntry({ entryDate: "2026-03-01", note: "older" })] });
    renderApp("/new");
    await tapKeypad("1780.80");
    await pickCategory("Groceries");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await screen.findByRole("heading", { level: 1, name: "Entries" });

    const toast = (await screen.findByText("Saved 1,780.80 din")).closest("li");
    expect(await screen.findByRole("link", { name: /1,780\.80 din/ })).toBeInTheDocument();
    await userEvent.click(within(toast ?? document.body).getByRole("button", { name: "Undo" }));

    await waitFor(() => {
      expect(screen.queryByRole("link", { name: /1,780\.80 din/ })).not.toBeInTheDocument();
    });
    expect(fake.deletes).toEqual([fake.entries.at(-1)?.id]);
    expect(screen.getByRole("link", { name: /older/ })).toBeInTheDocument();
  });
});

describe("Save + another", () => {
  it("saves, stays open keeping kind and date, and clears amount, vendor, category, note and Refund", async () => {
    fakeSessionApi({ loggedIn: true });
    const fake = fakeEntriesApi({ vendors: { expense: [maxi] } });
    const { router } = renderApp("/new");
    await tapKeypad("25");
    await userEvent.click(screen.getByRole("button", { name: "Refund" }));
    await pickVendor("Maxi");
    await pickCategory("Eating out");
    await pickDate("Yesterday");
    await typeNote("returned");

    await userEvent.click(screen.getByRole("button", { name: "Save + another" }));

    expect(await screen.findByText("Saved −25.00 din")).toBeInTheDocument();
    expect(fake.creates.map((c) => c.body)).toEqual([
      { kind: "expense", amount: -2500, categoryId: eatingOut.id, entryDate: "2026-03-01", note: "returned", vendorName: "Maxi" },
    ]);
    expect(router.state.location.pathname).toBe("/new");
    expect(screen.getByRole("radio", { name: "Expense" })).toBeChecked();
    expect(tile("Date")).toHaveTextContent("Yesterday");
    expect(amount()).toHaveTextContent(/^0\s*din$/);
    expect(screen.getByRole("button", { name: "Refund" })).toHaveAttribute("aria-pressed", "false");
    expect(tile("Vendor")).toHaveTextContent("Optional");
    expect(tile("Category")).toHaveTextContent("Pick one");
    expect(tile("Note")).toHaveTextContent("Optional");
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });

  it("keeps Income for the next entry and sends a new Idempotency-Key for each", async () => {
    const { fake } = await openKeypad();
    await userEvent.click(screen.getByRole("radio", { name: "Income" }));
    await tapKeypad("100");
    await pickCategory("Salary");
    await userEvent.click(screen.getByRole("button", { name: "Save + another" }));
    await screen.findByText("Saved 100.00 din");

    expect(screen.getByRole("radio", { name: "Income" })).toBeChecked();
    await tapKeypad("200");
    await pickCategory("Salary");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await screen.findByRole("heading", { level: 1, name: "Entries" });

    expect(fake.creates.map((c) => [c.body.kind, c.body.amount, c.body.categoryId])).toEqual([
      ["income", 10000, salary.id],
      ["income", 20000, salary.id],
    ]);
    const [first, second] = fake.creates.map((c) => c.idempotencyKey);
    expect(first).toMatch(/^[0-9a-f-]{36}$/);
    expect(second).toMatch(/^[0-9a-f-]{36}$/);
    expect(second).not.toBe(first);
  });
});

describe("Repeat", () => {
  it("opens the keypad filled with the row's amount, vendor, category, note and refund sign, dated today, with a new Idempotency-Key", async () => {
    fakeSessionApi({ loggedIn: true });
    const fake = fakeEntriesApi({ vendors: { expense: [maxi] } });
    const { router } = renderApp("/new");
    await tapKeypad("1990.5");
    await userEvent.click(screen.getByRole("button", { name: "Refund" }));
    await pickVendor("Maxi");
    await pickCategory("Eating out");
    await pickDate("Yesterday");
    await typeNote("returned");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await screen.findByRole("heading", { level: 1, name: "Entries" });

    await userEvent.click(await screen.findByRole("button", { name: "Actions for Maxi" }));
    await userEvent.click(await screen.findByRole("menuitem", { name: "Repeat" }));

    expect(await screen.findByRole("group", { name: "Keypad" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/new");
    expect(screen.getByRole("radio", { name: "Expense" })).toBeChecked();
    expect(amount()).toHaveTextContent(/^−1,990\.50\s*din$/);
    expect(screen.getByRole("button", { name: "Refund" })).toHaveAttribute("aria-pressed", "true");
    expect(tile("Vendor")).toHaveTextContent(/^VendorMaxi$/);
    expect(tile("Category")).toHaveTextContent(/^CategoryEating out$/);
    expect(tile("Date")).toHaveTextContent("Today");
    expect(tile("Note")).toHaveTextContent("returned");

    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await screen.findByRole("heading", { level: 1, name: "Entries" });

    const [original, repeated] = fake.creates;
    expect(repeated?.body).toEqual({ ...original?.body, entryDate: "2026-03-02" });
    expect(repeated?.idempotencyKey).toMatch(/^[0-9a-f-]{36}$/);
    expect(repeated?.idempotencyKey).not.toBe(original?.idempotencyKey);
  });

  it("opens an income row's Repeat on Income without Refund", async () => {
    fakeSessionApi({ loggedIn: true });
    fakeEntriesApi({
      entries: [fakeEntry({ entryDate: "2026-02-01", kind: "income", amount: 14500000, categoryId: salary.id, categoryName: salary.name })],
    });
    renderApp("/entries");

    await userEvent.click(await screen.findByRole("button", { name: "Actions for Salary" }));
    await userEvent.click(await screen.findByRole("menuitem", { name: "Repeat" }));

    expect(await screen.findByRole("radio", { name: "Income" })).toBeChecked();
    expect(amount()).toHaveTextContent(/^145,000\.00\s*din$/);
    expect(tile("Category")).toHaveTextContent("Salary");
    expect(tile("Source")).toHaveTextContent("Optional");
    expect(screen.queryByRole("button", { name: "Refund" })).not.toBeInTheDocument();
  });

  it("leaves the amount empty when the row is in another currency than the primary one, which the keypad records", async () => {
    fakeSessionApi({ loggedIn: true });
    fakeEntriesApi({
      entries: [fakeEntry({ entryDate: "2026-02-01", amount: 1250, currency: "EUR", categoryId: eatingOut.id, categoryName: eatingOut.name })],
    });
    renderApp("/entries");

    await userEvent.click(await screen.findByRole("button", { name: "Actions for Eating out" }));
    await userEvent.click(await screen.findByRole("menuitem", { name: "Repeat" }));

    expect(await screen.findByRole("group", { name: "Keypad" })).toBeInTheDocument();
    expect(amount()).toHaveTextContent(/^0\s*din$/);
    expect(tile("Category")).toHaveTextContent("Eating out");
  });

  it("fills the keypad from the entry as it is now when the same row is repeated again after it changed", async () => {
    fakeSessionApi({ loggedIn: true });
    const fake = fakeEntriesApi({ entries: [fakeEntry({ entryDate: "2026-02-01", amount: 45000, categoryId: eatingOut.id, categoryName: eatingOut.name })] });
    renderApp("/entries");
    const repeat = async () => {
      await userEvent.click(await screen.findByRole("button", { name: "Actions for Eating out" }));
      await userEvent.click(await screen.findByRole("menuitem", { name: "Repeat" }));
      await screen.findByRole("group", { name: "Keypad" });
    };

    await repeat();
    expect(amount()).toHaveTextContent(/^450\.00\s*din$/);
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await screen.findByRole("heading", { level: 1, name: "Entries" });
    Object.assign(fake.entries[0] ?? {}, { amount: 99900 });
    await repeat();

    expect(amount()).toHaveTextContent(/^999\.00\s*din$/);
  });

  it("keeps the row's category when it is archived, though the picker hides it", async () => {
    fakeSessionApi({ loggedIn: true });
    const entry = fakeEntry({ entryDate: "2026-02-01", amount: 45000, categoryId: oldHobby.id, categoryName: oldHobby.name });
    const fake = fakeEntriesApi({ entries: [entry] });
    renderApp(`/new?repeat=${entry.id}`);

    expect(await screen.findByRole("button", { name: /^Category/ })).toHaveTextContent("Old hobby");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    await screen.findByRole("heading", { level: 1, name: "Entries" });
    expect(fake.creates.map((c) => c.body.categoryId)).toEqual([oldHobby.id]);
  });

  it("opens an empty keypad when the repeated entry was deleted meanwhile", async () => {
    fakeSessionApi({ loggedIn: true });
    const entry = fakeEntry({ entryDate: "2026-02-01" });
    const fake = fakeEntriesApi({ entries: [entry] });
    fake.deleted.add(entry.id);
    renderApp(`/new?repeat=${entry.id}`);

    expect(await screen.findByRole("group", { name: "Keypad" })).toBeInTheDocument();
    expect(amount()).toHaveTextContent(/^0\s*din$/);
    expect(tile("Category")).toHaveTextContent("Pick one");
  });
});
