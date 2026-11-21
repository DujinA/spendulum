import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import {
  eatingOut,
  fakeCategoriesApi,
  fakeEntriesApi,
  fakeSessionApi,
  groceries,
  oldHobby,
  otherExpense,
  otherIncome,
  salary,
  type FakeCategoriesOptions,
} from "@/test/api";
import { renderApp } from "@/test/render";

async function openCategories(options?: FakeCategoriesOptions) {
  fakeSessionApi({ loggedIn: true });
  fakeEntriesApi();
  const fake = fakeCategoriesApi(options);
  const { router } = renderApp("/more/categories");
  await screen.findByRole("heading", { level: 1, name: "Categories" });
  return { fake, router };
}

function names(list: HTMLElement) {
  return within(list).queryAllByRole("listitem").map((item) => item.getAttribute("aria-label"));
}

function activeList(kind: "Expense" | "Income") {
  return screen.getByRole("list", { name: `${kind} categories` });
}

function archivedList(kind: "Expense" | "Income") {
  return screen.getByRole("list", { name: `Archived ${kind.toLowerCase()} categories` });
}

async function openCategory(name: string, kind: "Expense" | "Income" = "Expense") {
  const section = screen.getByRole("region", { name: kind });
  await userEvent.click(within(section).getByRole("button", { name }));
  return screen.findByRole("dialog", { name: "Edit category" });
}

async function closed(sheet: HTMLElement) {
  await waitFor(() => {
    expect(sheet).not.toBeInTheDocument();
  });
}

async function keypadCategoryNames() {
  await userEvent.click(within(screen.getByRole("navigation", { name: "Tabs" })).getByRole("link", { name: "Add entry" }));
  await userEvent.click(await screen.findByRole("button", { name: /^Category/ }));
  const sheet = await screen.findByRole("dialog", { name: "Category" });
  return within(sheet)
    .getAllByRole("button", { name: /^(?!Done)/ })
    .map((b) => b.textContent);
}

describe("More → Categories", () => {
  it("is reached from More", async () => {
    fakeSessionApi({ loggedIn: true });
    fakeEntriesApi();
    fakeCategoriesApi();
    const { router } = renderApp("/more");

    await userEvent.click(await screen.findByRole("link", { name: "Categories" }));

    expect(await screen.findByRole("heading", { level: 1, name: "Categories" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/more/categories");
  });

  it("shows the Expense and Income lists in manual order, with archived categories collapsed apart", async () => {
    await openCategories();

    expect(screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent)).toEqual(["Expense", "Income"]);
    expect(names(activeList("Expense"))).toEqual(["Groceries", "Eating out", "Other"]);
    expect(names(activeList("Income"))).toEqual(["Salary", "Other"]);
    expect(within(activeList("Expense")).getByText("Food and drugstore")).toBeVisible();

    const archived = archivedList("Expense");
    expect(names(archived)).toEqual(["Old hobby"]);
    expect(archived).not.toBeVisible();

    await userEvent.click(screen.getByText("Archived (1)"));
    expect(archived).toBeVisible();
    expect(screen.queryByRole("list", { name: "Archived income categories" })).not.toBeInTheDocument();
  });

  it("creates an income category with a note at the end of the Income list, sending an Idempotency-Key", async () => {
    const { fake } = await openCategories();

    await userEvent.click(screen.getByRole("button", { name: "Add income category" }));
    const sheet = await screen.findByRole("dialog", { name: "New income category" });
    await userEvent.type(within(sheet).getByLabelText("Name"), "Freelance");
    await userEvent.type(within(sheet).getByLabelText("Note"), "Side projects");
    await userEvent.click(within(sheet).getByRole("button", { name: "Create" }));
    await closed(sheet);

    expect(fake.creates).toEqual([
      { idempotencyKey: expect.any(String) as string, body: { kind: "income", name: "Freelance", note: "Side projects" } },
    ]);
    await waitFor(() => {
      expect(names(activeList("Income"))).toEqual(["Salary", "Other", "Freelance"]);
    });
  });

  it("keeps Create disabled until the name has text", async () => {
    await openCategories();

    await userEvent.click(screen.getByRole("button", { name: "Add expense category" }));
    const sheet = await screen.findByRole("dialog", { name: "New expense category" });
    const create = within(sheet).getByRole("button", { name: "Create" });

    expect(create).toBeDisabled();
    await userEvent.type(within(sheet).getByLabelText("Name"), "   ");
    expect(create).toBeDisabled();
    await userEvent.type(within(sheet).getByLabelText("Name"), "Rent");
    expect(create).toBeEnabled();
  });

  it("names the existing category when a new name is taken in the same kind, and keeps the sheet open", async () => {
    const cafes = { ...eatingOut, name: "Kafići" };
    const { fake } = await openCategories({ categories: [groceries, cafes, otherExpense, salary, otherIncome] });

    await userEvent.click(screen.getByRole("button", { name: "Add expense category" }));
    const sheet = await screen.findByRole("dialog", { name: "New expense category" });
    await userEvent.type(within(sheet).getByLabelText("Name"), "kafici ");
    await userEvent.click(within(sheet).getByRole("button", { name: "Create" }));

    expect(await within(sheet).findByRole("alert")).toHaveTextContent("An expense category named “Kafići” already exists.");
    expect(sheet).toBeInTheDocument();

    await userEvent.click(within(sheet).getByRole("button", { name: "Create" }));
    await waitFor(() => {
      expect(fake.creates).toHaveLength(2);
    });
    expect(fake.creates[1]?.idempotencyKey).toBe(fake.creates[0]?.idempotencyKey);
  });

  it("names the existing category when a rename is taken in the same kind", async () => {
    const cafes = { ...eatingOut, name: "Kafići" };
    await openCategories({ categories: [groceries, cafes, otherExpense, salary, otherIncome] });

    const sheet = await openCategory("Groceries");
    const name = within(sheet).getByLabelText("Name");
    await userEvent.clear(name);
    await userEvent.type(name, "KAFICI");
    await userEvent.click(within(sheet).getByRole("button", { name: "Save" }));

    expect(await within(sheet).findByRole("alert")).toHaveTextContent("An expense category named “Kafići” already exists.");
  });

  it("renames a category and changes its note, never sending a kind", async () => {
    const { fake } = await openCategories();

    const sheet = await openCategory("Groceries");
    expect(within(sheet).queryByRole("radio")).not.toBeInTheDocument();
    const name = within(sheet).getByLabelText("Name");
    const note = within(sheet).getByLabelText("Note");
    await userEvent.clear(name);
    await userEvent.type(name, "Food shop");
    await userEvent.clear(note);
    await userEvent.click(within(sheet).getByRole("button", { name: "Save" }));
    await closed(sheet);

    expect(fake.changes).toEqual([{ id: groceries.id, body: { name: "Food shop", note: "" } }]);
    expect(await within(activeList("Expense")).findByRole("button", { name: "Food shop" })).toBeInTheDocument();
  });

  it("moves a category down with its control, saves the whole order of the kind, and the keypad's Category sheet uses it", async () => {
    const { fake } = await openCategories();

    await userEvent.click(screen.getByRole("button", { name: "Move Groceries down" }));

    expect(names(activeList("Expense"))).toEqual(["Eating out", "Groceries", "Other"]);
    await waitFor(() => {
      expect(fake.orders).toEqual([{ kind: "expense", ids: [eatingOut.id, groceries.id, otherExpense.id, oldHobby.id] }]);
    });
    expect(await keypadCategoryNames()).toEqual(["Eating out", "Groceries", "Other"]);
  });

  it("moves a category up, and offers no move past either end", async () => {
    const { fake } = await openCategories();

    expect(screen.getByRole("button", { name: "Move Groceries up" })).toBeDisabled();
    expect(within(activeList("Expense")).getByRole("button", { name: "Move Other down" })).toBeDisabled();

    await userEvent.click(screen.getByRole("button", { name: "Move Salary down" }));

    await waitFor(() => {
      expect(fake.orders).toEqual([{ kind: "income", ids: [otherIncome.id, salary.id] }]);
    });
    expect(names(activeList("Income"))).toEqual(["Other", "Salary"]);
  });

  it("puts the order back and says so when saving it fails", async () => {
    const { fake } = await openCategories();
    fake.failOrders = 1;

    await userEvent.click(screen.getByRole("button", { name: "Move Groceries down" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Could not save the order. Check your connection and try again.");
    expect(names(activeList("Expense"))).toEqual(["Groceries", "Eating out", "Other"]);
  });

  it("archives a category in use: it leaves the keypad's Category sheet and moves to Archived, and can be unarchived", async () => {
    const { fake } = await openCategories({ inUse: [groceries.id] });

    await userEvent.click(within(await openCategory("Groceries")).getByRole("button", { name: "Archive" }));

    await waitFor(() => {
      expect(names(archivedList("Expense"))).toEqual(["Groceries", "Old hobby"]);
    });
    expect(names(activeList("Expense"))).toEqual(["Eating out", "Other"]);
    expect(fake.changes).toEqual([{ id: groceries.id, body: { archived: true } }]);

    await userEvent.click(screen.getByText("Archived (2)"));
    const sheet = await openCategory("Groceries");
    await userEvent.click(within(sheet).getByRole("button", { name: "Unarchive" }));
    await closed(sheet);

    await waitFor(() => {
      expect(names(activeList("Expense"))).toEqual(["Groceries", "Eating out", "Other"]);
    });
    expect(await keypadCategoryNames()).toEqual(["Groceries", "Eating out", "Other"]);
  });

  it("leaves an archived category out of the keypad's Category sheet", async () => {
    await openCategories();

    await userEvent.click(within(await openCategory("Eating out")).getByRole("button", { name: "Archive" }));
    await waitFor(() => {
      expect(names(activeList("Expense"))).toEqual(["Groceries", "Other"]);
    });

    expect(await keypadCategoryNames()).toEqual(["Groceries", "Other"]);
  });

  it("deletes a category that has no entries", async () => {
    const { fake } = await openCategories();

    const sheet = await openCategory("Eating out");
    await userEvent.click(within(sheet).getByRole("button", { name: "Delete" }));
    await closed(sheet);

    expect(fake.deletes).toEqual([eatingOut.id]);
    await waitFor(() => {
      expect(names(activeList("Expense"))).toEqual(["Groceries", "Other"]);
    });
  });

  it("refuses to delete a category with entries and offers Archive instead", async () => {
    const { fake } = await openCategories({ inUse: [groceries.id] });

    const sheet = await openCategory("Groceries");
    await userEvent.click(within(sheet).getByRole("button", { name: "Delete" }));

    const alert = await within(sheet).findByRole("alert");
    expect(alert).toHaveTextContent("This category has entries, so it cannot be deleted. Archive it instead.");

    await userEvent.click(within(alert).getByRole("button", { name: "Archive" }));
    await closed(sheet);
    expect(fake.changes).toEqual([{ id: groceries.id, body: { archived: true } }]);
  });

  it("manages each kind's Other like any other category: rename, move and archive", async () => {
    const { fake } = await openCategories();

    const sheet = await openCategory("Other", "Income");
    const name = within(sheet).getByLabelText("Name");
    await userEvent.clear(name);
    await userEvent.type(name, "Gifts");
    await userEvent.click(within(sheet).getByRole("button", { name: "Save" }));
    await closed(sheet);

    await userEvent.click(await screen.findByRole("button", { name: "Move Gifts up" }));
    await waitFor(() => {
      expect(names(activeList("Income"))).toEqual(["Gifts", "Salary"]);
    });

    await userEvent.click(within(await openCategory("Other")).getByRole("button", { name: "Archive" }));
    await waitFor(() => {
      expect(names(archivedList("Expense"))).toEqual(["Other", "Old hobby"]);
    });
    expect(fake.changes.map((c) => c.body)).toEqual([{ name: "Gifts", note: "" }, { archived: true }]);
  });
});
