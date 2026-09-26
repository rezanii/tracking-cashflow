// Mirrors the API response envelope so every call site handles one shape.
export type ApiEnvelope<T> = {
  success: boolean;
  message: string;
  data?: T;
  errors?: Record<string, string>;
};

export type Pagination = {
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
};

export type Paged<T> = {
  items: T[];
  pagination: Pagination;
};

export type TransactionType = "INCOME" | "EXPENSE" | "TRANSFER";
export type CategoryType = "INCOME" | "EXPENSE";

export type User = {
  id: number;
  name: string;
  email: string;
  is_active: boolean;
  created_at: string;
};

export type LoginResult = {
  access_token: string;
  token_type: string;
  expires_at: string;
  user: User;
};

export type Category = {
  id: number;
  name: string;
  type: CategoryType;
  description: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
};

// Money arrives as a string so the exact decimal survives JSON.
export type Transaction = {
  id: number;
  transaction_date: string;
  transaction_type: TransactionType;
  category_id: number | null;
  category_name: string;
  amount: string;
  description: string;
  reference_number: string;
  created_at: string;
  updated_at: string;
};

export type Period = { from: string; to: string };

export type ExpenseByCategoryRow = {
  category_id: number | null;
  category_name: string;
  total: string;
  percentage: string;
};

export type DashboardSummary = {
  period: Period;
  total_income: string;
  total_expense: string;
  balance: string;
  transaction_count: number;
  income_vs_expense: { label: string; income: string; expense: string }[];
  expense_by_category: ExpenseByCategoryRow[];
  cash_flow_trend: { date: string; income: string; expense: string; balance: string }[];
  recent_transactions: Transaction[];
};

export type CashFlowRow = {
  transaction_date: string;
  transaction_type: TransactionType;
  category_name: string;
  description: string;
  reference_number: string;
  income: string;
  expense: string;
  balance: string;
};

export type CashFlowReport = {
  period: Period;
  total_income: string;
  total_expense: string;
  net_cash_flow: string;
  rows: CashFlowRow[];
};

export type TransactionFilters = {
  search?: string;
  date_from?: string;
  date_to?: string;
  transaction_type?: string;
  category_id?: string;
  page?: number;
  page_size?: number;
  sort_by?: string;
  sort_dir?: "asc" | "desc";
};

export type ReportFilters = {
  date_from: string;
  date_to: string;
  category_id?: string;
  transaction_type?: string;
};

export type DashboardRange = "today" | "week" | "month" | "year" | "custom";

export type TelegramLink = {
  linked: boolean;
  chat_id?: number;
  username?: string;
  chat_title?: string;
  linked_at?: string;
};

export type TelegramPairingCode = {
  code: string;
  expires_at: string;
  instruction: string;
};

export type TelegramSendResult = {
  chat_id: number;
  messages: number;
  characters: number;
};

export type AccountType = "CASH_FLOW" | "WALLET" | "BANK" | "CREDIT_CARD" | "SAVINGS";

export type Account = {
  id: number;
  name: string;
  account_type: AccountType;
  opening_balance: string;
  description: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
};

export type BalanceSnapshot = {
  id: number;
  account_id: number;
  account_name: string;
  as_of_date: string;
  actual_balance: string;
  note: string;
  created_at: string;
  updated_at: string;
};
