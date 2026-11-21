import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { ChevronRight, LogOut, Shapes } from "lucide-react";

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
    <section className="flex flex-col gap-6 md:max-w-2xl">
      <h1 className="text-2xl font-semibold tracking-tight">More</h1>
      <ul className="border-t">
        <li>
          <Link to="/more/categories" className="flex h-12 items-center gap-3 border-b font-medium">
            <Shapes className="text-muted-foreground size-5" aria-hidden />
            Categories
            <ChevronRight className="text-muted-foreground ml-auto size-5" aria-hidden />
          </Link>
        </li>
      </ul>
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
