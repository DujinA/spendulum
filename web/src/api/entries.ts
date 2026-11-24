import { infiniteQueryOptions, queryOptions, type QueryClient } from "@tanstack/react-query";

import { apiError, type ApiClient } from "@/api/client";
import type { components } from "@/api/schema.gen";
import { vendorsQueryKey } from "@/api/vendors";

export type Entry = components["schemas"]["Entry"];
export type NewEntry = components["schemas"]["NewEntry"];
export type EntryChange = components["schemas"]["EntryChange"];
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

// A change to an entry also changes its vendor's use count and latest category.
export function invalidateEntries(queryClient: QueryClient) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: entriesQueryKey }),
    queryClient.invalidateQueries({ queryKey: vendorsQueryKey }),
  ]);
}

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

// Its own key root, so resetting the list never refetches an entry that is open while it is deleted.
export function entryQuery(api: ApiClient, id: string) {
  return queryOptions({
    queryKey: ["entry", id],
    queryFn: async () => {
      const { data, error, response } = await api.GET("/api/entries/{id}", { params: { path: { id } } });
      if (!data) throw apiError(response, error);
      return data;
    },
  });
}

export async function updateEntry(api: ApiClient, id: string, body: EntryChange) {
  const { data, error, response } = await api.PUT("/api/entries/{id}", { params: { path: { id } }, body });
  if (!data) throw apiError(response, error);
  return data;
}

export async function deleteEntry(api: ApiClient, id: string) {
  const { error, response } = await api.DELETE("/api/entries/{id}", { params: { path: { id } } });
  if (!response.ok) throw apiError(response, error);
}

export async function restoreEntry(api: ApiClient, id: string) {
  const { data, error, response } = await api.POST("/api/entries/{id}/restore", { params: { path: { id } } });
  if (!data) throw apiError(response, error);
  return data;
}
