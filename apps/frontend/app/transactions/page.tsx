"use client";

import Link from "next/link";
import { useState } from "react";

import { AppShell } from "@/components/layout/app-shell";
import { Badge, Button, Card, Field, Input, Modal, Pagination, Select } from "@/components/ui";
import { DataTable, type Column } from "@/components/ui/data-table";
import { useAsync } from "@/hooks/use-async";
import { formatCurrency, formatDate, transactionTypeLabels } from "@/lib/format";
import { categoryService } from "@/services/categories";
import { transactionService } from "@/services/transactions";
import type { Transaction } from "@/types";

const pageSize = 10;

export default function TransactionsPage() {
  const [search, setSearch] = useState("");
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");
  const [type, setType] = useState("");
  const [categoryId, setCategoryId] = useState("");
  const [sortBy, setSortBy] = useState("transaction_date");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");
  const [page, setPage] = useState(1);
  const [pendingDelete, setPendingDelete] = useState<Transaction | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const { data: categories } = useAsync(() => categoryService.listAllActive(), []);

  const { data, loading, error, reload } = useAsync(
    () =>
      transactionService.list({
        search,
        date_from: dateFrom,
        date_to: dateTo,
        transaction_type: type,
        category_id: categoryId,
        page,
        page_size: pageSize,
        sort_by: sortBy,
        sort_dir: sortDir,
      }),
    [search, dateFrom, dateTo, type, categoryId, page, sortBy, sortDir],
  );

  // Any filter change invalidates the current page number.
  function updateFilter(apply: () => void) {
    apply();
    setPage(1);
  }

  function resetFilters() {
    setSearch("");
    setDateFrom("");
    setDateTo("");
    setType("");
    setCategoryId("");
    setPage(1);
  }

  async function confirmDelete() {
    if (!pendingDelete) return;
    setDeleting(true);
    setDeleteError(null);
    try {
      await transactionService.remove(pendingDelete.id);
      setPendingDelete(null);
      reload();
    } catch (deleteFailure) {
      setDeleteError(deleteFailure instanceof Error ? deleteFailure.message : "Gagal menghapus transaksi");
    } finally {
      setDeleting(false);
    }
  }

  const columns: Column<Transaction>[] = [
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
    },
    { header: "Kategori", cell: (row) => row.category_name || "-" },
    { header: "Deskripsi", cell: (row) => row.description || "-", hideOnMobile: true },
    { header: "Referensi", cell: (row) => row.reference_number || "-", hideOnMobile: true },
    {
      header: "Nominal",
      align: "right",
      className: "font-medium tabular-nums",
      cell: (row) => (
        <span
          className={
            row.transaction_type === "EXPENSE"
              ? "text-red-600"
              : row.transaction_type === "INCOME"
                ? "text-emerald-600"
                : "text-slate-600"
          }
        >
          {row.transaction_type === "EXPENSE" ? "-" : row.transaction_type === "INCOME" ? "+" : ""}
          {formatCurrency(row.amount, true)}
        </span>
      ),
    },
    {
      header: "Aksi",
      align: "right",
      cell: (row) => (
        <div className="flex justify-end gap-1">
          <Link href={`/transactions/${row.id}`}>
            <Button variant="ghost" size="sm">
              Ubah
            </Button>
          </Link>
          <Button variant="ghost" size="sm" className="text-red-600" onClick={() => setPendingDelete(row)}>
            Hapus
          </Button>
        </div>
      ),
    },
  ];

  return (
    <AppShell>
      <div className="space-y-5">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm text-slate-500">Kelola pemasukan, pengeluaran dan transfer Anda</p>
          <Link href="/transactions/create">
            <Button>+ Tambah Transaksi</Button>
          </Link>
        </div>

        <Card>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
            <Field label="Cari" htmlFor="filter-search">
              <Input
                id="filter-search"
                placeholder="Deskripsi atau referensi"
                value={search}
                onChange={(event) => updateFilter(() => setSearch(event.target.value))}
              />
            </Field>

            <Field label="Dari tanggal" htmlFor="filter-from">
              <Input
                id="filter-from"
                type="date"
                value={dateFrom}
                onChange={(event) => updateFilter(() => setDateFrom(event.target.value))}
              />
            </Field>

            <Field
              label="Sampai tanggal"
              htmlFor="filter-to"
              error={dateFrom && dateTo && dateTo < dateFrom ? "Tidak boleh lebih awal" : undefined}
            >
              <Input
                id="filter-to"
                type="date"
                value={dateTo}
                onChange={(event) => updateFilter(() => setDateTo(event.target.value))}
              />
            </Field>

            <Field label="Jenis" htmlFor="filter-type">
              <Select
                id="filter-type"
                value={type}
                onChange={(event) => updateFilter(() => setType(event.target.value))}
              >
                <option value="">Semua</option>
                <option value="INCOME">Pemasukan</option>
                <option value="EXPENSE">Pengeluaran</option>
                <option value="TRANSFER">Transfer</option>
              </Select>
            </Field>

            <Field label="Kategori" htmlFor="filter-category">
              <Select
                id="filter-category"
                value={categoryId}
                onChange={(event) => updateFilter(() => setCategoryId(event.target.value))}
              >
                <option value="">Semua</option>
                {(categories ?? []).map((category) => (
                  <option key={category.id} value={category.id}>
                    {category.name}
                  </option>
                ))}
              </Select>
            </Field>

            <Field label="Urutkan" htmlFor="filter-sort">
              <Select
                id="filter-sort"
                value={`${sortBy}:${sortDir}`}
                onChange={(event) => {
                  const [nextBy, nextDir] = event.target.value.split(":");
                  updateFilter(() => {
                    setSortBy(nextBy ?? "transaction_date");
                    setSortDir((nextDir as "asc" | "desc") ?? "desc");
                  });
                }}
              >
                <option value="transaction_date:desc">Tanggal terbaru</option>
                <option value="transaction_date:asc">Tanggal terlama</option>
                <option value="amount:desc">Nominal terbesar</option>
                <option value="amount:asc">Nominal terkecil</option>
                <option value="created_at:desc">Terakhir dibuat</option>
              </Select>
            </Field>
          </div>

          <div className="mt-3 flex justify-end">
            <Button variant="ghost" size="sm" onClick={resetFilters}>
              Reset filter
            </Button>
          </div>
        </Card>

        <DataTable
          columns={columns}
          rows={data?.items ?? []}
          rowKey={(row) => row.id}
          loading={loading}
          error={error}
          onRetry={reload}
          emptyTitle="Tidak ada transaksi"
          emptyDescription="Ubah filter atau tambahkan transaksi baru."
          emptyAction={
            <Link href="/transactions/create">
              <Button size="sm">Tambah Transaksi</Button>
            </Link>
          }
          footer={
            data ? (
              <Pagination
                page={data.pagination.page}
                totalPages={data.pagination.total_pages}
                totalItems={data.pagination.total_items}
                onChange={setPage}
              />
            ) : null
          }
        />
      </div>

      <Modal
        open={pendingDelete !== null}
        title="Hapus transaksi"
        onClose={() => {
          setPendingDelete(null);
          setDeleteError(null);
        }}
        footer={
          <>
            <Button
              variant="secondary"
              onClick={() => {
                setPendingDelete(null);
                setDeleteError(null);
              }}
            >
              Batal
            </Button>
            <Button variant="danger" loading={deleting} onClick={() => void confirmDelete()}>
              Hapus
            </Button>
          </>
        }
      >
        <p className="text-sm text-slate-600">
          Transaksi {pendingDelete?.description || pendingDelete?.reference_number || "ini"} sebesar{" "}
          <span className="font-semibold">{formatCurrency(pendingDelete?.amount ?? "0")}</span> akan dihapus
          permanen.
        </p>
        {deleteError && <p className="mt-3 text-sm font-medium text-red-600">{deleteError}</p>}
      </Modal>
    </AppShell>
  );
}
