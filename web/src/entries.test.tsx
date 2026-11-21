import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { eatingOut, fakeEntriesApi, fakeEntry, fakeSessionApi, salary } from "@/test/api";
import { scrollObservedIntoView } from "@/test/intersection";
import { renderApp } from "@/test/render";

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"], now: new Date("2026-03-02T08:30:00Z") });
});

afterEach(() => {
  vi.useRealTimers();
});

function rows(day: HTMLElement) {
  return within(day).getAllByRole("listitem");
}

async function days() {
  return screen.findAllByRole("region");
}

describe("entry list", () => {
  it("groups all entries under day headers, newest entry date first and newest created first within a day", async () => {
    fakeSessionApi({ loggedIn: true });
    fakeEntriesApi({
      entries: [
        fakeEntry({ entryDate: "2025-12-24", note: "last year" }),
        fakeEntry({ entryDate: "2026-03-02", note: "first today", createdAt: "2026-03-02T07:00:00Z" }),
        fakeEntry({ entryDate: "2026-03-01", note: "yesterday" }),
        fakeEntry({ entryDate: "2026-03-02", note: "second today", createdAt: "2026-03-02T08:00:00Z" }),
      ],
    });
    renderApp("/entries");

    const sections = await days();

    expect(sections.map((s) => within(s).getByRole("heading").textContent)).toEqual([
      "Today",
      "Yesterday",
      "Wed 24 Dec 2025",
    ]);
    expect(rows(screen.getByRole("region", { name: "Today" })).map((r) => within(r).getByText(/today/).textContent)).toEqual([
      "second today",
      "first today",
    ]);
  });

  it("shows the category, a note snippet and the amount through formatMoney on each row", async () => {
    fakeSessionApi({ loggedIn: true });
    fakeEntriesApi({
      entries: [
        fakeEntry({ entryDate: "2026-03-02", amount: 178080, categoryName: eatingOut.name, note: "Lunch with the team at the usual place" }),
        fakeEntry({ entryDate: "2026-03-01", kind: "income", amount: 14500000, categoryId: salary.id, categoryName: salary.name }),
      ],
    });
    renderApp("/entries");

    await days();
    const [expense] = rows(screen.getByRole("region", { name: "Today" }));
    const [income] = rows(screen.getByRole("region", { name: "Yesterday" }));

    expect(expense).toHaveTextContent("Eating out");
    expect(screen.getByText("Lunch with the team at the usual place")).toHaveClass("truncate");
    expect(expense).toHaveTextContent("1,780.80 din");
    expect(income).toHaveTextContent("Salary");
    expect(income).toHaveTextContent("145,000.00 din");
  });

  it("formats each amount with its currency's minor units from the currencies list", async () => {
    fakeSessionApi({ loggedIn: true });
    fakeEntriesApi({
      entries: [
        fakeEntry({ entryDate: "2026-03-02", amount: 1780, currency: "JPY", note: "yen" }),
        fakeEntry({ entryDate: "2026-03-02", amount: 1780800, currency: "KWD", note: "dinar" }),
      ],
    });
    renderApp("/entries");

    await days();

    expect(screen.getByText("yen").closest("li")).toHaveTextContent("1,780 JPY");
    expect(screen.getByText("dinar").closest("li")).toHaveTextContent("1,780.800 KWD");
  });

  it("loads the next page when the end of the list scrolls into view", async () => {
    fakeSessionApi({ loggedIn: true });
    const api = fakeEntriesApi({
      pageSize: 2,
      entries: ["2026-03-02", "2026-03-01", "2026-02-28", "2026-02-27", "2026-02-26"].map((entryDate) =>
        fakeEntry({ entryDate }),
      ),
    });
    renderApp("/entries");

    expect(await days()).toHaveLength(2);

    scrollObservedIntoView();
    await waitFor(async () => {
      expect(await days()).toHaveLength(4);
    });
    scrollObservedIntoView();
    await waitFor(async () => {
      expect(await days()).toHaveLength(5);
    });

    expect(api.listRequests).toBe(3);
  });

  it("stops loading pages after one fails, until Try again is tapped", async () => {
    fakeSessionApi({ loggedIn: true });
    const api = fakeEntriesApi({
      pageSize: 2,
      entries: ["2026-03-02", "2026-03-01", "2026-02-28"].map((entryDate) => fakeEntry({ entryDate })),
    });
    renderApp("/entries");
    expect(await days()).toHaveLength(2);

    api.failLists = 1;
    scrollObservedIntoView();
    const retry = await screen.findByRole("button", { name: /Could not load more/ });
    scrollObservedIntoView();
    scrollObservedIntoView();
    await userEvent.click(retry);
    await waitFor(async () => {
      expect(await days()).toHaveLength(3);
    });
    expect(api.listRequests).toBe(3);
  });

  it("keeps a refresh that is still fetching when the end of the list scrolls into view", async () => {
    fakeSessionApi({ loggedIn: true });
    const api = fakeEntriesApi({
      pageSize: 2,
      entries: ["2026-03-01", "2026-02-28", "2026-02-27"].map((entryDate) => fakeEntry({ entryDate })),
    });
    renderApp("/entries");
    expect(await days()).toHaveLength(2);

    api.entries.push(fakeEntry({ entryDate: "2026-03-02", note: "from another device" }));
    const release = api.holdLists();
    await userEvent.click(screen.getByRole("button", { name: "Refresh" }));
    scrollObservedIntoView();
    release();

    expect(await screen.findByText("from another device")).toBeInTheDocument();
    expect(api.listRequests).toBe(2);
  });

  it("refetches the list from the refresh control in the header", async () => {
    fakeSessionApi({ loggedIn: true });
    const api = fakeEntriesApi({ entries: [fakeEntry({ entryDate: "2026-03-01", note: "old" })] });
    renderApp("/entries");
    expect(await days()).toHaveLength(1);

    api.entries.push(fakeEntry({ entryDate: "2026-03-02", note: "from another device" }));
    await userEvent.click(screen.getByRole("button", { name: "Refresh" }));

    expect(await screen.findByText("from another device")).toBeInTheDocument();
    expect(api.listRequests).toBe(2);
  });

  it("says there are no entries yet", async () => {
    fakeSessionApi({ loggedIn: true });
    fakeEntriesApi();
    renderApp("/entries");

    expect(await screen.findByText(/No entries yet/)).toBeInTheDocument();
  });
});
