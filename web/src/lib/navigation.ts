import { useCanGoBack, useNavigate, useRouter } from "@tanstack/react-router";

// Back keeps the list where it was scrolled; a screen opened from a link has no history and goes to the list.
export function useBackToEntries() {
  const router = useRouter();
  const navigate = useNavigate();
  const canGoBack = useCanGoBack();

  return () => {
    if (canGoBack) router.history.back();
    else void navigate({ to: "/entries", replace: true });
  };
}
