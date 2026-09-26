"use client";

import { useCallback, useEffect, useState } from "react";

import { AppShell } from "@/components/layout/app-shell";
import { Badge, Button, Card, ErrorState, Field, Input, Loading } from "@/components/ui";
import { ApiError } from "@/lib/api-client";
import { formatDateTime, todayInJakarta } from "@/lib/format";
import { telegramService } from "@/services/telegram";
import type { TelegramLink, TelegramPairingCode } from "@/types";

export default function SettingsPage() {
  const [link, setLink] = useState<TelegramLink | null>(null);
  const [pairing, setPairing] = useState<TelegramPairingCode | null>(null);
  const [date, setDate] = useState(todayInJakarta());

  const [loading, setLoading] = useState(true);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setLink(await telegramService.status());
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Gagal memuat status Telegram");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  // Every action reloads the status afterwards, so what is on screen is what the server holds.
  async function run(action: () => Promise<string>) {
    setWorking(true);
    setError(null);
    setNotice(null);
    try {
      setNotice(await action());
      setLink(await telegramService.status());
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Tindakan gagal");
    } finally {
      setWorking(false);
    }
  }

  return (
    <AppShell>
      <div className="space-y-6">
        <Card>
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div>
              <h2 className="text-lg font-semibold text-slate-900">Notifikasi Telegram</h2>
              <p className="mt-1 max-w-2xl text-sm text-slate-600">
                Hubungkan satu chat Telegram untuk menerima laporan cash flow harian. Selama
                belum terhubung, bot tidak menjawab apa pun: chat id saja bukan bukti identitas.
              </p>
            </div>
            {link ? (
              <Badge tone={link.linked ? "green" : "slate"}>
                {link.linked ? "Terhubung" : "Belum terhubung"}
              </Badge>
            ) : null}
          </div>

          {loading ? (
            <div className="mt-6">
              <Loading label="Memuat status Telegram..." />
            </div>
          ) : error && !link ? (
            <div className="mt-6">
              <ErrorState message={error} onRetry={() => void load()} />
            </div>
          ) : (
            <div className="mt-6 space-y-6">
              {error ? (
                <p className="rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">{error}</p>
              ) : null}
              {notice ? (
                <p className="rounded-lg bg-emerald-50 px-4 py-3 text-sm text-emerald-700">{notice}</p>
              ) : null}

              {link?.linked ? (
                <dl className="grid gap-4 sm:grid-cols-3">
                  <div>
                    <dt className="text-xs uppercase tracking-wide text-slate-500">Chat ID</dt>
                    <dd className="mt-1 font-mono text-sm text-slate-900">{link.chat_id}</dd>
                  </div>
                  <div>
                    <dt className="text-xs uppercase tracking-wide text-slate-500">Username</dt>
                    <dd className="mt-1 text-sm text-slate-900">{link.username || "—"}</dd>
                  </div>
                  <div>
                    <dt className="text-xs uppercase tracking-wide text-slate-500">Terhubung sejak</dt>
                    <dd className="mt-1 text-sm text-slate-900">
                      {link.linked_at ? formatDateTime(link.linked_at) : "—"}
                    </dd>
                  </div>
                </dl>
              ) : (
                <div className="space-y-4">
                  <ol className="list-decimal space-y-2 pl-5 text-sm text-slate-600">
                    <li>Tekan <span className="font-medium">Buat Kode Pairing</span>.</li>
                    <li>Buka bot Telegram Anda.</li>
                    <li>
                      Kirim <span className="font-mono">/start &lt;kode&gt;</span>. Kode hanya
                      berlaku sekali dan akan kedaluwarsa.
                    </li>
                  </ol>

                  {pairing ? (
                    <div className="rounded-lg border border-brand-200 bg-brand-50 px-4 py-3">
                      <p className="font-mono text-2xl tracking-widest text-brand-800">
                        {pairing.code}
                      </p>
                      <p className="mt-1 text-sm text-brand-700">{pairing.instruction}</p>
                      <p className="mt-1 text-xs text-brand-600">
                        Berlaku sampai {formatDateTime(pairing.expires_at)}
                      </p>
                    </div>
                  ) : null}
                </div>
              )}

              <div className="flex flex-wrap gap-3">
                {link?.linked ? (
                  <Button
                    variant="danger"
                    loading={working}
                    onClick={() =>
                      void run(async () => {
                        await telegramService.unlink();
                        setPairing(null);
                        return "Koneksi Telegram dilepas.";
                      })
                    }
                  >
                    Lepas Koneksi
                  </Button>
                ) : (
                  <Button
                    loading={working}
                    onClick={() =>
                      void run(async () => {
                        const code = await telegramService.pairingCode();
                        setPairing(code);
                        return "Kode pairing dibuat. Kirim ke bot sebelum kedaluwarsa.";
                      })
                    }
                  >
                    Buat Kode Pairing
                  </Button>
                )}
              </div>
            </div>
          )}
        </Card>

        <Card>
          <h2 className="text-lg font-semibold text-slate-900">Kirim Laporan Sekarang</h2>
          <p className="mt-1 max-w-2xl text-sm text-slate-600">
            Mengirim laporan cash flow harian ke chat yang terhubung. Isi yang dikirim sama
            dengan yang dijawab bot untuk perintah <span className="font-mono">/report</span>.
          </p>

          <div className="mt-4 flex flex-wrap items-end gap-3">
            <div className="w-full sm:w-48">
              <Field label="Tanggal" htmlFor="report-date">
                <Input
                  id="report-date"
                  type="date"
                  value={date}
                  onChange={(event) => setDate(event.target.value)}
                />
              </Field>
            </div>
            <Button
              variant="secondary"
              loading={working}
              disabled={!link?.linked}
              onClick={() =>
                void run(async () => {
                  const result = await telegramService.sendDailyReport(date);
                  return `Laporan terkirim ke chat ${result.chat_id} dalam ${result.messages} pesan.`;
                })
              }
            >
              Kirim ke Telegram
            </Button>
          </div>
          {!link?.linked ? (
            <p className="mt-3 text-sm text-slate-500">
              Hubungkan chat terlebih dahulu untuk mengaktifkan tombol ini.
            </p>
          ) : null}
        </Card>
      </div>
    </AppShell>
  );
}
