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

type AccountHandler struct {
	accounts  service.AccountService
	validator *validator.Validator
}

func NewAccountHandler(accounts service.AccountService, v *validator.Validator) *AccountHandler {
	return &AccountHandler{accounts: accounts, validator: v}
}

// List returns the caller's accounts.
//
//	@Summary		List accounts
//	@Tags			Accounts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			account_type	query		string	false	"CASH_FLOW, WALLET, BANK, CREDIT_CARD or SAVINGS"
//	@Param			is_active		query		bool	false	"Filter by active flag"
//	@Param			search			query		string	false	"Match name or description"
//	@Param			page			query		int		false	"Page number"
//	@Param			page_size		query		int		false	"Rows per page, max 100"
//	@Param			sort_by			query		string	false	"name, account_type, opening_balance or created_at"
//	@Param			sort_dir		query		string	false	"asc or desc"
//	@Success		200				{object}	utils.Envelope{data=utils.PagedData{items=[]dto.AccountResponse}}
//	@Failure		401				{object}	utils.Envelope
//	@Router			/accounts [get]
func (h *AccountHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserID(r.Context())
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	isActive, err := queryBoolPtr(r, "is_active")
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	query := dto.AccountListQuery{
		AccountType: strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("account_type"))),
		IsActive:    isActive,
		Search:      strings.TrimSpace(r.URL.Query().Get("search")),
		Page:        queryInt(r, "page", 1),
		PageSize:    queryInt(r, "page_size", 50),
		SortBy:      r.URL.Query().Get("sort_by"),
		SortDir:     r.URL.Query().Get("sort_dir"),
	}

	items, pagination, err := h.accounts.List(r.Context(), userID, query)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Data retrieved successfully", utils.PagedData{Items: items, Pagination: pagination})
}

// Create adds an account.
//
//	@Summary		Create account
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		dto.AccountCreateRequest	true	"Account"
//	@Success		201		{object}	utils.Envelope{data=dto.AccountResponse}
//	@Failure		409		{object}	utils.Envelope
//	@Failure		422		{object}	utils.Envelope
//	@Router			/accounts [post]
func (h *AccountHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserID(r.Context())
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	var request dto.AccountCreateRequest
	if err := decode(r, h.validator, &request); err != nil {
		utils.WriteError(w, err)
		return
	}

	account, err := h.accounts.Create(r.Context(), userID, request)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.Created(w, "Account created successfully", account)
}

// Get returns one account.
//
//	@Summary		Get account
//	@Tags			Accounts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Account ID"
//	@Success		200	{object}	utils.Envelope{data=dto.AccountResponse}
//	@Failure		404	{object}	utils.Envelope
//	@Router			/accounts/{id} [get]
func (h *AccountHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, id, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	account, err := h.accounts.Get(r.Context(), userID, id)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Data retrieved successfully", account)
}

// Update replaces an account.
//
//	@Summary		Update account
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int							true	"Account ID"
//	@Param			request	body		dto.AccountCreateRequest	true	"Account"
//	@Success		200		{object}	utils.Envelope{data=dto.AccountResponse}
//	@Failure		404		{object}	utils.Envelope
//	@Router			/accounts/{id} [put]
func (h *AccountHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, id, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	var request dto.AccountUpdateRequest
	if err := decode(r, h.validator, &request); err != nil {
		utils.WriteError(w, err)
		return
	}

	account, err := h.accounts.Update(r.Context(), userID, id, request)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Account updated successfully", account)
}

// SetStatus activates or deactivates an account.
//
//	@Summary		Set account status
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int							true	"Account ID"
//	@Param			request	body		dto.AccountStatusRequest	true	"Status"
//	@Success		200		{object}	utils.Envelope{data=dto.AccountResponse}
//	@Failure		404		{object}	utils.Envelope
//	@Router			/accounts/{id}/status [patch]
func (h *AccountHandler) SetStatus(w http.ResponseWriter, r *http.Request) {
	userID, id, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	var request dto.AccountStatusRequest
	if err := decode(r, h.validator, &request); err != nil {
		utils.WriteError(w, err)
		return
	}

	account, err := h.accounts.SetActive(r.Context(), userID, id, *request.IsActive)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Account status updated successfully", account)
}

// Delete removes an unused account.
//
//	@Summary		Delete account
//	@Tags			Accounts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Account ID"
//	@Success		200	{object}	utils.Envelope
//	@Failure		409	{object}	utils.Envelope
//	@Router			/accounts/{id} [delete]
func (h *AccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, id, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	if err := h.accounts.Delete(r.Context(), userID, id); err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Account deleted successfully", nil)
}

// RecordBalance stores the balance an account actually held on a date.
//
//	@Summary		Record observed balance
//	@Description	Stores what the account really held on a day. Recording the same day twice
//	@Description	overwrites the figure. The gap against the recorded transactions is what the
//	@Description	daily report shows as spending that was never written down.
//	@Tags			Accounts
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int							true	"Account ID"
//	@Param			request	body		dto.BalanceSnapshotRequest	true	"Balance"
//	@Success		201		{object}	utils.Envelope{data=dto.BalanceSnapshotResponse}
//	@Failure		404		{object}	utils.Envelope
//	@Router			/accounts/{id}/balances [post]
func (h *AccountHandler) RecordBalance(w http.ResponseWriter, r *http.Request) {
	userID, id, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	var request dto.BalanceSnapshotRequest
	if err := decode(r, h.validator, &request); err != nil {
		utils.WriteError(w, err)
		return
	}

	snapshot, err := h.accounts.RecordBalance(r.Context(), userID, id, request)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.Created(w, "Balance recorded successfully", snapshot)
}

// ListBalances returns the recorded balance history for an account.
//
//	@Summary		List observed balances
//	@Tags			Accounts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"Account ID"
//	@Success		200	{object}	utils.Envelope{data=[]dto.BalanceSnapshotResponse}
//	@Failure		404	{object}	utils.Envelope
//	@Router			/accounts/{id}/balances [get]
func (h *AccountHandler) ListBalances(w http.ResponseWriter, r *http.Request) {
	userID, id, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	snapshots, err := h.accounts.ListBalances(r.Context(), userID, id)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Data retrieved successfully", snapshots)
}

// DeleteBalance removes one recorded balance.
//
//	@Summary		Delete observed balance
//	@Tags			Accounts
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id			path		int	true	"Account ID"
//	@Param			balance_id	path		int	true	"Balance ID"
//	@Success		200			{object}	utils.Envelope
//	@Failure		404			{object}	utils.Envelope
//	@Router			/accounts/{id}/balances/{balance_id} [delete]
func (h *AccountHandler) DeleteBalance(w http.ResponseWriter, r *http.Request) {
	userID, id, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	balanceID, err := idParam(r, "balance_id")
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	if err := h.accounts.DeleteBalance(r.Context(), userID, id, balanceID); err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Balance deleted successfully", nil)
}

func (h *AccountHandler) scope(r *http.Request) (int64, int64, error) {
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
