package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type TransactionCreateRequest struct {
	TransactionDate string          `json:"transaction_date" validate:"required,datetime=2006-01-02" example:"2026-09-25"`
	TransactionType string          `json:"transaction_type" validate:"required,oneof=INCOME EXPENSE TRANSFER" example:"EXPENSE"`
	CategoryID      *int64          `json:"category_id" example:"5"`
	Amount          decimal.Decimal `json:"amount" validate:"required" swaggertype:"string" example:"1150000.00"`
	Description     string          `json:"description" validate:"max=500" example:"Cicilan Rumah September"`
	ReferenceNumber string          `json:"reference_number" validate:"max=100" example:"INV-0001"`
}

type TransactionUpdateRequest = TransactionCreateRequest

type TransactionResponse struct {
	ID              int64           `json:"id" example:"1"`
	TransactionDate string          `json:"transaction_date" example:"2026-09-25"`
	TransactionType string          `json:"transaction_type" example:"EXPENSE"`
	CategoryID      *int64          `json:"category_id" example:"5"`
	CategoryName    string          `json:"category_name" example:"Cicilan Rumah"`
	Amount          decimal.Decimal `json:"amount" swaggertype:"string" example:"1150000.00"`
	Description     string          `json:"description" example:"Cicilan Rumah September"`
	ReferenceNumber string          `json:"reference_number" example:"INV-0001"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// TransactionFilter is the resolved, validated form of the list query.
type TransactionFilter struct {
	Search          string
	DateFrom        *time.Time
	DateTo          *time.Time
	TransactionType string
	CategoryID      *int64
	Page            int
	PageSize        int
	SortBy          string
	SortDir         string
}
