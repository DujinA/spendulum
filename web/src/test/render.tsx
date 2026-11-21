import { render } from "@testing-library/react";
import { createMemoryHistory } from "@tanstack/react-router";

import { App } from "@/app";
import { createApp } from "@/create-app";

export function renderApp(path: string) {
  const app = createApp({ history: createMemoryHistory({ initialEntries: [path] }) });
  // A failed request shows at once instead of after the retries' backoff.
  app.queryClient.setDefaultOptions({ queries: { retry: false } });
  render(<App app={app} />);
  return app;
}
