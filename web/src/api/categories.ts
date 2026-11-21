import { queryOptions } from "@tanstack/react-query";

import { apiError, type ApiClient } from "@/api/client";
import type { components } from "@/api/schema.gen";

export type NewCategory = components["schemas"]["NewCategory"];
export type CategoryChange = components["schemas"]["CategoryChange"];
export type CategoryOrder = components["schemas"]["CategoryOrder"];

// The picker's categoriesQuery shares this prefix, so invalidating it refreshes both.
export const categoriesQueryKey = ["categories"] as const;

export function allCategoriesQuery(api: ApiClient) {
  return queryOptions({
    queryKey: [...categoriesQueryKey, "all"],
    queryFn: async () => {
      const { data, error, response } = await api.GET("/api/categories", { params: { query: { includeArchived: true } } });
      if (!data) throw apiError(response, error);
      return data;
    },
  });
}

export async function createCategory(api: ApiClient, idempotencyKey: string, body: NewCategory) {
  const { data, error, response } = await api.POST("/api/categories", {
    params: { header: { "Idempotency-Key": idempotencyKey } },
    body,
  });
  if (!data) throw apiError(response, error);
  return data;
}

export async function updateCategory(api: ApiClient, id: string, body: CategoryChange) {
  const { data, error, response } = await api.PATCH("/api/categories/{id}", { params: { path: { id } }, body });
  if (!data) throw apiError(response, error);
  return data;
}

export async function deleteCategory(api: ApiClient, id: string) {
  const { error, response } = await api.DELETE("/api/categories/{id}", { params: { path: { id } } });
  if (!response.ok) throw apiError(response, error);
}

export async function reorderCategories(api: ApiClient, body: CategoryOrder) {
  const { error, response } = await api.PUT("/api/categories/order", { body });
  if (!response.ok) throw apiError(response, error);
}
