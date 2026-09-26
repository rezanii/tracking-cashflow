import { apiClient, toQuery, unwrap } from "@/lib/api-client";
import type { TransactionInput } from "@/schemas";
import type { ApiEnvelope, Paged, Transaction, TransactionFilters } from "@/types";

// toPayload normalises the form values into the shape the API validates: a transfer never
// carries a category, only a transfer carries a destination account, and the amount stays a
// string so the decimal is exact.
function toPayload(input: TransactionInput) {
  const isTransfer = input.transaction_type === "TRANSFER";
  return {
    transaction_date: input.transaction_date,
    transaction_type: input.transaction_type,
    category_id: isTransfer || !input.category_id ? null : Number(input.category_id),
    amount: input.amount.replace(/,/g, ""),
    description: input.description ?? "",
    reference_number: input.reference_number ?? "",
    account_id: input.account_id ? Number(input.account_id) : null,
    to_account_id: isTransfer && input.to_account_id ? Number(input.to_account_id) : null,
    parent_id: input.parent_id ? Number(input.parent_id) : null,
  };
}

export const transactionService = {
  async list(filters: TransactionFilters = {}): Promise<Paged<Transaction>> {
    const { data } = await apiClient.get<ApiEnvelope<Paged<Transaction>>>(
      `/transactions${toQuery(filters)}`,
    );
    return unwrap(data);
  },

  async get(id: number): Promise<Transaction> {
    const { data } = await apiClient.get<ApiEnvelope<Transaction>>(`/transactions/${id}`);
    return unwrap(data);
  },

  async create(input: TransactionInput): Promise<Transaction> {
    const { data } = await apiClient.post<ApiEnvelope<Transaction>>("/transactions", toPayload(input));
    return unwrap(data);
  },

  async update(id: number, input: TransactionInput): Promise<Transaction> {
    const { data } = await apiClient.put<ApiEnvelope<Transaction>>(`/transactions/${id}`, toPayload(input));
    return unwrap(data);
  },

  async remove(id: number): Promise<void> {
    await apiClient.delete<ApiEnvelope<null>>(`/transactions/${id}`);
  },
};
