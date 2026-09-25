package service

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/repository"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

// recentTransactionLimit is how many rows the dashboard shows under "Recent Transactions".
const recentTransactionLimit = 5

type ReportService interface {
	Summary(ctx context.Context, userID int64, filter dto.ReportFilter) (dto.SummaryResponse, error)
	CashFlow(ctx context.Context, userID int64, filter dto.ReportFilter) (dto.CashFlowResponse, error)
	ExpenseByCategory(ctx context.Context, userID int64, filter dto.ReportFilter) (dto.ExpenseByCategoryResponse, error)
	Monthly(ctx context.Context, userID int64, filter dto.ReportFilter) (dto.MonthlyResponse, error)
	Dashboard(ctx context.Context, userID int64, filter dto.ReportFilter) (dto.DashboardSummaryResponse, error)
	CashFlowExcel(ctx context.Context, userID int64, filter dto.ReportFilter) ([]byte, string, error)
	CashFlowPDF(ctx context.Context, userID int64, filter dto.ReportFilter) ([]byte, string, error)
}

type reportService struct {
	reports      repository.ReportRepository
	transactions repository.TransactionRepository
}

func NewReportService(reports repository.ReportRepository, transactions repository.TransactionRepository) ReportService {
	return &reportService{reports: reports, transactions: transactions}
}

func (s *reportService) Summary(ctx context.Context, userID int64, filter dto.ReportFilter) (dto.SummaryResponse, error) {
	totals, err := s.reports.Totals(ctx, userID, filter)
	if err != nil {
		return dto.SummaryResponse{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to build summary", err)
	}

	income := utils.Round(totals.TotalIncome)
	expense := utils.Round(totals.TotalExpense)

	return dto.SummaryResponse{
		Period:       period(filter),
		TotalIncome:  income,
		TotalExpense: expense,
		NetCashFlow:  utils.Balance(income, expense),
	}, nil
}

func (s *reportService) CashFlow(ctx context.Context, userID int64, filter dto.ReportFilter) (dto.CashFlowResponse, error) {
	transactions, err := s.transactions.ListForReport(ctx, userID, filter)
	if err != nil {
		return dto.CashFlowResponse{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to build cash flow report", err)
	}

	rows := make([]dto.CashFlowRow, 0, len(transactions))
	running := utils.Zero()
	totalIncome := utils.Zero()
	totalExpense := utils.Zero()

	for _, transaction := range transactions {
		income, expense := split(transaction)
		totalIncome = totalIncome.Add(income)
		totalExpense = totalExpense.Add(expense)
		running = utils.Round(running.Add(income).Sub(expense))

		rows = append(rows, dto.CashFlowRow{
			TransactionDate: utils.FormatDate(transaction.TransactionDate),
			TransactionType: string(transaction.TransactionType),
			CategoryName:    transaction.CategoryName(),
			Description:     transaction.Description,
			ReferenceNumber: transaction.ReferenceNumber,
			Income:          income,
			Expense:         expense,
			Balance:         running,
		})
	}

	totalIncome = utils.Round(totalIncome)
	totalExpense = utils.Round(totalExpense)

	return dto.CashFlowResponse{
		Period:       period(filter),
		TotalIncome:  totalIncome,
		TotalExpense: totalExpense,
		NetCashFlow:  utils.Balance(totalIncome, totalExpense),
		Rows:         rows,
	}, nil
}

func (s *reportService) ExpenseByCategory(ctx context.Context, userID int64, filter dto.ReportFilter) (dto.ExpenseByCategoryResponse, error) {
	rows, err := s.expenseByCategory(ctx, userID, filter)
	if err != nil {
		return dto.ExpenseByCategoryResponse{}, err
	}

	total := utils.Zero()
	for _, row := range rows {
		total = total.Add(row.Total)
	}

	return dto.ExpenseByCategoryResponse{
		Period: period(filter),
		Total:  utils.Round(total),
		Rows:   rows,
	}, nil
}

func (s *reportService) Monthly(ctx context.Context, userID int64, filter dto.ReportFilter) (dto.MonthlyResponse, error) {
	totals, err := s.reports.Monthly(ctx, userID, filter)
	if err != nil {
		return dto.MonthlyResponse{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to build monthly report", err)
	}

	rows := make([]dto.MonthlyRow, 0, len(totals))
	for _, item := range totals {
		income := utils.Round(item.Income)
		expense := utils.Round(item.Expense)
		rows = append(rows, dto.MonthlyRow{
			Month:       item.Label,
			Income:      income,
			Expense:     expense,
			NetCashFlow: utils.Balance(income, expense),
		})
	}

	return dto.MonthlyResponse{Period: period(filter), Rows: rows}, nil
}

func (s *reportService) Dashboard(ctx context.Context, userID int64, filter dto.ReportFilter) (dto.DashboardSummaryResponse, error) {
	totals, err := s.reports.Totals(ctx, userID, filter)
	if err != nil {
		return dto.DashboardSummaryResponse{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to build dashboard summary", err)
	}

	income := utils.Round(totals.TotalIncome)
	expense := utils.Round(totals.TotalExpense)

	monthly, err := s.reports.Monthly(ctx, userID, filter)
	if err != nil {
		return dto.DashboardSummaryResponse{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to build income versus expense chart", err)
	}
	incomeVsExpense := make([]dto.IncomeVsExpensePoint, 0, len(monthly))
	for _, item := range monthly {
		incomeVsExpense = append(incomeVsExpense, dto.IncomeVsExpensePoint{
			Label:   item.Label,
			Income:  utils.Round(item.Income),
			Expense: utils.Round(item.Expense),
		})
	}

	byCategory, err := s.expenseByCategory(ctx, userID, filter)
	if err != nil {
		return dto.DashboardSummaryResponse{}, err
	}

	daily, err := s.reports.Daily(ctx, userID, filter)
	if err != nil {
		return dto.DashboardSummaryResponse{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to build cash flow trend", err)
	}
	trend := make([]dto.CashFlowPoint, 0, len(daily))
	running := utils.Zero()
	for _, item := range daily {
		dayIncome := utils.Round(item.Income)
		dayExpense := utils.Round(item.Expense)
		running = utils.Round(running.Add(dayIncome).Sub(dayExpense))
		trend = append(trend, dto.CashFlowPoint{
			Date:    item.Label,
			Income:  dayIncome,
			Expense: dayExpense,
			Balance: running,
		})
	}

	recent, _, err := s.transactions.List(ctx, userID, dto.TransactionFilter{
		DateFrom: &filter.DateFrom,
		DateTo:   &filter.DateTo,
		Page:     1,
		PageSize: recentTransactionLimit,
		SortBy:   "transaction_date",
		SortDir:  "desc",
	})
	if err != nil {
		return dto.DashboardSummaryResponse{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to load recent transactions", err)
	}
	recentResponses := make([]dto.TransactionResponse, 0, len(recent))
	for _, transaction := range recent {
		recentResponses = append(recentResponses, toTransactionResponse(transaction))
	}

	return dto.DashboardSummaryResponse{
		Period:             period(filter),
		TotalIncome:        income,
		TotalExpense:       expense,
		Balance:            utils.Balance(income, expense),
		TransactionCount:   totals.TransactionCount,
		IncomeVsExpense:    incomeVsExpense,
		ExpenseByCategory:  byCategory,
		CashFlowTrend:      trend,
		RecentTransactions: recentResponses,
	}, nil
}

func (s *reportService) expenseByCategory(ctx context.Context, userID int64, filter dto.ReportFilter) ([]dto.ExpenseByCategoryRow, error) {
	totals, err := s.reports.ExpenseByCategory(ctx, userID, filter)
	if err != nil {
		return nil, utils.WrapDomainError(utils.ErrNotFound, "Failed to build expense by category report", err)
	}

	grand := utils.Zero()
	for _, item := range totals {
		grand = grand.Add(item.Total)
	}

	hundred := decimal.NewFromInt(100)
	rows := make([]dto.ExpenseByCategoryRow, 0, len(totals))
	for _, item := range totals {
		total := utils.Round(item.Total)
		percentage := utils.Zero()
		if grand.IsPositive() {
			percentage = total.Div(grand).Mul(hundred).Round(2)
		}
		rows = append(rows, dto.ExpenseByCategoryRow{
			CategoryID:   item.CategoryID,
			CategoryName: item.CategoryName,
			Total:        total,
			Percentage:   percentage,
		})
	}
	return rows, nil
}

// split turns a transaction into its income and expense contribution. A transfer
// contributes to neither, so it leaves the running balance untouched.
func split(transaction model.Transaction) (decimal.Decimal, decimal.Decimal) {
	amount := utils.Round(transaction.Amount)
	switch transaction.TransactionType {
	case model.TransactionTypeIncome:
		return amount, utils.Zero()
	case model.TransactionTypeExpense:
		return utils.Zero(), amount
	default:
		return utils.Zero(), utils.Zero()
	}
}

func period(filter dto.ReportFilter) dto.Period {
	return dto.Period{
		From: utils.FormatDate(filter.DateFrom),
		To:   utils.FormatDate(filter.DateTo),
	}
}
