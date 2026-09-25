package dto

import "github.com/shopspring/decimal"

type IncomeVsExpensePoint struct {
	Label   string          `json:"label" example:"2026-09"`
	Income  decimal.Decimal `json:"income" swaggertype:"string" example:"15000000.00"`
	Expense decimal.Decimal `json:"expense" swaggertype:"string" example:"8500000.00"`
}

type CashFlowPoint struct {
	Date    string          `json:"date" example:"2026-09-25"`
	Income  decimal.Decimal `json:"income" swaggertype:"string" example:"15000000.00"`
	Expense decimal.Decimal `json:"expense" swaggertype:"string" example:"0.00"`
	Balance decimal.Decimal `json:"balance" swaggertype:"string" example:"15000000.00"`
}

type DashboardSummaryResponse struct {
	Period             Period                 `json:"period"`
	TotalIncome        decimal.Decimal        `json:"total_income" swaggertype:"string" example:"15000000.00"`
	TotalExpense       decimal.Decimal        `json:"total_expense" swaggertype:"string" example:"8500000.00"`
	Balance            decimal.Decimal        `json:"balance" swaggertype:"string" example:"6500000.00"`
	TransactionCount   int64                  `json:"transaction_count" example:"125"`
	IncomeVsExpense    []IncomeVsExpensePoint `json:"income_vs_expense"`
	ExpenseByCategory  []ExpenseByCategoryRow `json:"expense_by_category"`
	CashFlowTrend      []CashFlowPoint        `json:"cash_flow_trend"`
	RecentTransactions []TransactionResponse  `json:"recent_transactions"`
}
