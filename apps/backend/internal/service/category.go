package service

import (
	"context"
	"strings"
	"time"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/repository"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

type CategoryService interface {
	Create(ctx context.Context, userID int64, request dto.CategoryCreateRequest) (dto.CategoryResponse, error)
	Update(ctx context.Context, userID, id int64, request dto.CategoryUpdateRequest) (dto.CategoryResponse, error)
	Delete(ctx context.Context, userID, id int64) error
	SetActive(ctx context.Context, userID, id int64, isActive bool) (dto.CategoryResponse, error)
	Get(ctx context.Context, userID, id int64) (dto.CategoryResponse, error)
	List(ctx context.Context, userID int64, query dto.CategoryListQuery) ([]dto.CategoryResponse, utils.Pagination, error)
}

type categoryService struct {
	categories repository.CategoryRepository
}

func NewCategoryService(categories repository.CategoryRepository) CategoryService {
	return &categoryService{categories: categories}
}

func (s *categoryService) Create(ctx context.Context, userID int64, request dto.CategoryCreateRequest) (dto.CategoryResponse, error) {
	name := strings.TrimSpace(request.Name)
	categoryType := model.CategoryType(request.Type)

	duplicate, err := s.categories.FindByName(ctx, userID, categoryType, name)
	if err != nil {
		return dto.CategoryResponse{}, utils.WrapDomainError(utils.ErrConflict, "Failed to verify category name", err)
	}
	if duplicate != nil {
		return dto.CategoryResponse{}, utils.Conflict("Category with the same name and type already exists")
	}

	now := time.Now().UTC()
	category := &model.Category{
		UserID:      userID,
		Name:        name,
		Type:        categoryType,
		Description: strings.TrimSpace(request.Description),
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.categories.Create(ctx, category); err != nil {
		return dto.CategoryResponse{}, utils.WrapDomainError(utils.ErrConflict, "Failed to create category", err)
	}
	return toCategoryResponse(*category), nil
}

func (s *categoryService) Update(ctx context.Context, userID, id int64, request dto.CategoryUpdateRequest) (dto.CategoryResponse, error) {
	existing, err := s.mustFind(ctx, userID, id)
	if err != nil {
		return dto.CategoryResponse{}, err
	}

	name := strings.TrimSpace(request.Name)
	categoryType := model.CategoryType(request.Type)

	if name != existing.Name || categoryType != existing.Type {
		duplicate, err := s.categories.FindByName(ctx, userID, categoryType, name)
		if err != nil {
			return dto.CategoryResponse{}, utils.WrapDomainError(utils.ErrConflict, "Failed to verify category name", err)
		}
		if duplicate != nil && duplicate.ID != id {
			return dto.CategoryResponse{}, utils.Conflict("Category with the same name and type already exists")
		}
	}

	// Changing the type of a category that is already in use would silently reclassify
	// historical transactions, so it is refused.
	if categoryType != existing.Type {
		used, err := s.categories.CountTransactions(ctx, userID, id)
		if err != nil {
			return dto.CategoryResponse{}, utils.WrapDomainError(utils.ErrConflict, "Failed to check category usage", err)
		}
		if used > 0 {
			return dto.CategoryResponse{}, utils.Conflict("Category type cannot change while transactions still use it")
		}
	}

	existing.Name = name
	existing.Type = categoryType
	existing.Description = strings.TrimSpace(request.Description)
	existing.UpdatedAt = time.Now().UTC()

	if err := s.categories.Update(ctx, existing); err != nil {
		return dto.CategoryResponse{}, wrapIfInternal(err, "Failed to update category")
	}
	return toCategoryResponse(*existing), nil
}

func (s *categoryService) Delete(ctx context.Context, userID, id int64) error {
	if _, err := s.mustFind(ctx, userID, id); err != nil {
		return err
	}

	used, err := s.categories.CountTransactions(ctx, userID, id)
	if err != nil {
		return utils.WrapDomainError(utils.ErrConflict, "Failed to check category usage", err)
	}
	if used > 0 {
		return utils.Conflict("Category is used by existing transactions, deactivate it instead")
	}

	if err := s.categories.Delete(ctx, userID, id); err != nil {
		return wrapIfInternal(err, "Failed to delete category")
	}
	return nil
}

func (s *categoryService) SetActive(ctx context.Context, userID, id int64, isActive bool) (dto.CategoryResponse, error) {
	if _, err := s.mustFind(ctx, userID, id); err != nil {
		return dto.CategoryResponse{}, err
	}
	if err := s.categories.SetActive(ctx, userID, id, isActive); err != nil {
		return dto.CategoryResponse{}, wrapIfInternal(err, "Failed to update category status")
	}
	return s.Get(ctx, userID, id)
}

func (s *categoryService) Get(ctx context.Context, userID, id int64) (dto.CategoryResponse, error) {
	category, err := s.mustFind(ctx, userID, id)
	if err != nil {
		return dto.CategoryResponse{}, err
	}
	return toCategoryResponse(*category), nil
}

func (s *categoryService) List(ctx context.Context, userID int64, query dto.CategoryListQuery) ([]dto.CategoryResponse, utils.Pagination, error) {
	query.Page, query.PageSize = utils.NormalizePaging(query.Page, query.PageSize)

	categories, total, err := s.categories.List(ctx, userID, query)
	if err != nil {
		return nil, utils.Pagination{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to list categories", err)
	}

	responses := make([]dto.CategoryResponse, 0, len(categories))
	for _, category := range categories {
		responses = append(responses, toCategoryResponse(category))
	}
	return responses, utils.NewPagination(query.Page, query.PageSize, total), nil
}

func (s *categoryService) mustFind(ctx context.Context, userID, id int64) (*model.Category, error) {
	category, err := s.categories.FindByID(ctx, userID, id)
	if err != nil {
		return nil, utils.WrapDomainError(utils.ErrNotFound, "Failed to load category", err)
	}
	if category == nil {
		return nil, utils.NotFound("Category")
	}
	return category, nil
}

func toCategoryResponse(category model.Category) dto.CategoryResponse {
	return dto.CategoryResponse{
		ID:          category.ID,
		Name:        category.Name,
		Type:        string(category.Type),
		Description: category.Description,
		IsActive:    category.IsActive,
		CreatedAt:   category.CreatedAt,
		UpdatedAt:   category.UpdatedAt,
	}
}

// wrapIfInternal keeps an already-classified domain error intact and wraps anything else.
func wrapIfInternal(err error, message string) error {
	if _, ok := utils.AsDomainError(err); ok {
		return err
	}
	return utils.WrapDomainError(utils.ErrConflict, message, err)
}
