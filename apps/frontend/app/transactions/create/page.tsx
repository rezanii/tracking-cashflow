"use client";

import { AppShell } from "@/components/layout/app-shell";
import { TransactionForm } from "@/components/transaction-form";

export default function CreateTransactionPage() {
  return (
    <AppShell>
      <div className="space-y-4">
        <p className="text-sm text-slate-500">Catat pemasukan, pengeluaran atau transfer baru</p>
        <TransactionForm />
      </div>
    </AppShell>
  );
}
