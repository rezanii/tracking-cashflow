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

type TransactionHandler struct {
	transactions service.TransactionService
	validator    *validator.Validator
}

func NewTransactionHandler(transactions service.TransactionService, v *validator.Validator) *TransactionHandler {
	return &TransactionHandler{transactions: transactions, validator: v}
}

// List returns a filtered, paginated page of the caller's transactions.
//
//	@Summary		List transactions
//	@Tags			Transactions
//	@Produce		json
//	@Security		BearerAuth
//	@Param			search				query		string	false	"Match description or reference number"
//	@Param			date_from			query		string	false	"Start date, YYYY-MM-DD"
//	@Param			date_to				query		string	false	"End date, YYYY-MM-DD"
//	@Param			transaction_type	query		string	false	"INCOME, EXPENSE or TRANSFER"
//	@Param			category_id			query		int		false	"Category id"
//	@Param			page				query		int		false	"Page number"
//	@Param			page_size			query		int		false	"Rows per page, max 100"
//	@Param			sort_by				query		string	false	"transaction_date, amount or created_at"
//	@Param			sort_dir			query		string	false	"asc or desc"
//	@Success		200					{object}	utils.Envelope{data=utils.PagedData{items=[]dto.TransactionResponse}}
//	@Failure		401					{object}	utils.Envelope
//	@Router			/transactions [get]
func (h *TransactionHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserID(r.Context())
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	filter, err := h.filter(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	items, pagination, err := h.transactions.List(r.Context(), userID, filter)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Data retrieved successfully", utils.PagedData{Items: items, Pagination: pagination})
}

// Create records a transaction.
//
//	@Summary		Create transaction
//	@Tags			Transactions
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		dto.TransactionCreateRequest	true	"Transaction payload"
//	@Success		201		{object}	utils.Envelope{data=dto.TransactionResponse}
//	@Failure		422		{object}	utils.Envelope
//	@Router			/transactions [post]
func (h *TransactionHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserID(r.Context())
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	var request dto.TransactionCreateRequest
	if err := decode(r, h.validator, &request); err != nil {
		utils.WriteError(w, err)
		return
	}

	transaction, err := h.transactions.Create(r.Context(), userID, request)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.Created(w, "Transaction created successfully", transaction)
}

// Get returns one transaction.
//
//	@Summary		Get transaction
//	@Tags			Transactions
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Transaction id"
//	@Success		200	{object}	utils.Envelope{data=dto.TransactionResponse}
//	@Failure		404	{object}	utils.Envelope
//	@Router			/transactions/{id} [get]
func (h *TransactionHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, id, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	transaction, err := h.transactions.Get(r.Context(), userID, id)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Data retrieved successfully", transaction)
}

// Update replaces a transaction.
//
//	@Summary		Update transaction
//	@Tags			Transactions
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int								true	"Transaction id"
//	@Param			request	body		dto.TransactionCreateRequest	true	"Transaction payload"
//	@Success		200		{object}	utils.Envelope{data=dto.TransactionResponse}
//	@Failure		404		{object}	utils.Envelope
//	@Failure		422		{object}	utils.Envelope
//	@Router			/transactions/{id} [put]
func (h *TransactionHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, id, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	var request dto.TransactionUpdateRequest
	if err := decode(r, h.validator, &request); err != nil {
		utils.WriteError(w, err)
		return
	}

	transaction, err := h.transactions.Update(r.Context(), userID, id, request)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Transaction updated successfully", transaction)
}

// Delete removes a transaction.
//
//	@Summary		Delete transaction
//	@Tags			Transactions
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Transaction id"
//	@Success		200	{object}	utils.Envelope
//	@Failure		404	{object}	utils.Envelope
//	@Router			/transactions/{id} [delete]
func (h *TransactionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, id, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	if err := h.transactions.Delete(r.Context(), userID, id); err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Transaction deleted successfully", nil)
}

func (h *TransactionHandler) filter(r *http.Request) (dto.TransactionFilter, error) {
	from, err := queryDatePtr(r, "date_from")
	if err != nil {
		return dto.TransactionFilter{}, err
	}
	to, err := queryDatePtr(r, "date_to")
	if err != nil {
		return dto.TransactionFilter{}, err
	}
	if from != nil && to != nil && to.Before(*from) {
		return dto.TransactionFilter{}, utils.NewFieldError("Validation failed", map[string]string{
			"date_to": "date_to must not be earlier than date_from",
		})
	}

	categoryID, err := queryInt64Ptr(r, "category_id")
	if err != nil {
		return dto.TransactionFilter{}, err
	}
	transactionType, err := queryTransactionType(r, "transaction_type")
	if err != nil {
		return dto.TransactionFilter{}, err
	}

	return dto.TransactionFilter{
		Search:          strings.TrimSpace(r.URL.Query().Get("search")),
		DateFrom:        from,
		DateTo:          to,
		TransactionType: transactionType,
		CategoryID:      categoryID,
		Page:            queryInt(r, "page", 1),
		PageSize:        queryInt(r, "page_size", 10),
		SortBy:          r.URL.Query().Get("sort_by"),
		SortDir:         r.URL.Query().Get("sort_dir"),
	}, nil
}

func (h *TransactionHandler) scope(r *http.Request) (int64, int64, error) {
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
