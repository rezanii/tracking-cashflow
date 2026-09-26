Buatkan aplikasi **Financial Management / Personal Finance** untuk pencatatan keuangan dengan arsitektur **Monorepo**, menggunakan teknologi berikut:

## 1. Tech Stack

### Backend

- Language: **Golang**
- Go version: gunakan versi stable terbaru yang tersedia
- HTTP Router: **go-chi/chi**
- ORM: **GORM**
- Database: **Microsoft SQL Server**
- Authentication: **JWT**
- Password hashing: **bcrypt**
- API documentation: **Swagger/OpenAPI**
- Validation: gunakan library validation yang idiomatis di Go
- Configuration: `.env`
- Logging: structured logging
- Testing: Go testing + integration/unit test
- Migration: gunakan database migration yang proper
- Report generation:
  - Excel: `excelize`
  - PDF: library PDF yang stabil untuk Go

### Frontend

- Framework: **Next.js**
- Language: **TypeScript**
- UI: gunakan component-based architecture
- Styling: **Tailwind CSS**
- Form validation: **React Hook Form + Zod**
- API communication: gunakan fetch/axios dengan abstraction yang jelas
- Authentication: JWT-based authentication
- Responsive design
- Desktop dan mobile friendly

### Database

- Microsoft SQL Server
- Gunakan migration
- Gunakan foreign key dan index yang sesuai
- Gunakan naming convention `snake_case`

---

# 2. Arsitektur Repository

Gunakan struktur monorepo seperti:

```text
financial-management/
│
├── apps/
│   ├── backend/
│   │   ├── cmd/
│   │   │   └── api/
│   │   │       └── main.go
│   │   │
│   │   ├── internal/
│   │   │   ├── config/
│   │   │   ├── handler/
│   │   │   ├── service/
│   │   │   ├── repository/
│   │   │   ├── middleware/
│   │   │   ├── model/
│   │   │   ├── dto/
│   │   │   ├── validator/
│   │   │   ├── utils/
│   │   │   └── router/
│   │   │
│   │   ├── migrations/
│   │   ├── docs/
│   │   ├── tests/
│   │   ├── go.mod
│   │   └── Dockerfile
│   │
│   └── frontend/
│       ├── app/
│       │   ├── login/
│       │   ├── dashboard/
│       │   ├── transactions/
│       │   ├── categories/
│       │   ├── reports/
│       │   └── layout.tsx
│       │
│       ├── components/
│       ├── hooks/
│       ├── services/
│       ├── lib/
│       ├── types/
│       ├── schemas/
│       ├── public/
│       ├── package.json
│       └── Dockerfile
│
├── database/
│   ├── migrations/
│   ├── seeds/
│   └── scripts/
│
├── docs/
│   ├── architecture.md
│   ├── api.md
│   └── database.md
│
├── docker-compose.yml
├── .env.example
├── .gitignore
├── README.md
└── Makefile
```

Pisahkan backend dan frontend dengan jelas tetapi tetap berada dalam satu repository.

---

# 3. Core Features

Aplikasi minimal memiliki menu:

1. Login
2. Dashboard
3. Transactions
4. Categories
5. Reports
6. Logout

Tambahkan sidebar navigation pada halaman setelah login.

---

# 4. Authentication

Buat sistem authentication lengkap.

## Login

Endpoint:

```http
POST /api/v1/auth/login
```

Request:

```json
{
  "email": "user@example.com",
  "password": "password"
}
```

Response:

```json
{
  "success": true,
  "message": "Login successful",
  "data": {
    "access_token": "...",
    "user": {
      "id": 1,
      "name": "John Doe",
      "email": "user@example.com"
    }
  }
}
```

Implementasikan:

- Register
- Login
- Logout
- JWT middleware
- Password hashing bcrypt
- Token expiration
- Authentication middleware
- Current user endpoint

Endpoint:

```http
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/logout
GET  /api/v1/auth/me
```

Gunakan pendekatan security yang baik.

Jangan menyimpan password dalam plain text.

---

# 5. User Management

Buat tabel:

```text
users
```

Field minimal:

```text
id
name
email
password_hash
is_active
created_at
updated_at
```

Email harus unique.

---

# 6. Transaction Management

Fitur utama aplikasi adalah pencatatan transaksi keuangan.

Transaction memiliki:

```text
income
expense
transfer
```

Buat tabel:

```text
transactions
```

Field:

```text
id
user_id
transaction_date
transaction_type
category_id
amount
description
reference_number
created_at
updated_at
```

Gunakan decimal/numeric untuk nominal uang.

Jangan gunakan float untuk menyimpan nilai uang.

SQL Server gunakan misalnya:

```sql
DECIMAL(18,2)
```

---

# 7. Transaction Input

Buat halaman:

```text
Transactions
```

Fitur:

- List transaction
- Add transaction
- Edit transaction
- Delete transaction
- Detail transaction
- Search
- Filter tanggal
- Filter transaction type
- Filter category
- Pagination
- Sorting

Form:

```text
Tanggal
Jenis Transaksi
Kategori
Nominal
Deskripsi
Reference Number
```

Jenis transaksi:

```text
INCOME
EXPENSE
TRANSFER
```

---

# 8. Categories

Buat category management.

Tabel:

```text
categories
```

Field:

```text
id
user_id
name
type
description
is_active
created_at
updated_at
```

Category type:

```text
INCOME
EXPENSE
```

Contoh category:

```text
Salary
Bonus
Food
Transport
Shopping
Bills
Household
Education
Entertainment
Healthcare
Investment
Debt
Other
```

User dapat:

- Create category
- Update category
- Delete category
- Activate/deactivate category

---

# 9. Dashboard

Buat dashboard yang menampilkan financial summary.

Minimal:

### Total Income

```text
Rp 15.000.000
```

### Total Expense

```text
Rp 8.500.000
```

### Balance

```text
Rp 6.500.000
```

### Transaction Count

```text
125 Transactions
```

Dashboard dapat difilter berdasarkan:

```text
Today
This Week
This Month
This Year
Custom Date Range
```

Tampilkan chart:

1. Income vs Expense
2. Expense by Category
3. Cash Flow Trend
4. Recent Transactions

Gunakan chart library yang cocok untuk Next.js.

---

# 10. Financial Calculation

Balance:

```text
Balance = Total Income - Total Expense
```

Untuk transfer, jangan dianggap sebagai income/expense apabila transfer hanya perpindahan antar rekening.

Pastikan seluruh kalkulasi menggunakan decimal/precise arithmetic dan bukan floating-point.

---

# 11. Reports

Buat halaman:

```text
Reports
```

User dapat memilih:

```text
Report Type
Date From
Date To
Category
Transaction Type
```

Report minimal:

### Cash Flow Report

Kolom:

```text
Date
Description
Category
Income
Expense
Balance
```

Contoh:

```text
25/09/2026
Salary
Income
15.000.000
0
15.000.000
```

```text
25/09/2026
Cicilan Rumah
Housing
0
1.150.000
13.850.000
```

---

# 12. Excel Export

Buat endpoint:

```http
GET /api/v1/reports/cash-flow/excel
```

Parameter:

```text
date_from
date_to
category_id
transaction_type
```

Generate file:

```text
cash-flow-2026-09-01-2026-09-30.xlsx
```

Excel harus memiliki:

### Sheet 1 - Summary

```text
Period
Total Income
Total Expense
Net Cash Flow
```

### Sheet 2 - Transactions

```text
Date
Type
Category
Description
Income
Expense
Balance
```

Tambahkan:

- Header formatting
- Currency formatting
- Date formatting
- Auto width
- Total row
- Freeze header
- Proper number format

---

# 13. PDF Export

Buat endpoint:

```http
GET /api/v1/reports/cash-flow/pdf
```

Generate:

```text
cash-flow-2026-09-01-2026-09-30.pdf
```

PDF minimal berisi:

```text
FINANCIAL REPORT

Period:
01 September 2026 - 30 September 2026

Total Income:
Rp 15.000.000

Total Expense:
Rp 8.500.000

Net Cash Flow:
Rp 6.500.000
```

Kemudian tabel:

```text
Date | Type | Category | Description | Income | Expense | Balance
```

Gunakan landscape apabila jumlah kolom terlalu banyak.

---

# 14. Report API

Buat endpoint:

```http
GET /api/v1/reports/cash-flow
GET /api/v1/reports/cash-flow/excel
GET /api/v1/reports/cash-flow/pdf

GET /api/v1/reports/summary
GET /api/v1/reports/expense-by-category
GET /api/v1/reports/monthly
```

API JSON report:

```json
{
  "success": true,
  "data": {
    "period": {
      "from": "2026-09-01",
      "to": "2026-09-30"
    },
    "total_income": 15000000,
    "total_expense": 8500000,
    "net_cash_flow": 6500000
  }
}
```

---

# 15. API Response Standard

Semua API harus memiliki response structure konsisten.

Success:

```json
{
  "success": true,
  "message": "Data retrieved successfully",
  "data": {}
}
```

Error:

```json
{
  "success": false,
  "message": "Validation failed",
  "errors": {
    "email": "Email is required"
  }
}
```

HTTP status harus digunakan dengan benar:

```text
200 OK
201 Created
400 Bad Request
401 Unauthorized
403 Forbidden
404 Not Found
409 Conflict
422 Unprocessable Entity
500 Internal Server Error
```

---

# 16. Backend Architecture

Gunakan flow:

```text
HTTP Request
     ↓
Router
     ↓
Middleware
     ↓
Handler
     ↓
Service
     ↓
Repository
     ↓
GORM
     ↓
SQL Server
```

Jangan meletakkan business logic di handler.

Handler hanya bertanggung jawab untuk:

- Parse request
- Validate request
- Call service
- Return response

Service bertanggung jawab terhadap:

- Business logic
- Calculation
- Transaction orchestration

Repository bertanggung jawab terhadap:

- Database access
- Query
- Persistence

---

# 17. Database Transaction

Untuk operasi yang membutuhkan lebih dari satu database operation gunakan SQL transaction:

```text
BEGIN TRANSACTION
    operation 1
    operation 2
    operation 3
COMMIT
```

Jika terjadi error:

```text
ROLLBACK
```

Pastikan repository/service mendukung transaction context.

---

# 18. Database Index

Buat index untuk field yang sering digunakan untuk filter:

```text
users.email
transactions.user_id
transactions.transaction_date
transactions.category_id
transactions.transaction_type
categories.user_id
```

Gunakan composite index jika memang diperlukan berdasarkan query pattern.

---

# 19. Frontend Architecture

Next.js gunakan App Router.

Struktur:

```text
app/
├── login/
├── dashboard/
├── transactions/
│   ├── page.tsx
│   ├── create/
│   └── [id]/
├── categories/
├── reports/
└── layout.tsx
```

Buat reusable components:

```text
DataTable
Modal
Button
Input
Select
DatePicker
CurrencyInput
Pagination
Loading
EmptyState
ErrorState
StatCard
```

---

# 20. Dashboard UI

Buat dashboard modern dan clean.

Layout:

```text
------------------------------------------------
| Sidebar | Header                             |
|         |                                    |
|         | Total Income | Total Expense       |
|         | --------------------------------- |
|         | Balance       | Transactions       |
|         |                                    |
|         | Income vs Expense Chart            |
|         |                                    |
|         | Expense by Category                |
|         |                                    |
|         | Recent Transactions                |
------------------------------------------------
```

Gunakan responsive design.

---

# 21. Transaction UI

Table:

```text
Date
Type
Category
Description
Amount
Actions
```

Untuk expense tampilkan sebagai pengeluaran.

Untuk income tampilkan sebagai pemasukan.

Tambahkan:

```text
+ Add Transaction
```

Button.

---

# 22. Report UI

Halaman report memiliki filter:

```text
Date From
Date To
Category
Transaction Type
```

Button:

```text
Generate Report
Export Excel
Export PDF
```

Report dapat ditampilkan terlebih dahulu di browser sebelum di-export.

---

# 23. Security

Implementasikan minimal:

- bcrypt password hashing
- JWT authentication
- Authorization middleware
- Input validation
- SQL injection protection melalui GORM parameterized query
- CORS configuration
- Secure HTTP headers
- Rate limiting untuk authentication endpoint
- Jangan expose password hash
- Jangan expose JWT secret
- Environment-based configuration
- Proper error handling tanpa membocorkan database error ke client

User hanya boleh melihat dan mengubah data miliknya sendiri.

Contoh:

```text
WHERE user_id = current_user_id
```

Jangan pernah mengambil transaction berdasarkan ID saja tanpa memvalidasi ownership.

---

# 24. Environment Configuration

Buat:

```text
.env.example
```

Contoh:

```env
APP_ENV=development
APP_PORT=8080

DB_HOST=localhost
DB_PORT=1433
DB_USER=sa
DB_PASSWORD=YourStrongPassword
DB_NAME=financial_management

JWT_SECRET=change-me
JWT_EXPIRATION=24h

NEXT_PUBLIC_API_URL=http://localhost:8080/api/v1
```

Jangan commit `.env`.

---

# 25. Docker

Buat Docker Compose untuk:

```text
frontend
backend
sqlserver
```

Contoh:

```text
docker-compose.yml
```

Pastikan seluruh aplikasi dapat dijalankan dengan:

```bash
docker compose up -d
```

---

# 26. Makefile

Buat command:

```bash
make run
make test
make build
make migrate
make swagger
make lint
make docker-up
make docker-down
```

---

# 27. Testing

Backend minimal memiliki:

### Unit Test

Test:

```text
Authentication Service
Transaction Service
Category Service
Report Service
Financial Calculation
```

### Repository Test

Test query utama.

### API Integration Test

Test:

```text
Register
Login
Create transaction
Get transaction
Update transaction
Delete transaction
Generate report
Export Excel
Export PDF
```

Pastikan test juga mencakup:

```text
Unauthorized access
Invalid token
Invalid request
Transaction ownership
Empty data
Pagination
Date filtering
```

---

# 28. Swagger

Expose Swagger:

```text
/swagger/index.html
```

Dokumentasikan seluruh endpoint:

```text
Auth
Users
Transactions
Categories
Reports
```

---

# 29. Seed Data

Buat database seed untuk development.

Default user:

```text
email:
admin@example.com

password:
Admin123!
```

Buat beberapa category:

```text
Salary
Bonus
Food
Transport
Cicilan Rumah
Household
Shopping
Entertainment
Other
```

Dan contoh transaksi.

Jangan gunakan seed password tersebut untuk production.

---

# 30. UX Requirements

Gunakan bahasa UI yang sederhana dan mudah dipahami.

Dashboard menggunakan format Rupiah:

```text
Rp 15.000.000
```

Date format:

```text
DD/MM/YYYY
```

Gunakan timezone:

```text
Asia/Jakarta
```

Currency:

```text
IDR
```

---

# 31. Deliverables

Jangan hanya membuat prototype UI.

Implementasikan aplikasi sampai dapat dijalankan.

Deliverables:

1. Complete monorepo
2. Backend Go
3. Frontend Next.js
4. SQL Server schema
5. Migration
6. Seed
7. REST API
8. JWT authentication
9. Dashboard
10. Transaction CRUD
11. Category CRUD
12. Cash flow report
13. Excel export
14. PDF export
15. Swagger
16. Unit test
17. Integration test
18. Docker Compose
19. `.env.example`
20. Makefile
21. README

---

# 32. Development Rules

Ikuti prinsip:

- Clean Architecture
- SOLID
- Separation of concerns
- DRY
- KISS
- Secure by default
- Production-ready code
- Avoid unnecessary abstraction
- Avoid over-engineering
- Use dependency injection where useful
- Use context.Context
- Handle errors explicitly
- Do not ignore errors
- Do not use `panic` for normal application errors
- Do not put SQL directly inside HTTP handlers
- Do not put business logic inside repositories

---

# 33. Important Implementation Requirement

Sebelum menulis kode:

1. Buat architecture design.
2. Buat database ERD.
3. Buat database schema.
4. Buat API endpoint specification.
5. Buat folder structure.
6. Tentukan dependency.
7. Baru implementasikan backend.
8. Setelah backend stabil, implementasikan frontend.
9. Implementasikan report.
10. Implementasikan Excel/PDF export.
11. Tambahkan testing.
12. Tambahkan Docker.
13. Jalankan build dan test.
14. Perbaiki semua error.
15. Pastikan aplikasi dapat dijalankan dari README.

Jangan membuat fake API atau mock data sebagai implementasi final.

Semua halaman frontend harus terhubung ke backend API nyata.

Semua data transaksi harus tersimpan di SQL Server.

---

# 34. Expected Final Flow

User membuka aplikasi:

```text
Login
  ↓
Dashboard
  ↓
Transactions
  ↓
Add Income / Expense
  ↓
Data tersimpan ke SQL Server
  ↓
Dashboard otomatis menghitung balance
  ↓
Reports
  ↓
Filter tanggal
  ↓
Generate Cash Flow
  ↓
Preview Report
  ↓
Export Excel / PDF
```

---

# 35. Definition of Done

Aplikasi dianggap selesai apabila:

- `docker compose up -d` berhasil menjalankan environment.
- Backend berhasil connect ke SQL Server.
- Frontend berhasil connect ke backend.
- User dapat register.
- User dapat login.
- User dapat logout.
- JWT authentication berjalan.
- User dapat membuat transaction.
- User dapat edit transaction.
- User dapat delete transaction.
- User dapat melihat transaction.
- User dapat membuat category.
- Dashboard menampilkan income, expense, dan balance.
- Filter tanggal bekerja.
- Cash flow report bekerja.
- Excel export menghasilkan file `.xlsx` valid.
- PDF export menghasilkan file `.pdf` valid.
- Swagger dapat diakses.
- Unit test berhasil.
- Integration test berhasil.
- Tidak ada credential atau secret hardcoded.
- README berisi langkah menjalankan project dari awal.

Setelah implementasi selesai, berikan:

1. Architecture overview
2. Folder structure
3. Database ERD
4. API endpoint list
5. Cara menjalankan project
6. Cara menjalankan migration
7. Cara menjalankan test
8. Contoh login
9. Contoh API request
10. Contoh generate Excel/PDF
11. Known limitations jika ada
12. Next improvement yang direkomendasikan

Gunakan **real implementation**, bukan pseudo-code.
Pastikan seluruh kode yang dibuat dapat di-build dan dijalankan.
