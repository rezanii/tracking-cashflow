"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm } from "react-hook-form";

import { DailyReport } from "@/components/daily-report";
import { AppShell } from "@/components/layout/app-shell";
import { Badge, Button, Card, Field, Input, Select } from "@/components/ui";
import { DataTable, type Column } from "@/components/ui/data-table";
import { StatCard } from "@/components/ui/stat-card";
import { useAsync } from "@/hooks/use-async";
import { firstDayOfCurrentMonth, formatCurrency, formatDate, todayInJakarta, transactionTypeLabels } from "@/lib/format";
import { reportFilterSchema, type ReportFilterInput } from "@/schemas";
import { categoryService } from "@/services/categories";
import { reportService, saveBlob } from "@/services/reports";
import type { CashFlowReport, CashFlowRow, ReportFilters } from "@/types";

type ExportKind = "excel" | "pdf";

// The two reports answer different questions, so they sit side by side rather than in separate
// menus: one is a flat ledger with a running balance, the other is the accounts-based daily
// reconciliation the Telegram bot sends.
type Tab = "cash-flow" | "daily";

export default function ReportsPage() {
  const [tab, setTab] = useState<Tab>("cash-flow");
  const [applied, setApplied] = useState<ReportFilters | null>(null);
  const [report, setReport] = useState<CashFlowReport | null>(null);
  const [generating, setGenerating] = useState(false);
  const [exporting, setExporting] = useState<ExportKind | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  const { data: categories } = useAsync(() => categoryService.listAllActive(), []);

  const form = useForm<ReportFilterInput>({
    resolver: zodResolver(reportFilterSchema),
    defaultValues: {
      date_from: firstDayOfCurrentMonth(),
      date_to: todayInJakarta(),
      category_id: "",
      transaction_type: "",
    },
  });

  function toFilters(values: ReportFilterInput): ReportFilters {
    return {
      date_from: values.date_from,
      date_to: values.date_to,
      category_id: values.category_id || undefined,
      transaction_type: values.transaction_type || undefined,
    };
  }

  async function generate(values: ReportFilterInput) {
    setGenerating(true);
    setMessage(null);
    const filters = toFilters(values);
    try {
      const result = await reportService.cashFlow(filters);
      setReport(result);
      setApplied(filters);
    } catch (failure) {
      setReport(null);
      setMessage(failure instanceof Error ? failure.message : "Gagal membuat laporan");
    } finally {
      setGenerating(false);
    }
  }

  // Exports reuse the filters that produced the preview, so the file always matches what
  // is on screen rather than whatever is currently typed in the form.
  async function exportReport(kind: ExportKind) {
    if (!applied) {
      setMessage("Tampilkan laporan terlebih dahulu sebelum mengekspor");
      return;
    }
    setExporting(kind);
    setMessage(null);
    try {
      const { blob, filename } =
        kind === "excel"
          ? await reportService.cashFlowExcel(applied)
          : await reportService.cashFlowPdf(applied);
      saveBlob(blob, filename);
    } catch (failure) {
      setMessage(failure instanceof Error ? failure.message : "Gagal mengunduh laporan");
    } finally {
      setExporting(null);
    }
  }

  const columns: Column<CashFlowRow>[] = [
    { header: "Tanggal", cell: (row) => formatDate(row.transaction_date) },
    {
      header: "Jenis",
      cell: (row) => (
        <Badge
          tone={row.transaction_type === "INCOME" ? "green" : row.transaction_type === "EXPENSE" ? "red" : "slate"}
        >
          {transactionTypeLabels[row.transaction_type]}
        </Badge>
      ),
      hideOnMobile: true,
    },
    { header: "Kategori", cell: (row) => row.category_name || "-" },
    { header: "Deskripsi", cell: (row) => row.description || "-", hideOnMobile: true },
    {
      header: "Pemasukan",
      align: "right",
      className: "tabular-nums text-emerald-600",
      cell: (row) => (row.income === "0" ? "-" : formatCurrency(row.income)),
    },
    {
      header: "Pengeluaran",
      align: "right",
      className: "tabular-nums text-red-600",
      cell: (row) => (row.expense === "0" ? "-" : formatCurrency(row.expense)),
    },
    {
      header: "Saldo",
      align: "right",
      className: "font-medium tabular-nums",
      cell: (row) => formatCurrency(row.balance),
    },
  ];

  const tabs: { id: Tab; label: string; hint: string }[] = [
    { id: "cash-flow", label: "Arus Kas", hint: "Tabel per transaksi dengan saldo berjalan, bisa diekspor" },
    { id: "daily", label: "Harian", hint: "Rekonsiliasi per akun, sama dengan yang dikirim bot" },
  ];

  return (
    <AppShell>
      <div className="space-y-5">
        <div className="flex flex-wrap gap-2 border-b border-slate-200 pb-1" role="tablist">
          {tabs.map((item) => (
            <button
              key={item.id}
              type="button"
              role="tab"
              aria-selected={tab === item.id}
              onClick={() => setTab(item.id)}
              className={
                tab === item.id
                  ? "rounded-t-lg border-b-2 border-brand-600 px-4 py-2 text-sm font-medium text-brand-700"
                  : "rounded-t-lg border-b-2 border-transparent px-4 py-2 text-sm text-slate-600 hover:text-slate-900"
              }
            >
              {item.label}
            </button>
          ))}
        </div>

        <p className="text-sm text-slate-500">{tabs.find((item) => item.id === tab)?.hint}</p>
      </div>

      {tab === "daily" ? (
        <div className="mt-5">
          <DailyReport />
        </div>
      ) : (
      <div className="mt-5 space-y-5">
        <Card>
          <form className="space-y-4" onSubmit={form.handleSubmit(generate)} noValidate>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <Field
                label="Dari tanggal"
                htmlFor="report-from"
                required
                error={form.formState.errors.date_from?.message}
              >
                <Input
                  id="report-from"
                  type="date"
                  invalid={Boolean(form.formState.errors.date_from)}
                  {...form.register("date_from")}
                />
              </Field>

              <Field
                label="Sampai tanggal"
                htmlFor="report-to"
                required
                error={form.formState.errors.date_to?.message}
              >
                <Input
                  id="report-to"
                  type="date"
                  invalid={Boolean(form.formState.errors.date_to)}
                  {...form.register("date_to")}
                />
              </Field>

              <Field label="Kategori" htmlFor="report-category">
                <Select id="report-category" {...form.register("category_id")}>
                  <option value="">Semua kategori</option>
                  {(categories ?? []).map((category) => (
                    <option key={category.id} value={category.id}>
                      {category.name}
                    </option>
                  ))}
                </Select>
              </Field>

              <Field label="Jenis transaksi" htmlFor="report-type">
                <Select id="report-type" {...form.register("transaction_type")}>
                  <option value="">Semua jenis</option>
                  <option value="INCOME">Pemasukan</option>
                  <option value="EXPENSE">Pengeluaran</option>
                  <option value="TRANSFER">Transfer</option>
                </Select>
              </Field>
            </div>

            <div className="flex flex-col gap-2 sm:flex-row">
              <Button type="submit" loading={generating}>
                Tampilkan Laporan
              </Button>
              <Button
                type="button"
                variant="secondary"
                loading={exporting === "excel"}
                disabled={!applied}
                onClick={() => void exportReport("excel")}
              >
                Ekspor Excel
              </Button>
              <Button
                type="button"
                variant="secondary"
                loading={exporting === "pdf"}
                disabled={!applied}
                onClick={() => void exportReport("pdf")}
              >
                Ekspor PDF
              </Button>
            </div>
          </form>
        </Card>

        {message && (
          <p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
            {message}
          </p>
        )}

        {report && (
          <>
            <p className="text-sm text-slate-500">
              Periode {formatDate(report.period.from)} sampai {formatDate(report.period.to)}
            </p>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <StatCard label="Total Pemasukan" value={report.total_income} tone="income" />
              <StatCard label="Total Pengeluaran" value={report.total_expense} tone="expense" />
              <StatCard label="Arus Kas Bersih" value={report.net_cash_flow} tone="balance" />
            </div>

            <DataTable
              columns={columns}
              rows={report.rows}
              rowKey={(row) => `${row.transaction_date}-${row.reference_number}-${row.balance}`}
              emptyTitle="Tidak ada transaksi pada periode ini"
              emptyDescription="Ubah rentang tanggal atau filter, lalu tampilkan ulang."
            />
          </>
        )}

        {!report && !generating && (
          <Card>
            <p className="py-6 text-center text-sm text-slate-500">
              Pilih rentang tanggal lalu tekan Tampilkan Laporan.
            </p>
          </Card>
        )}
      </div>
      )}
    </AppShell>
  );
}
