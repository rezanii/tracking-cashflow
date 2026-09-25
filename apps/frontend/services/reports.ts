import { apiClient, toQuery, unwrap } from "@/lib/api-client";
import { authStorage } from "@/lib/auth-storage";
import type {
  ApiEnvelope,
  CashFlowReport,
  DashboardRange,
  DashboardSummary,
  ExpenseByCategoryRow,
  Period,
  ReportFilters,
} from "@/types";

type SummaryResponse = {
  period: Period;
  total_income: string;
  total_expense: string;
  net_cash_flow: string;
};

type ExpenseByCategoryResponse = {
  period: Period;
  total: string;
  rows: ExpenseByCategoryRow[];
};

type MonthlyResponse = {
  period: Period;
  rows: { month: string; income: string; expense: string; net_cash_flow: string }[];
};

// Downloads go through fetch rather than axios so the response stays a Blob and the
// browser can save it without the JSON interceptors touching it.
async function download(path: string, filters: ReportFilters): Promise<{ blob: Blob; filename: string }> {
  const baseURL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api/v1";
  const token = authStorage.getToken();

  const response = await fetch(`${baseURL}${path}${toQuery({ ...filters })}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });

  if (!response.ok) {
    // The error body is JSON even though the success body is binary.
    let message = "Gagal mengunduh laporan";
    try {
      const body = (await response.json()) as ApiEnvelope<unknown>;
      message = body.message || message;
    } catch {
      // A non-JSON error body leaves the default message in place.
    }
    throw new Error(message);
  }

  const disposition = response.headers.get("Content-Disposition") ?? "";
  const match = /filename="?([^"]+)"?/.exec(disposition);
  const fallback = path.endsWith("pdf") ? "cash-flow.pdf" : "cash-flow.xlsx";

  return { blob: await response.blob(), filename: match?.[1] ?? fallback };
}

export const reportService = {
  async dashboard(range: DashboardRange, custom?: { date_from: string; date_to: string }): Promise<DashboardSummary> {
    const query = range === "custom" && custom ? { range, ...custom } : { range };
    const { data } = await apiClient.get<ApiEnvelope<DashboardSummary>>(
      `/dashboard/summary${toQuery(query)}`,
    );
    return unwrap(data);
  },

  async summary(filters: ReportFilters): Promise<SummaryResponse> {
    const { data } = await apiClient.get<ApiEnvelope<SummaryResponse>>(
      `/reports/summary${toQuery({ ...filters })}`,
    );
    return unwrap(data);
  },

  async cashFlow(filters: ReportFilters): Promise<CashFlowReport> {
    const { data } = await apiClient.get<ApiEnvelope<CashFlowReport>>(
      `/reports/cash-flow${toQuery({ ...filters })}`,
    );
    return unwrap(data);
  },

  async expenseByCategory(filters: ReportFilters): Promise<ExpenseByCategoryResponse> {
    const { data } = await apiClient.get<ApiEnvelope<ExpenseByCategoryResponse>>(
      `/reports/expense-by-category${toQuery({ ...filters })}`,
    );
    return unwrap(data);
  },

  async monthly(filters: ReportFilters): Promise<MonthlyResponse> {
    const { data } = await apiClient.get<ApiEnvelope<MonthlyResponse>>(
      `/reports/monthly${toQuery({ ...filters })}`,
    );
    return unwrap(data);
  },

  cashFlowExcel(filters: ReportFilters) {
    return download("/reports/cash-flow/excel", filters);
  },

  cashFlowPdf(filters: ReportFilters) {
    return download("/reports/cash-flow/pdf", filters);
  },
};

// saveBlob triggers the browser download and releases the object URL afterwards.
export function saveBlob(blob: Blob, filename: string): void {
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.URL.revokeObjectURL(url);
}
