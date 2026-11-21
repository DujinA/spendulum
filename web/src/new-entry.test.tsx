import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { fakeEntriesApi, fakeEntry, fakeSessionApi, groceries } from "@/test/api";
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
});
