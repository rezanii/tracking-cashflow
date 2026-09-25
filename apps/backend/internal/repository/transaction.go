package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

type TransactionRepository interface {
	WithTx(tx *gorm.DB) TransactionRepository
	Create(ctx context.Context, transaction *model.Transaction) error
	Update(ctx context.Context, transaction *model.Transaction) error
	Delete(ctx context.Context, userID, id int64) error
	FindByID(ctx context.Context, userID, id int64) (*model.Transaction, error)
	List(ctx context.Context, userID int64, filter dto.TransactionFilter) ([]model.Transaction, int64, error)
	ListForReport(ctx context.Context, userID int64, filter dto.ReportFilter) ([]model.Transaction, error)
}

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

func (r *transactionRepository) WithTx(tx *gorm.DB) TransactionRepository {
	return &transactionRepository{db: tx}
}

func (r *transactionRepository) Create(ctx context.Context, transaction *model.Transaction) error {
	return r.db.WithContext(ctx).Omit("Category").Create(transaction).Error
}

func (r *transactionRepository) Update(ctx context.Context, transaction *model.Transaction) error {
	result := r.db.WithContext(ctx).
		Model(&model.Transaction{}).
		Where("id = ? AND user_id = ?", transaction.ID, transaction.UserID).
		Updates(map[string]any{
			"transaction_date": transaction.TransactionDate,
			"transaction_type": transaction.TransactionType,
			"category_id":      transaction.CategoryID,
			"amount":           transaction.Amount,
			"description":      transaction.Description,
			"reference_number": transaction.ReferenceNumber,
			"updated_at":       transaction.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return utils.NotFound("Transaction")
	}
	return nil
}

func (r *transactionRepository) Delete(ctx context.Context, userID, id int64) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Transaction{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return utils.NotFound("Transaction")
	}
	return nil
}

func (r *transactionRepository) FindByID(ctx context.Context, userID, id int64) (*model.Transaction, error) {
	var transaction model.Transaction
	err := r.db.WithContext(ctx).
		Preload("Category").
		Where("id = ? AND user_id = ?", id, userID).
		Take(&transaction).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &transaction, nil
}

var transactionSortColumns = map[string]string{
	"transaction_date": "transaction_date",
	"amount":           "amount",
	"created_at":       "created_at",
}

func (r *transactionRepository) List(ctx context.Context, userID int64, filter dto.TransactionFilter) ([]model.Transaction, int64, error) {
	base := r.applyFilters(r.db.WithContext(ctx).Model(&model.Transaction{}).Where("user_id = ?", userID), filter)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	column, direction := utils.NormalizeSort(filter.SortBy, filter.SortDir, transactionSortColumns, "transaction_date")

	var transactions []model.Transaction
	err := base.
		Preload("Category").
		Order(column + " " + direction).
		Order("id DESC").
		Limit(filter.PageSize).
		Offset(utils.Offset(filter.Page, filter.PageSize)).
		Find(&transactions).Error
	if err != nil {
		return nil, 0, err
	}
	return transactions, total, nil
}

// ListForReport returns the whole filtered period ordered deterministically, so the running
// balance a report shows is reproducible for the same filter.
func (r *transactionRepository) ListForReport(ctx context.Context, userID int64, filter dto.ReportFilter) ([]model.Transaction, error) {
	query := r.db.WithContext(ctx).
		Model(&model.Transaction{}).
		Where("user_id = ?", userID).
		Where("transaction_date >= ? AND transaction_date <= ?", filter.DateFrom, filter.DateTo)

	if filter.TransactionType != "" {
		query = query.Where("transaction_type = ?", filter.TransactionType)
	}
	if filter.CategoryID != nil {
		query = query.Where("category_id = ?", *filter.CategoryID)
	}

	var transactions []model.Transaction
	err := query.
		Preload("Category").
		Order("transaction_date ASC").
		Order("id ASC").
		Find(&transactions).Error
	if err != nil {
		return nil, err
	}
	return transactions, nil
}

func (r *transactionRepository) applyFilters(query *gorm.DB, filter dto.TransactionFilter) *gorm.DB {
	if filter.Search != "" {
		pattern := "%" + filter.Search + "%"
		query = query.Where("(description LIKE ? OR reference_number LIKE ?)", pattern, pattern)
	}
	if filter.DateFrom != nil {
		query = query.Where("transaction_date >= ?", *filter.DateFrom)
	}
	if filter.DateTo != nil {
		query = query.Where("transaction_date <= ?", *filter.DateTo)
	}
	if filter.TransactionType != "" {
		query = query.Where("transaction_type = ?", filter.TransactionType)
	}
	if filter.CategoryID != nil {
		query = query.Where("category_id = ?", *filter.CategoryID)
	}
	return query
}
