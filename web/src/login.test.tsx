import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { sessionQueryKey } from "@/api/session";
import { testEmail, testPassword, tooManyLogins, fakeSessionApi } from "@/test/api";
import { renderApp } from "@/test/render";

async function logIn(email = testEmail, password = testPassword) {
  await userEvent.type(await screen.findByLabelText("Email"), email);
  await userEvent.type(screen.getByLabelText("Password"), password);
  await userEvent.click(screen.getByRole("button", { name: "Log in" }));
}

describe("login", () => {
  it("opens the Dashboard after logging in with the right email and password", async () => {
    const api = fakeSessionApi({ loggedIn: false });
    const { router } = renderApp("/login");

    await logIn();

    expect(await screen.findByRole("heading", { level: 1, name: "Dashboard" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
    expect(api.logins).toBe(1);
  });

  it("goes to the login screen without a session and returns to the original route after login", async () => {
    fakeSessionApi({ loggedIn: false });
    const { router } = renderApp("/entries");

    expect(await screen.findByRole("heading", { level: 1, name: "Log in" })).toBeInTheDocument();
    expect(screen.queryByRole("navigation", { name: "Tabs" })).not.toBeInTheDocument();

    await logIn();

    expect(await screen.findByRole("heading", { level: 1, name: "Entries" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/entries");
  });

  it("goes to the login screen when any API request answers 401, and back after login", async () => {
    const api = fakeSessionApi({ loggedIn: true });
    const { router, queryClient } = renderApp("/more");
    expect(await screen.findByRole("heading", { level: 1, name: "More" })).toBeInTheDocument();

    api.loggedIn = false;
    await queryClient.refetchQueries({ queryKey: sessionQueryKey });

    expect(await screen.findByRole("heading", { level: 1, name: "Log in" })).toBeInTheDocument();

    await logIn();

    expect(await screen.findByRole("heading", { level: 1, name: "More" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/more");
  });

  it("says the email or password is incorrect on a 401 and stays on the login screen", async () => {
    fakeSessionApi({ loggedIn: false });
    const { router } = renderApp("/login");

    await logIn(testEmail, "wrong password!");

    expect(await screen.findByRole("alert")).toHaveTextContent("Email or password is incorrect.");
    expect(router.state.location.pathname).toBe("/login");
  });

  it("says how long to wait after too many failed logins", async () => {
    fakeSessionApi({ loggedIn: false });
    tooManyLogins(660);
    renderApp("/login");

    await logIn();

    expect(await screen.findByRole("alert")).toHaveTextContent("Too many failed logins. Try again in 11 minutes.");
  });

  it("never follows a redirect to another site", async () => {
    fakeSessionApi({ loggedIn: false });
    const { router } = renderApp("/login?redirect=%2F%2Fevil.example%2F");

    await logIn();

    expect(await screen.findByRole("heading", { level: 1, name: "Dashboard" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/");
  });
});

describe("logout", () => {
  it("logs out from More, deletes the server session and shows the login screen", async () => {
    const api = fakeSessionApi({ loggedIn: true });
    const { router } = renderApp("/");
    const tabs = await screen.findByRole("navigation", { name: "Tabs" });

    await userEvent.click(within(tabs).getByRole("link", { name: "More" }));
    await userEvent.click(await screen.findByRole("button", { name: "Log out" }));

    expect(await screen.findByRole("heading", { level: 1, name: "Log in" })).toBeInTheDocument();
    expect(api.logouts).toBe(1);
    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/login");
    });

    await router.navigate({ to: "/" });
    expect(await screen.findByRole("heading", { level: 1, name: "Log in" })).toBeInTheDocument();
  });
});
