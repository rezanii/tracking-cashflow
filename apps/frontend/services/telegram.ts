import { apiClient, toQuery, unwrap } from "@/lib/api-client";
import type { ApiEnvelope, TelegramLink, TelegramPairingCode, TelegramSendResult } from "@/types";

export const telegramService = {
  async status(): Promise<TelegramLink> {
    const { data } = await apiClient.get<ApiEnvelope<TelegramLink>>("/telegram/link");
    return unwrap(data);
  },

  async pairingCode(): Promise<TelegramPairingCode> {
    const { data } = await apiClient.post<ApiEnvelope<TelegramPairingCode>>("/telegram/pairing-code");
    return unwrap(data);
  },

  async unlink(): Promise<void> {
    await apiClient.delete<ApiEnvelope<null>>("/telegram/link");
  },

  // date is optional; the API defaults to today.
  async sendDailyReport(date?: string): Promise<TelegramSendResult> {
    const { data } = await apiClient.post<ApiEnvelope<TelegramSendResult>>(
      `/telegram/send/daily-report${toQuery({ date })}`,
    );
    return unwrap(data);
  },
};
