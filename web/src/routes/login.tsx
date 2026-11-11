import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { ApiError } from "@/api/client";
import { useApi } from "@/api/context";
import { logIn, safeRedirect } from "@/api/session";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

function errorMessage(error: Error) {
  if (error instanceof ApiError && error.status === 401) return "Email or password is incorrect.";
  if (error instanceof ApiError && error.status === 429) {
    const minutes = Math.max(1, Math.ceil((error.retryAfterSeconds ?? 60) / 60));
    return `Too many failed logins. Try again in ${String(minutes)} ${minutes === 1 ? "minute" : "minutes"}.`;
  }
  return "Could not log in. Check your connection and try again.";
}

export function LoginScreen({ redirect }: { redirect: string | undefined }) {
  const api = useApi();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  const login = useMutation({
    mutationFn: () => logIn(api, email, password),
    onSuccess: async () => {
      queryClient.clear();
      await navigate({ href: safeRedirect(redirect), replace: true });
    },
  });

  return (
    <main className="flex min-h-dvh items-center justify-center px-[max(1rem,env(safe-area-inset-left))] pt-[env(safe-area-inset-top)] pb-[env(safe-area-inset-bottom)]">
      <form
        onSubmit={(event) => {
          event.preventDefault();
          login.mutate();
        }}
        className="flex w-full max-w-sm flex-col gap-5">
        <div className="flex flex-col gap-1">
          <span className="text-muted-foreground text-sm font-medium">Spendulum</span>
          <h1 className="text-2xl font-semibold tracking-tight">Log in</h1>
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="email">Email</Label>
          <Input
            id="email"
            type="email"
            autoComplete="username"
            autoCapitalize="none"
            required
            className="h-11"
            value={email}
            onChange={(e) => {
              setEmail(e.target.value);
            }}
          />
        </div>
        <div className="flex flex-col gap-2">
          <Label htmlFor="password">Password</Label>
          <Input
            id="password"
            type="password"
            autoComplete="current-password"
            required
            className="h-11"
            value={password}
            onChange={(e) => {
              setPassword(e.target.value);
            }}
          />
        </div>
        {login.error && (
          <p role="alert" className="text-destructive text-sm">
            {errorMessage(login.error)}
          </p>
        )}
        <Button type="submit" className="h-11" disabled={login.isPending}>
          Log in
        </Button>
      </form>
    </main>
  );
}
