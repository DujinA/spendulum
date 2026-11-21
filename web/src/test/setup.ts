import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";

import { server } from "@/test/api";
import { installIntersectionObserver } from "@/test/intersection";

// jsdom does not implement scrolling; the router scrolls to the top on navigation.
window.scrollTo = () => undefined;

// Nor pointer capture, which the Drawer uses for dragging.
Element.prototype.setPointerCapture = () => undefined;
Element.prototype.releasePointerCapture = () => undefined;

// jsdom never fires animationend, so an animated Drawer would never finish closing.
const noAnimation = document.createElement("style");
noAnimation.textContent = "[data-vaul-drawer], [data-vaul-overlay] { animation-name: none !important; }";
document.head.append(noAnimation);

installIntersectionObserver();

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
