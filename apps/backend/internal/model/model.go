package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type TransactionType string

const (
	TransactionTypeIncome   TransactionType = "INCOME"
	TransactionTypeExpense  TransactionType = "EXPENSE"
	TransactionTypeTransfer TransactionType = "TRANSFER"
)

func (t TransactionType) Valid() bool {
	switch t {
	case TransactionTypeIncome, TransactionTypeExpense, TransactionTypeTransfer:
		return true
	}
	return false
}

// RequiresCategory reports whether a transaction of this type must carry a category.
// A transfer only moves money between accounts, so it is not classified.
func (t TransactionType) RequiresCategory() bool {
	return t == TransactionTypeIncome || t == TransactionTypeExpense
}

type CategoryType string

const (
	CategoryTypeIncome  CategoryType = "INCOME"
	CategoryTypeExpense CategoryType = "EXPENSE"
)

func (c CategoryType) Valid() bool {
	return c == CategoryTypeIncome || c == CategoryTypeExpense
}

type User struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Name         string    `gorm:"column:name;size:150;not null"`
	Email        string    `gorm:"column:email;size:255;not null;uniqueIndex:uq_users_email"`
	PasswordHash string    `gorm:"column:password_hash;size:255;not null"`
	IsActive     bool      `gorm:"column:is_active;not null;default:1"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null"`
}

func (User) TableName() string { return "users" }

type Category struct {
	ID          int64        `gorm:"column:id;primaryKey;autoIncrement"`
	UserID      int64        `gorm:"column:user_id;not null;index:ix_categories_user_type"`
	Name        string       `gorm:"column:name;size:100;not null"`
	Type        CategoryType `gorm:"column:type;size:10;not null;index:ix_categories_user_type"`
	Description string       `gorm:"column:description;size:255"`
	IsActive    bool         `gorm:"column:is_active;not null;default:1;index:ix_categories_user_type"`
	CreatedAt   time.Time    `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time    `gorm:"column:updated_at;not null"`
}

func (Category) TableName() string { return "categories" }

type Transaction struct {
	ID              int64           `gorm:"column:id;primaryKey;autoIncrement"`
	UserID          int64           `gorm:"column:user_id;not null;index:ix_transactions_user_date"`
	TransactionDate time.Time       `gorm:"column:transaction_date;type:date;not null;index:ix_transactions_user_date"`
	TransactionType TransactionType `gorm:"column:transaction_type;size:10;not null"`
	CategoryID      *int64          `gorm:"column:category_id"`
	Amount          decimal.Decimal `gorm:"column:amount;type:decimal(18,2);not null"`
	Description     string          `gorm:"column:description;size:500"`
	ReferenceNumber string          `gorm:"column:reference_number;size:100"`
	AccountID       *int64          `gorm:"column:account_id"`
	ToAccountID     *int64          `gorm:"column:to_account_id"`
	ParentID        *int64          `gorm:"column:parent_id"`
	CreatedAt       time.Time       `gorm:"column:created_at;not null"`
	UpdatedAt       time.Time       `gorm:"column:updated_at;not null"`

	Category  *Category     `gorm:"foreignKey:CategoryID;references:ID"`
	Account   *Account      `gorm:"foreignKey:AccountID;references:ID"`
	ToAccount *Account      `gorm:"foreignKey:ToAccountID;references:ID"`
	Children  []Transaction `gorm:"foreignKey:ParentID;references:ID"`
}

func (Transaction) TableName() string { return "transactions" }

// CategoryName returns the joined category name, or an empty string for a transfer or a
// transaction whose category was not preloaded.
func (t Transaction) CategoryName() string {
	if t.Category == nil {
		return ""
	}
	return t.Category.Name
}

// AccountName returns the joined account name, or an empty string when the transaction is
// not tied to an account or the relation was not preloaded.
func (t Transaction) AccountName() string {
	if t.Account == nil {
		return ""
	}
	return t.Account.Name
}
