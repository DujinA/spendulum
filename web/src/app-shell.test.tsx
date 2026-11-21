import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { fakeEntriesApi, fakeSessionApi } from "@/test/api";
import { renderApp } from "@/test/render";

function renderAt(path: string) {
  fakeSessionApi({ loggedIn: true });
  fakeEntriesApi();
  return renderApp(path).router;
}

const items = ["Dashboard", "Entries", "Add entry", "More"];

describe("app shell", () => {
  it("shows the bottom tab bar with Dashboard, Entries, a centre + and More", async () => {
    renderAt("/");

    const tabs = await screen.findByRole("navigation", { name: "Tabs" });
    const links = within(tabs).getAllByRole("link");

    expect(links.map((l) => l.getAttribute("aria-label") ?? l.textContent)).toEqual(items);
  });

  it("shows the same items in the sidebar for desktop widths", async () => {
    renderAt("/");

    const sidebar = await screen.findByRole("navigation", { name: "Sidebar" });
    const links = within(sidebar).getAllByRole("link");

    expect(links.map((l) => l.getAttribute("aria-label") ?? l.textContent)).toEqual(items);
  });

  it.each([
    ["/", "Dashboard"],
    ["/entries", "Entries"],
    ["/new", "Add entry"],
    ["/more", "More"],
  ])("renders the %s route", async (path, heading) => {
    renderAt(path);

    expect(await screen.findByRole("heading", { level: 1, name: heading })).toBeInTheDocument();
  });

  it("leaves the tab bar off the keypad screen, which has its own Cancel and Save", async () => {
    renderAt("/new");

    expect(await screen.findByRole("button", { name: "Cancel" })).toBeInTheDocument();
    expect(screen.queryByRole("navigation", { name: "Tabs" })).not.toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "Sidebar" })).toBeInTheDocument();
  });

  it("navigates when a tab is tapped and marks it current", async () => {
    const router = renderAt("/");
    const tabs = await screen.findByRole("navigation", { name: "Tabs" });

    await userEvent.click(within(tabs).getByRole("link", { name: "Entries" }));

    expect(await screen.findByRole("heading", { level: 1, name: "Entries" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/entries");
    expect(within(tabs).getByRole("link", { name: "Entries" })).toHaveAttribute("aria-current", "page");
  });
});
