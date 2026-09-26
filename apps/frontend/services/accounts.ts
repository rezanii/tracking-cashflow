import { apiClient, toQuery, unwrap } from "@/lib/api-client";
import type { AccountInput, BalanceSnapshotInput } from "@/schemas";
import type { Account, ApiEnvelope, BalanceSnapshot, Paged } from "@/types";

export type AccountQuery = {
  account_type?: string;
  is_active?: string;
  search?: string;
  page?: number;
  page_size?: number;
  sort_by?: string;
  sort_dir?: string;
};

// The API keeps money as a string so the exact decimal survives; an empty field means zero
// rather than an invalid number.
function toPayload(input: AccountInput) {
  return {
    name: input.name,
    account_type: input.account_type,
    opening_balance: (input.opening_balance ?? "").replace(/,/g, "") || "0",
    description: input.description ?? "",
  };
}

export const accountService = {
  async list(query: AccountQuery = {}): Promise<Paged<Account>> {
    const { data } = await apiClient.get<ApiEnvelope<Paged<Account>>>(`/accounts${toQuery(query)}`);
    return unwrap(data);
  },

  // The pickers need every active account, not one page of them.
  async listAllActive(): Promise<Account[]> {
    const page = await this.list({
      is_active: "true",
      page: 1,
      page_size: 100,
      sort_by: "account_type",
      sort_dir: "asc",
    });
    return page.items;
  },

  async get(id: number): Promise<Account> {
    const { data } = await apiClient.get<ApiEnvelope<Account>>(`/accounts/${id}`);
    return unwrap(data);
  },

  async create(input: AccountInput): Promise<Account> {
    const { data } = await apiClient.post<ApiEnvelope<Account>>("/accounts", toPayload(input));
    return unwrap(data);
  },

  async update(id: number, input: AccountInput): Promise<Account> {
    const { data } = await apiClient.put<ApiEnvelope<Account>>(`/accounts/${id}`, toPayload(input));
    return unwrap(data);
  },

  async setActive(id: number, isActive: boolean): Promise<Account> {
    const { data } = await apiClient.patch<ApiEnvelope<Account>>(`/accounts/${id}/status`, {
      is_active: isActive,
    });
    return unwrap(data);
  },

  async remove(id: number): Promise<void> {
    await apiClient.delete<ApiEnvelope<null>>(`/accounts/${id}`);
  },

  async listBalances(id: number): Promise<BalanceSnapshot[]> {
    const { data } = await apiClient.get<ApiEnvelope<BalanceSnapshot[]>>(`/accounts/${id}/balances`);
    return unwrap(data) ?? [];
  },

  // Recording the same day again overwrites it, so correcting a typo is one more call.
  async recordBalance(id: number, input: BalanceSnapshotInput): Promise<BalanceSnapshot> {
    const { data } = await apiClient.post<ApiEnvelope<BalanceSnapshot>>(`/accounts/${id}/balances`, {
      as_of_date: input.as_of_date,
      actual_balance: input.actual_balance.replace(/,/g, ""),
      note: input.note ?? "",
    });
    return unwrap(data);
  },

  async removeBalance(accountId: number, balanceId: number): Promise<void> {
    await apiClient.delete<ApiEnvelope<null>>(`/accounts/${accountId}/balances/${balanceId}`);
  },
};

// accountTypeLabels keeps the Indonesian wording in one place; the report calls these roles
// rather than instrument names, because the type is what decides the section.
export const accountTypeLabels: Record<string, string> = {
  CASH_FLOW: "Cash Flow",
  WALLET: "Dompet / Jatah",
  BANK: "Bank",
  CREDIT_CARD: "Kartu Kredit",
  SAVINGS: "Tabungan",
};

export const accountTypeHints: Record<string, string> = {
  CASH_FLOW: "Sumber saldo awal dan daftar pengeluaran rumah tangga.",
  WALLET: "Jatah yang di-top-up lalu dibelanjakan, dan direkonsiliasi dengan saldo nyata.",
  BANK: "Mendapat bagian mutasi: dana masuk, biaya, dana keluar.",
  CREDIT_CARD: "Mendapat bagian pembayaran: tagihan dibayar dinetkan dengan dana yang diambil kembali.",
  SAVINGS: "Tujuan top-up yang disisihkan, bukan dibelanjakan.",
};
