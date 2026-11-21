import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { eatingOut, fakeEntriesApi, fakeSessionApi, fakeVendor, groceries, type FakeEntriesOptions } from "@/test/api";
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
const kafeterija = fakeVendor({
  name: "Kafeterija",
  useCount: 4,
  latest: { categoryId: eatingOut.id, categoryName: eatingOut.name, currency: "EUR" },
});
const apoteka = fakeVendor({ name: "Apoteka" });
const knjizara = fakeVendor({
  name: "Stara Knjižara",
  archived: true,
  useCount: 2,
  latest: { categoryId: groceries.id, categoryName: groceries.name, currency: "RSD" },
});

async function openKeypad(options: FakeEntriesOptions = {}) {
  fakeSessionApi({ loggedIn: true });
  const fake = fakeEntriesApi({ vendors: { expense: [maxi, kafeterija, apoteka, knjizara] }, ...options });
  renderApp("/new");
  await screen.findByRole("group", { name: "Keypad" });
  return fake;
}

async function tapKeypad(keys: string) {
  const keypad = screen.getByRole("group", { name: "Keypad" });
  for (const key of keys) {
    await userEvent.click(within(keypad).getByRole("button", { name: key }));
  }
}

function tile(label: string) {
  return screen.getByRole("button", { name: new RegExp(`^${label}`) });
}

async function openSheet(label: string) {
  await userEvent.click(tile(label));
  return screen.findByRole("dialog", { name: label });
}

async function closed(sheet: HTMLElement) {
  await waitFor(() => {
    expect(sheet).not.toBeInTheDocument();
  });
}

// Types the name and waits until the sheet shows the server's answer for it.
async function typeName(sheet: HTMLElement, name: string) {
  const search = within(sheet).getByRole("searchbox", { name: "Search" });
  await userEvent.clear(search);
  await userEvent.type(search, name);
  await waitFor(() => {
    expect(within(sheet).getByRole("region", { name: "Results" })).toHaveAttribute("aria-busy", "false");
  });
}

function listedVendors(sheet: HTMLElement) {
  return within(within(sheet).getByRole("list", { name: "Vendors" })).getAllByRole("button");
}

async function save() {
  await userEvent.click(screen.getByRole("button", { name: "Save" }));
  await screen.findByRole("heading", { level: 1, name: "Entries" });
}

describe("Vendor sheet", () => {
  it("starts the search with the current vendor's name, selected so typing replaces it", async () => {
    await openKeypad();
    let sheet = await openSheet("Vendor");
    await userEvent.click(await within(sheet).findByRole("button", { name: "Maxi" }));
    await closed(sheet);

    sheet = await openSheet("Vendor");
    await userEvent.keyboard("Apo");

    expect(within(sheet).getByRole("searchbox", { name: "Search" })).toHaveValue("Apo");
  });

  it("shows a search box and the vendors ranked by use of the current kind, each with its use count, last category and a currency other than the primary one", async () => {
    const fake = await openKeypad();

    const sheet = await openSheet("Vendor");

    expect(within(sheet).getByRole("searchbox", { name: "Search" })).toBeInTheDocument();
    const rows = await waitFor(() => {
      const found = listedVendors(sheet);
      expect(found).toHaveLength(3);
      return found;
    });
    expect(rows.map((r) => r.textContent)).toEqual([
      expect.stringMatching(/^Maxi/),
      expect.stringMatching(/^Kafeterija/),
      expect.stringMatching(/^Apoteka/),
    ]);
    expect(rows[0]).toHaveAccessibleDescription("12× · Groceries");
    expect(rows[1]).toHaveAccessibleDescription("4× · Eating out · EUR");
    expect(rows[2]).toHaveAccessibleDescription("0×");
    expect(fake.vendorSearches.at(-1)).toEqual({ kind: "expense", name: null });
  });

  it("says Source on income and ranks by the income entries", async () => {
    const fake = await openKeypad({ vendors: { income: [fakeVendor({ name: "Employer", useCount: 3 })] } });

    await userEvent.click(screen.getByRole("radio", { name: "Income" }));
    expect(tile("Source")).toHaveTextContent("Optional");
    const sheet = await openSheet("Source");

    expect(await within(sheet).findByRole("button", { name: "Employer" })).toBeInTheDocument();
    expect(within(sheet).getByRole("button", { name: "Leave source empty" })).toBeInTheDocument();
    expect(fake.vendorSearches.at(-1)).toEqual({ kind: "income", name: null });
  });

  it('uses "Maxi" for "maxi " or "MAXI" without offering Create or Did you mean', async () => {
    const fake = await openKeypad();
    await tapKeypad("5");

    let sheet = await openSheet("Vendor");
    await typeName(sheet, "maxi ");
    expect(await within(sheet).findByRole("button", { name: "Maxi" })).toBeInTheDocument();
    expect(within(sheet).queryByRole("button", { name: /create/i })).not.toBeInTheDocument();
    expect(within(sheet).queryByText(/Did you mean/)).not.toBeInTheDocument();
    await userEvent.keyboard("{Enter}");
    await closed(sheet);
    expect(tile("Vendor")).toHaveTextContent(/^VendorMaxi$/);

    sheet = await openSheet("Vendor");
    await typeName(sheet, "MAXI");
    await userEvent.click(within(sheet).getByRole("button", { name: "Maxi" }));
    await closed(sheet);
    await save();

    expect(fake.creates.map((c) => c.body.vendorName)).toEqual(["Maxi"]);
  });

  it("waits for the answer for the typed name when Enter comes first, then picks its match", async () => {
    const fake = await openKeypad();
    const sheet = await openSheet("Vendor");
    await within(sheet).findByRole("button", { name: "Maxi" });

    const release = fake.holdVendorSearches();
    await userEvent.type(within(sheet).getByRole("searchbox", { name: "Search" }), "maxi{Enter}");
    expect(sheet).toBeInTheDocument();
    release();
    await closed(sheet);

    expect(tile("Vendor")).toHaveTextContent(/^VendorMaxi$/);
    expect(tile("Category")).toHaveTextContent("Groceries");
    expect(tile("Category")).toHaveTextContent("last time");
  });

  it("offers Did you mean with the near-matches, then No, create «Kafetrija», and creating never blocks", async () => {
    const fake = await openKeypad({ nearMatches: { Kafetrija: [kafeterija] } });
    await tapKeypad("5");

    const sheet = await openSheet("Vendor");
    await typeName(sheet, "Kafetrija");

    const didYouMean = await within(sheet).findByRole("list", { name: "Did you mean …?" });
    expect(within(didYouMean).getAllByRole("button").map((b) => b.textContent)).toEqual([
      expect.stringMatching(/^Kafeterija/),
      "No, create «Kafetrija»",
    ]);
    await userEvent.click(within(didYouMean).getByRole("button", { name: "No, create «Kafetrija»" }));
    await closed(sheet);
    expect(tile("Vendor")).toHaveTextContent("Kafetrija");
    expect(tile("Vendor")).toHaveTextContent("new");

    await userEvent.click(tile("Category"));
    await userEvent.click(within(await screen.findByRole("dialog", { name: "Category" })).getByRole("button", { name: "Groceries" }));
    await save();

    expect(fake.creates.map((c) => c.body.vendorName)).toEqual(["Kafetrija"]);
  });

  it('offers Create "…" for a name with no match and no near-match', async () => {
    await openKeypad();

    const sheet = await openSheet("Vendor");
    await typeName(sheet, "Mesara");

    expect(await within(sheet).findByRole("button", { name: 'Create "Mesara"' })).toBeInTheDocument();
    expect(within(sheet).queryByText(/Did you mean/)).not.toBeInTheDocument();
  });

  it("creates a new vendor only by saving the entry, so Cancel sends nothing", async () => {
    const fake = await openKeypad();
    await tapKeypad("5");

    const sheet = await openSheet("Vendor");
    await typeName(sheet, "Mesara");
    await userEvent.click(await within(sheet).findByRole("button", { name: 'Create "Mesara"' }));
    await closed(sheet);
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));

    await screen.findByRole("heading", { level: 1, name: "Entries" });
    expect(fake.creates).toEqual([]);
  });

  it("shows an archived vendor typed by its exact name or offered as a near-match, and saves it by its name", async () => {
    const fake = await openKeypad({ nearMatches: { "Stara Knjizra": [knjizara] } });
    await tapKeypad("5");

    let sheet = await openSheet("Vendor");
    await typeName(sheet, "stara knjizara");
    expect(await within(sheet).findByRole("button", { name: "Stara Knjižara" })).toHaveAccessibleDescription(
      "Archived · 2× · Groceries",
    );
    await userEvent.click(screen.getByRole("button", { name: "Done" }));
    await closed(sheet);

    sheet = await openSheet("Vendor");
    await typeName(sheet, "Stara Knjizra");
    const didYouMean = await within(sheet).findByRole("list", { name: "Did you mean …?" });
    await userEvent.click(within(didYouMean).getByRole("button", { name: "Stara Knjižara" }));
    await closed(sheet);
    await save();

    expect(fake.creates.map((c) => c.body.vendorName)).toEqual(["Stara Knjižara"]);
  });

  it("always offers Leave vendor empty, which saves the entry without a vendor", async () => {
    const fake = await openKeypad({ nearMatches: { Kafetrija: [kafeterija] } });
    await tapKeypad("5");

    let sheet = await openSheet("Vendor");
    expect(within(sheet).getByRole("button", { name: "Leave vendor empty" })).toBeInTheDocument();
    await userEvent.click(await within(sheet).findByRole("button", { name: "Maxi" }));
    await closed(sheet);
    expect(tile("Vendor")).toHaveTextContent("Maxi");

    sheet = await openSheet("Vendor");
    for (const typed of ["Maxi", "Kafetrija", "Mesara"]) {
      await typeName(sheet, typed);
      expect(within(sheet).getByRole("button", { name: "Leave vendor empty" })).toBeInTheDocument();
    }
    await userEvent.click(within(sheet).getByRole("button", { name: "Leave vendor empty" }));
    await closed(sheet);
    expect(tile("Vendor")).toHaveTextContent("Optional");
    await save();

    expect(fake.creates).toHaveLength(1);
    expect(fake.creates[0]?.body).not.toHaveProperty("vendorName");
  });
});

describe("prefill from the picked vendor", () => {
  it("fills the category from the vendor's latest entry of the kind, marks it last time, and keeps the primary currency", async () => {
    const fake = await openKeypad();
    await tapKeypad("5");

    const sheet = await openSheet("Vendor");
    await userEvent.click(await within(sheet).findByRole("button", { name: "Kafeterija" }));
    await closed(sheet);

    expect(tile("Category")).toHaveTextContent("Eating out");
    expect(tile("Category")).toHaveTextContent("last time");
    expect(screen.getByRole("status", { name: "Amount" })).toHaveTextContent(/^5\s*din$/);
    await save();

    expect(fake.creates.map((c) => c.body)).toEqual([
      { kind: "expense", amount: 500, categoryId: eatingOut.id, vendorName: "Kafeterija", entryDate: "2026-03-02", note: "" },
    ]);
  });

  it("replaces a prefilled category with the next vendor's, but never a category the user picked", async () => {
    await openKeypad();

    let sheet = await openSheet("Vendor");
    await userEvent.click(await within(sheet).findByRole("button", { name: "Maxi" }));
    await closed(sheet);
    expect(tile("Category")).toHaveTextContent("Groceries");

    sheet = await openSheet("Vendor");
    await typeName(sheet, "Kafeterija");
    await userEvent.click(within(sheet).getByRole("button", { name: "Kafeterija" }));
    await closed(sheet);
    expect(tile("Category")).toHaveTextContent(/Eating out$/);

    sheet = await openSheet("Category");
    await userEvent.click(within(sheet).getByRole("button", { name: "Groceries" }));
    await closed(sheet);
    expect(tile("Category")).not.toHaveTextContent("last time");

    sheet = await openSheet("Vendor");
    await typeName(sheet, "Kafeterija");
    await userEvent.click(within(sheet).getByRole("button", { name: "Kafeterija" }));
    await closed(sheet);
    expect(tile("Category")).toHaveTextContent(/^CategoryGroceries$/);
  });
});
