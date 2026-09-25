package handler

import (
	"net/http"
	"strings"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/middleware"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/service"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/validator"
)

type CategoryHandler struct {
	categories service.CategoryService
	validator  *validator.Validator
}

func NewCategoryHandler(categories service.CategoryService, v *validator.Validator) *CategoryHandler {
	return &CategoryHandler{categories: categories, validator: v}
}

// List returns the caller's categories.
//
//	@Summary		List categories
//	@Tags			Categories
//	@Produce		json
//	@Security		BearerAuth
//	@Param			type		query		string	false	"INCOME or EXPENSE"
//	@Param			is_active	query		bool	false	"Filter by active flag"
//	@Param			search		query		string	false	"Match name or description"
//	@Param			page		query		int		false	"Page number"
//	@Param			page_size	query		int		false	"Rows per page, max 100"
//	@Param			sort_by		query		string	false	"name, type or created_at"
//	@Param			sort_dir	query		string	false	"asc or desc"
//	@Success		200			{object}	utils.Envelope{data=utils.PagedData{items=[]dto.CategoryResponse}}
//	@Failure		401			{object}	utils.Envelope
//	@Router			/categories [get]
func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserID(r.Context())
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	categoryType, err := queryCategoryType(r, "type")
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	isActive, err := queryBoolPtr(r, "is_active")
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	query := dto.CategoryListQuery{
		Type:     categoryType,
		IsActive: isActive,
		Search:   strings.TrimSpace(r.URL.Query().Get("search")),
		Page:     queryInt(r, "page", 1),
		PageSize: queryInt(r, "page_size", 10),
		SortBy:   r.URL.Query().Get("sort_by"),
		SortDir:  r.URL.Query().Get("sort_dir"),
	}

	items, pagination, err := h.categories.List(r.Context(), userID, query)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Data retrieved successfully", utils.PagedData{Items: items, Pagination: pagination})
}

// Create adds a category.
//
//	@Summary		Create category
//	@Tags			Categories
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		dto.CategoryCreateRequest	true	"Category payload"
//	@Success		201		{object}	utils.Envelope{data=dto.CategoryResponse}
//	@Failure		409		{object}	utils.Envelope
//	@Failure		422		{object}	utils.Envelope
//	@Router			/categories [post]
func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserID(r.Context())
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	var request dto.CategoryCreateRequest
	if err := decode(r, h.validator, &request); err != nil {
		utils.WriteError(w, err)
		return
	}

	category, err := h.categories.Create(r.Context(), userID, request)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.Created(w, "Category created successfully", category)
}

// Get returns one category.
//
//	@Summary		Get category
//	@Tags			Categories
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Category id"
//	@Success		200	{object}	utils.Envelope{data=dto.CategoryResponse}
//	@Failure		404	{object}	utils.Envelope
//	@Router			/categories/{id} [get]
func (h *CategoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, id, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	category, err := h.categories.Get(r.Context(), userID, id)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Data retrieved successfully", category)
}

// Update replaces a category.
//
//	@Summary		Update category
//	@Tags			Categories
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int							true	"Category id"
//	@Param			request	body		dto.CategoryUpdateRequest	true	"Category payload"
//	@Success		200		{object}	utils.Envelope{data=dto.CategoryResponse}
//	@Failure		404		{object}	utils.Envelope
//	@Failure		409		{object}	utils.Envelope
//	@Router			/categories/{id} [put]
func (h *CategoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, id, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	var request dto.CategoryUpdateRequest
	if err := decode(r, h.validator, &request); err != nil {
		utils.WriteError(w, err)
		return
	}

	category, err := h.categories.Update(r.Context(), userID, id, request)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Category updated successfully", category)
}

// SetStatus activates or deactivates a category.
//
//	@Summary		Activate or deactivate category
//	@Tags			Categories
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int							true	"Category id"
//	@Param			request	body		dto.CategoryStatusRequest	true	"Status payload"
//	@Success		200		{object}	utils.Envelope{data=dto.CategoryResponse}
//	@Failure		404		{object}	utils.Envelope
//	@Router			/categories/{id}/status [patch]
func (h *CategoryHandler) SetStatus(w http.ResponseWriter, r *http.Request) {
	userID, id, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	var request dto.CategoryStatusRequest
	if err := decode(r, h.validator, &request); err != nil {
		utils.WriteError(w, err)
		return
	}

	category, err := h.categories.SetActive(r.Context(), userID, id, *request.IsActive)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Category status updated successfully", category)
}

// Delete removes a category that no transaction references.
//
//	@Summary		Delete category
//	@Tags			Categories
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Category id"
//	@Success		200	{object}	utils.Envelope
//	@Failure		404	{object}	utils.Envelope
//	@Failure		409	{object}	utils.Envelope
//	@Router			/categories/{id} [delete]
func (h *CategoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, id, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	if err := h.categories.Delete(r.Context(), userID, id); err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Category deleted successfully", nil)
}

// scope resolves the caller and the path id together, so every handler below is scoped to
// the owner without repeating the checks.
func (h *CategoryHandler) scope(r *http.Request) (int64, int64, error) {
	userID, err := middleware.UserID(r.Context())
	if err != nil {
		return 0, 0, err
	}
	id, err := idParam(r, "id")
	if err != nil {
		return 0, 0, err
	}
	return userID, id, nil
}
