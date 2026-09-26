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
  account_id: number | null;
  account_name: string;
  to_account_id: number | null;
  to_account_name: string;
  parent_id: number | null;
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
  // Absent when the bot could not be reached; the UI then shows the code instead.
  bot_username?: string;
  deep_link?: string;
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

// --- Daily cash flow report -------------------------------------------------
// Mirrors dto.DailyCashFlowReport. Money is a string so the exact decimal survives JSON.

export type AmountLine = {
  label: string;
  amount: string;
};

export type WalletItem = {
  label: string;
  amount: string;
  sub_items: AmountLine[];
};

export type WalletSection = {
  account_id: number;
  name: string;
  allotment: string;
  items: WalletItem[];
  recorded_total: string;
  expected_remaining: string;
  has_actual_balance: boolean;
  actual_balance: string;
  variance: string;
};

export type BankSection = {
  account_id: number;
  name: string;
  money_in: string;
  fees: string;
  money_out: string;
  computed: string;
  has_actual_balance: boolean;
  actual_balance: string;
  previous_balance: string;
};

export type CreditCardSection = {
  account_id: number;
  name: string;
  payments: string;
  taken_back: string;
  net: string;
};

export type TopUpSection = {
  total: string;
  allocations: AmountLine[];
  allocation_total: string;
  balanced: boolean;
};

export type ReconciliationSection = {
  top_up_total: string;
  parts: AmountLine[];
  parts_total: string;
  difference: string;
  balanced: boolean;
};

export type StatusLine = {
  icon: string;
  label: string;
  amount: string;
};

export type DailyCashFlowReport = {
  date: string;
  period_label: string;
  date_label: string;
  opening_balance_label: string;
  opening_balance: string;
  cash_flow_expenses: AmountLine[];
  cash_flow_total: string;
  credit_cards: CreditCardSection[];
  top_up: TopUpSection;
  wallets: WalletSection[];
  banks: BankSection[];
  reconciliation: ReconciliationSection;
  status: StatusLine[];
};
