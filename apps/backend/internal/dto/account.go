package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type AccountCreateRequest struct {
	Name           string          `json:"name" validate:"required,min=2,max=100" example:"Dompet Harian"`
	AccountType    string          `json:"account_type" validate:"required,oneof=CASH_FLOW WALLET BANK CREDIT_CARD SAVINGS" example:"WALLET"`
	OpeningBalance decimal.Decimal `json:"opening_balance" swaggertype:"string" example:"0.00"`
	Description    string          `json:"description" validate:"max=255" example:"Allowance wallet"`
}

type AccountUpdateRequest = AccountCreateRequest

type AccountStatusRequest struct {
	IsActive *bool `json:"is_active" validate:"required" example:"false"`
}

type AccountResponse struct {
	ID             int64           `json:"id" example:"1"`
	Name           string          `json:"name" example:"Dompet Harian"`
	AccountType    string          `json:"account_type" example:"WALLET"`
	OpeningBalance decimal.Decimal `json:"opening_balance" swaggertype:"string" example:"0.00"`
	Description    string          `json:"description" example:"Allowance wallet"`
	IsActive       bool            `json:"is_active" example:"true"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type AccountListQuery struct {
	AccountType string
	IsActive    *bool
	Search      string
	Page        int
	PageSize    int
	SortBy      string
	SortDir     string
}

type BalanceSnapshotRequest struct {
	AsOfDate      string          `json:"as_of_date" validate:"required,datetime=2006-01-02" example:"2026-09-25"`
	ActualBalance decimal.Decimal `json:"actual_balance" validate:"required" swaggertype:"string" example:"72500.00"`
	Note          string          `json:"note" validate:"max=255" example:"Counted by hand"`
}

type BalanceSnapshotResponse struct {
	ID            int64           `json:"id" example:"1"`
	AccountID     int64           `json:"account_id" example:"1"`
	AccountName   string          `json:"account_name" example:"Dompet Harian"`
	AsOfDate      string          `json:"as_of_date" example:"2026-09-25"`
	ActualBalance decimal.Decimal `json:"actual_balance" swaggertype:"string" example:"72500.00"`
	Note          string          `json:"note" example:"Counted by hand"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}
