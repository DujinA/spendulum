import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { fakeEntriesApi, fakeEntry, fakeSessionApi } from "@/test/api";
import { renderApp } from "@/test/render";

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"], now: new Date("2026-03-02T08:30:00Z") });
});

afterEach(() => {
  vi.useRealTimers();
});

const lunch = () => fakeEntry({ entryDate: "2026-03-02", vendorId: "0195f3a2-0000-7000-8000-000000000099", vendorName: "Kafeterija", note: "lunch" });
const older = () => fakeEntry({ entryDate: "2026-03-01", note: "older" });

async function openList() {
  fakeSessionApi({ loggedIn: true });
  const entries = [lunch(), older()];
  const fake = fakeEntriesApi({ entries });
  const app = renderApp("/entries");
  await screen.findByRole("heading", { level: 1, name: "Entries" });
  return { fake, entries, ...app };
}

function row(name: RegExp) {
  const link = screen.getByRole("link", { name });
  const listRow = link.closest("li");
  if (!listRow) throw new Error("the link is not in a row");
  return listRow;
}

function swipe(element: HTMLElement, from: number, to: number) {
  fireEvent.pointerDown(element, { pointerId: 1, pointerType: "touch", clientX: from, clientY: 20 });
  fireEvent.pointerMove(element, { pointerId: 1, pointerType: "touch", clientX: (from + to) / 2, clientY: 22 });
  fireEvent.pointerMove(element, { pointerId: 1, pointerType: "touch", clientX: to, clientY: 22 });
  fireEvent.pointerUp(element, { pointerId: 1, pointerType: "touch", clientX: to, clientY: 22 });
}

describe("entry row actions", () => {
  it("deletes from the row menu: the entry leaves the list at once and a toast shows Deleted with Undo", async () => {
    const { fake, entries } = await openList();

    await userEvent.click(within(row(/Kafeterija/)).getByRole("button", { name: /^Actions/ }));
    await userEvent.click(await screen.findByRole("menuitem", { name: "Delete" }));

    expect(screen.queryByRole("link", { name: /Kafeterija/ })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: /older/ })).toBeInTheDocument();
    expect(await screen.findByText("Deleted")).toBeInTheDocument();
    expect(fake.deletes).toEqual([entries[0]?.id]);

    await userEvent.click(screen.getByRole("button", { name: "Undo" }));

    expect(await screen.findByRole("link", { name: /Kafeterija/ })).toBeInTheDocument();
    expect(fake.restores).toEqual([entries[0]?.id]);
  });

  it("brings a row back and says so when the delete fails", async () => {
    const { fake } = await openList();
    fake.failDeletes = 1;

    await userEvent.click(within(row(/Kafeterija/)).getByRole("button", { name: /^Actions/ }));
    await userEvent.click(await screen.findByRole("menuitem", { name: "Delete" }));

    expect(await screen.findByText(/Could not delete/)).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: /Kafeterija/ })).toBeInTheDocument();
    expect(screen.queryByText("Deleted")).not.toBeInTheDocument();
  });

  it("brings the row back when the delete fails even if the list cannot reload either", async () => {
    const { fake } = await openList();
    fake.failDeletes = 1;
    fake.failLists = 1;

    await userEvent.click(within(row(/Kafeterija/)).getByRole("button", { name: /^Actions/ }));
    await userEvent.click(await screen.findByRole("menuitem", { name: "Delete" }));

    expect(await screen.findByText(/Could not delete/)).toBeInTheDocument();
    expect(await screen.findByRole("link", { name: /Kafeterija/ })).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: /older|Kafeterija/ }).map((l) => l.textContent)).toEqual([
      expect.stringMatching(/^Kafeterija/),
      expect.stringMatching(/older/),
    ]);
  });

  it("keeps Undo in the toast when undoing fails, so it can be tapped again", async () => {
    const { fake, entries } = await openList();
    fake.failRestores = 1;
    await userEvent.click(within(row(/Kafeterija/)).getByRole("button", { name: /^Actions/ }));
    await userEvent.click(await screen.findByRole("menuitem", { name: "Delete" }));

    await userEvent.click(await screen.findByRole("button", { name: "Undo" }));
    const failed = (await screen.findByText(/Could not undo/)).closest("li");
    // Long enough for a closing toast to leave the page.
    await new Promise((resolve) => setTimeout(resolve, 500));
    expect(within(failed ?? document.body).getByText("Deleted")).toBeInTheDocument();
    await userEvent.click(within(failed ?? document.body).getByRole("button", { name: "Undo" }));

    expect(await screen.findByRole("link", { name: /Kafeterija/ })).toBeInTheDocument();
    expect(fake.restores).toEqual([entries[0]?.id, entries[0]?.id]);
    await waitFor(() => {
      expect(screen.queryByRole("button", { name: "Undo" })).not.toBeInTheDocument();
    });
  });

  it("reveals Repeat and Delete when a row is swiped left on a touch screen, and hides them when swiped back", async () => {
    await openList();
    const listRow = row(/Kafeterija/);
    const content = screen.getByRole("link", { name: /Kafeterija/ });

    expect(within(listRow).queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();

    swipe(content, 300, 150);

    expect(within(listRow).getByRole("button", { name: "Repeat" })).toBeInTheDocument();
    expect(within(listRow).getByRole("button", { name: "Delete" })).toBeInTheDocument();
    expect(within(row(/older/)).queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();

    swipe(content, 150, 300);

    expect(within(listRow).queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();
  });

  it("deletes a swiped row with its Delete button", async () => {
    const { fake, entries } = await openList();

    swipe(screen.getByRole("link", { name: /Kafeterija/ }), 300, 150);
    await userEvent.click(within(row(/Kafeterija/)).getByRole("button", { name: "Delete" }));

    await waitFor(() => {
      expect(screen.queryByRole("link", { name: /Kafeterija/ })).not.toBeInTheDocument();
    });
    expect(fake.deletes).toEqual([entries[0]?.id]);
    expect(await screen.findByText("Deleted")).toBeInTheDocument();
  });

  it("neither opens the edit form after a swipe nor swipes on a mouse drag", async () => {
    const { router } = await openList();
    const content = screen.getByRole("link", { name: /Kafeterija/ });

    swipe(content, 300, 150);
    fireEvent.click(content);
    expect(router.state.location.pathname).toBe("/entries");

    fireEvent.click(content);
    expect(within(row(/Kafeterija/)).queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/entries");

    fireEvent.pointerDown(content, { pointerId: 2, pointerType: "mouse", clientX: 300, clientY: 20 });
    fireEvent.pointerMove(content, { pointerId: 2, pointerType: "mouse", clientX: 100, clientY: 20 });
    fireEvent.pointerUp(content, { pointerId: 2, pointerType: "mouse", clientX: 100, clientY: 20 });
    expect(within(row(/Kafeterija/)).queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();
  });

  it("leaves a vertical scroll alone", async () => {
    await openList();
    const content = screen.getByRole("link", { name: /Kafeterija/ });

    fireEvent.pointerDown(content, { pointerId: 1, pointerType: "touch", clientX: 300, clientY: 20 });
    fireEvent.pointerMove(content, { pointerId: 1, pointerType: "touch", clientX: 280, clientY: 120 });
    fireEvent.pointerMove(content, { pointerId: 1, pointerType: "touch", clientX: 150, clientY: 130 });
    fireEvent.pointerUp(content, { pointerId: 1, pointerType: "touch", clientX: 150, clientY: 130 });

    expect(within(row(/Kafeterija/)).queryByRole("button", { name: "Delete" })).not.toBeInTheDocument();
  });
});
