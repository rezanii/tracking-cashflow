package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type Period struct {
	From string `json:"from" example:"2026-09-01"`
	To   string `json:"to" example:"2026-09-30"`
}

// ReportFilter is shared by every report endpoint and by the export handlers.
type ReportFilter struct {
	DateFrom        time.Time
	DateTo          time.Time
	CategoryID      *int64
	TransactionType string
}

type SummaryResponse struct {
	Period       Period          `json:"period"`
	TotalIncome  decimal.Decimal `json:"total_income" swaggertype:"string" example:"15000000.00"`
	TotalExpense decimal.Decimal `json:"total_expense" swaggertype:"string" example:"8500000.00"`
	NetCashFlow  decimal.Decimal `json:"net_cash_flow" swaggertype:"string" example:"6500000.00"`
}

type CashFlowRow struct {
	TransactionDate string          `json:"transaction_date" example:"2026-09-25"`
	TransactionType string          `json:"transaction_type" example:"INCOME"`
	CategoryName    string          `json:"category_name" example:"Salary"`
	Description     string          `json:"description" example:"Salary September"`
	ReferenceNumber string          `json:"reference_number" example:"INV-0001"`
	Income          decimal.Decimal `json:"income" swaggertype:"string" example:"15000000.00"`
	Expense         decimal.Decimal `json:"expense" swaggertype:"string" example:"0.00"`
	Balance         decimal.Decimal `json:"balance" swaggertype:"string" example:"15000000.00"`
}

type CashFlowResponse struct {
	Period       Period          `json:"period"`
	TotalIncome  decimal.Decimal `json:"total_income" swaggertype:"string" example:"15000000.00"`
	TotalExpense decimal.Decimal `json:"total_expense" swaggertype:"string" example:"8500000.00"`
	NetCashFlow  decimal.Decimal `json:"net_cash_flow" swaggertype:"string" example:"6500000.00"`
	Rows         []CashFlowRow   `json:"rows"`
}

type ExpenseByCategoryRow struct {
	CategoryID   *int64          `json:"category_id" example:"5"`
	CategoryName string          `json:"category_name" example:"Cicilan Rumah"`
	Total        decimal.Decimal `json:"total" swaggertype:"string" example:"1150000.00"`
	Percentage   decimal.Decimal `json:"percentage" swaggertype:"string" example:"13.53"`
}

type ExpenseByCategoryResponse struct {
	Period Period                 `json:"period"`
	Total  decimal.Decimal        `json:"total" swaggertype:"string" example:"8500000.00"`
	Rows   []ExpenseByCategoryRow `json:"rows"`
}

type MonthlyRow struct {
	Month       string          `json:"month" example:"2026-09"`
	Income      decimal.Decimal `json:"income" swaggertype:"string" example:"15000000.00"`
	Expense     decimal.Decimal `json:"expense" swaggertype:"string" example:"8500000.00"`
	NetCashFlow decimal.Decimal `json:"net_cash_flow" swaggertype:"string" example:"6500000.00"`
}

type MonthlyResponse struct {
	Period Period       `json:"period"`
	Rows   []MonthlyRow `json:"rows"`
}
