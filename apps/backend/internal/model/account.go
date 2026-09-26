package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// AccountType decides which part of the daily report an account appears in, so it is a
// reporting role rather than a label for the instrument.
type AccountType string

const (
	// AccountTypeCashFlow holds the household budget: it supplies the opening balance and
	// the "pengeluaran cash flow" lines.
	AccountTypeCashFlow AccountType = "CASH_FLOW"
	// AccountTypeWallet is an allowance that gets topped up and then spent, and whose
	// recorded spending is reconciled against a counted balance.
	AccountTypeWallet AccountType = "WALLET"
	// AccountTypeBank gets a mutation section: money in, fees, money out.
	AccountTypeBank AccountType = "BANK"
	// AccountTypeCreditCard gets the payment section: bill paid against money taken back.
	AccountTypeCreditCard AccountType = "CREDIT_CARD"
	// AccountTypeSavings is a top-up destination that is set aside rather than spent.
	AccountTypeSavings AccountType = "SAVINGS"
)

func (a AccountType) Valid() bool {
	switch a {
	case AccountTypeCashFlow, AccountTypeWallet, AccountTypeBank, AccountTypeCreditCard, AccountTypeSavings:
		return true
	}
	return false
}

// NeedsReconciliation reports whether the report should compare recorded spending against
// an observed balance for this type.
func (a AccountType) NeedsReconciliation() bool {
	return a == AccountTypeWallet || a == AccountTypeBank
}

func AccountTypes() []AccountType {
	return []AccountType{
		AccountTypeCashFlow,
		AccountTypeWallet,
		AccountTypeBank,
		AccountTypeCreditCard,
		AccountTypeSavings,
	}
}

type Account struct {
	ID             int64           `gorm:"column:id;primaryKey;autoIncrement"`
	UserID         int64           `gorm:"column:user_id;not null;index:ix_accounts_user_type"`
	Name           string          `gorm:"column:name;size:100;not null"`
	AccountType    AccountType     `gorm:"column:account_type;size:20;not null;index:ix_accounts_user_type"`
	OpeningBalance decimal.Decimal `gorm:"column:opening_balance;type:decimal(18,2);not null"`
	Description    string          `gorm:"column:description;size:255"`
	IsActive       bool            `gorm:"column:is_active;not null;default:1"`
	CreatedAt      time.Time       `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time       `gorm:"column:updated_at;not null"`
}

func (Account) TableName() string { return "accounts" }

// AccountBalanceSnapshot is a balance observed in the real world on a given day. The
// report compares it against what the transactions add up to; the difference is spending
// that was never written down.
type AccountBalanceSnapshot struct {
	ID            int64           `gorm:"column:id;primaryKey;autoIncrement"`
	UserID        int64           `gorm:"column:user_id;not null"`
	AccountID     int64           `gorm:"column:account_id;not null"`
	AsOfDate      time.Time       `gorm:"column:as_of_date;type:date;not null"`
	ActualBalance decimal.Decimal `gorm:"column:actual_balance;type:decimal(18,2);not null"`
	Note          string          `gorm:"column:note;size:255"`
	CreatedAt     time.Time       `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time       `gorm:"column:updated_at;not null"`

	Account *Account `gorm:"foreignKey:AccountID;references:ID"`
}

func (AccountBalanceSnapshot) TableName() string { return "account_balance_snapshots" }
