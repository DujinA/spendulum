import { queryOptions } from "@tanstack/react-query";

import { apiError, type ApiClient } from "@/api/client";

export const sessionQueryKey = ["session"] as const;

export function sessionQuery(api: ApiClient) {
  return queryOptions({
    queryKey: sessionQueryKey,
    queryFn: async () => {
      const { data, error, response } = await api.GET("/api/session");
      if (!data) throw apiError(response, error);
      return data;
    },
    retry: false,
    staleTime: 60_000,
  });
}

export async function logIn(api: ApiClient, email: string, password: string) {
  const { error, response } = await api.POST("/api/session", { body: { email, password } });
  if (!response.ok) throw apiError(response, error);
}

export async function logOut(api: ApiClient) {
  const { error, response } = await api.DELETE("/api/session");
  if (!response.ok && response.status !== 401) throw apiError(response, error);
}

// Only a path on this site: "//evil.example" or a full URL would send the user elsewhere.
export function safeRedirect(target: string | undefined) {
  if (!target?.startsWith("/") || target.startsWith("//") || target.startsWith("/\\") || target.startsWith("/login")) {
    return "/";
  }
  return target;
}
