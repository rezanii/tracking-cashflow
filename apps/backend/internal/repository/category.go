package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

type CategoryRepository interface {
	WithTx(tx *gorm.DB) CategoryRepository
	Create(ctx context.Context, category *model.Category) error
	Update(ctx context.Context, category *model.Category) error
	Delete(ctx context.Context, userID, id int64) error
	FindByID(ctx context.Context, userID, id int64) (*model.Category, error)
	FindByName(ctx context.Context, userID int64, categoryType model.CategoryType, name string) (*model.Category, error)
	List(ctx context.Context, userID int64, query dto.CategoryListQuery) ([]model.Category, int64, error)
	CountTransactions(ctx context.Context, userID, id int64) (int64, error)
	SetActive(ctx context.Context, userID, id int64, isActive bool) error
}

type categoryRepository struct {
	db *gorm.DB
}

func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{db: db}
}

func (r *categoryRepository) WithTx(tx *gorm.DB) CategoryRepository {
	return &categoryRepository{db: tx}
}

func (r *categoryRepository) Create(ctx context.Context, category *model.Category) error {
	return r.db.WithContext(ctx).Create(category).Error
}

// Update scopes by user id as well as primary key, so a mismatched pair changes nothing
// instead of touching another user's row.
func (r *categoryRepository) Update(ctx context.Context, category *model.Category) error {
	result := r.db.WithContext(ctx).
		Model(&model.Category{}).
		Where("id = ? AND user_id = ?", category.ID, category.UserID).
		Updates(map[string]any{
			"name":        category.Name,
			"type":        category.Type,
			"description": category.Description,
			"updated_at":  category.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return utils.NotFound("Category")
	}
	return nil
}

func (r *categoryRepository) Delete(ctx context.Context, userID, id int64) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Category{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return utils.NotFound("Category")
	}
	return nil
}

func (r *categoryRepository) FindByID(ctx context.Context, userID, id int64) (*model.Category, error) {
	var category model.Category
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Take(&category).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *categoryRepository) FindByName(ctx context.Context, userID int64, categoryType model.CategoryType, name string) (*model.Category, error) {
	var category model.Category
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND type = ? AND name = ?", userID, categoryType, name).
		Take(&category).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &category, nil
}

var categorySortColumns = map[string]string{
	"name":       "name",
	"type":       "type",
	"created_at": "created_at",
}

func (r *categoryRepository) List(ctx context.Context, userID int64, query dto.CategoryListQuery) ([]model.Category, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.Category{}).Where("user_id = ?", userID)

	if query.Type != "" {
		base = base.Where("type = ?", query.Type)
	}
	if query.IsActive != nil {
		base = base.Where("is_active = ?", *query.IsActive)
	}
	if query.Search != "" {
		pattern := "%" + query.Search + "%"
		base = base.Where("(name LIKE ? OR description LIKE ?)", pattern, pattern)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	column, direction := utils.NormalizeSort(query.SortBy, query.SortDir, categorySortColumns, "name")

	var categories []model.Category
	err := base.
		Order(column + " " + direction).
		Order("id DESC").
		Limit(query.PageSize).
		Offset(utils.Offset(query.Page, query.PageSize)).
		Find(&categories).Error
	if err != nil {
		return nil, 0, err
	}
	return categories, total, nil
}

func (r *categoryRepository) CountTransactions(ctx context.Context, userID, id int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Transaction{}).
		Where("user_id = ? AND category_id = ?", userID, id).
		Count(&count).Error
	return count, err
}

func (r *categoryRepository) SetActive(ctx context.Context, userID, id int64, isActive bool) error {
	result := r.db.WithContext(ctx).
		Model(&model.Category{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{"is_active": isActive, "updated_at": gorm.Expr("NOW()")})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return utils.NotFound("Category")
	}
	return nil
}
