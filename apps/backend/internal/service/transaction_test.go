package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

const (
	ownerID     int64 = 1
	intruderID  int64 = 2
	defaultDate       = "2026-09-25"
)

func newTransactionService() (TransactionService, *fakeTransactionRepository, *fakeCategoryRepository, *fakeTxManager) {
	categories := newFakeCategoryRepository()
	transactions := newFakeTransactionRepository(categories)
	txManager := &fakeTxManager{}
	return NewTransactionService(transactions, categories, txManager), transactions, categories, txManager
}

func TestCreateTransactionStoresTheRow(t *testing.T) {
	service, _, categories, txManager := newTransactionService()
	expense := categories.seed(ownerID, "Cicilan Rumah", model.CategoryTypeExpense, true)

	created, err := service.Create(context.Background(), ownerID, dto.TransactionCreateRequest{
		TransactionDate: defaultDate,
		TransactionType: "EXPENSE",
		CategoryID:      &expense.ID,
		Amount:          decimalOf("1150000.00"),
		Description:     " Cicilan Rumah September ",
		ReferenceNumber: " INV-0001 ",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if created.ID == 0 {
		t.Fatal("the created transaction has no id")
	}
	if created.TransactionDate != defaultDate {
		t.Fatalf("date = %q, want %q", created.TransactionDate, defaultDate)
	}
	if created.CategoryName != "Cicilan Rumah" {
		t.Fatalf("category name = %q, want Cicilan Rumah from the preloaded join", created.CategoryName)
	}
	if created.Description != "Cicilan Rumah September" || created.ReferenceNumber != "INV-0001" {
		t.Fatalf("text fields were not trimmed: %+v", created)
	}
	if !created.Amount.Equal(decimalOf("1150000.00")) {
		t.Fatalf("amount = %s, want 1150000.00", created.Amount)
	}
	// The write and the read-back share one transaction.
	if txManager.calls != 1 {
		t.Fatalf("transaction manager calls = %d, want 1", txManager.calls)
	}
}

func TestCreateTransactionValidation(t *testing.T) {
	service, _, categories, _ := newTransactionService()
	expense := categories.seed(ownerID, "Cicilan Rumah", model.CategoryTypeExpense, true)
	income := categories.seed(ownerID, "Salary", model.CategoryTypeIncome, true)
	inactive := categories.seed(ownerID, "Old", model.CategoryTypeExpense, false)
	foreign := categories.seed(intruderID, "Theirs", model.CategoryTypeExpense, true)

	cases := []struct {
		name      string
		request   dto.TransactionCreateRequest
		wantField string
	}{
		{
			name: "bad date",
			request: dto.TransactionCreateRequest{
				TransactionDate: "25/09/2026", TransactionType: "EXPENSE",
				CategoryID: &expense.ID, Amount: decimalOf("1000"),
			},
			wantField: "transaction_date",
		},
		{
			name: "unknown type",
			request: dto.TransactionCreateRequest{
				TransactionDate: defaultDate, TransactionType: "REFUND",
				CategoryID: &expense.ID, Amount: decimalOf("1000"),
			},
			wantField: "transaction_type",
		},
		{
			name: "zero amount",
			request: dto.TransactionCreateRequest{
				TransactionDate: defaultDate, TransactionType: "EXPENSE",
				CategoryID: &expense.ID, Amount: decimalOf("0"),
			},
			wantField: "amount",
		},
		{
			name: "negative amount",
			request: dto.TransactionCreateRequest{
				TransactionDate: defaultDate, TransactionType: "EXPENSE",
				CategoryID: &expense.ID, Amount: decimalOf("-5000"),
			},
			wantField: "amount",
		},
		{
			name: "expense without category",
			request: dto.TransactionCreateRequest{
				TransactionDate: defaultDate, TransactionType: "EXPENSE",
				Amount: decimalOf("1000"),
			},
			wantField: "category_id",
		},
		{
			name: "category type mismatch",
			request: dto.TransactionCreateRequest{
				TransactionDate: defaultDate, TransactionType: "EXPENSE",
				CategoryID: &income.ID, Amount: decimalOf("1000"),
			},
			wantField: "category_id",
		},
		{
			name: "inactive category",
			request: dto.TransactionCreateRequest{
				TransactionDate: defaultDate, TransactionType: "EXPENSE",
				CategoryID: &inactive.ID, Amount: decimalOf("1000"),
			},
			wantField: "category_id",
		},
		{
			// A category owned by somebody else must read as missing, not as forbidden,
			// so the endpoint cannot be used to probe other accounts.
			name: "category owned by another user",
			request: dto.TransactionCreateRequest{
				TransactionDate: defaultDate, TransactionType: "EXPENSE",
				CategoryID: &foreign.ID, Amount: decimalOf("1000"),
			},
			wantField: "category_id",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := service.Create(context.Background(), ownerID, testCase.request)
			if err == nil {
				t.Fatal("expected a validation error")
			}
			if !errors.Is(err, utils.ErrValidation) {
				t.Fatalf("error kind = %v, want ErrValidation", err)
			}

			domainErr, ok := utils.AsDomainError(err)
			if !ok {
				t.Fatalf("error %v is not a domain error", err)
			}
			if _, present := domainErr.Fields[testCase.wantField]; !present {
				t.Fatalf("fields = %v, want an entry for %s", domainErr.Fields, testCase.wantField)
			}
		})
	}
}

func TestCreateTransferDropsTheCategory(t *testing.T) {
	service, _, categories, _ := newTransactionService()
	expense := categories.seed(ownerID, "Cicilan Rumah", model.CategoryTypeExpense, true)

	created, err := service.Create(context.Background(), ownerID, dto.TransactionCreateRequest{
		TransactionDate: defaultDate,
		TransactionType: "TRANSFER",
		CategoryID:      &expense.ID, // sent by a sloppy client
		Amount:          decimalOf("2000000"),
		Description:     "Move to savings",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	// Keeping the category would make the transfer show up in expense-by-category.
	if created.CategoryID != nil {
		t.Fatalf("category id = %v, want nil for a transfer", *created.CategoryID)
	}
	if created.CategoryName != "" {
		t.Fatalf("category name = %q, want empty for a transfer", created.CategoryName)
	}
}

func TestTransactionOwnership(t *testing.T) {
	service, _, categories, _ := newTransactionService()
	expense := categories.seed(ownerID, "Cicilan Rumah", model.CategoryTypeExpense, true)
	ctx := context.Background()

	created, err := service.Create(ctx, ownerID, dto.TransactionCreateRequest{
		TransactionDate: defaultDate,
		TransactionType: "EXPENSE",
		CategoryID:      &expense.ID,
		Amount:          decimalOf("1150000"),
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	t.Run("get", func(t *testing.T) {
		if _, err := service.Get(ctx, intruderID, created.ID); !errors.Is(err, utils.ErrNotFound) {
			t.Fatalf("Get as another user = %v, want ErrNotFound", err)
		}
	})

	t.Run("update", func(t *testing.T) {
		_, err := service.Update(ctx, intruderID, created.ID, dto.TransactionUpdateRequest{
			TransactionDate: defaultDate,
			TransactionType: "TRANSFER",
			Amount:          decimalOf("1"),
		})
		if !errors.Is(err, utils.ErrNotFound) {
			t.Fatalf("Update as another user = %v, want ErrNotFound", err)
		}
	})

	t.Run("delete", func(t *testing.T) {
		if err := service.Delete(ctx, intruderID, created.ID); !errors.Is(err, utils.ErrNotFound) {
			t.Fatalf("Delete as another user = %v, want ErrNotFound", err)
		}
	})

	t.Run("still readable by the owner", func(t *testing.T) {
		found, err := service.Get(ctx, ownerID, created.ID)
		if err != nil {
			t.Fatalf("the owner can no longer read the row: %v", err)
		}
		if found.ID != created.ID {
			t.Fatalf("id = %d, want %d", found.ID, created.ID)
		}
	})
}

func TestUpdateTransactionReplacesFields(t *testing.T) {
	service, _, categories, _ := newTransactionService()
	expense := categories.seed(ownerID, "Cicilan Rumah", model.CategoryTypeExpense, true)
	other := categories.seed(ownerID, "Food", model.CategoryTypeExpense, true)
	ctx := context.Background()

	created, err := service.Create(ctx, ownerID, dto.TransactionCreateRequest{
		TransactionDate: defaultDate,
		TransactionType: "EXPENSE",
		CategoryID:      &expense.ID,
		Amount:          decimalOf("1150000"),
		Description:     "Cicilan Rumah",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	updated, err := service.Update(ctx, ownerID, created.ID, dto.TransactionUpdateRequest{
		TransactionDate: "2026-09-26",
		TransactionType: "EXPENSE",
		CategoryID:      &other.ID,
		Amount:          decimalOf("275500.50"),
		Description:     "Groceries",
		ReferenceNumber: "FOOD-9",
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	if updated.TransactionDate != "2026-09-26" {
		t.Fatalf("date = %q, want 2026-09-26", updated.TransactionDate)
	}
	if updated.CategoryName != "Food" {
		t.Fatalf("category name = %q, want Food", updated.CategoryName)
	}
	if !updated.Amount.Equal(decimalOf("275500.50")) {
		t.Fatalf("amount = %s, want 275500.50", updated.Amount)
	}
}

func TestDeleteUnknownTransaction(t *testing.T) {
	service, _, _, _ := newTransactionService()
	if err := service.Delete(context.Background(), ownerID, 999); !errors.Is(err, utils.ErrNotFound) {
		t.Fatalf("Delete of an unknown id = %v, want ErrNotFound", err)
	}
}

func TestListTransactionsPaginatesAndFilters(t *testing.T) {
	service, _, categories, _ := newTransactionService()
	expense := categories.seed(ownerID, "Food", model.CategoryTypeExpense, true)
	income := categories.seed(ownerID, "Salary", model.CategoryTypeIncome, true)
	ctx := context.Background()

	seed := []struct {
		date            string
		transactionType string
		categoryID      *int64
		amount          string
		description     string
	}{
		{"2026-09-01", "EXPENSE", &expense.ID, "100000", "groceries one"},
		{"2026-09-05", "EXPENSE", &expense.ID, "200000", "groceries two"},
		{"2026-09-10", "INCOME", &income.ID, "15000000", "salary"},
		{"2026-10-01", "EXPENSE", &expense.ID, "300000", "next month"},
	}
	for _, item := range seed {
		if _, err := service.Create(ctx, ownerID, dto.TransactionCreateRequest{
			TransactionDate: item.date,
			TransactionType: item.transactionType,
			CategoryID:      item.categoryID,
			Amount:          decimalOf(item.amount),
			Description:     item.description,
		}); err != nil {
			t.Fatalf("seeding %s failed: %v", item.description, err)
		}
	}

	t.Run("page size is honoured", func(t *testing.T) {
		items, pagination, err := service.List(ctx, ownerID, dto.TransactionFilter{Page: 1, PageSize: 2})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("items = %d, want 2", len(items))
		}
		if pagination.TotalItems != 4 {
			t.Fatalf("total items = %d, want 4", pagination.TotalItems)
		}
		if pagination.TotalPages != 2 {
			t.Fatalf("total pages = %d, want 2", pagination.TotalPages)
		}
	})

	t.Run("date range excludes other months", func(t *testing.T) {
		from := dateOf("2026-09-01")
		to := dateOf("2026-09-30")
		items, pagination, err := service.List(ctx, ownerID, dto.TransactionFilter{DateFrom: &from, DateTo: &to})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if pagination.TotalItems != 3 {
			t.Fatalf("total items = %d, want 3", pagination.TotalItems)
		}
		for _, item := range items {
			if item.Description == "next month" {
				t.Fatal("a transaction outside the range was returned")
			}
		}
	})

	t.Run("type filter", func(t *testing.T) {
		_, pagination, err := service.List(ctx, ownerID, dto.TransactionFilter{TransactionType: "INCOME"})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if pagination.TotalItems != 1 {
			t.Fatalf("total items = %d, want 1", pagination.TotalItems)
		}
	})

	t.Run("search", func(t *testing.T) {
		_, pagination, err := service.List(ctx, ownerID, dto.TransactionFilter{Search: "groceries"})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if pagination.TotalItems != 2 {
			t.Fatalf("total items = %d, want 2", pagination.TotalItems)
		}
	})

	t.Run("another user sees nothing", func(t *testing.T) {
		items, pagination, err := service.List(ctx, intruderID, dto.TransactionFilter{})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(items) != 0 || pagination.TotalItems != 0 {
			t.Fatalf("another user saw %d rows", pagination.TotalItems)
		}
	})

	t.Run("empty result is not an error", func(t *testing.T) {
		from := dateOf("2030-01-01")
		to := dateOf("2030-12-31")
		items, pagination, err := service.List(ctx, ownerID, dto.TransactionFilter{DateFrom: &from, DateTo: &to})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(items) != 0 || pagination.TotalItems != 0 {
			t.Fatalf("expected no rows, got %d", pagination.TotalItems)
		}
		if items == nil {
			t.Fatal("items should be an empty slice so the JSON is [] rather than null")
		}
	})
}
