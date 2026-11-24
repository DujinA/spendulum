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

import { allCategoriesQuery } from "@/api/categories";
import { ApiError, type ApiClient } from "@/api/client";
import { categoriesQuery, currenciesQuery, entriesQuery, entryQuery, settingsQuery, type Entry } from "@/api/entries";
import { sessionQuery } from "@/api/session";
import { AppShell } from "@/components/app-shell";
import { CategoriesScreen } from "@/routes/categories";
import { EditEntryScreen } from "@/routes/edit-entry";
import { EntriesScreen } from "@/routes/entries";
import { LoginScreen } from "@/routes/login";
import { MoreScreen } from "@/routes/more";
import { NewEntryScreen } from "@/routes/new-entry";
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
      await context.queryClient.query(sessionQuery(context.api));
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        // eslint-disable-next-line @typescript-eslint/only-throw-error -- TanStack Router redirects by throwing
        throw redirect({ to: "/login", search: { redirect: location.href } });
      }
      throw error;
    }
  },
});

const entriesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/entries",
  component: EntriesScreen,
  loader: ({ context: { queryClient, api } }) =>
    Promise.all([
      queryClient.query(settingsQuery(api)),
      queryClient.query(currenciesQuery(api)),
      queryClient.infiniteQuery(entriesQuery(api)),
    ]),
});

export const editEntryRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/entries/$entryId",
  loader: async ({ context: { queryClient, api }, params: { entryId } }) => {
    try {
      await Promise.all([
        queryClient.query(settingsQuery(api)),
        queryClient.query(currenciesQuery(api)),
        queryClient.query(categoriesQuery(api)),
        queryClient.query(entryQuery(api, entryId)),
      ]);
    } catch (error) {
      // An entry deleted on another device has nothing to edit.
      if (error instanceof ApiError && error.status === 404) {
        // eslint-disable-next-line @typescript-eslint/only-throw-error -- TanStack Router redirects by throwing
        throw redirect({ to: "/entries", replace: true });
      }
      throw error;
    }
  },
  component: function EditEntry(): ReactNode {
    const { entryId }: { entryId: string } = editEntryRoute.useParams();
    return <EditEntryScreen key={entryId} entryId={entryId} />;
  },
});

export interface NewEntrySearch {
  repeat?: string;
}

const newEntryRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/new",
  validateSearch: (search: Record<string, unknown>): NewEntrySearch =>
    typeof search.repeat === "string" ? { repeat: search.repeat } : {},
  loaderDeps: ({ search }) => ({ repeat: search.repeat }),
  loader: async ({ context: { queryClient, api }, deps: { repeat } }) => {
    const [, , , source] = await Promise.all([
      queryClient.query(settingsQuery(api)),
      queryClient.query(currenciesQuery(api)),
      queryClient.query(categoriesQuery(api)),
      repeat === undefined ? undefined : repeatSource(queryClient, api, repeat),
    ]);
    return { source };
  },
  component: function NewEntry(): ReactNode {
    const { source }: { source: Entry | undefined } = newEntryRoute.useLoaderData();
    return <NewEntryScreen key={source?.id ?? "new"} repeat={source} />;
  },
});

// An entry deleted on another device leaves nothing to repeat, so the keypad opens empty.
async function repeatSource(queryClient: QueryClient, api: ApiClient, id: string) {
  try {
    return await queryClient.query(entryQuery(api, id));
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) return undefined;
    throw error;
  }
}

const categoriesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/more/categories",
  component: CategoriesScreen,
  loader: ({ context: { queryClient, api } }) => queryClient.query(allCategoriesQuery(api)),
});

const routeTree = rootRoute.addChildren([
  loginRoute,
  appRoute.addChildren([
    createRoute({ getParentRoute: () => appRoute, path: "/", component: () => <Placeholder title="Dashboard" /> }),
    entriesRoute,
    editEntryRoute,
    newEntryRoute,
    createRoute({ getParentRoute: () => appRoute, path: "/more", component: MoreScreen }),
    categoriesRoute,
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
