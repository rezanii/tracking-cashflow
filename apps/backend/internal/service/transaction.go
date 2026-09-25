package service

import (
	"context"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/repository"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

type TransactionService interface {
	Create(ctx context.Context, userID int64, request dto.TransactionCreateRequest) (dto.TransactionResponse, error)
	Update(ctx context.Context, userID, id int64, request dto.TransactionUpdateRequest) (dto.TransactionResponse, error)
	Delete(ctx context.Context, userID, id int64) error
	Get(ctx context.Context, userID, id int64) (dto.TransactionResponse, error)
	List(ctx context.Context, userID int64, filter dto.TransactionFilter) ([]dto.TransactionResponse, utils.Pagination, error)
}

type transactionService struct {
	transactions repository.TransactionRepository
	categories   repository.CategoryRepository
	tx           repository.TxManager
}

func NewTransactionService(
	transactions repository.TransactionRepository,
	categories repository.CategoryRepository,
	tx repository.TxManager,
) TransactionService {
	return &transactionService{transactions: transactions, categories: categories, tx: tx}
}

// resolved holds the cross-checked form of a create or update request.
type resolved struct {
	date            time.Time
	transactionType model.TransactionType
	categoryID      *int64
	amount          decimal.Decimal
}

func (s *transactionService) Create(ctx context.Context, userID int64, request dto.TransactionCreateRequest) (dto.TransactionResponse, error) {
	input, err := s.resolve(ctx, userID, request)
	if err != nil {
		return dto.TransactionResponse{}, err
	}

	now := time.Now().UTC()
	transaction := &model.Transaction{
		UserID:          userID,
		TransactionDate: input.date,
		TransactionType: input.transactionType,
		CategoryID:      input.categoryID,
		Amount:          input.amount,
		Description:     strings.TrimSpace(request.Description),
		ReferenceNumber: strings.TrimSpace(request.ReferenceNumber),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// The insert and the read-back of the joined category are one unit of work, so the
	// response can never disagree with what was stored.
	var created dto.TransactionResponse
	err = s.tx.Run(ctx, func(tx *gorm.DB) error {
		transactions := s.transactions.WithTx(tx)

		if err := transactions.Create(ctx, transaction); err != nil {
			return utils.WrapDomainError(utils.ErrConflict, "Failed to create transaction", err)
		}

		stored, err := transactions.FindByID(ctx, userID, transaction.ID)
		if err != nil {
			return utils.WrapDomainError(utils.ErrNotFound, "Failed to load created transaction", err)
		}
		if stored == nil {
			return utils.NotFound("Transaction")
		}
		created = toTransactionResponse(*stored)
		return nil
	})
	if err != nil {
		return dto.TransactionResponse{}, err
	}
	return created, nil
}

func (s *transactionService) Update(ctx context.Context, userID, id int64, request dto.TransactionUpdateRequest) (dto.TransactionResponse, error) {
	input, err := s.resolve(ctx, userID, request)
	if err != nil {
		return dto.TransactionResponse{}, err
	}

	var updated dto.TransactionResponse
	err = s.tx.Run(ctx, func(tx *gorm.DB) error {
		transactions := s.transactions.WithTx(tx)

		existing, err := transactions.FindByID(ctx, userID, id)
		if err != nil {
			return utils.WrapDomainError(utils.ErrNotFound, "Failed to load transaction", err)
		}
		if existing == nil {
			return utils.NotFound("Transaction")
		}

		existing.TransactionDate = input.date
		existing.TransactionType = input.transactionType
		existing.CategoryID = input.categoryID
		existing.Amount = input.amount
		existing.Description = strings.TrimSpace(request.Description)
		existing.ReferenceNumber = strings.TrimSpace(request.ReferenceNumber)
		existing.UpdatedAt = time.Now().UTC()

		if err := transactions.Update(ctx, existing); err != nil {
			return wrapIfInternal(err, "Failed to update transaction")
		}

		stored, err := transactions.FindByID(ctx, userID, id)
		if err != nil {
			return utils.WrapDomainError(utils.ErrNotFound, "Failed to load updated transaction", err)
		}
		if stored == nil {
			return utils.NotFound("Transaction")
		}
		updated = toTransactionResponse(*stored)
		return nil
	})
	if err != nil {
		return dto.TransactionResponse{}, err
	}
	return updated, nil
}

func (s *transactionService) Delete(ctx context.Context, userID, id int64) error {
	if err := s.transactions.Delete(ctx, userID, id); err != nil {
		return wrapIfInternal(err, "Failed to delete transaction")
	}
	return nil
}

func (s *transactionService) Get(ctx context.Context, userID, id int64) (dto.TransactionResponse, error) {
	transaction, err := s.transactions.FindByID(ctx, userID, id)
	if err != nil {
		return dto.TransactionResponse{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to load transaction", err)
	}
	if transaction == nil {
		return dto.TransactionResponse{}, utils.NotFound("Transaction")
	}
	return toTransactionResponse(*transaction), nil
}

func (s *transactionService) List(ctx context.Context, userID int64, filter dto.TransactionFilter) ([]dto.TransactionResponse, utils.Pagination, error) {
	filter.Page, filter.PageSize = utils.NormalizePaging(filter.Page, filter.PageSize)

	transactions, total, err := s.transactions.List(ctx, userID, filter)
	if err != nil {
		return nil, utils.Pagination{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to list transactions", err)
	}

	responses := make([]dto.TransactionResponse, 0, len(transactions))
	for _, transaction := range transactions {
		responses = append(responses, toTransactionResponse(transaction))
	}
	return responses, utils.NewPagination(filter.Page, filter.PageSize, total), nil
}

// resolve validates the request and checks the category against the transaction type and
// the calling user, so a category belonging to somebody else can never be attached.
func (s *transactionService) resolve(ctx context.Context, userID int64, request dto.TransactionCreateRequest) (resolved, error) {
	fields := map[string]string{}

	date, dateErr := utils.ParseDate(request.TransactionDate)
	if dateErr != nil {
		fields["transaction_date"] = dateErr.Error()
	}

	transactionType := model.TransactionType(strings.ToUpper(strings.TrimSpace(request.TransactionType)))
	if !transactionType.Valid() {
		fields["transaction_type"] = "transaction_type must be one of: INCOME, EXPENSE, TRANSFER"
	}

	amount := utils.Round(request.Amount)
	if amount.LessThanOrEqual(utils.Zero()) {
		fields["amount"] = "amount must be greater than 0"
	}

	categoryID := request.CategoryID
	if transactionType.RequiresCategory() {
		if categoryID == nil {
			fields["category_id"] = "category_id is required for INCOME and EXPENSE"
		}
	} else {
		// A transfer is not classified. Dropping the category here keeps it out of the
		// reports that group by category.
		categoryID = nil
	}

	if len(fields) > 0 {
		return resolved{}, utils.NewFieldError("Validation failed", fields)
	}

	if categoryID != nil {
		category, err := s.categories.FindByID(ctx, userID, *categoryID)
		if err != nil {
			return resolved{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to load category", err)
		}
		switch {
		case category == nil:
			return resolved{}, utils.NewFieldError("Validation failed", map[string]string{
				"category_id": "category not found",
			})
		case !category.IsActive:
			return resolved{}, utils.NewFieldError("Validation failed", map[string]string{
				"category_id": "category is inactive",
			})
		case string(category.Type) != string(transactionType):
			return resolved{}, utils.NewFieldError("Validation failed", map[string]string{
				"category_id": "category type must match the transaction type",
			})
		}
	}

	return resolved{
		date:            date,
		transactionType: transactionType,
		categoryID:      categoryID,
		amount:          amount,
	}, nil
}

func toTransactionResponse(transaction model.Transaction) dto.TransactionResponse {
	return dto.TransactionResponse{
		ID:              transaction.ID,
		TransactionDate: utils.FormatDate(transaction.TransactionDate),
		TransactionType: string(transaction.TransactionType),
		CategoryID:      transaction.CategoryID,
		CategoryName:    transaction.CategoryName(),
		Amount:          utils.Round(transaction.Amount),
		Description:     transaction.Description,
		ReferenceNumber: transaction.ReferenceNumber,
		CreatedAt:       transaction.CreatedAt,
		UpdatedAt:       transaction.UpdatedAt,
	}
}
