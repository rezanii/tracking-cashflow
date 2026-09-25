"use client";

import Link from "next/link";
import { useParams } from "next/navigation";

import { AppShell } from "@/components/layout/app-shell";
import { TransactionForm } from "@/components/transaction-form";
import { Badge, Button, Card, ErrorState, Loading } from "@/components/ui";
import { useAsync } from "@/hooks/use-async";
import { formatCurrency, formatDate, formatDateTime, transactionTypeLabels } from "@/lib/format";
import { transactionService } from "@/services/transactions";

export default function TransactionDetailPage() {
  const params = useParams<{ id: string }>();
  const id = Number(params.id);

  const { data, loading, error, reload } = useAsync(() => transactionService.get(id), [id]);

  if (!Number.isFinite(id) || id <= 0) {
    return (
      <AppShell>
        <ErrorState message="Nomor transaksi tidak valid" />
      </AppShell>
    );
  }

  return (
    <AppShell>
      <div className="space-y-5">
        <div className="flex items-center justify-between">
          <p className="text-sm text-slate-500">Detail dan ubah transaksi</p>
          <Link href="/transactions">
            <Button variant="secondary" size="sm">
              Kembali
            </Button>
          </Link>
        </div>

        {loading && <Loading />}
        {!loading && error && <ErrorState message={error} onRetry={reload} />}

        {!loading && !error && data && (
          <>
            <Card>
              <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
                <div>
                  <dt className="text-xs uppercase tracking-wide text-slate-500">Tanggal</dt>
                  <dd className="mt-1 text-sm font-medium text-slate-900">
                    {formatDate(data.transaction_date)}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs uppercase tracking-wide text-slate-500">Jenis</dt>
                  <dd className="mt-1">
                    <Badge
                      tone={
                        data.transaction_type === "INCOME"
                          ? "green"
                          : data.transaction_type === "EXPENSE"
                            ? "red"
                            : "slate"
                      }
                    >
                      {transactionTypeLabels[data.transaction_type]}
                    </Badge>
                  </dd>
                </div>
                <div>
                  <dt className="text-xs uppercase tracking-wide text-slate-500">Kategori</dt>
                  <dd className="mt-1 text-sm font-medium text-slate-900">{data.category_name || "-"}</dd>
                </div>
                <div>
                  <dt className="text-xs uppercase tracking-wide text-slate-500">Nominal</dt>
                  <dd className="mt-1 text-sm font-semibold tabular-nums text-slate-900">
                    {formatCurrency(data.amount, true)}
                  </dd>
                </div>
                <div className="sm:col-span-2">
                  <dt className="text-xs uppercase tracking-wide text-slate-500">Deskripsi</dt>
                  <dd className="mt-1 text-sm text-slate-700">{data.description || "-"}</dd>
                </div>
                <div>
                  <dt className="text-xs uppercase tracking-wide text-slate-500">Referensi</dt>
                  <dd className="mt-1 text-sm text-slate-700">{data.reference_number || "-"}</dd>
                </div>
                <div>
                  <dt className="text-xs uppercase tracking-wide text-slate-500">Terakhir diubah</dt>
                  <dd className="mt-1 text-sm text-slate-700">{formatDateTime(data.updated_at)}</dd>
                </div>
              </dl>
            </Card>

            <TransactionForm transaction={data} />
          </>
        )}
      </div>
    </AppShell>
  );
}
