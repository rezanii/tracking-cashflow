package dto

import "time"

type CategoryCreateRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=100" example:"Food"`
	Type        string `json:"type" validate:"required,oneof=INCOME EXPENSE" example:"EXPENSE"`
	Description string `json:"description" validate:"max=255" example:"Daily meals"`
}

type CategoryUpdateRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=100" example:"Food"`
	Type        string `json:"type" validate:"required,oneof=INCOME EXPENSE" example:"EXPENSE"`
	Description string `json:"description" validate:"max=255" example:"Daily meals"`
}

type CategoryStatusRequest struct {
	IsActive *bool `json:"is_active" validate:"required" example:"false"`
}

type CategoryResponse struct {
	ID          int64     `json:"id" example:"1"`
	Name        string    `json:"name" example:"Food"`
	Type        string    `json:"type" example:"EXPENSE"`
	Description string    `json:"description" example:"Daily meals"`
	IsActive    bool      `json:"is_active" example:"true"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CategoryListQuery struct {
	Type     string
	IsActive *bool
	Search   string
	Page     int
	PageSize int
	SortBy   string
	SortDir  string
}
