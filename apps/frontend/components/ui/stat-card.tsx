import clsx from "clsx";

import { formatCurrency } from "@/lib/format";

type StatCardProps = {
  label: string;
  value: string | number;
  kind?: "currency" | "count";
  tone?: "income" | "expense" | "balance" | "neutral";
  hint?: string;
};

const tones = {
  income: "text-emerald-600",
  expense: "text-red-600",
  balance: "text-brand-700",
  neutral: "text-slate-900",
} as const;

const accents = {
  income: "bg-emerald-500",
  expense: "bg-red-500",
  balance: "bg-brand-600",
  neutral: "bg-slate-400",
} as const;

export function StatCard({ label, value, kind = "currency", tone = "neutral", hint }: StatCardProps) {
  const display =
    kind === "currency" ? formatCurrency(value) : `${Number(value).toLocaleString("id-ID")}`;

  return (
    <div className="relative overflow-hidden rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
      <span aria-hidden className={clsx("absolute inset-x-0 top-0 h-1", accents[tone])} />
      <p className="text-xs font-medium uppercase tracking-wide text-slate-500">{label}</p>
      <p className={clsx("mt-2 text-2xl font-semibold tabular-nums", tones[tone])}>{display}</p>
      {hint && <p className="mt-1 text-xs text-slate-500">{hint}</p>}
    </div>
  );
}
