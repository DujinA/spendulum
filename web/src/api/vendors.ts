import { queryOptions } from "@tanstack/react-query";

import { apiError, type ApiClient } from "@/api/client";
import type { Kind } from "@/api/entries";
import type { components } from "@/api/schema.gen";

export type Vendor = components["schemas"]["Vendor"];

export const vendorsQueryKey = ["vendors"] as const;

// The server matches the name by its key and finds the near-matches, so every typed name is its own query.
export function vendorsQuery(api: ApiClient, kind: Kind, name: string) {
  return queryOptions({
    queryKey: [...vendorsQueryKey, kind, name],
    queryFn: async () => {
      const { data, error, response } = await api.GET("/api/vendors", {
        params: { query: name ? { kind, name } : { kind } },
      });
      if (!data) throw apiError(response, error);
      return data;
    },
  });
}

// On income the vendor is where the money came from.
export function vendorLabelFor(kind: Kind) {
  return kind === "income" ? "Source" : "Vendor";
}
