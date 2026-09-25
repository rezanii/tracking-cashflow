"use client";

import clsx from "clsx";

import { EmptyState, ErrorState, Loading } from "@/components/ui";

export type Column<T> = {
  header: string;
  // A render function keeps formatting in the caller, so the table stays agnostic.
  cell: (row: T) => React.ReactNode;
  align?: "left" | "right" | "center";
  className?: string;
  // Set for columns that are noise on a phone.
  hideOnMobile?: boolean;
};

type DataTableProps<T> = {
  columns: Column<T>[];
  rows: T[];
  rowKey: (row: T) => string | number;
  loading?: boolean;
  error?: string | null;
  onRetry?: () => void;
  emptyTitle?: string;
  emptyDescription?: string;
  emptyAction?: React.ReactNode;
  footer?: React.ReactNode;
};

export function DataTable<T>({
  columns,
  rows,
  rowKey,
  loading = false,
  error = null,
  onRetry,
  emptyTitle = "Belum ada data",
  emptyDescription,
  emptyAction,
  footer,
}: DataTableProps<T>) {
  const alignment = { left: "text-left", right: "text-right", center: "text-center" } as const;

  return (
    <div className="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
      {loading && <Loading />}
      {!loading && error && <ErrorState message={error} onRetry={onRetry} />}
      {!loading && !error && rows.length === 0 && (
        <EmptyState title={emptyTitle} description={emptyDescription} action={emptyAction} />
      )}

      {!loading && !error && rows.length > 0 && (
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-slate-200 text-sm">
            <thead className="bg-slate-50">
              <tr>
                {columns.map((column) => (
                  <th
                    key={column.header}
                    scope="col"
                    className={clsx(
                      "px-4 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500",
                      alignment[column.align ?? "left"],
                      column.hideOnMobile && "hidden md:table-cell",
                    )}
                  >
                    {column.header}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {rows.map((row) => (
                <tr key={rowKey(row)} className="transition hover:bg-slate-50/70">
                  {columns.map((column) => (
                    <td
                      key={column.header}
                      className={clsx(
                        "px-4 py-3 text-slate-700",
                        alignment[column.align ?? "left"],
                        column.className,
                        column.hideOnMobile && "hidden md:table-cell",
                      )}
                    >
                      {column.cell(row)}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {footer}
    </div>
  );
}
