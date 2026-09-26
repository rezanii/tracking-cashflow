package repository

import (
	"context"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
)

// Totals holds the period aggregate. Transfers are counted in TransactionCount but excluded
// from both money columns, because moving funds between accounts is not income or expense.
type Totals struct {
	TotalIncome      decimal.Decimal `gorm:"column:total_income"`
	TotalExpense     decimal.Decimal `gorm:"column:total_expense"`
	TransactionCount int64           `gorm:"column:transaction_count"`
}

type CategoryTotal struct {
	CategoryID   *int64          `gorm:"column:category_id"`
	CategoryName string          `gorm:"column:category_name"`
	Total        decimal.Decimal `gorm:"column:total"`
}

type PeriodTotal struct {
	Label   string          `gorm:"column:label"`
	Income  decimal.Decimal `gorm:"column:income"`
	Expense decimal.Decimal `gorm:"column:expense"`
}

type ReportRepository interface {
	Totals(ctx context.Context, userID int64, filter dto.ReportFilter) (Totals, error)
	ExpenseByCategory(ctx context.Context, userID int64, filter dto.ReportFilter) ([]CategoryTotal, error)
	Monthly(ctx context.Context, userID int64, filter dto.ReportFilter) ([]PeriodTotal, error)
	Daily(ctx context.Context, userID int64, filter dto.ReportFilter) ([]PeriodTotal, error)
}

type reportRepository struct {
	db *gorm.DB
}

func NewReportRepository(db *gorm.DB) ReportRepository {
	return &reportRepository{db: db}
}

func (r *reportRepository) Totals(ctx context.Context, userID int64, filter dto.ReportFilter) (Totals, error) {
	var totals Totals
	err := r.scoped(ctx, userID, filter).
		Select(`COALESCE(SUM(CASE WHEN transaction_type = ? THEN amount ELSE 0 END), 0) AS total_income,
		        COALESCE(SUM(CASE WHEN transaction_type = ? THEN amount ELSE 0 END), 0) AS total_expense,
		        COUNT(*) AS transaction_count`,
			model.TransactionTypeIncome, model.TransactionTypeExpense).
		Scan(&totals).Error
	return totals, err
}

func (r *reportRepository) ExpenseByCategory(ctx context.Context, userID int64, filter dto.ReportFilter) ([]CategoryTotal, error) {
	forced := filter
	forced.TransactionType = string(model.TransactionTypeExpense)

	var rows []CategoryTotal
	err := r.scoped(ctx, userID, forced).
		Select(`transactions.category_id AS category_id,
		        COALESCE(categories.name, 'Uncategorized') AS category_name,
		        COALESCE(SUM(transactions.amount), 0) AS total`).
		Joins("LEFT JOIN categories ON categories.id = transactions.category_id").
		Group("transactions.category_id, categories.name").
		Order("total DESC").
		Scan(&rows).Error
	return rows, err
}

// Monthly groups on the ISO year-month prefix. to_char is not sargable but
// cheap here because the period is already narrowed by the indexed date range.
func (r *reportRepository) Monthly(ctx context.Context, userID int64, filter dto.ReportFilter) ([]PeriodTotal, error) {
	return r.grouped(ctx, userID, filter, "to_char(transaction_date, 'YYYY-MM')")
}

func (r *reportRepository) Daily(ctx context.Context, userID int64, filter dto.ReportFilter) ([]PeriodTotal, error) {
	return r.grouped(ctx, userID, filter, "to_char(transaction_date, 'YYYY-MM-DD')")
}

func (r *reportRepository) grouped(ctx context.Context, userID int64, filter dto.ReportFilter, expression string) ([]PeriodTotal, error) {
	var rows []PeriodTotal
	err := r.scoped(ctx, userID, filter).
		Select(expression+` AS label,
		        COALESCE(SUM(CASE WHEN transaction_type = ? THEN amount ELSE 0 END), 0) AS income,
		        COALESCE(SUM(CASE WHEN transaction_type = ? THEN amount ELSE 0 END), 0) AS expense`,
			model.TransactionTypeIncome, model.TransactionTypeExpense).
		Group(expression).
		Order("label ASC").
		Scan(&rows).Error
	return rows, err
}

func (r *reportRepository) scoped(ctx context.Context, userID int64, filter dto.ReportFilter) *gorm.DB {
	query := r.db.WithContext(ctx).
		Model(&model.Transaction{}).
		Where("transactions.user_id = ?", userID).
		Where("transactions.transaction_date >= ? AND transactions.transaction_date <= ?", filter.DateFrom, filter.DateTo)

	if filter.TransactionType != "" {
		query = query.Where("transactions.transaction_type = ?", filter.TransactionType)
	}
	if filter.CategoryID != nil {
		query = query.Where("transactions.category_id = ?", *filter.CategoryID)
	}
	return query
}
