package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/repository"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

// errBoom stands in for an unexpected driver failure.
var errBoom = errors.New("database unavailable")

// fakeTxManager runs the unit of work inline. The fake repositories ignore the *gorm.DB they
// are handed, so the services can be exercised without a database.
type fakeTxManager struct {
	calls int
}

func (m *fakeTxManager) Run(ctx context.Context, fn func(tx *gorm.DB) error) error {
	m.calls++
	return fn(nil)
}

type fakeUserRepository struct {
	users     map[string]*model.User
	byID      map[int64]*model.User
	nextID    int64
	failOn    string
	createErr error
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{
		users:  map[string]*model.User{},
		byID:   map[int64]*model.User{},
		nextID: 1,
	}
}

func (r *fakeUserRepository) WithTx(*gorm.DB) repository.UserRepository { return r }

func (r *fakeUserRepository) Create(_ context.Context, user *model.User) error {
	if r.createErr != nil {
		return r.createErr
	}
	user.ID = r.nextID
	r.nextID++
	copied := *user
	r.users[user.Email] = &copied
	r.byID[user.ID] = &copied
	return nil
}

func (r *fakeUserRepository) FindByEmail(_ context.Context, email string) (*model.User, error) {
	if r.failOn == "FindByEmail" {
		return nil, errBoom
	}
	user, ok := r.users[email]
	if !ok {
		return nil, nil
	}
	copied := *user
	return &copied, nil
}

func (r *fakeUserRepository) FindByID(_ context.Context, id int64) (*model.User, error) {
	if r.failOn == "FindByID" {
		return nil, errBoom
	}
	user, ok := r.byID[id]
	if !ok {
		return nil, nil
	}
	copied := *user
	return &copied, nil
}

func (r *fakeUserRepository) ExistsByEmail(_ context.Context, email string) (bool, error) {
	if r.failOn == "ExistsByEmail" {
		return false, errBoom
	}
	_, ok := r.users[email]
	return ok, nil
}

type fakeCategoryRepository struct {
	categories map[int64]*model.Category
	nextID     int64
	usage      map[int64]int64
}

func newFakeCategoryRepository() *fakeCategoryRepository {
	return &fakeCategoryRepository{
		categories: map[int64]*model.Category{},
		nextID:     1,
		usage:      map[int64]int64{},
	}
}

// seed inserts a category directly, bypassing the service, so a test can start from a
// known state.
func (r *fakeCategoryRepository) seed(userID int64, name string, categoryType model.CategoryType, active bool) *model.Category {
	category := &model.Category{
		ID:        r.nextID,
		UserID:    userID,
		Name:      name,
		Type:      categoryType,
		IsActive:  active,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	r.categories[category.ID] = category
	r.nextID++
	return category
}

func (r *fakeCategoryRepository) WithTx(*gorm.DB) repository.CategoryRepository { return r }

func (r *fakeCategoryRepository) Create(_ context.Context, category *model.Category) error {
	category.ID = r.nextID
	r.nextID++
	copied := *category
	r.categories[category.ID] = &copied
	return nil
}

func (r *fakeCategoryRepository) Update(_ context.Context, category *model.Category) error {
	existing, ok := r.categories[category.ID]
	if !ok || existing.UserID != category.UserID {
		return utils.NotFound("Category")
	}
	copied := *category
	r.categories[category.ID] = &copied
	return nil
}

func (r *fakeCategoryRepository) Delete(_ context.Context, userID, id int64) error {
	existing, ok := r.categories[id]
	if !ok || existing.UserID != userID {
		return utils.NotFound("Category")
	}
	delete(r.categories, id)
	return nil
}

func (r *fakeCategoryRepository) FindByID(_ context.Context, userID, id int64) (*model.Category, error) {
	existing, ok := r.categories[id]
	// Ownership is part of the lookup: another user's row must read as absent.
	if !ok || existing.UserID != userID {
		return nil, nil
	}
	copied := *existing
	return &copied, nil
}

func (r *fakeCategoryRepository) FindByName(_ context.Context, userID int64, categoryType model.CategoryType, name string) (*model.Category, error) {
	for _, category := range r.categories {
		if category.UserID == userID && category.Type == categoryType && category.Name == name {
			copied := *category
			return &copied, nil
		}
	}
	return nil, nil
}

func (r *fakeCategoryRepository) List(_ context.Context, userID int64, query dto.CategoryListQuery) ([]model.Category, int64, error) {
	var matched []model.Category
	for _, category := range r.categories {
		if category.UserID != userID {
			continue
		}
		if query.Type != "" && string(category.Type) != query.Type {
			continue
		}
		if query.IsActive != nil && category.IsActive != *query.IsActive {
			continue
		}
		if query.Search != "" && !strings.Contains(strings.ToLower(category.Name), strings.ToLower(query.Search)) {
			continue
		}
		matched = append(matched, *category)
	}

	sort.Slice(matched, func(i, j int) bool { return matched[i].Name < matched[j].Name })

	total := int64(len(matched))
	start := utils.Offset(query.Page, query.PageSize)
	if start >= len(matched) {
		return nil, total, nil
	}
	end := start + query.PageSize
	if end > len(matched) {
		end = len(matched)
	}
	return matched[start:end], total, nil
}

func (r *fakeCategoryRepository) CountTransactions(_ context.Context, _, id int64) (int64, error) {
	return r.usage[id], nil
}

func (r *fakeCategoryRepository) SetActive(_ context.Context, userID, id int64, isActive bool) error {
	existing, ok := r.categories[id]
	if !ok || existing.UserID != userID {
		return utils.NotFound("Category")
	}
	existing.IsActive = isActive
	return nil
}

type fakeTransactionRepository struct {
	transactions map[int64]*model.Transaction
	categories   *fakeCategoryRepository
	nextID       int64
	failOn       string
}

func newFakeTransactionRepository(categories *fakeCategoryRepository) *fakeTransactionRepository {
	return &fakeTransactionRepository{
		transactions: map[int64]*model.Transaction{},
		categories:   categories,
		nextID:       1,
	}
}

func (r *fakeTransactionRepository) WithTx(*gorm.DB) repository.TransactionRepository { return r }

func (r *fakeTransactionRepository) Create(_ context.Context, transaction *model.Transaction) error {
	if r.failOn == "Create" {
		return errBoom
	}
	transaction.ID = r.nextID
	r.nextID++
	copied := *transaction
	r.transactions[transaction.ID] = &copied
	return nil
}

func (r *fakeTransactionRepository) Update(_ context.Context, transaction *model.Transaction) error {
	existing, ok := r.transactions[transaction.ID]
	if !ok || existing.UserID != transaction.UserID {
		return utils.NotFound("Transaction")
	}
	copied := *transaction
	r.transactions[transaction.ID] = &copied
	return nil
}

func (r *fakeTransactionRepository) Delete(_ context.Context, userID, id int64) error {
	existing, ok := r.transactions[id]
	if !ok || existing.UserID != userID {
		return utils.NotFound("Transaction")
	}
	delete(r.transactions, id)
	return nil
}

func (r *fakeTransactionRepository) FindByID(_ context.Context, userID, id int64) (*model.Transaction, error) {
	existing, ok := r.transactions[id]
	if !ok || existing.UserID != userID {
		return nil, nil
	}
	copied := *existing
	r.attachCategory(&copied)
	return &copied, nil
}

func (r *fakeTransactionRepository) List(_ context.Context, userID int64, filter dto.TransactionFilter) ([]model.Transaction, int64, error) {
	matched := r.match(userID, filter.DateFrom, filter.DateTo, filter.TransactionType, filter.CategoryID)

	if filter.Search != "" {
		needle := strings.ToLower(filter.Search)
		var filtered []model.Transaction
		for _, transaction := range matched {
			if strings.Contains(strings.ToLower(transaction.Description), needle) ||
				strings.Contains(strings.ToLower(transaction.ReferenceNumber), needle) {
				filtered = append(filtered, transaction)
			}
		}
		matched = filtered
	}

	sort.Slice(matched, func(i, j int) bool {
		if matched[i].TransactionDate.Equal(matched[j].TransactionDate) {
			return matched[i].ID > matched[j].ID
		}
		return matched[i].TransactionDate.After(matched[j].TransactionDate)
	})

	total := int64(len(matched))
	start := utils.Offset(filter.Page, filter.PageSize)
	if start >= len(matched) {
		return nil, total, nil
	}
	end := start + filter.PageSize
	if end > len(matched) {
		end = len(matched)
	}
	return matched[start:end], total, nil
}

func (r *fakeTransactionRepository) ListForReport(_ context.Context, userID int64, filter dto.ReportFilter) ([]model.Transaction, error) {
	if r.failOn == "ListForReport" {
		return nil, errBoom
	}

	matched := r.match(userID, &filter.DateFrom, &filter.DateTo, filter.TransactionType, filter.CategoryID)
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].TransactionDate.Equal(matched[j].TransactionDate) {
			return matched[i].ID < matched[j].ID
		}
		return matched[i].TransactionDate.Before(matched[j].TransactionDate)
	})
	return matched, nil
}

func (r *fakeTransactionRepository) match(
	userID int64,
	from, to *time.Time,
	transactionType string,
	categoryID *int64,
) []model.Transaction {
	var matched []model.Transaction
	for _, transaction := range r.transactions {
		if transaction.UserID != userID {
			continue
		}
		if from != nil && transaction.TransactionDate.Before(*from) {
			continue
		}
		if to != nil && transaction.TransactionDate.After(*to) {
			continue
		}
		if transactionType != "" && string(transaction.TransactionType) != transactionType {
			continue
		}
		if categoryID != nil && (transaction.CategoryID == nil || *transaction.CategoryID != *categoryID) {
			continue
		}
		copied := *transaction
		r.attachCategory(&copied)
		matched = append(matched, copied)
	}
	return matched
}

// attachCategory mirrors the Preload the real repository performs.
func (r *fakeTransactionRepository) attachCategory(transaction *model.Transaction) {
	if transaction.CategoryID == nil || r.categories == nil {
		return
	}
	if category, ok := r.categories.categories[*transaction.CategoryID]; ok {
		copied := *category
		transaction.Category = &copied
	}
}

type fakeReportRepository struct {
	totals    repository.Totals
	byCategor []repository.CategoryTotal
	monthly   []repository.PeriodTotal
	daily     []repository.PeriodTotal
	failOn    string
}

func (r *fakeReportRepository) Totals(context.Context, int64, dto.ReportFilter) (repository.Totals, error) {
	if r.failOn == "Totals" {
		return repository.Totals{}, errBoom
	}
	return r.totals, nil
}

func (r *fakeReportRepository) ExpenseByCategory(context.Context, int64, dto.ReportFilter) ([]repository.CategoryTotal, error) {
	if r.failOn == "ExpenseByCategory" {
		return nil, errBoom
	}
	return r.byCategor, nil
}

func (r *fakeReportRepository) Monthly(context.Context, int64, dto.ReportFilter) ([]repository.PeriodTotal, error) {
	if r.failOn == "Monthly" {
		return nil, errBoom
	}
	return r.monthly, nil
}

func (r *fakeReportRepository) Daily(context.Context, int64, dto.ReportFilter) ([]repository.PeriodTotal, error) {
	if r.failOn == "Daily" {
		return nil, errBoom
	}
	return r.daily, nil
}

func decimalOf(value string) decimal.Decimal {
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func dateOf(value string) time.Time {
	parsed, err := time.Parse(utils.DateLayout, value)
	if err != nil {
		panic(err)
	}
	return parsed
}
