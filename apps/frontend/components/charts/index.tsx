"use client";

import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { formatCurrency, formatDate, formatMonthLabel, toNumber } from "@/lib/format";
import type { DashboardSummary, ExpenseByCategoryRow } from "@/types";

// A fixed, ordered palette keeps a category the same colour across renders.
const categoryColors = [
  "#2563eb",
  "#0d9488",
  "#f59e0b",
  "#db2777",
  "#7c3aed",
  "#dc2626",
  "#0891b2",
  "#65a30d",
  "#c2410c",
  "#4f46e5",
];

const axisStyle = { fontSize: 12, fill: "#64748b" };

// Amounts are strings on the wire. They are converted only here, for pixel positions.
function compactRupiah(value: number): string {
  if (Math.abs(value) >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(1)} M`;
  if (Math.abs(value) >= 1_000_000) return `${(value / 1_000_000).toFixed(1)} jt`;
  if (Math.abs(value) >= 1_000) return `${(value / 1_000).toFixed(0)} rb`;
  return String(value);
}

function ChartFrame({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
      <h3 className="text-sm font-semibold text-slate-900">{title}</h3>
      <div className="mt-4 h-72 w-full">{children}</div>
    </div>
  );
}

function NoData() {
  return (
    <div className="flex h-full items-center justify-center text-sm text-slate-500">
      Belum ada data pada periode ini
    </div>
  );
}

export function IncomeVsExpenseChart({ data }: { data: DashboardSummary["income_vs_expense"] }) {
  const series = data.map((point) => ({
    label: formatMonthLabel(point.label),
    Pemasukan: toNumber(point.income),
    Pengeluaran: toNumber(point.expense),
  }));

  return (
    <ChartFrame title="Pemasukan vs Pengeluaran">
      {series.length === 0 ? (
        <NoData />
      ) : (
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={series} margin={{ top: 8, right: 8, left: 8, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
            <XAxis dataKey="label" tick={axisStyle} axisLine={false} tickLine={false} />
            <YAxis tick={axisStyle} axisLine={false} tickLine={false} tickFormatter={compactRupiah} width={64} />
            <Tooltip formatter={(value: number) => formatCurrency(value)} />
            <Legend wrapperStyle={{ fontSize: 12 }} />
            <Bar dataKey="Pemasukan" fill="#10b981" radius={[4, 4, 0, 0]} />
            <Bar dataKey="Pengeluaran" fill="#ef4444" radius={[4, 4, 0, 0]} />
          </BarChart>
        </ResponsiveContainer>
      )}
    </ChartFrame>
  );
}

export function ExpenseByCategoryChart({ data }: { data: ExpenseByCategoryRow[] }) {
  const series = data.map((row) => ({
    name: row.category_name,
    value: toNumber(row.total),
    percentage: row.percentage,
  }));

  return (
    <ChartFrame title="Pengeluaran per Kategori">
      {series.length === 0 ? (
        <NoData />
      ) : (
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie data={series} dataKey="value" nameKey="name" innerRadius={55} outerRadius={95} paddingAngle={2}>
              {series.map((entry, index) => (
                <Cell key={entry.name} fill={categoryColors[index % categoryColors.length]} />
              ))}
            </Pie>
            <Tooltip formatter={(value: number, name: string) => [formatCurrency(value), name]} />
            <Legend wrapperStyle={{ fontSize: 12 }} />
          </PieChart>
        </ResponsiveContainer>
      )}
    </ChartFrame>
  );
}

export function CashFlowTrendChart({ data }: { data: DashboardSummary["cash_flow_trend"] }) {
  const series = data.map((point) => ({
    label: formatDate(point.date),
    Saldo: toNumber(point.balance),
  }));

  return (
    <ChartFrame title="Tren Arus Kas">
      {series.length === 0 ? (
        <NoData />
      ) : (
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={series} margin={{ top: 8, right: 8, left: 8, bottom: 0 }}>
            <defs>
              <linearGradient id="balanceGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#2563eb" stopOpacity={0.35} />
                <stop offset="100%" stopColor="#2563eb" stopOpacity={0.02} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
            <XAxis dataKey="label" tick={axisStyle} axisLine={false} tickLine={false} />
            <YAxis tick={axisStyle} axisLine={false} tickLine={false} tickFormatter={compactRupiah} width={64} />
            <Tooltip formatter={(value: number) => formatCurrency(value)} />
            <Area
              type="monotone"
              dataKey="Saldo"
              stroke="#2563eb"
              strokeWidth={2}
              fill="url(#balanceGradient)"
            />
          </AreaChart>
        </ResponsiveContainer>
      )}
    </ChartFrame>
  );
}
