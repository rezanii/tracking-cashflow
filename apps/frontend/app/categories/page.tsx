"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm } from "react-hook-form";

import { AppShell } from "@/components/layout/app-shell";
import { Badge, Button, Card, Field, Input, Modal, Pagination, Select, Textarea } from "@/components/ui";
import { DataTable, type Column } from "@/components/ui/data-table";
import { useAsync } from "@/hooks/use-async";
import { ApiError } from "@/lib/api-client";
import { formatDate } from "@/lib/format";
import { categorySchema, type CategoryInput } from "@/schemas";
import { categoryService } from "@/services/categories";
import type { Category } from "@/types";

const pageSize = 10;

export default function CategoriesPage() {
  const [search, setSearch] = useState("");
  const [type, setType] = useState("");
  const [activeFilter, setActiveFilter] = useState("");
  const [page, setPage] = useState(1);

  const [editing, setEditing] = useState<Category | null>(null);
  const [creating, setCreating] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<Category | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<number | null>(null);

  const { data, loading, error, reload } = useAsync(
    () =>
      categoryService.list({
        search,
        type,
        is_active: activeFilter,
        page,
        page_size: pageSize,
        sort_by: "name",
        sort_dir: "asc",
      }),
    [search, type, activeFilter, page],
  );

  const form = useForm<CategoryInput>({
    resolver: zodResolver(categorySchema),
    defaultValues: { name: "", type: "EXPENSE", description: "" },
  });

  function openCreate() {
    form.reset({ name: "", type: "EXPENSE", description: "" });
    setActionError(null);
    setCreating(true);
  }

  function openEdit(category: Category) {
    form.reset({ name: category.name, type: category.type, description: category.description });
    setActionError(null);
    setEditing(category);
  }

  function closeForm() {
    setCreating(false);
    setEditing(null);
    setActionError(null);
  }

  async function submitForm(values: CategoryInput) {
    setActionError(null);
    try {
      if (editing) {
        await categoryService.update(editing.id, values);
      } else {
        await categoryService.create(values);
      }
      closeForm();
      reload();
    } catch (failure) {
      if (failure instanceof ApiError) {
        for (const [field, message] of Object.entries(failure.fields)) {
          form.setError(field as never, { type: "server", message });
        }
        // A duplicate name or a type change on a used category arrives as 409 without fields.
        setActionError(Object.keys(failure.fields).length > 0 ? null : failure.message);
        return;
      }
      setActionError("Terjadi kesalahan tidak terduga");
    }
  }

  async function toggleActive(category: Category) {
    setBusyId(category.id);
    setActionError(null);
    try {
      await categoryService.setActive(category.id, !category.is_active);
      reload();
    } catch (failure) {
      setActionError(failure instanceof Error ? failure.message : "Gagal mengubah status kategori");
    } finally {
      setBusyId(null);
    }
  }

  async function confirmDelete() {
    if (!pendingDelete) return;
    setBusyId(pendingDelete.id);
    setActionError(null);
    try {
      await categoryService.remove(pendingDelete.id);
      setPendingDelete(null);
      reload();
    } catch (failure) {
      setActionError(failure instanceof Error ? failure.message : "Gagal menghapus kategori");
    } finally {
      setBusyId(null);
    }
  }

  const columns: Column<Category>[] = [
    { header: "Nama", cell: (row) => <span className="font-medium text-slate-900">{row.name}</span> },
    {
      header: "Tipe",
      cell: (row) => (
        <Badge tone={row.type === "INCOME" ? "green" : "red"}>
          {row.type === "INCOME" ? "Pemasukan" : "Pengeluaran"}
        </Badge>
      ),
    },
    { header: "Deskripsi", cell: (row) => row.description || "-", hideOnMobile: true },
    {
      header: "Status",
      cell: (row) => <Badge tone={row.is_active ? "blue" : "slate"}>{row.is_active ? "Aktif" : "Nonaktif"}</Badge>,
    },
    { header: "Dibuat", cell: (row) => formatDate(row.created_at), hideOnMobile: true },
    {
      header: "Aksi",
      align: "right",
      cell: (row) => (
        <div className="flex justify-end gap-1">
          <Button variant="ghost" size="sm" onClick={() => openEdit(row)}>
            Ubah
          </Button>
          <Button
            variant="ghost"
            size="sm"
            loading={busyId === row.id}
            onClick={() => void toggleActive(row)}
          >
            {row.is_active ? "Nonaktifkan" : "Aktifkan"}
          </Button>
          <Button variant="ghost" size="sm" className="text-red-600" onClick={() => setPendingDelete(row)}>
            Hapus
          </Button>
        </div>
      ),
    },
  ];

  const formOpen = creating || editing !== null;

  return (
    <AppShell>
      <div className="space-y-5">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm text-slate-500">Kelompokkan transaksi Anda ke dalam kategori</p>
          <Button onClick={openCreate}>+ Tambah Kategori</Button>
        </div>

        {actionError && !formOpen && (
          <p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
            {actionError}
          </p>
        )}

        <Card>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <Field label="Cari" htmlFor="cat-search">
              <Input
                id="cat-search"
                placeholder="Nama atau deskripsi"
                value={search}
                onChange={(event) => {
                  setSearch(event.target.value);
                  setPage(1);
                }}
              />
            </Field>

            <Field label="Tipe" htmlFor="cat-type">
              <Select
                id="cat-type"
                value={type}
                onChange={(event) => {
                  setType(event.target.value);
                  setPage(1);
                }}
              >
                <option value="">Semua</option>
                <option value="INCOME">Pemasukan</option>
                <option value="EXPENSE">Pengeluaran</option>
              </Select>
            </Field>

            <Field label="Status" htmlFor="cat-active">
              <Select
                id="cat-active"
                value={activeFilter}
                onChange={(event) => {
                  setActiveFilter(event.target.value);
                  setPage(1);
                }}
              >
                <option value="">Semua</option>
                <option value="true">Aktif</option>
                <option value="false">Nonaktif</option>
              </Select>
            </Field>
          </div>
        </Card>

        <DataTable
          columns={columns}
          rows={data?.items ?? []}
          rowKey={(row) => row.id}
          loading={loading}
          error={error}
          onRetry={reload}
          emptyTitle="Belum ada kategori"
          emptyDescription="Tambahkan kategori agar transaksi dapat dikelompokkan."
          emptyAction={
            <Button size="sm" onClick={openCreate}>
              Tambah Kategori
            </Button>
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
        open={formOpen}
        title={editing ? "Ubah kategori" : "Tambah kategori"}
        onClose={closeForm}
        footer={
          <>
            <Button variant="secondary" onClick={closeForm}>
              Batal
            </Button>
            <Button
              loading={form.formState.isSubmitting}
              onClick={() => void form.handleSubmit(submitForm)()}
            >
              Simpan
            </Button>
          </>
        }
      >
        {actionError && (
          <p role="alert" className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
            {actionError}
          </p>
        )}

        <form className="space-y-4" onSubmit={form.handleSubmit(submitForm)} noValidate>
          <Field label="Nama" htmlFor="category-name" required error={form.formState.errors.name?.message}>
            <Input
              id="category-name"
              placeholder="Contoh: Makan"
              invalid={Boolean(form.formState.errors.name)}
              {...form.register("name")}
            />
          </Field>

          <Field
            label="Tipe"
            htmlFor="category-type"
            required
            hint={editing ? "Tipe tidak dapat diubah jika sudah dipakai transaksi" : undefined}
            error={form.formState.errors.type?.message}
          >
            <Select id="category-type" invalid={Boolean(form.formState.errors.type)} {...form.register("type")}>
              <option value="INCOME">Pemasukan</option>
              <option value="EXPENSE">Pengeluaran</option>
            </Select>
          </Field>

          <Field label="Deskripsi" htmlFor="category-description" error={form.formState.errors.description?.message}>
            <Textarea
              id="category-description"
              placeholder="Opsional"
              invalid={Boolean(form.formState.errors.description)}
              {...form.register("description")}
            />
          </Field>
        </form>
      </Modal>

      <Modal
        open={pendingDelete !== null}
        title="Hapus kategori"
        onClose={() => {
          setPendingDelete(null);
          setActionError(null);
        }}
        footer={
          <>
            <Button
              variant="secondary"
              onClick={() => {
                setPendingDelete(null);
                setActionError(null);
              }}
            >
              Batal
            </Button>
            <Button variant="danger" loading={busyId === pendingDelete?.id} onClick={() => void confirmDelete()}>
              Hapus
            </Button>
          </>
        }
      >
        <p className="text-sm text-slate-600">
          Kategori <span className="font-semibold">{pendingDelete?.name}</span> akan dihapus. Kategori yang
          masih dipakai transaksi tidak dapat dihapus, nonaktifkan saja.
        </p>
        {actionError && <p className="mt-3 text-sm font-medium text-red-600">{actionError}</p>}
      </Modal>
    </AppShell>
  );
}
