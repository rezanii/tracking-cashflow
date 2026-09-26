"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { useForm } from "react-hook-form";

import { Button, Card, CurrencyInput, Field, Input, Select, Textarea } from "@/components/ui";
import { useAsync } from "@/hooks/use-async";
import { ApiError } from "@/lib/api-client";
import { todayInJakarta } from "@/lib/format";
import { transactionSchema, type TransactionInput } from "@/schemas";
import { accountService, accountTypeLabels } from "@/services/accounts";
import { categoryService } from "@/services/categories";
import { transactionService } from "@/services/transactions";
import type { Transaction } from "@/types";

type TransactionFormProps = {
  // Present when editing; absent when creating.
  transaction?: Transaction;
};

export function TransactionForm({ transaction }: TransactionFormProps) {
  const router = useRouter();
  const [formError, setFormError] = useState<string | null>(null);

  const { data: categories, loading: loadingCategories } = useAsync(
    () => categoryService.listAllActive(),
    [],
  );
  const { data: accounts, loading: loadingAccounts } = useAsync(
    () => accountService.listAllActive(),
    [],
  );

  const form = useForm<TransactionInput>({
    resolver: zodResolver(transactionSchema),
    defaultValues: {
      transaction_date: transaction?.transaction_date ?? todayInJakarta(),
      transaction_type: transaction?.transaction_type ?? "EXPENSE",
      category_id: transaction?.category_id ? String(transaction.category_id) : "",
      amount: transaction?.amount ?? "",
      description: transaction?.description ?? "",
      reference_number: transaction?.reference_number ?? "",
      account_id: transaction?.account_id ? String(transaction.account_id) : "",
      to_account_id: transaction?.to_account_id ? String(transaction.to_account_id) : "",
      parent_id: transaction?.parent_id ? String(transaction.parent_id) : "",
    },
  });

  const selectedType = form.watch("transaction_type");
  const isTransfer = selectedType === "TRANSFER";

  // Only categories of the matching type may be chosen; the API rejects a mismatch anyway.
  const availableCategories = useMemo(
    () => (categories ?? []).filter((category) => category.type === selectedType),
    [categories, selectedType],
  );

  // Switching type invalidates a category picked for the previous type.
  useEffect(() => {
    const current = form.getValues("category_id");
    if (!current) return;
    const stillValid = availableCategories.some((category) => String(category.id) === current);
    if (isTransfer || !stillValid) {
      form.setValue("category_id", "");
    }
  }, [availableCategories, isTransfer, form]);

  // A destination account only means anything on a transfer, so leaving it set after a switch
  // would send the API something it refuses.
  useEffect(() => {
    if (!isTransfer && form.getValues("to_account_id")) {
      form.setValue("to_account_id", "");
    }
  }, [isTransfer, form]);

  const accountOptions = accounts ?? [];
  const sourceAccountID = form.watch("account_id");

  async function onSubmit(values: TransactionInput) {
    setFormError(null);
    try {
      if (transaction) {
        await transactionService.update(transaction.id, values);
      } else {
        await transactionService.create(values);
      }
      router.push("/transactions");
    } catch (error) {
      if (error instanceof ApiError) {
        for (const [field, message] of Object.entries(error.fields)) {
          form.setError(field as never, { type: "server", message });
        }
        setFormError(Object.keys(error.fields).length > 0 ? null : error.message);
        return;
      }
      setFormError("Terjadi kesalahan tidak terduga");
    }
  }

  return (
    <Card className="max-w-2xl">
      {formError && (
        <p role="alert" className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
          {formError}
        </p>
      )}

      <form className="space-y-4" onSubmit={form.handleSubmit(onSubmit)} noValidate>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field
            label="Tanggal"
            htmlFor="transaction_date"
            required
            error={form.formState.errors.transaction_date?.message}
          >
            <Input
              id="transaction_date"
              type="date"
              invalid={Boolean(form.formState.errors.transaction_date)}
              {...form.register("transaction_date")}
            />
          </Field>

          <Field
            label="Jenis Transaksi"
            htmlFor="transaction_type"
            required
            error={form.formState.errors.transaction_type?.message}
          >
            <Select
              id="transaction_type"
              invalid={Boolean(form.formState.errors.transaction_type)}
              {...form.register("transaction_type")}
            >
              <option value="INCOME">Pemasukan</option>
              <option value="EXPENSE">Pengeluaran</option>
              <option value="TRANSFER">Transfer</option>
            </Select>
          </Field>
        </div>

        <Field
          label="Kategori"
          htmlFor="category_id"
          required={!isTransfer}
          hint={isTransfer ? "Transfer tidak memakai kategori karena hanya memindahkan dana" : undefined}
          error={form.formState.errors.category_id?.message}
        >
          <Select
            id="category_id"
            disabled={isTransfer || loadingCategories}
            invalid={Boolean(form.formState.errors.category_id)}
            {...form.register("category_id")}
          >
            <option value="">{isTransfer ? "Tidak berlaku" : "Pilih kategori"}</option>
            {availableCategories.map((category) => (
              <option key={category.id} value={category.id}>
                {category.name}
              </option>
            ))}
          </Select>
        </Field>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field
            label={isTransfer ? "Akun Sumber" : "Akun"}
            htmlFor="account_id"
            required={isTransfer}
            hint={
              isTransfer
                ? "Dana keluar dari akun ini"
                : "Kosongkan bila termasuk cash flow rumah tangga"
            }
            error={form.formState.errors.account_id?.message}
          >
            <Select
              id="account_id"
              disabled={loadingAccounts}
              invalid={Boolean(form.formState.errors.account_id)}
              {...form.register("account_id")}
            >
              <option value="">{isTransfer ? "Pilih akun sumber" : "Cash flow (tanpa akun)"}</option>
              {accountOptions.map((account) => (
                <option key={account.id} value={account.id}>
                  {account.name} — {accountTypeLabels[account.account_type] ?? account.account_type}
                </option>
              ))}
            </Select>
          </Field>

          <Field
            label="Akun Tujuan"
            htmlFor="to_account_id"
            required={isTransfer}
            hint={isTransfer ? "Dana masuk ke akun ini" : "Hanya berlaku untuk transfer"}
            error={form.formState.errors.to_account_id?.message}
          >
            <Select
              id="to_account_id"
              disabled={!isTransfer || loadingAccounts}
              invalid={Boolean(form.formState.errors.to_account_id)}
              {...form.register("to_account_id")}
            >
              <option value="">{isTransfer ? "Pilih akun tujuan" : "Tidak berlaku"}</option>
              {accountOptions
                .filter((account) => String(account.id) !== sourceAccountID)
                .map((account) => (
                  <option key={account.id} value={account.id}>
                    {account.name} — {accountTypeLabels[account.account_type] ?? account.account_type}
                  </option>
                ))}
            </Select>
          </Field>
        </div>

        <Field
          label="Nominal"
          htmlFor="amount"
          required
          hint="Gunakan titik untuk desimal, contoh 1150000.50"
          error={form.formState.errors.amount?.message}
        >
          <CurrencyInput
            id="amount"
            invalid={Boolean(form.formState.errors.amount)}
            {...form.register("amount")}
          />
        </Field>

        <Field label="Deskripsi" htmlFor="description" error={form.formState.errors.description?.message}>
          <Textarea
            id="description"
            placeholder="Contoh: Angsuran Cicilan Rumah September"
            invalid={Boolean(form.formState.errors.description)}
            {...form.register("description")}
          />
        </Field>

        <Field
          label="Nomor Referensi"
          htmlFor="reference_number"
          error={form.formState.errors.reference_number?.message}
        >
          <Input
            id="reference_number"
            placeholder="Contoh: INV-0001"
            invalid={Boolean(form.formState.errors.reference_number)}
            {...form.register("reference_number")}
          />
        </Field>

        <Field
          label="Bagian dari Transaksi"
          htmlFor="parent_id"
          hint="Isi id transaksi induk bila baris ini merinci transaksi lain, misalnya isi dari satu tarik tunai. Nominalnya tidak ditambahkan ke total."
          error={form.formState.errors.parent_id?.message}
        >
          <Input
            id="parent_id"
            inputMode="numeric"
            placeholder="Kosongkan bila bukan rincian"
            invalid={Boolean(form.formState.errors.parent_id)}
            {...form.register("parent_id")}
          />
        </Field>

        <div className="flex flex-col gap-2 pt-2 sm:flex-row">
          <Button type="submit" loading={form.formState.isSubmitting}>
            {transaction ? "Simpan Perubahan" : "Simpan Transaksi"}
          </Button>
          <Button type="button" variant="secondary" onClick={() => router.push("/transactions")}>
            Batal
          </Button>
        </div>
      </form>
    </Card>
  );
}
