import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { toast } from "sonner";

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

// Nor matchMedia, which the toaster reads to follow the system theme.
Object.defineProperty(window, "matchMedia", {
  value: (media: string) => ({
    matches: false,
    media,
    onchange: null,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  }),
});

installIntersectionObserver();

beforeAll(() => {
  server.listen({ onUnhandledFrame: "error" });
});

afterEach(() => {
  cleanup();
  // Toasts live in the toaster's module state, and a new toaster shows every toast still active.
  toast.dismiss();
  server.resetHandlers();
});

afterAll(() => {
  server.close();
});
