import { createContext, use } from "react";

import type { ApiClient } from "@/api/client";

export const ApiContext = createContext<ApiClient | null>(null);

export function useApi() {
  const api = use(ApiContext);
  if (!api) throw new Error("useApi must be used inside <App>");
  return api;
}
