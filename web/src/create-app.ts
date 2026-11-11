import { QueryClient } from "@tanstack/react-query";
import type { RouterHistory } from "@tanstack/react-router";

import { createApiClient } from "@/api/client";
import { createAppRouter } from "@/router";

export type AppInstance = ReturnType<typeof createApp>;

// The installed web app has no reload button, so a 401 anywhere must lead to the login screen by itself.
export function createApp(options: { history?: RouterHistory } = {}) {
  const queryClient = new QueryClient();
  const api = createApiClient({
    onUnauthorized: () => {
      const { pathname, href } = router.state.location;
      if (pathname === "/login") return;
      queryClient.clear();
      void router.navigate({ to: "/login", search: { redirect: href }, replace: true });
    },
  });
  const router = createAppRouter({ context: { queryClient, api }, ...options });

  return { queryClient, api, router };
}
