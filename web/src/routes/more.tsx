import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { LogOut } from "lucide-react";

import { useApi } from "@/api/context";
import { logOut, sessionQuery } from "@/api/session";
import { Button } from "@/components/ui/button";

export function MoreScreen() {
  const api = useApi();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { data: session } = useQuery(sessionQuery(api));

  const logout = useMutation({
    mutationFn: () => logOut(api),
    onSuccess: async () => {
      queryClient.clear();
      await navigate({ to: "/login", replace: true });
    },
  });

  return (
    <section className="flex flex-col gap-6">
      <h1 className="text-2xl font-semibold tracking-tight">More</h1>
      <div className="flex flex-col gap-3">
        {session && <p className="text-muted-foreground text-sm">Logged in as {session.email}</p>}
        <Button
          variant="outline"
          className="h-11 justify-start self-start"
          disabled={logout.isPending}
          onClick={() => {
            logout.mutate();
          }}
        >
          <LogOut aria-hidden />
          Log out
        </Button>
        {logout.isError && (
          <p role="alert" className="text-destructive text-sm">
            Could not log out. Check your connection and try again.
          </p>
        )}
      </div>
    </section>
  );
}
