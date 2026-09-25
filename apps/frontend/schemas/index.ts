import { z } from "zod";

// Money is validated as a string so the exact decimal typed by the user reaches the API
// untouched by floating point.
const amountSchema = z
  .string()
  .min(1, "Nominal wajib diisi")
  .refine((value) => /^\d+(\.\d{1,2})?$/.test(value.replace(/,/g, "")), {
    message: "Nominal harus angka positif, maksimal 2 desimal",
  })
  .refine((value) => Number(value.replace(/,/g, "")) > 0, {
    message: "Nominal harus lebih besar dari 0",
  });

const dateSchema = z
  .string()
  .min(1, "Tanggal wajib diisi")
  .regex(/^\d{4}-\d{2}-\d{2}$/, "Tanggal tidak valid");

export const loginSchema = z.object({
  email: z.string().min(1, "Email wajib diisi").email("Format email tidak valid"),
  password: z.string().min(1, "Password wajib diisi"),
});

export const registerSchema = z.object({
  name: z.string().min(2, "Nama minimal 2 karakter").max(150, "Nama maksimal 150 karakter"),
  email: z.string().min(1, "Email wajib diisi").email("Format email tidak valid"),
  password: z
    .string()
    .min(8, "Password minimal 8 karakter")
    .max(72, "Password maksimal 72 karakter")
    .regex(/[A-Z]/, "Password harus memuat huruf besar")
    .regex(/[a-z]/, "Password harus memuat huruf kecil")
    .regex(/\d/, "Password harus memuat angka"),
});

export const transactionSchema = z
  .object({
    transaction_date: dateSchema,
    transaction_type: z.enum(["INCOME", "EXPENSE", "TRANSFER"], {
      message: "Jenis transaksi wajib dipilih",
    }),
    category_id: z.string().optional(),
    amount: amountSchema,
    description: z.string().max(500, "Deskripsi maksimal 500 karakter").optional(),
    reference_number: z.string().max(100, "Nomor referensi maksimal 100 karakter").optional(),
  })
  // The API enforces this too; checking here means the user sees it before a round trip.
  .refine(
    (value) => value.transaction_type === "TRANSFER" || (value.category_id ?? "") !== "",
    { message: "Kategori wajib dipilih untuk pemasukan dan pengeluaran", path: ["category_id"] },
  );

export const categorySchema = z.object({
  name: z.string().min(2, "Nama minimal 2 karakter").max(100, "Nama maksimal 100 karakter"),
  type: z.enum(["INCOME", "EXPENSE"], { message: "Tipe kategori wajib dipilih" }),
  description: z.string().max(255, "Deskripsi maksimal 255 karakter").optional(),
});

export const reportFilterSchema = z
  .object({
    date_from: dateSchema,
    date_to: dateSchema,
    category_id: z.string().optional(),
    transaction_type: z.string().optional(),
  })
  .refine((value) => value.date_to >= value.date_from, {
    message: "Tanggal akhir tidak boleh lebih awal dari tanggal mulai",
    path: ["date_to"],
  });

export type LoginInput = z.infer<typeof loginSchema>;
export type RegisterInput = z.infer<typeof registerSchema>;
export type TransactionInput = z.infer<typeof transactionSchema>;
export type CategoryInput = z.infer<typeof categorySchema>;
export type ReportFilterInput = z.infer<typeof reportFilterSchema>;
