import { createRootRoute, createRoute, createRouter, type RouterHistory } from "@tanstack/react-router";

import { AppShell } from "@/components/app-shell";
import { Placeholder } from "@/routes/placeholder";

const rootRoute = createRootRoute({ component: AppShell });

const routeTree = rootRoute.addChildren([
  createRoute({ getParentRoute: () => rootRoute, path: "/", component: () => <Placeholder title="Dashboard" /> }),
  createRoute({ getParentRoute: () => rootRoute, path: "/entries", component: () => <Placeholder title="Entries" /> }),
  createRoute({ getParentRoute: () => rootRoute, path: "/new", component: () => <Placeholder title="Add entry" /> }),
  createRoute({ getParentRoute: () => rootRoute, path: "/more", component: () => <Placeholder title="More" /> }),
]);

export function createAppRouter(options: { history?: RouterHistory } = {}) {
  return createRouter({ routeTree, ...options });
}

declare module "@tanstack/react-router" {
  interface Register {
    router: ReturnType<typeof createAppRouter>;
  }
}
