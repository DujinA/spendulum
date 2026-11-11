import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";

import type { components } from "@/api/schema.gen";

type Problem = components["schemas"]["Problem"];
type Session = components["schemas"]["Session"];
type Credentials = components["schemas"]["Credentials"];

export const server = setupServer();

export const testEmail = "ana@example.test";
export const testPassword = "correct horse battery";

function problem(status: number, title: string, detail: string) {
  return HttpResponse.json<Problem>(
    { type: "about:blank", status, title, detail },
    { status, headers: { "Content-Type": "application/problem+json" } },
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

export function tooManyLogins(retryAfterSeconds: number) {
  server.use(
    http.post("*/api/session", () =>
      HttpResponse.json<Problem>(
        { type: "about:blank", status: 429, title: "Too Many Requests", detail: "Too many failed logins. Try again later." },
        {
          status: 429,
          headers: { "Content-Type": "application/problem+json", "Retry-After": String(retryAfterSeconds) },
        },
      ),
    ),
  );
}
