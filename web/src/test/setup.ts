import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";

// jsdom does not implement scrolling; the router scrolls to the top on navigation.
window.scrollTo = () => undefined;

afterEach(() => {
  cleanup();
});
