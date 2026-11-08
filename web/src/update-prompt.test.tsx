import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { UpdatePrompt } from "@/components/update-prompt";

interface Options {
  onRegisteredSW?: (url: string, registration: ServiceWorkerRegistration | undefined) => void;
}

const sw = vi.hoisted(() => ({
  needRefresh: false,
  options: undefined as Options | undefined,
  updateServiceWorker: vi.fn(),
}));

vi.mock("virtual:pwa-register/react", () => ({
  useRegisterSW: (options: Options) => {
    sw.options = options;
    return {
      needRefresh: [sw.needRefresh, vi.fn()],
      offlineReady: [false, vi.fn()],
      updateServiceWorker: sw.updateServiceWorker,
    };
  },
}));

function setVisibility(state: DocumentVisibilityState) {
  Object.defineProperty(document, "visibilityState", { configurable: true, get: () => state });
}

beforeEach(() => {
  sw.needRefresh = false;
  sw.options = undefined;
  sw.updateServiceWorker.mockReset();
  setVisibility("visible");
});

describe("update prompt", () => {
  it("stays hidden while no new service worker is waiting", () => {
    render(<UpdatePrompt />);

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("appears when a new service worker is waiting, and reloads into it", async () => {
    sw.needRefresh = true;
    render(<UpdatePrompt />);

    expect(screen.getByRole("alert")).toHaveTextContent("A new version of Spendulum is ready.");

    await userEvent.click(screen.getByRole("button", { name: "Reload" }));

    expect(sw.updateServiceWorker).toHaveBeenCalledWith(true);
  });

  it("checks for a new service worker when the app resumes", () => {
    render(<UpdatePrompt />);
    const registration = { update: vi.fn(() => Promise.resolve()) };
    act(() => {
      sw.options?.onRegisteredSW?.("/sw.js", registration as unknown as ServiceWorkerRegistration);
    });

    setVisibility("hidden");
    act(() => {
      document.dispatchEvent(new Event("visibilitychange"));
    });
    expect(registration.update).not.toHaveBeenCalled();

    setVisibility("visible");
    act(() => {
      document.dispatchEvent(new Event("visibilitychange"));
    });
    expect(registration.update).toHaveBeenCalledTimes(1);
  });
});
