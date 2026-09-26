"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useForm } from "react-hook-form";

import { AppShell } from "@/components/layout/app-shell";
import {
  Badge,
  Button,
  Card,
  CurrencyInput,
  EmptyState,
  Field,
  Input,
  Loading,
  Modal,
  Select,
  Textarea,
} from "@/components/ui";
import { DataTable, type Column } from "@/components/ui/data-table";
import { ApiError } from "@/lib/api-client";
import { formatCurrency, formatDate, todayInJakarta } from "@/lib/format";
import {
  accountSchema,
  balanceSnapshotSchema,
  type AccountInput,
  type BalanceSnapshotInput,
} from "@/schemas";
import { accountService, accountTypeHints, accountTypeLabels } from "@/services/accounts";
import type { Account, BalanceSnapshot } from "@/types";

const accountTypes = ["CASH_FLOW", "WALLET", "BANK", "CREDIT_CARD", "SAVINGS"] as const;

export default function AccountsPage() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const [editing, setEditing] = useState<Account | null>(null);
  const [formOpen, setFormOpen] = useState(false);

  const [balanceFor, setBalanceFor] = useState<Account | null>(null);
  const [balances, setBalances] = useState<BalanceSnapshot[]>([]);
  const [loadingBalances, setLoadingBalances] = useState(false);

  const accountForm = useForm<AccountInput>({
    resolver: zodResolver(accountSchema),
    defaultValues: { name: "", account_type: "WALLET", opening_balance: "", description: "" },
  });

  const balanceForm = useForm<BalanceSnapshotInput>({
    resolver: zodResolver(balanceSnapshotSchema),
    defaultValues: { as_of_date: todayInJakarta(), actual_balance: "", note: "" },
  });

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const page = await accountService.list({ page: 1, page_size: 100, sort_by: "account_type" });
      setAccounts(page.items);
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Gagal memuat akun");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const selectedType = accountForm.watch("account_type");

  function openCreate() {
    setEditing(null);
    accountForm.reset({ name: "", account_type: "WALLET", opening_balance: "", description: "" });
    setFormOpen(true);
  }

  function openEdit(account: Account) {
    setEditing(account);
    accountForm.reset({
      name: account.name,
      account_type: account.account_type,
      opening_balance: account.opening_balance,
      description: account.description,
    });
    setFormOpen(true);
  }

  // Server field errors are mapped onto the inputs so the message lands where the mistake is.
  function applyFieldErrors(cause: unknown, form: typeof accountForm | typeof balanceForm) {
    if (!(cause instanceof ApiError)) {
      setError("Terjadi kesalahan tidak terduga");
      return;
    }
    const fields = Object.entries(cause.fields);
    if (fields.length === 0) {
      setError(cause.message);
      return;
    }
    for (const [field, message] of fields) {
      form.setError(field as never, { type: "server", message });
    }
  }

  async function submitAccount(values: AccountInput) {
    setError(null);
    setNotice(null);
    try {
      if (editing) {
        await accountService.update(editing.id, values);
        setNotice(`Akun ${values.name} diperbarui.`);
      } else {
        await accountService.create(values);
        setNotice(`Akun ${values.name} dibuat.`);
      }
      setFormOpen(false);
      await load();
    } catch (cause) {
      applyFieldErrors(cause, accountForm);
    }
  }

  async function openBalances(account: Account) {
    setBalanceFor(account);
    balanceForm.reset({ as_of_date: todayInJakarta(), actual_balance: "", note: "" });
    setLoadingBalances(true);
    try {
      setBalances(await accountService.listBalances(account.id));
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Gagal memuat riwayat saldo");
    } finally {
      setLoadingBalances(false);
    }
  }

  async function submitBalance(values: BalanceSnapshotInput) {
    if (!balanceFor) return;
    setError(null);
    setNotice(null);
    try {
      await accountService.recordBalance(balanceFor.id, values);
      setNotice(`Saldo ${balanceFor.name} per ${values.as_of_date} dicatat.`);
      setBalances(await accountService.listBalances(balanceFor.id));
      balanceForm.reset({ as_of_date: values.as_of_date, actual_balance: "", note: "" });
    } catch (cause) {
      applyFieldErrors(cause, balanceForm);
    }
  }

  async function removeBalance(snapshot: BalanceSnapshot) {
    if (!balanceFor) return;
    setError(null);
    try {
      await accountService.removeBalance(balanceFor.id, snapshot.id);
      setBalances(await accountService.listBalances(balanceFor.id));
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Gagal menghapus saldo");
    }
  }

  async function toggleActive(account: Account) {
    setError(null);
    try {
      await accountService.setActive(account.id, !account.is_active);
      await load();
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Gagal mengubah status akun");
    }
  }

  async function removeAccount(account: Account) {
    setError(null);
    setNotice(null);
    try {
      await accountService.remove(account.id);
      setNotice(`Akun ${account.name} dihapus.`);
      await load();
    } catch (cause) {
      // A used account cannot be deleted; the API says so and the message is worth showing.
      setError(cause instanceof ApiError ? cause.message : "Gagal menghapus akun");
    }
  }

  const accountColumns: Column<Account>[] = [
    { header: "Nama", cell: (row) => <span className="font-medium text-slate-900">{row.name}</span> },
    {
      header: "Saldo Awal",
      align: "right",
      cell: (row) => formatCurrency(row.opening_balance),
      hideOnMobile: true,
    },
    { header: "Keterangan", cell: (row) => row.description || "-", hideOnMobile: true },
    {
      header: "Status",
      cell: (row) => (
        <Badge tone={row.is_active ? "green" : "slate"}>{row.is_active ? "Aktif" : "Nonaktif"}</Badge>
      ),
    },
    {
      header: "Aksi",
      align: "right",
      cell: (row) => (
        <div className="flex flex-wrap justify-end gap-1">
          <Button size="sm" variant="secondary" onClick={() => void openBalances(row)}>
            Catat Saldo
          </Button>
          <Button size="sm" variant="ghost" onClick={() => openEdit(row)}>
            Ubah
          </Button>
          <Button size="sm" variant="ghost" onClick={() => void toggleActive(row)}>
            {row.is_active ? "Nonaktifkan" : "Aktifkan"}
          </Button>
          <Button size="sm" variant="danger" onClick={() => void removeAccount(row)}>
            Hapus
          </Button>
        </div>
      ),
    },
  ];

  const grouped = useMemo(() => {
    const byType = new Map<string, Account[]>();
    for (const account of accounts) {
      const list = byType.get(account.account_type) ?? [];
      list.push(account);
      byType.set(account.account_type, list);
    }
    return accountTypes
      .map((type) => ({ type, items: byType.get(type) ?? [] }))
      .filter((group) => group.items.length > 0);
  }, [accounts]);

  return (
    <AppShell>
      <div className="space-y-6">
        <Card>
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div>
              <h2 className="text-lg font-semibold text-slate-900">Akun</h2>
              <p className="mt-1 max-w-2xl text-sm text-slate-600">
                Tipe akun menentukan bagian mana dari laporan harian yang diisi akun ini, jadi
                ia adalah peran pelaporan, bukan sekadar nama instrumen.
              </p>
            </div>
            <Button onClick={openCreate}>Tambah Akun</Button>
          </div>

          {error ? (
            <p role="alert" className="mt-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">
              {error}
            </p>
          ) : null}
          {notice ? (
            <p className="mt-4 rounded-lg bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
              {notice}
            </p>
          ) : null}
        </Card>

        {loading ? (
          <Card>
            <Loading label="Memuat akun..." />
          </Card>
        ) : accounts.length === 0 ? (
          <Card>
            <EmptyState
              title="Belum ada akun"
              description="Laporan harian dibangun dari akun. Buat minimal satu akun cash flow untuk mulai."
              action={<Button onClick={openCreate}>Tambah Akun</Button>}
            />
          </Card>
        ) : (
          grouped.map((group) => (
            <Card key={group.type}>
              <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-500">
                {accountTypeLabels[group.type] ?? group.type}
              </h3>
              <p className="mt-1 text-sm text-slate-600">{accountTypeHints[group.type]}</p>

              <div className="mt-4">
                <DataTable rows={group.items} rowKey={(row) => row.id} columns={accountColumns} />
              </div>
            </Card>
          ))
        )}
      </div>

      <Modal
        open={formOpen}
        onClose={() => setFormOpen(false)}
        title={editing ? `Ubah ${editing.name}` : "Tambah Akun"}
      >
        <form className="space-y-4" onSubmit={accountForm.handleSubmit(submitAccount)} noValidate>
          <Field
            label="Nama"
            htmlFor="account-name"
            required
            error={accountForm.formState.errors.name?.message}
          >
            <Input
              id="account-name"
              placeholder="Contoh: Dompet Harian"
              invalid={Boolean(accountForm.formState.errors.name)}
              {...accountForm.register("name")}
            />
          </Field>

          <Field
            label="Tipe"
            htmlFor="account-type"
            required
            hint={accountTypeHints[selectedType]}
            error={accountForm.formState.errors.account_type?.message}
          >
            <Select
              id="account-type"
              invalid={Boolean(accountForm.formState.errors.account_type)}
              {...accountForm.register("account_type")}
            >
              {accountTypes.map((type) => (
                <option key={type} value={type}>
                  {accountTypeLabels[type]}
                </option>
              ))}
            </Select>
          </Field>

          <Field
            label="Saldo Awal"
            htmlFor="account-opening"
            hint="Ikut dihitung sebagai saldo awal laporan untuk akun cash flow. Kosongkan bila nol."
            error={accountForm.formState.errors.opening_balance?.message}
          >
            <CurrencyInput
              id="account-opening"
              invalid={Boolean(accountForm.formState.errors.opening_balance)}
              {...accountForm.register("opening_balance")}
            />
          </Field>

          <Field
            label="Keterangan"
            htmlFor="account-description"
            error={accountForm.formState.errors.description?.message}
          >
            <Textarea
              id="account-description"
              invalid={Boolean(accountForm.formState.errors.description)}
              {...accountForm.register("description")}
            />
          </Field>

          <div className="flex flex-col gap-2 pt-2 sm:flex-row">
            <Button type="submit" loading={accountForm.formState.isSubmitting}>
              {editing ? "Simpan Perubahan" : "Simpan Akun"}
            </Button>
            <Button type="button" variant="secondary" onClick={() => setFormOpen(false)}>
              Batal
            </Button>
          </div>
        </form>
      </Modal>

      <Modal
        open={balanceFor !== null}
        onClose={() => setBalanceFor(null)}
        title={balanceFor ? `Saldo Aktual — ${balanceFor.name}` : "Saldo Aktual"}
      >
        <p className="text-sm text-slate-600">
          Saldo yang benar-benar ada, dihitung manual atau dilihat dari mobile banking. Angka ini
          tidak bisa diturunkan dari transaksi, dan selisihnya terhadap catatan itulah yang
          dilaporkan sebagai pengeluaran yang belum tercatat.
        </p>

        <form
          className="mt-4 space-y-4"
          onSubmit={balanceForm.handleSubmit(submitBalance)}
          noValidate
        >
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field
              label="Tanggal"
              htmlFor="balance-date"
              required
              error={balanceForm.formState.errors.as_of_date?.message}
            >
              <Input
                id="balance-date"
                type="date"
                invalid={Boolean(balanceForm.formState.errors.as_of_date)}
                {...balanceForm.register("as_of_date")}
              />
            </Field>

            <Field
              label="Saldo"
              htmlFor="balance-amount"
              required
              error={balanceForm.formState.errors.actual_balance?.message}
            >
              <CurrencyInput
                id="balance-amount"
                invalid={Boolean(balanceForm.formState.errors.actual_balance)}
                {...balanceForm.register("actual_balance")}
              />
            </Field>
          </div>

          <Field label="Catatan" htmlFor="balance-note" error={balanceForm.formState.errors.note?.message}>
            <Input
              id="balance-note"
              placeholder="Contoh: dihitung manual"
              invalid={Boolean(balanceForm.formState.errors.note)}
              {...balanceForm.register("note")}
            />
          </Field>

          <Button type="submit" loading={balanceForm.formState.isSubmitting}>
            Catat Saldo
          </Button>
          <p className="text-xs text-slate-500">
            Mencatat tanggal yang sama akan menimpa angka sebelumnya, jadi salah ketik cukup
            dicatat ulang.
          </p>
        </form>

        <div className="mt-6">
          <h4 className="text-sm font-semibold text-slate-900">Riwayat</h4>
          {loadingBalances ? (
            <div className="mt-2">
              <Loading label="Memuat riwayat..." />
            </div>
          ) : balances.length === 0 ? (
            <p className="mt-2 text-sm text-slate-500">Belum ada saldo yang dicatat.</p>
          ) : (
            <ul className="mt-2 divide-y divide-slate-200">
              {balances.map((snapshot) => (
                <li key={snapshot.id} className="flex items-center justify-between gap-3 py-2 text-sm">
                  <span className="text-slate-600">{formatDate(snapshot.as_of_date)}</span>
                  <span className="font-medium text-slate-900">
                    {formatCurrency(snapshot.actual_balance)}
                  </span>
                  <Button size="sm" variant="ghost" onClick={() => void removeBalance(snapshot)}>
                    Hapus
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </Modal>
    </AppShell>
  );
}
