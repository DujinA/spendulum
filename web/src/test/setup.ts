import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";

import { server } from "@/test/api";

// jsdom does not implement scrolling; the router scrolls to the top on navigation.
window.scrollTo = () => undefined;

beforeAll(() => {
  server.listen({ onUnhandledFrame: "error" });
});

afterEach(() => {
  cleanup();
  server.resetHandlers();
});

afterAll(() => {
  server.close();
});
