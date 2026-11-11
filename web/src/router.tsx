import type { QueryClient } from "@tanstack/react-query";
import {
  createRootRouteWithContext,
  createRoute,
  createRouter,
  Outlet,
  redirect,
  type RouterHistory,
} from "@tanstack/react-router";
import type { ReactNode } from "react";

import { ApiError, type ApiClient } from "@/api/client";
import { sessionQuery } from "@/api/session";
import { AppShell } from "@/components/app-shell";
import { LoginScreen } from "@/routes/login";
import { MoreScreen } from "@/routes/more";
import { Placeholder } from "@/routes/placeholder";

export interface RouterContext {
  queryClient: QueryClient;
  api: ApiClient;
}

const rootRoute = createRootRouteWithContext<RouterContext>()({ component: Outlet });

export interface LoginSearch {
  redirect?: string;
}

export const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/login",
  validateSearch: (search: Record<string, unknown>): LoginSearch =>
    typeof search.redirect === "string" ? { redirect: search.redirect } : {},
  // The return type breaks the cycle between the route's type and its component.
  component: function Login(): ReactNode {
    const { redirect }: LoginSearch = loginRoute.useSearch();
    return <LoginScreen redirect={redirect} />;
  },
});

const appRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "app",
  component: AppShell,
  beforeLoad: async ({ context, location }) => {
    try {
      await context.queryClient.fetchQuery(sessionQuery(context.api));
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        // eslint-disable-next-line @typescript-eslint/only-throw-error -- TanStack Router redirects by throwing
        throw redirect({ to: "/login", search: { redirect: location.href } });
      }
      throw error;
    }
  },
});

const routeTree = rootRoute.addChildren([
  loginRoute,
  appRoute.addChildren([
    createRoute({ getParentRoute: () => appRoute, path: "/", component: () => <Placeholder title="Dashboard" /> }),
    createRoute({ getParentRoute: () => appRoute, path: "/entries", component: () => <Placeholder title="Entries" /> }),
    createRoute({ getParentRoute: () => appRoute, path: "/new", component: () => <Placeholder title="Add entry" /> }),
    createRoute({ getParentRoute: () => appRoute, path: "/more", component: MoreScreen }),
  ]),
]);

export function createAppRouter(options: { context: RouterContext; history?: RouterHistory }) {
  return createRouter({ routeTree, ...options });
}

declare module "@tanstack/react-router" {
  interface Register {
    router: ReturnType<typeof createAppRouter>;
  }
}
