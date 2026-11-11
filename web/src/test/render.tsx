import { render } from "@testing-library/react";
import { createMemoryHistory } from "@tanstack/react-router";

import { App } from "@/app";
import { createApp } from "@/create-app";

export function renderApp(path: string) {
  const app = createApp({ history: createMemoryHistory({ initialEntries: [path] }) });
  render(<App app={app} />);
  return app;
}
