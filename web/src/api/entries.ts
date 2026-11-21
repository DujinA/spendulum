import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import { apiError, type ApiClient } from "@/api/client";
import type { components } from "@/api/schema.gen";

export type Entry = components["schemas"]["Entry"];
export type NewEntry = components["schemas"]["NewEntry"];
export type Kind = components["schemas"]["Kind"];
export type Category = components["schemas"]["Category"];

export function settingsQuery(api: ApiClient) {
  return queryOptions({
    queryKey: ["settings"],
    queryFn: async () => {
      const { data, error, response } = await api.GET("/api/settings");
      if (!data) throw apiError(response, error);
      return data;
    },
    staleTime: 5 * 60_000,
  });
}

export function currenciesQuery(api: ApiClient) {
  return queryOptions({
    queryKey: ["currencies"],
    queryFn: async () => {
      const { data, error, response } = await api.GET("/api/currencies");
      if (!data) throw apiError(response, error);
      return data;
    },
    staleTime: 60 * 60_000,
  });
}

export function categoriesQuery(api: ApiClient) {
  return queryOptions({
    queryKey: ["categories"],
    queryFn: async () => {
      const { data, error, response } = await api.GET("/api/categories");
      if (!data) throw apiError(response, error);
      return data;
    },
  });
}

export const entriesQueryKey = ["entries"] as const;

export function entriesQuery(api: ApiClient) {
  return infiniteQueryOptions({
    queryKey: entriesQueryKey,
    queryFn: async ({ pageParam }) => {
      const { data, error, response } = await api.GET("/api/entries", {
        params: { query: pageParam === undefined ? {} : { cursor: pageParam } },
      });
      if (!data) throw apiError(response, error);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.nextCursor,
  });
}

export async function createEntry(api: ApiClient, idempotencyKey: string, body: NewEntry) {
  const { data, error, response } = await api.POST("/api/entries", {
    params: { header: { "Idempotency-Key": idempotencyKey } },
    body,
  });
  if (!data) throw apiError(response, error);
  return data;
}
