import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import {
  eatingOut,
  fakeEntriesApi,
  fakeEntry,
  fakeSessionApi,
  fakeVendor,
  groceries,
  oldHobby,
  otherIncome,
  salary,
  type FakeEntriesOptions,
} from "@/test/api";
import { renderApp } from "@/test/render";

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"], now: new Date("2026-03-02T08:30:00Z") });
});

afterEach(() => {
  vi.useRealTimers();
});

const maxi = fakeVendor({
  name: "Maxi",
  useCount: 12,
  latest: { categoryId: groceries.id, categoryName: groceries.name, currency: "RSD" },
});

function shop(fields: Partial<Parameters<typeof fakeEntry>[0]> = {}) {
  return fakeEntry({
    entryDate: "2026-03-01",
    amount: 178080,
    vendorId: maxi.id,
    vendorName: maxi.name,
    note: "weekly shop",
    ...fields,
  });
}

async function openList(options: FakeEntriesOptions) {
  fakeSessionApi({ loggedIn: true });
  const fake = fakeEntriesApi({ vendors: { expense: [maxi] }, ...options });
  const app = renderApp("/entries");
  await screen.findByRole("heading", { level: 1, name: "Entries" });
  return { fake, ...app };
}

async function openEdit(name: RegExp) {
  await userEvent.click(await screen.findByRole("link", { name }));
  return screen.findByRole("form", { name: /^Edit/ });
}

describe("edit form", () => {
  it("opens from a tapped row with amount, Refund, vendor, category, date, note and currency, and shows the kind without a way to change it", async () => {
    await openList({ entries: [shop({ amount: -178080, currency: "EUR", categoryId: eatingOut.id, categoryName: eatingOut.name })] });

    const form = await openEdit(/Maxi/);

    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Edit expense");
    expect(within(form).queryByRole("radio")).not.toBeInTheDocument();
    expect(within(form).getByRole("textbox", { name: "Amount" })).toHaveValue("1780.80");
    expect(within(form).getByText("EUR")).toBeInTheDocument();
    expect(within(form).getByRole("button", { name: "Refund" })).toHaveAttribute("aria-pressed", "true");
    expect(within(form).getByRole("button", { name: /^Vendor/ })).toHaveTextContent("Maxi");
    expect(within(form).getByRole("combobox", { name: "Category" })).toHaveDisplayValue("Eating out");
    expect(within(form).getByLabelText("Date")).toHaveValue("2026-03-01");
    expect(within(form).getByRole("textbox", { name: "Note" })).toHaveValue("weekly shop");
  });

  it("offers only the entry's own kind's categories, keeping an archived one it already has", async () => {
    await openList({
      categories: [groceries, eatingOut, oldHobby, salary, otherIncome],
      entries: [shop({ categoryId: oldHobby.id, categoryName: oldHobby.name })],
    });

    const form = await openEdit(/Maxi/);
    const category = within(form).getByRole("combobox", { name: "Category" });

    expect(within(category).getAllByRole("option").map((o) => o.textContent)).toEqual(["Groceries", "Eating out", "Old hobby"]);
    expect(category).toHaveDisplayValue("Old hobby");
  });

  it("says Source and hides Refund on an income", async () => {
    await openList({
      entries: [fakeEntry({ entryDate: "2026-03-01", kind: "income", amount: 500000, categoryId: salary.id, categoryName: salary.name })],
    });

    const form = await openEdit(/Salary/);

    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Edit income");
    expect(within(form).getByRole("button", { name: /^Source/ })).toHaveTextContent("Optional");
    expect(within(form).queryByRole("button", { name: "Refund" })).not.toBeInTheDocument();
  });

  it("saves the change without an Idempotency-Key and the list shows it", async () => {
    const entry = shop();
    const { fake } = await openList({ entries: [entry] });
    const form = await openEdit(/Maxi/);

    const amount = within(form).getByRole("textbox", { name: "Amount" });
    await userEvent.clear(amount);
    await userEvent.type(amount, "1990,5");
    await userEvent.click(within(form).getByRole("button", { name: "Refund" }));
    await userEvent.selectOptions(within(form).getByRole("combobox", { name: "Category" }), "Eating out");
    fireEvent.change(within(form).getByLabelText("Date"), { target: { value: "2026-02-27" } });
    const note = within(form).getByRole("textbox", { name: "Note" });
    await userEvent.clear(note);
    await userEvent.type(note, "  returned  ");
    await userEvent.click(within(form).getByRole("button", { name: "Save" }));

    expect(await screen.findByRole("heading", { level: 1, name: "Entries" })).toBeInTheDocument();
    expect(fake.updates).toEqual([
      {
        id: entry.id,
        idempotencyKey: null,
        body: { amount: -199050, categoryId: eatingOut.id, entryDate: "2026-02-27", note: "returned", vendorName: "Maxi" },
      },
    ]);
    const row = await screen.findByRole("link", { name: /Maxi/ });
    expect(row).toHaveTextContent("−1,990.50 din");
    expect(row).toHaveTextContent("Eating out · returned");
  });

  it("changes the vendor in the sheet or leaves it empty", async () => {
    const { fake } = await openList({ entries: [shop()], vendors: { expense: [maxi, fakeVendor({ name: "Lidl" })] } });
    const form = await openEdit(/Maxi/);

    await userEvent.click(within(form).getByRole("button", { name: /^Vendor/ }));
    let sheet = await screen.findByRole("dialog", { name: "Vendor" });
    const search = within(sheet).getByRole("searchbox", { name: "Search" });
    await userEvent.clear(search);
    await userEvent.type(search, "Lid");
    await userEvent.click(await within(sheet).findByRole("button", { name: "Lidl" }));
    await waitFor(() => {
      expect(sheet).not.toBeInTheDocument();
    });
    expect(within(form).getByRole("button", { name: /^Vendor/ })).toHaveTextContent("Lidl");

    await userEvent.click(within(form).getByRole("button", { name: /^Vendor/ }));
    sheet = await screen.findByRole("dialog", { name: "Vendor" });
    await userEvent.click(within(sheet).getByRole("button", { name: "Leave vendor empty" }));
    await waitFor(() => {
      expect(sheet).not.toBeInTheDocument();
    });
    await userEvent.click(within(form).getByRole("button", { name: "Save" }));

    await screen.findByRole("heading", { level: 1, name: "Entries" });
    expect(fake.updates.map((u) => u.body.vendorName)).toEqual([undefined]);
  });

  it.each([
    ["0", "The amount must not be 0."],
    ["12.345", "Type an amount like 1780.80."],
    ["", "Type an amount like 1780.80."],
  ])("refuses the amount %j before sending anything", async (typed, message) => {
    const { fake } = await openList({ entries: [shop()] });
    const form = await openEdit(/Maxi/);

    const amount = within(form).getByRole("textbox", { name: "Amount" });
    await userEvent.clear(amount);
    if (typed) await userEvent.type(amount, typed);
    await userEvent.click(within(form).getByRole("button", { name: "Save" }));

    expect(await within(form).findByText(message)).toBeInTheDocument();
    expect(amount).toHaveAttribute("aria-invalid", "true");
    expect(fake.updates).toEqual([]);
  });

  it("shows the server's reason when it refuses the change", async () => {
    await openList({ entries: [shop()], categories: [eatingOut, salary] });
    const form = await openEdit(/Maxi/);

    await userEvent.click(within(form).getByRole("button", { name: "Save" }));

    expect(await within(form).findByRole("alert")).toHaveTextContent("The category does not exist or is of the other kind.");
  });

  it("goes back to the list without saving on Cancel", async () => {
    const { fake } = await openList({ entries: [shop()] });
    await openEdit(/Maxi/);

    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));

    expect(await screen.findByRole("heading", { level: 1, name: "Entries" })).toBeInTheDocument();
    expect(fake.updates).toEqual([]);
  });
});

describe("delete with Undo", () => {
  async function deleteFromForm() {
    const kept = fakeEntry({ entryDate: "2026-02-28", note: "kept" });
    const deleted = shop();
    const { fake } = await openList({ entries: [kept, deleted] });
    const form = await openEdit(/Maxi/);
    await userEvent.click(within(form).getByRole("button", { name: "Delete" }));
    await screen.findByRole("heading", { level: 1, name: "Entries" });
    return { fake, deleted };
  }

  it("deletes from the edit form: the entry leaves the list at once and a toast shows Deleted with Undo", async () => {
    const { fake, deleted } = await deleteFromForm();

    expect(fake.deletes).toEqual([deleted.id]);
    expect(screen.queryByRole("link", { name: /Maxi/ })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: /kept/ })).toBeInTheDocument();
    const toast = (await screen.findByText("Deleted")).closest("li");
    expect(toast).not.toBeNull();
    expect(within(toast ?? document.body).getByRole("button", { name: "Undo" })).toBeInTheDocument();
  });

  it("brings the same entry back when Undo is tapped", async () => {
    const { fake, deleted } = await deleteFromForm();

    await userEvent.click(await screen.findByRole("button", { name: "Undo" }));

    expect(await screen.findByRole("link", { name: /Maxi/ })).toHaveTextContent("1,780.80 din");
    expect(fake.restores).toEqual([deleted.id]);
    await waitFor(() => {
      expect(screen.queryByText("Deleted")).not.toBeInTheDocument();
    });
  });

  it("closes the toast after about 8 seconds, and then nothing can restore the entry", async () => {
    vi.useFakeTimers({
      toFake: ["Date", "setTimeout", "clearTimeout"],
      now: new Date("2026-03-02T08:30:00Z"),
      shouldAdvanceTime: true,
    });
    const { fake } = await deleteFromForm();
    await screen.findByText("Deleted");

    await vi.advanceTimersByTimeAsync(7000);
    expect(screen.getByRole("button", { name: "Undo" })).toBeInTheDocument();

    await vi.advanceTimersByTimeAsync(2000);
    await waitFor(() => {
      expect(screen.queryByText("Deleted")).not.toBeInTheDocument();
    });
    expect(screen.queryByRole("button", { name: "Undo" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Maxi/ })).not.toBeInTheDocument();
    expect(fake.restores).toEqual([]);
  });
});
