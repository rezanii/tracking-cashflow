import axios, { AxiosError, type AxiosInstance } from "axios";

import { authStorage } from "@/lib/auth-storage";
import type { ApiEnvelope } from "@/types";

const baseURL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api/v1";

// ApiError carries the field errors the API returns so a form can show them inline.
export class ApiError extends Error {
  readonly status: number;
  readonly fields: Record<string, string>;

  constructor(message: string, status: number, fields: Record<string, string> = {}) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.fields = fields;
  }
}

export const apiClient: AxiosInstance = axios.create({
  baseURL,
  timeout: 30_000,
  headers: { "Content-Type": "application/json" },
});

apiClient.interceptors.request.use((config) => {
  const token = authStorage.getToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

apiClient.interceptors.response.use(
  (response) => response,
  (error: AxiosError<ApiEnvelope<unknown>>) => {
    const status = error.response?.status ?? 0;
    const body = error.response?.data;

    // An expired or rejected token means the stored session is useless. Clearing it here
    // keeps the app from looping on requests that can never succeed.
    if (status === 401) {
      authStorage.clear();
      if (typeof window !== "undefined" && !window.location.pathname.startsWith("/login")) {
        window.location.href = "/login";
      }
    }

    const message =
      body?.message ??
      (status === 0 ? "Tidak dapat menghubungi server" : "Terjadi kesalahan pada server");

    return Promise.reject(new ApiError(message, status, body?.errors ?? {}));
  },
);

// unwrap returns the data field and turns a success:false body into an ApiError, so callers
// never have to check the envelope themselves.
export function unwrap<T>(envelope: ApiEnvelope<T>, status = 200): T {
  if (!envelope.success || envelope.data === undefined) {
    throw new ApiError(envelope.message || "Permintaan gagal", status, envelope.errors ?? {});
  }
  return envelope.data;
}

// toQuery drops empty values so the API never receives blank filters.
export function toQuery(params: Record<string, string | number | undefined | null>): string {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === "") continue;
    query.set(key, String(value));
  }
  const serialised = query.toString();
  return serialised ? `?${serialised}` : "";
}
