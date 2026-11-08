import { useEffect, useState } from "react";
import { useRegisterSW } from "virtual:pwa-register/react";

import { Button } from "@/components/ui/button";

const hourly = 60 * 60 * 1000;

// The installed web app on iOS is resumed, not reloaded, so it checks for a new
// service worker on every resume and hourly, and offers a reload when one is waiting.
export function UpdatePrompt() {
  const [registration, setRegistration] = useState<ServiceWorkerRegistration>();
  const {
    needRefresh: [needRefresh],
    updateServiceWorker,
  } = useRegisterSW({
    onRegisteredSW(_url, r) {
      setRegistration(r);
    },
  });

  useEffect(() => {
    if (!registration) return;

    // A failed check (offline, for example) is retried on the next resume.
    const check = () => void registration.update().catch(() => undefined);
    const onVisibilityChange = () => {
      if (document.visibilityState === "visible") check();
    };

    document.addEventListener("visibilitychange", onVisibilityChange);
    const interval = setInterval(check, hourly);

    return () => {
      document.removeEventListener("visibilitychange", onVisibilityChange);
      clearInterval(interval);
    };
  }, [registration]);

  if (!needRefresh) return null;

  return (
    <div
      role="alert"
      className="bg-popover text-popover-foreground fixed inset-x-4 top-[calc(env(safe-area-inset-top)+0.75rem)] z-50 mx-auto flex max-w-md items-center justify-between gap-3 rounded-lg border p-3 shadow-lg"
    >
      <span className="text-sm">A new version of Spendulum is ready.</span>
      <Button className="h-11" onClick={() => void updateServiceWorker(true)}>
        Reload
      </Button>
    </div>
  );
}
