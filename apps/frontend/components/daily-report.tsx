"use client";

import { useCallback, useEffect, useState } from "react";

import { Badge, Button, Card, Field, Input, Loading } from "@/components/ui";
import { ApiError } from "@/lib/api-client";
import { formatCurrency, todayInJakarta } from "@/lib/format";
import { reportService } from "@/services/reports";
import { telegramService } from "@/services/telegram";
import type { AmountLine, DailyCashFlowReport } from "@/types";

/** Row renders one label-and-amount line, which is most of this report. */
function Row({ label, amount, strong }: { label: string; amount: string; strong?: boolean }) {
  return (
    <div className="flex items-baseline justify-between gap-4 py-1">
      <span className={strong ? "text-sm font-medium text-slate-900" : "text-sm text-slate-600"}>
        {label}
      </span>
      <span className={strong ? "text-sm font-semibold text-slate-900" : "text-sm text-slate-800"}>
        {formatCurrency(amount)}
      </span>
    </div>
  );
}

function Section({
  icon,
  title,
  children,
}: {
  icon: string;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <Card>
      <h3 className="flex items-center gap-2 text-sm font-semibold uppercase tracking-wide text-slate-500">
        <span aria-hidden>{icon}</span>
        {title}
      </h3>
      <div className="mt-3 divide-y divide-slate-100">{children}</div>
    </Card>
  );
}

function Lines({ lines }: { lines: AmountLine[] }) {
  return (
    <>
      {lines.map((line, index) => (
        <Row key={`${line.label}-${index}`} label={line.label} amount={line.amount} />
      ))}
    </>
  );
}

export function DailyReport() {
  const [date, setDate] = useState(todayInJakarta());
  const [report, setReport] = useState<DailyCashFlowReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const load = useCallback(async (forDate: string) => {
    setLoading(true);
    setError(null);
    try {
      setReport(await reportService.dailyCashFlow(forDate));
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Gagal memuat laporan harian");
      setReport(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load(date);
  }, [date, load]);

  async function sendToTelegram() {
    setSending(true);
    setError(null);
    setNotice(null);
    try {
      const result = await telegramService.sendDailyReport(date);
      setNotice(`Terkirim ke chat ${result.chat_id} dalam ${result.messages} pesan.`);
    } catch (cause) {
      // The usual cause is no linked chat, and the API says so in words worth showing.
      setError(cause instanceof ApiError ? cause.message : "Gagal mengirim ke Telegram");
    } finally {
      setSending(false);
    }
  }

  return (
    <div className="space-y-5">
      <Card>
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div className="w-full sm:w-48">
            <Field label="Tanggal" htmlFor="daily-date">
              <Input
                id="daily-date"
                type="date"
                value={date}
                onChange={(event) => setDate(event.target.value)}
              />
            </Field>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" onClick={() => void load(date)} loading={loading}>
              Muat Ulang
            </Button>
            <Button onClick={() => void sendToTelegram()} loading={sending}>
              Kirim ke Telegram
            </Button>
          </div>
        </div>

        <p className="mt-3 text-sm text-slate-600">
          Laporan yang sama dengan yang dijawab bot untuk <span className="font-mono">/report</span>.
          Bagian yang tidak punya data akan disembunyikan, dan selisih hanya muncul setelah saldo
          aktual dicatat di halaman Akun.
        </p>

        {error ? (
          <p role="alert" className="mt-3 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">
            {error}
          </p>
        ) : null}
        {notice ? (
          <p className="mt-3 rounded-lg bg-emerald-50 px-4 py-3 text-sm text-emerald-700">{notice}</p>
        ) : null}
      </Card>

      {loading ? (
        <Card>
          <Loading label="Menyusun laporan harian..." />
        </Card>
      ) : !report ? null : (
        <>
          <Card>
            <div className="flex flex-wrap items-baseline justify-between gap-3">
              <h2 className="text-lg font-semibold text-slate-900">
                Cash Flow {report.period_label}
              </h2>
              <Badge tone="blue">{report.date_label}</Badge>
            </div>
            <div className="mt-4 rounded-lg bg-slate-50 px-4 py-3">
              <p className="text-xs uppercase tracking-wide text-slate-500">
                {report.opening_balance_label}
              </p>
              <p className="mt-1 text-2xl font-semibold text-slate-900">
                {formatCurrency(report.opening_balance)}
              </p>
            </div>
          </Card>

          {report.cash_flow_expenses.length > 0 ? (
            <Section icon="💸" title="Pengeluaran Cash Flow">
              <Lines lines={report.cash_flow_expenses} />
              <Row label="Total Pengeluaran" amount={report.cash_flow_total} strong />
            </Section>
          ) : null}

          {report.credit_cards.map((card) => (
            <Section key={card.account_id} icon="💳" title={`Payment ${card.name}`}>
              <Row label={`Bayar ${card.name}`} amount={card.payments} />
              <Row label="Ambil kembali / Top-up" amount={card.taken_back} />
              <Row label={`Net Payment ${card.name}`} amount={card.net} strong />
            </Section>
          ))}

          {report.top_up.total !== "0" ? (
            <Section icon="💰" title="Top-up">
              <Row label="Total Top-up" amount={report.top_up.total} strong />
              <Lines lines={report.top_up.allocations} />
              <div className="flex items-center justify-between gap-4 py-2">
                <span className="text-sm text-slate-600">Total alokasi</span>
                <span className="flex items-center gap-2 text-sm font-semibold text-slate-900">
                  {formatCurrency(report.top_up.allocation_total)}
                  <Badge tone={report.top_up.balanced ? "green" : "red"}>
                    {report.top_up.balanced ? "cocok" : "tidak cocok"}
                  </Badge>
                </span>
              </div>
            </Section>
          ) : null}

          {report.wallets.map((wallet) => (
            <Section key={wallet.account_id} icon="🏦" title={wallet.name}>
              <Row label={`Jatah ${wallet.name}`} amount={wallet.allotment} strong />
              {wallet.items.map((item, index) => (
                <div key={`${item.label}-${index}`} className="py-1">
                  <Row label={item.label} amount={item.amount} />
                  {item.sub_items.length > 0 ? (
                    <ul className="ml-4 border-l border-slate-200 pl-3">
                      {item.sub_items.map((sub, subIndex) => (
                        <li
                          key={`${sub.label}-${subIndex}`}
                          className="flex justify-between gap-4 py-0.5 text-xs text-slate-500"
                        >
                          <span>{sub.label}</span>
                          <span>{formatCurrency(sub.amount)}</span>
                        </li>
                      ))}
                    </ul>
                  ) : null}
                </div>
              ))}
              <Row label="Total transaksi tercatat" amount={wallet.recorded_total} strong />
              <Row label="Sisa menurut catatan" amount={wallet.expected_remaining} />
              {wallet.has_actual_balance ? (
                <>
                  <Row label="Saldo aktual" amount={wallet.actual_balance} />
                  <div className="flex items-center justify-between gap-4 py-2">
                    <span className="text-sm font-medium text-slate-900">
                      {wallet.variance === "0" ? "Tidak ada selisih" : "Belum tercatat"}
                    </span>
                    <span className="flex items-center gap-2">
                      <span className="text-sm font-semibold text-slate-900">
                        {formatCurrency(wallet.variance)}
                      </span>
                      <Badge tone={wallet.variance === "0" ? "green" : "red"}>
                        {wallet.variance === "0" ? "cocok" : "selisih"}
                      </Badge>
                    </span>
                  </div>
                </>
              ) : (
                <p className="py-2 text-xs text-slate-500">
                  Saldo aktual belum dicatat, jadi selisih belum bisa dihitung. Catat di halaman
                  Akun.
                </p>
              )}
            </Section>
          ))}

          {report.banks.map((bank) => (
            <Section key={bank.account_id} icon="🏦" title={bank.name}>
              <Row label="Dana masuk" amount={bank.money_in} />
              {bank.fees !== "0" ? <Row label="Biaya admin" amount={bank.fees} /> : null}
              <Row label="Dana keluar" amount={bank.money_out} />
              <Row label="Mutasi hari ini" amount={bank.computed} strong />
              {bank.has_actual_balance ? (
                <>
                  <Row label="Saldo aktual" amount={bank.actual_balance} />
                  <Row label="Saldo sebelumnya" amount={bank.previous_balance} />
                </>
              ) : null}
            </Section>
          ))}

          {report.reconciliation.top_up_total !== "0" ? (
            <Section icon="📊" title="Rekonsiliasi Top-up">
              <Row label="Total Top-up" amount={report.reconciliation.top_up_total} strong />
              <Lines lines={report.reconciliation.parts} />
              <Row label="Jumlah bagian" amount={report.reconciliation.parts_total} />
              <div className="flex items-center justify-between gap-4 py-2">
                <span className="text-sm font-medium text-slate-900">Selisih Top-up</span>
                <span className="flex items-center gap-2">
                  <span className="text-sm font-semibold text-slate-900">
                    {formatCurrency(report.reconciliation.difference)}
                  </span>
                  <Badge tone={report.reconciliation.balanced ? "green" : "red"}>
                    {report.reconciliation.balanced ? "seimbang" : "tidak seimbang"}
                  </Badge>
                </span>
              </div>
            </Section>
          ) : null}

          {report.status.length > 0 ? (
            <Section icon="📌" title={`Status ${report.date_label}`}>
              {report.status.map((line, index) => (
                <Row
                  key={`${line.label}-${index}`}
                  label={`${line.icon} ${line.label}`}
                  amount={line.amount}
                />
              ))}
            </Section>
          ) : null}
        </>
      )}
    </div>
  );
}
