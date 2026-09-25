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

  const form = useForm<TransactionInput>({
    resolver: zodResolver(transactionSchema),
    defaultValues: {
      transaction_date: transaction?.transaction_date ?? todayInJakarta(),
      transaction_type: transaction?.transaction_type ?? "EXPENSE",
      category_id: transaction?.category_id ? String(transaction.category_id) : "",
      amount: transaction?.amount ?? "",
      description: transaction?.description ?? "",
      reference_number: transaction?.reference_number ?? "",
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
