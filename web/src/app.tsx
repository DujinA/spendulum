import { QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";

import { ApiContext } from "@/api/context";
import type { AppInstance } from "@/create-app";

export function App({ app }: { app: AppInstance }) {
  return (
    <ApiContext value={app.api}>
      <QueryClientProvider client={app.queryClient}>
        <RouterProvider router={app.router} />
      </QueryClientProvider>
    </ApiContext>
  );
}
