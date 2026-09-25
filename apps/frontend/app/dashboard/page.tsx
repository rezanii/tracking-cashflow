"use client";

import Link from "next/link";
import { useState } from "react";

import { CashFlowTrendChart, ExpenseByCategoryChart, IncomeVsExpenseChart } from "@/components/charts";
import { AppShell } from "@/components/layout/app-shell";
import { Badge, Button, Card, ErrorState, Field, Input, Loading } from "@/components/ui";
import { DataTable, type Column } from "@/components/ui/data-table";
import { StatCard } from "@/components/ui/stat-card";
import { useAsync } from "@/hooks/use-async";
import { firstDayOfCurrentMonth, formatCurrency, formatDate, todayInJakarta, transactionTypeLabels } from "@/lib/format";
import { reportService } from "@/services/reports";
import type { DashboardRange, Transaction } from "@/types";

const ranges: { value: DashboardRange; label: string }[] = [
  { value: "today", label: "Hari ini" },
  { value: "week", label: "Minggu ini" },
  { value: "month", label: "Bulan ini" },
  { value: "year", label: "Tahun ini" },
  { value: "custom", label: "Kustom" },
];

export default function DashboardPage() {
  const [range, setRange] = useState<DashboardRange>("month");
  const [dateFrom, setDateFrom] = useState(firstDayOfCurrentMonth());
  const [dateTo, setDateTo] = useState(todayInJakarta());

  // A custom range only loads once both ends are set, so the API is never asked for a
  // half-specified period.
  const customReady = range !== "custom" || (dateFrom !== "" && dateTo !== "" && dateTo >= dateFrom);

  const { data, loading, error, reload } = useAsync(
    () => reportService.dashboard(range, { date_from: dateFrom, date_to: dateTo }),
    [range, range === "custom" ? dateFrom : "", range === "custom" ? dateTo : ""],
  );

  const recentColumns: Column<Transaction>[] = [
    { header: "Tanggal", cell: (row) => formatDate(row.transaction_date) },
    {
      header: "Jenis",
      cell: (row) => (
        <Badge tone={row.transaction_type === "INCOME" ? "green" : row.transaction_type === "EXPENSE" ? "red" : "slate"}>
          {transactionTypeLabels[row.transaction_type]}
        </Badge>
      ),
    },
    { header: "Kategori", cell: (row) => row.category_name || "-", hideOnMobile: true },
    { header: "Deskripsi", cell: (row) => row.description || "-", hideOnMobile: true },
    {
      header: "Nominal",
      align: "right",
      className: "font-medium tabular-nums",
      cell: (row) => (
        <span className={row.transaction_type === "EXPENSE" ? "text-red-600" : row.transaction_type === "INCOME" ? "text-emerald-600" : "text-slate-600"}>
          {row.transaction_type === "EXPENSE" ? "-" : row.transaction_type === "INCOME" ? "+" : ""}
          {formatCurrency(row.amount)}
        </span>
      ),
    },
  ];

  return (
    <AppShell>
      <div className="space-y-6">
        <Card>
          <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div className="flex flex-wrap gap-2">
              {ranges.map((item) => (
                <Button
                  key={item.value}
                  size="sm"
                  variant={range === item.value ? "primary" : "secondary"}
                  onClick={() => setRange(item.value)}
                >
                  {item.label}
                </Button>
              ))}
            </div>

            {range === "custom" && (
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <Field label="Dari tanggal" htmlFor="dash-from">
                  <Input id="dash-from" type="date" value={dateFrom} onChange={(event) => setDateFrom(event.target.value)} />
                </Field>
                <Field
                  label="Sampai tanggal"
                  htmlFor="dash-to"
                  error={dateTo && dateTo < dateFrom ? "Tidak boleh lebih awal dari tanggal mulai" : undefined}
                >
                  <Input id="dash-to" type="date" value={dateTo} onChange={(event) => setDateTo(event.target.value)} />
                </Field>
              </div>
            )}
          </div>
        </Card>

        {loading && <Loading />}
        {!loading && error && <ErrorState message={error} onRetry={reload} />}

        {!loading && !error && data && customReady && (
          <>
            <p className="text-sm text-slate-500">
              Periode {formatDate(data.period.from)} sampai {formatDate(data.period.to)}
            </p>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
              <StatCard label="Total Pemasukan" value={data.total_income} tone="income" />
              <StatCard label="Total Pengeluaran" value={data.total_expense} tone="expense" />
              <StatCard label="Saldo" value={data.balance} tone="balance" hint="Pemasukan dikurangi pengeluaran" />
              <StatCard
                label="Jumlah Transaksi"
                value={data.transaction_count}
                kind="count"
                tone="neutral"
                hint="Termasuk transfer"
              />
            </div>

            <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
              <IncomeVsExpenseChart data={data.income_vs_expense} />
              <ExpenseByCategoryChart data={data.expense_by_category} />
            </div>

            <CashFlowTrendChart data={data.cash_flow_trend} />

            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <h2 className="text-sm font-semibold text-slate-900">Transaksi Terakhir</h2>
                <Link href="/transactions" className="text-sm font-medium text-brand-600 hover:text-brand-700">
                  Lihat semua
                </Link>
              </div>
              <DataTable
                columns={recentColumns}
                rows={data.recent_transactions}
                rowKey={(row) => row.id}
                emptyTitle="Belum ada transaksi"
                emptyDescription="Tambahkan transaksi pertama Anda untuk melihat ringkasan di sini."
                emptyAction={
                  <Link href="/transactions/create">
                    <Button size="sm">Tambah Transaksi</Button>
                  </Link>
                }
              />
            </div>
          </>
        )}
      </div>
    </AppShell>
  );
}
