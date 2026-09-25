import Link from "next/link";

export default function NotFound() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-3 bg-slate-50 px-4 text-center">
      <p className="text-sm font-semibold text-slate-500">404</p>
      <h1 className="text-xl font-semibold text-slate-900">Halaman tidak ditemukan</h1>
      <Link
        href="/dashboard"
        className="mt-2 rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-brand-700"
      >
        Kembali ke dashboard
      </Link>
    </div>
  );
}
