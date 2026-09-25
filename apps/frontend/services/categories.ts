import { apiClient, toQuery, unwrap } from "@/lib/api-client";
import type { CategoryInput } from "@/schemas";
import type { ApiEnvelope, Category, Paged } from "@/types";

export type CategoryQuery = {
  type?: string;
  is_active?: string;
  search?: string;
  page?: number;
  page_size?: number;
  sort_by?: string;
  sort_dir?: string;
};

export const categoryService = {
  async list(query: CategoryQuery = {}): Promise<Paged<Category>> {
    const { data } = await apiClient.get<ApiEnvelope<Paged<Category>>>(
      `/categories${toQuery(query)}`,
    );
    return unwrap(data);
  },

  // The form pickers need every active category, not one page of them.
  async listAllActive(type?: string): Promise<Category[]> {
    const page = await this.list({ type, is_active: "true", page: 1, page_size: 100, sort_by: "name", sort_dir: "asc" });
    return page.items;
  },

  async get(id: number): Promise<Category> {
    const { data } = await apiClient.get<ApiEnvelope<Category>>(`/categories/${id}`);
    return unwrap(data);
  },

  async create(input: CategoryInput): Promise<Category> {
    const { data } = await apiClient.post<ApiEnvelope<Category>>("/categories", {
      name: input.name,
      type: input.type,
      description: input.description ?? "",
    });
    return unwrap(data);
  },

  async update(id: number, input: CategoryInput): Promise<Category> {
    const { data } = await apiClient.put<ApiEnvelope<Category>>(`/categories/${id}`, {
      name: input.name,
      type: input.type,
      description: input.description ?? "",
    });
    return unwrap(data);
  },

  async setActive(id: number, isActive: boolean): Promise<Category> {
    const { data } = await apiClient.patch<ApiEnvelope<Category>>(`/categories/${id}/status`, {
      is_active: isActive,
    });
    return unwrap(data);
  },

  async remove(id: number): Promise<void> {
    await apiClient.delete<ApiEnvelope<null>>(`/categories/${id}`);
  },
};
