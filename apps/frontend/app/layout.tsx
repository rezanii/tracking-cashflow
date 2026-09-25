import type { Metadata, Viewport } from "next";

import { AuthProvider } from "@/hooks/use-auth";

import "./globals.css";

export const metadata: Metadata = {
  title: "Tracking Cashflow",
  description: "Pencatatan keuangan pribadi: transaksi, kategori, dashboard dan laporan arus kas.",
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="id">
      <body>
        <AuthProvider>{children}</AuthProvider>
      </body>
    </html>
  );
}
