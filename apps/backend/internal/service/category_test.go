package service

import (
	"context"
	"errors"
	"testing"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

func newCategoryService() (CategoryService, *fakeCategoryRepository) {
	categories := newFakeCategoryRepository()
	return NewCategoryService(categories), categories
}

func TestCreateCategory(t *testing.T) {
	service, _ := newCategoryService()

	created, err := service.Create(context.Background(), ownerID, dto.CategoryCreateRequest{
		Name:        "  Food  ",
		Type:        "EXPENSE",
		Description: "  Daily meals  ",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.Name != "Food" || created.Description != "Daily meals" {
		t.Fatalf("fields were not trimmed: %+v", created)
	}
	if !created.IsActive {
		t.Fatal("a new category should be active")
	}
}

func TestCreateCategoryRejectsDuplicateNamePerType(t *testing.T) {
	service, _ := newCategoryService()
	ctx := context.Background()

	request := dto.CategoryCreateRequest{Name: "Food", Type: "EXPENSE"}
	if _, err := service.Create(ctx, ownerID, request); err != nil {
		t.Fatalf("first Create returned error: %v", err)
	}

	if _, err := service.Create(ctx, ownerID, request); !errors.Is(err, utils.ErrConflict) {
		t.Fatalf("duplicate Create = %v, want ErrConflict", err)
	}

	// The same name is allowed under the other type, and for another user.
	if _, err := service.Create(ctx, ownerID, dto.CategoryCreateRequest{Name: "Food", Type: "INCOME"}); err != nil {
		t.Fatalf("same name under another type should be allowed: %v", err)
	}
	if _, err := service.Create(ctx, intruderID, request); err != nil {
		t.Fatalf("same name for another user should be allowed: %v", err)
	}
}

func TestCategoryOwnership(t *testing.T) {
	service, categories := newCategoryService()
	owned := categories.seed(ownerID, "Food", model.CategoryTypeExpense, true)
	ctx := context.Background()

	if _, err := service.Get(ctx, intruderID, owned.ID); !errors.Is(err, utils.ErrNotFound) {
		t.Fatalf("Get as another user = %v, want ErrNotFound", err)
	}

	_, err := service.Update(ctx, intruderID, owned.ID, dto.CategoryUpdateRequest{Name: "Hijacked", Type: "EXPENSE"})
	if !errors.Is(err, utils.ErrNotFound) {
		t.Fatalf("Update as another user = %v, want ErrNotFound", err)
	}

	if err := service.Delete(ctx, intruderID, owned.ID); !errors.Is(err, utils.ErrNotFound) {
		t.Fatalf("Delete as another user = %v, want ErrNotFound", err)
	}

	if _, err := service.SetActive(ctx, intruderID, owned.ID, false); !errors.Is(err, utils.ErrNotFound) {
		t.Fatalf("SetActive as another user = %v, want ErrNotFound", err)
	}

	// The row is untouched.
	still, err := service.Get(ctx, ownerID, owned.ID)
	if err != nil {
		t.Fatalf("the owner lost access: %v", err)
	}
	if still.Name != "Food" || !still.IsActive {
		t.Fatalf("category was modified by another user: %+v", still)
	}
}

func TestDeleteCategoryInUseIsRefused(t *testing.T) {
	service, categories := newCategoryService()
	inUse := categories.seed(ownerID, "Cicilan Rumah", model.CategoryTypeExpense, true)
	categories.usage[inUse.ID] = 3

	err := service.Delete(context.Background(), ownerID, inUse.ID)
	if !errors.Is(err, utils.ErrConflict) {
		t.Fatalf("Delete of a category in use = %v, want ErrConflict", err)
	}
	// Deleting would either orphan or erase financial history, so the row must survive.
	if _, present := categories.categories[inUse.ID]; !present {
		t.Fatal("the category was deleted even though transactions reference it")
	}
}

func TestDeleteUnusedCategory(t *testing.T) {
	service, categories := newCategoryService()
	unused := categories.seed(ownerID, "Unused", model.CategoryTypeExpense, true)

	if err := service.Delete(context.Background(), ownerID, unused.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, present := categories.categories[unused.ID]; present {
		t.Fatal("the category was not deleted")
	}
}

func TestChangingTypeOfUsedCategoryIsRefused(t *testing.T) {
	service, categories := newCategoryService()
	inUse := categories.seed(ownerID, "Cicilan Rumah", model.CategoryTypeExpense, true)
	categories.usage[inUse.ID] = 2

	// Flipping the type would silently reclassify historical transactions.
	_, err := service.Update(context.Background(), ownerID, inUse.ID, dto.CategoryUpdateRequest{
		Name: "Cicilan Rumah", Type: "INCOME",
	})
	if !errors.Is(err, utils.ErrConflict) {
		t.Fatalf("type change on a used category = %v, want ErrConflict", err)
	}

	// Renaming without changing the type stays allowed.
	updated, err := service.Update(context.Background(), ownerID, inUse.ID, dto.CategoryUpdateRequest{
		Name: "Mortgage", Type: "EXPENSE",
	})
	if err != nil {
		t.Fatalf("rename returned error: %v", err)
	}
	if updated.Name != "Mortgage" {
		t.Fatalf("name = %q, want Mortgage", updated.Name)
	}
}

func TestSetActiveTogglesTheFlag(t *testing.T) {
	service, categories := newCategoryService()
	category := categories.seed(ownerID, "Food", model.CategoryTypeExpense, true)
	ctx := context.Background()

	deactivated, err := service.SetActive(ctx, ownerID, category.ID, false)
	if err != nil {
		t.Fatalf("SetActive returned error: %v", err)
	}
	if deactivated.IsActive {
		t.Fatal("category should be inactive")
	}

	reactivated, err := service.SetActive(ctx, ownerID, category.ID, true)
	if err != nil {
		t.Fatalf("SetActive returned error: %v", err)
	}
	if !reactivated.IsActive {
		t.Fatal("category should be active again")
	}
}

func TestListCategoriesFilters(t *testing.T) {
	service, categories := newCategoryService()
	categories.seed(ownerID, "Salary", model.CategoryTypeIncome, true)
	categories.seed(ownerID, "Food", model.CategoryTypeExpense, true)
	categories.seed(ownerID, "Retired", model.CategoryTypeExpense, false)
	categories.seed(intruderID, "Theirs", model.CategoryTypeExpense, true)
	ctx := context.Background()

	t.Run("owner only", func(t *testing.T) {
		_, pagination, err := service.List(ctx, ownerID, dto.CategoryListQuery{})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if pagination.TotalItems != 3 {
			t.Fatalf("total items = %d, want 3", pagination.TotalItems)
		}
	})

	t.Run("by type", func(t *testing.T) {
		items, _, err := service.List(ctx, ownerID, dto.CategoryListQuery{Type: "INCOME"})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(items) != 1 || items[0].Name != "Salary" {
			t.Fatalf("items = %+v, want only Salary", items)
		}
	})

	t.Run("by active flag", func(t *testing.T) {
		active := true
		items, _, err := service.List(ctx, ownerID, dto.CategoryListQuery{IsActive: &active})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("items = %d, want 2 active", len(items))
		}
	})

	t.Run("paging defaults are applied", func(t *testing.T) {
		_, pagination, err := service.List(ctx, ownerID, dto.CategoryListQuery{})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if pagination.Page != 1 || pagination.PageSize != 10 {
			t.Fatalf("pagination = %+v, want page 1 size 10", pagination)
		}
	})
}
