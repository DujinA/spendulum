import createClient from "openapi-fetch";

import type { paths } from "@/api/schema.gen";

export type ApiClient = ReturnType<typeof createApiClient>;

export class ApiError extends Error {
  readonly status: number;
  readonly retryAfterSeconds: number | undefined;

  constructor(status: number, detail: string | undefined, retryAfterSeconds?: number) {
    super(detail ?? `The server answered ${String(status)}.`);
    this.status = status;
    this.retryAfterSeconds = retryAfterSeconds;
  }
}

// The server refuses mutating requests without application/json (415), so every request sends it.
export function createApiClient({ onUnauthorized }: { onUnauthorized: () => void }) {
  const client = createClient<paths>({
    baseUrl: window.location.origin,
    headers: { "Content-Type": "application/json" },
  });

  client.use({
    onResponse({ request, response }) {
      const isLogin = request.method === "POST" && new URL(request.url).pathname === "/api/session";
      if (response.status === 401 && !isLogin) onUnauthorized();
    },
  });

  return client;
}

export function apiError(response: Response, error: { detail?: string } | undefined) {
  const retryAfter = Number(response.headers.get("Retry-After"));
  return new ApiError(response.status, error?.detail, retryAfter > 0 ? retryAfter : undefined);
}
