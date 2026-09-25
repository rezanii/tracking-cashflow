const JAKARTA = "Asia/Jakarta";

const rupiah = new Intl.NumberFormat("id-ID", {
  style: "currency",
  currency: "IDR",
  minimumFractionDigits: 0,
  maximumFractionDigits: 0,
});

const rupiahWithCents = new Intl.NumberFormat("id-ID", {
  style: "currency",
  currency: "IDR",
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

// Amounts arrive as decimal strings. Number() is only used for display, never for
// arithmetic: every calculation happens in the API where the type is exact.
export function formatCurrency(amount: string | number, withCents = false): string {
  const value = typeof amount === "number" ? amount : Number(amount);
  if (!Number.isFinite(value)) return "Rp 0";
  return (withCents ? rupiahWithCents : rupiah).format(value);
}

export function toNumber(amount: string | number): number {
  const value = typeof amount === "number" ? amount : Number(amount);
  return Number.isFinite(value) ? value : 0;
}

// formatDate renders an API date (YYYY-MM-DD) as DD/MM/YYYY without touching timezones,
// because a date column has no time component to shift.
export function formatDate(isoDate: string): string {
  const [year, month, day] = isoDate.split("T")[0]?.split("-") ?? [];
  if (!year || !month || !day) return isoDate;
  return `${day}/${month}/${year}`;
}

export function formatDateTime(isoTimestamp: string): string {
  const parsed = new Date(isoTimestamp);
  if (Number.isNaN(parsed.getTime())) return isoTimestamp;
  return new Intl.DateTimeFormat("id-ID", {
    dateStyle: "short",
    timeStyle: "short",
    timeZone: JAKARTA,
  }).format(parsed);
}

export function formatMonthLabel(label: string): string {
  const [year, month] = label.split("-");
  if (!year || !month) return label;
  const names = ["Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"];
  return `${names[Number(month) - 1] ?? month} ${year}`;
}

// todayInJakarta returns YYYY-MM-DD for the user's day, not the server's.
export function todayInJakarta(): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: JAKARTA,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date());
  return parts;
}

export function firstDayOfCurrentMonth(): string {
  const today = todayInJakarta();
  return `${today.slice(0, 7)}-01`;
}

export const transactionTypeLabels: Record<string, string> = {
  INCOME: "Pemasukan",
  EXPENSE: "Pengeluaran",
  TRANSFER: "Transfer",
};
