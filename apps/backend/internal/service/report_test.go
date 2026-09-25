package service

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/repository"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

func septemberFilter() dto.ReportFilter {
	return dto.ReportFilter{DateFrom: dateOf("2026-09-01"), DateTo: dateOf("2026-09-30")}
}

// seedCashFlow inserts income, expense and transfer rows directly through the fake
// repository so the report service sees a known period.
func seedCashFlow(transactions *fakeTransactionRepository, categories *fakeCategoryRepository) {
	salary := categories.seed(ownerID, "Salary", model.CategoryTypeIncome, true)
	kpr := categories.seed(ownerID, "Cicilan Rumah", model.CategoryTypeExpense, true)
	food := categories.seed(ownerID, "Food", model.CategoryTypeExpense, true)

	rows := []model.Transaction{
		{UserID: ownerID, TransactionDate: dateOf("2026-09-25"), TransactionType: model.TransactionTypeIncome, CategoryID: &salary.ID, Amount: decimalOf("15000000"), Description: "Salary"},
		{UserID: ownerID, TransactionDate: dateOf("2026-09-25"), TransactionType: model.TransactionTypeExpense, CategoryID: &kpr.ID, Amount: decimalOf("1150000"), Description: "Cicilan Rumah"},
		{UserID: ownerID, TransactionDate: dateOf("2026-09-26"), TransactionType: model.TransactionTypeExpense, CategoryID: &food.ID, Amount: decimalOf("350000"), Description: "Groceries"},
		// A transfer must not move the balance.
		{UserID: ownerID, TransactionDate: dateOf("2026-09-27"), TransactionType: model.TransactionTypeTransfer, Amount: decimalOf("2000000"), Description: "To savings"},
		// Another user's row must never appear.
		{UserID: intruderID, TransactionDate: dateOf("2026-09-25"), TransactionType: model.TransactionTypeIncome, CategoryID: &salary.ID, Amount: decimalOf("99000000"), Description: "Theirs"},
	}
	for index := range rows {
		row := rows[index]
		_ = transactions.Create(context.Background(), &row)
	}
}

func TestCashFlowRunningBalanceSkipsTransfers(t *testing.T) {
	categories := newFakeCategoryRepository()
	transactions := newFakeTransactionRepository(categories)
	seedCashFlow(transactions, categories)

	service := NewReportService(&fakeReportRepository{}, transactions)

	report, err := service.CashFlow(context.Background(), ownerID, septemberFilter())
	if err != nil {
		t.Fatalf("CashFlow returned error: %v", err)
	}

	if len(report.Rows) != 4 {
		t.Fatalf("rows = %d, want 4 for the owner only", len(report.Rows))
	}

	wantBalances := []string{"15000000", "13850000", "13500000", "13500000"}
	for index, want := range wantBalances {
		if !report.Rows[index].Balance.Equal(decimalOf(want)) {
			t.Fatalf("row %d balance = %s, want %s", index, report.Rows[index].Balance, want)
		}
	}

	// The transfer row carries no income and no expense.
	transfer := report.Rows[3]
	if !transfer.Income.IsZero() || !transfer.Expense.IsZero() {
		t.Fatalf("transfer row = income %s expense %s, want both zero", transfer.Income, transfer.Expense)
	}

	if !report.TotalIncome.Equal(decimalOf("15000000")) {
		t.Fatalf("total income = %s, want 15000000", report.TotalIncome)
	}
	if !report.TotalExpense.Equal(decimalOf("1500000")) {
		t.Fatalf("total expense = %s, want 1500000", report.TotalExpense)
	}
	if !report.NetCashFlow.Equal(decimalOf("13500000")) {
		t.Fatalf("net cash flow = %s, want 13500000", report.NetCashFlow)
	}
}

func TestCashFlowEmptyPeriod(t *testing.T) {
	categories := newFakeCategoryRepository()
	transactions := newFakeTransactionRepository(categories)
	service := NewReportService(&fakeReportRepository{}, transactions)

	report, err := service.CashFlow(context.Background(), ownerID, septemberFilter())
	if err != nil {
		t.Fatalf("CashFlow returned error: %v", err)
	}
	if len(report.Rows) != 0 {
		t.Fatalf("rows = %d, want 0", len(report.Rows))
	}
	if !report.NetCashFlow.IsZero() {
		t.Fatalf("net cash flow = %s, want 0", report.NetCashFlow)
	}
	if report.Rows == nil {
		t.Fatal("rows should be an empty slice so the JSON is [] rather than null")
	}
}

func TestSummaryUsesAggregates(t *testing.T) {
	reports := &fakeReportRepository{
		totals: repository.Totals{
			TotalIncome:      decimalOf("15000000"),
			TotalExpense:     decimalOf("8500000"),
			TransactionCount: 125,
		},
	}
	service := NewReportService(reports, newFakeTransactionRepository(nil))

	summary, err := service.Summary(context.Background(), ownerID, septemberFilter())
	if err != nil {
		t.Fatalf("Summary returned error: %v", err)
	}
	if !summary.NetCashFlow.Equal(decimalOf("6500000")) {
		t.Fatalf("net cash flow = %s, want 6500000", summary.NetCashFlow)
	}
	if summary.Period.From != "2026-09-01" || summary.Period.To != "2026-09-30" {
		t.Fatalf("period = %+v, want the requested range", summary.Period)
	}
}

func TestExpenseByCategoryPercentages(t *testing.T) {
	first := int64(5)
	second := int64(6)
	reports := &fakeReportRepository{
		byCategor: []repository.CategoryTotal{
			{CategoryID: &first, CategoryName: "Cicilan Rumah", Total: decimalOf("1150000")},
			{CategoryID: &second, CategoryName: "Food", Total: decimalOf("350000")},
		},
	}
	service := NewReportService(reports, newFakeTransactionRepository(nil))

	report, err := service.ExpenseByCategory(context.Background(), ownerID, septemberFilter())
	if err != nil {
		t.Fatalf("ExpenseByCategory returned error: %v", err)
	}
	if !report.Total.Equal(decimalOf("1500000")) {
		t.Fatalf("total = %s, want 1500000", report.Total)
	}

	// 1150000 / 1500000 is 76.666..., rounded to two places.
	if !report.Rows[0].Percentage.Equal(decimalOf("76.67")) {
		t.Fatalf("first percentage = %s, want 76.67", report.Rows[0].Percentage)
	}
	if !report.Rows[1].Percentage.Equal(decimalOf("23.33")) {
		t.Fatalf("second percentage = %s, want 23.33", report.Rows[1].Percentage)
	}
}

func TestExpenseByCategoryWithoutSpending(t *testing.T) {
	service := NewReportService(&fakeReportRepository{}, newFakeTransactionRepository(nil))

	report, err := service.ExpenseByCategory(context.Background(), ownerID, septemberFilter())
	if err != nil {
		t.Fatalf("ExpenseByCategory returned error: %v", err)
	}
	// Dividing by a zero total must not panic or produce NaN.
	if !report.Total.IsZero() {
		t.Fatalf("total = %s, want 0", report.Total)
	}
	if len(report.Rows) != 0 {
		t.Fatalf("rows = %d, want 0", len(report.Rows))
	}
}

func TestMonthlyReport(t *testing.T) {
	reports := &fakeReportRepository{
		monthly: []repository.PeriodTotal{
			{Label: "2026-08", Income: decimalOf("14000000"), Expense: decimalOf("9000000")},
			{Label: "2026-09", Income: decimalOf("15000000"), Expense: decimalOf("8500000")},
		},
	}
	service := NewReportService(reports, newFakeTransactionRepository(nil))

	report, err := service.Monthly(context.Background(), ownerID, septemberFilter())
	if err != nil {
		t.Fatalf("Monthly returned error: %v", err)
	}
	if len(report.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(report.Rows))
	}
	if !report.Rows[0].NetCashFlow.Equal(decimalOf("5000000")) {
		t.Fatalf("August net = %s, want 5000000", report.Rows[0].NetCashFlow)
	}
	if !report.Rows[1].NetCashFlow.Equal(decimalOf("6500000")) {
		t.Fatalf("September net = %s, want 6500000", report.Rows[1].NetCashFlow)
	}
}

func TestDashboardBuildsEverySeries(t *testing.T) {
	categories := newFakeCategoryRepository()
	transactions := newFakeTransactionRepository(categories)
	seedCashFlow(transactions, categories)

	categoryID := int64(5)
	reports := &fakeReportRepository{
		totals: repository.Totals{
			TotalIncome:      decimalOf("15000000"),
			TotalExpense:     decimalOf("1500000"),
			TransactionCount: 4,
		},
		monthly:   []repository.PeriodTotal{{Label: "2026-09", Income: decimalOf("15000000"), Expense: decimalOf("1500000")}},
		byCategor: []repository.CategoryTotal{{CategoryID: &categoryID, CategoryName: "Cicilan Rumah", Total: decimalOf("1150000")}},
		daily: []repository.PeriodTotal{
			{Label: "2026-09-25", Income: decimalOf("15000000"), Expense: decimalOf("1150000")},
			{Label: "2026-09-26", Income: decimalOf("0"), Expense: decimalOf("350000")},
		},
	}
	service := NewReportService(reports, transactions)

	dashboard, err := service.Dashboard(context.Background(), ownerID, septemberFilter())
	if err != nil {
		t.Fatalf("Dashboard returned error: %v", err)
	}

	if !dashboard.Balance.Equal(decimalOf("13500000")) {
		t.Fatalf("balance = %s, want 13500000", dashboard.Balance)
	}
	if dashboard.TransactionCount != 4 {
		t.Fatalf("transaction count = %d, want 4", dashboard.TransactionCount)
	}
	if len(dashboard.IncomeVsExpense) != 1 {
		t.Fatalf("income versus expense points = %d, want 1", len(dashboard.IncomeVsExpense))
	}
	if len(dashboard.ExpenseByCategory) != 1 {
		t.Fatalf("expense by category rows = %d, want 1", len(dashboard.ExpenseByCategory))
	}

	// The trend accumulates: 13.85M on the first day, 13.5M after the second.
	if len(dashboard.CashFlowTrend) != 2 {
		t.Fatalf("trend points = %d, want 2", len(dashboard.CashFlowTrend))
	}
	if !dashboard.CashFlowTrend[0].Balance.Equal(decimalOf("13850000")) {
		t.Fatalf("first trend balance = %s, want 13850000", dashboard.CashFlowTrend[0].Balance)
	}
	if !dashboard.CashFlowTrend[1].Balance.Equal(decimalOf("13500000")) {
		t.Fatalf("second trend balance = %s, want 13500000", dashboard.CashFlowTrend[1].Balance)
	}

	if len(dashboard.RecentTransactions) == 0 {
		t.Fatal("recent transactions should not be empty")
	}
	for _, transaction := range dashboard.RecentTransactions {
		if transaction.Description == "Theirs" {
			t.Fatal("another user's transaction leaked into the dashboard")
		}
	}
}

func TestReportFailurePropagates(t *testing.T) {
	reports := &fakeReportRepository{failOn: "Totals"}
	service := NewReportService(reports, newFakeTransactionRepository(nil))

	_, err := service.Summary(context.Background(), ownerID, septemberFilter())
	if err == nil {
		t.Fatal("a repository failure must not be swallowed")
	}
	domainErr, ok := utils.AsDomainError(err)
	if !ok {
		t.Fatalf("error %v is not a domain error", err)
	}
	if !errors.Is(domainErr.Cause(), errBoom) {
		t.Fatalf("cause = %v, want the repository failure", domainErr.Cause())
	}
}

func TestCashFlowExcelProducesAWorkbook(t *testing.T) {
	categories := newFakeCategoryRepository()
	transactions := newFakeTransactionRepository(categories)
	seedCashFlow(transactions, categories)
	service := NewReportService(&fakeReportRepository{}, transactions)

	content, filename, err := service.CashFlowExcel(context.Background(), ownerID, septemberFilter())
	if err != nil {
		t.Fatalf("CashFlowExcel returned error: %v", err)
	}
	if filename != "cash-flow-2026-09-01-2026-09-30.xlsx" {
		t.Fatalf("filename = %q", filename)
	}
	if len(content) == 0 {
		t.Fatal("workbook is empty")
	}
	// An xlsx file is a zip archive, so it starts with the PK signature.
	if !bytes.HasPrefix(content, []byte("PK")) {
		t.Fatalf("content does not look like an xlsx file, first bytes: %q", content[:min(4, len(content))])
	}
}

func TestCashFlowPDFProducesADocument(t *testing.T) {
	categories := newFakeCategoryRepository()
	transactions := newFakeTransactionRepository(categories)
	seedCashFlow(transactions, categories)
	service := NewReportService(&fakeReportRepository{}, transactions)

	content, filename, err := service.CashFlowPDF(context.Background(), ownerID, septemberFilter())
	if err != nil {
		t.Fatalf("CashFlowPDF returned error: %v", err)
	}
	if filename != "cash-flow-2026-09-01-2026-09-30.pdf" {
		t.Fatalf("filename = %q", filename)
	}
	if !bytes.HasPrefix(content, []byte("%PDF-")) {
		t.Fatalf("content does not start with the PDF header, first bytes: %q", content[:min(8, len(content))])
	}
}

func TestCashFlowPDFWithNoRows(t *testing.T) {
	service := NewReportService(&fakeReportRepository{}, newFakeTransactionRepository(nil))

	content, _, err := service.CashFlowPDF(context.Background(), ownerID, septemberFilter())
	if err != nil {
		t.Fatalf("CashFlowPDF returned error: %v", err)
	}
	// An empty period still has to render a valid document with the placeholder row.
	if !bytes.HasPrefix(content, []byte("%PDF-")) {
		t.Fatal("an empty report did not produce a valid PDF")
	}
}

func TestFormatRupiah(t *testing.T) {
	cases := []struct {
		amount string
		want   string
	}{
		{amount: "15000000", want: "Rp 15.000.000"},
		{amount: "1150000", want: "Rp 1.150.000"},
		{amount: "0", want: "Rp 0"},
		{amount: "999", want: "Rp 999"},
		{amount: "1000", want: "Rp 1.000"},
		{amount: "-1500000", want: "-Rp 1.500.000"},
	}

	for _, testCase := range cases {
		if got := formatRupiah(decimalOf(testCase.amount)); got != testCase.want {
			t.Fatalf("formatRupiah(%s) = %q, want %q", testCase.amount, got, testCase.want)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
