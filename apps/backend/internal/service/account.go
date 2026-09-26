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

// snapshotHistoryLimit bounds the history a single request returns.
const snapshotHistoryLimit = 60

type AccountService interface {
	Create(ctx context.Context, userID int64, request dto.AccountCreateRequest) (dto.AccountResponse, error)
	Update(ctx context.Context, userID, id int64, request dto.AccountUpdateRequest) (dto.AccountResponse, error)
	Delete(ctx context.Context, userID, id int64) error
	SetActive(ctx context.Context, userID, id int64, isActive bool) (dto.AccountResponse, error)
	Get(ctx context.Context, userID, id int64) (dto.AccountResponse, error)
	List(ctx context.Context, userID int64, query dto.AccountListQuery) ([]dto.AccountResponse, utils.Pagination, error)

	RecordBalance(ctx context.Context, userID, accountID int64, request dto.BalanceSnapshotRequest) (dto.BalanceSnapshotResponse, error)
	ListBalances(ctx context.Context, userID, accountID int64) ([]dto.BalanceSnapshotResponse, error)
	DeleteBalance(ctx context.Context, userID, accountID, id int64) error
}

type accountService struct {
	accounts repository.AccountRepository
}

func NewAccountService(accounts repository.AccountRepository) AccountService {
	return &accountService{accounts: accounts}
}

func (s *accountService) Create(ctx context.Context, userID int64, request dto.AccountCreateRequest) (dto.AccountResponse, error) {
	name := strings.TrimSpace(request.Name)
	accountType := model.AccountType(strings.ToUpper(strings.TrimSpace(request.AccountType)))
	if !accountType.Valid() {
		return dto.AccountResponse{}, utils.NewFieldError("Validation failed", map[string]string{
			"account_type": "account_type must be one of: CASH_FLOW, WALLET, BANK, CREDIT_CARD, SAVINGS",
		})
	}

	duplicate, err := s.accounts.FindByName(ctx, userID, name)
	if err != nil {
		return dto.AccountResponse{}, utils.WrapDomainError(utils.ErrConflict, "Failed to verify account name", err)
	}
	if duplicate != nil {
		return dto.AccountResponse{}, utils.Conflict("Account with the same name already exists")
	}

	now := time.Now().UTC()
	account := &model.Account{
		UserID:         userID,
		Name:           name,
		AccountType:    accountType,
		OpeningBalance: utils.Round(request.OpeningBalance),
		Description:    strings.TrimSpace(request.Description),
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.accounts.Create(ctx, account); err != nil {
		return dto.AccountResponse{}, utils.WrapDomainError(utils.ErrConflict, "Failed to create account", err)
	}
	return toAccountResponse(*account), nil
}

func (s *accountService) Update(ctx context.Context, userID, id int64, request dto.AccountUpdateRequest) (dto.AccountResponse, error) {
	existing, err := s.mustFind(ctx, userID, id)
	if err != nil {
		return dto.AccountResponse{}, err
	}

	name := strings.TrimSpace(request.Name)
	accountType := model.AccountType(strings.ToUpper(strings.TrimSpace(request.AccountType)))
	if !accountType.Valid() {
		return dto.AccountResponse{}, utils.NewFieldError("Validation failed", map[string]string{
			"account_type": "account_type must be one of: CASH_FLOW, WALLET, BANK, CREDIT_CARD, SAVINGS",
		})
	}

	if name != existing.Name {
		duplicate, err := s.accounts.FindByName(ctx, userID, name)
		if err != nil {
			return dto.AccountResponse{}, utils.WrapDomainError(utils.ErrConflict, "Failed to verify account name", err)
		}
		if duplicate != nil && duplicate.ID != id {
			return dto.AccountResponse{}, utils.Conflict("Account with the same name already exists")
		}
	}

	existing.Name = name
	existing.AccountType = accountType
	existing.OpeningBalance = utils.Round(request.OpeningBalance)
	existing.Description = strings.TrimSpace(request.Description)
	existing.UpdatedAt = time.Now().UTC()

	if err := s.accounts.Update(ctx, existing); err != nil {
		return dto.AccountResponse{}, wrapIfInternal(err, "Failed to update account")
	}
	return toAccountResponse(*existing), nil
}

// Delete refuses while transactions still point at the account, because the foreign key is
// NO ACTION and a silent orphan would quietly change every past report.
func (s *accountService) Delete(ctx context.Context, userID, id int64) error {
	if _, err := s.mustFind(ctx, userID, id); err != nil {
		return err
	}

	used, err := s.accounts.CountTransactions(ctx, userID, id)
	if err != nil {
		return utils.WrapDomainError(utils.ErrConflict, "Failed to check account usage", err)
	}
	if used > 0 {
		return utils.Conflict("Account is used by transactions; deactivate it instead")
	}

	if err := s.accounts.Delete(ctx, userID, id); err != nil {
		return wrapIfInternal(err, "Failed to delete account")
	}
	return nil
}

func (s *accountService) SetActive(ctx context.Context, userID, id int64, isActive bool) (dto.AccountResponse, error) {
	if err := s.accounts.SetActive(ctx, userID, id, isActive); err != nil {
		return dto.AccountResponse{}, wrapIfInternal(err, "Failed to update account status")
	}
	account, err := s.mustFind(ctx, userID, id)
	if err != nil {
		return dto.AccountResponse{}, err
	}
	return toAccountResponse(*account), nil
}

func (s *accountService) Get(ctx context.Context, userID, id int64) (dto.AccountResponse, error) {
	account, err := s.mustFind(ctx, userID, id)
	if err != nil {
		return dto.AccountResponse{}, err
	}
	return toAccountResponse(*account), nil
}

func (s *accountService) List(ctx context.Context, userID int64, query dto.AccountListQuery) ([]dto.AccountResponse, utils.Pagination, error) {
	query.Page, query.PageSize = utils.NormalizePaging(query.Page, query.PageSize)

	accounts, total, err := s.accounts.List(ctx, userID, query)
	if err != nil {
		return nil, utils.Pagination{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to list accounts", err)
	}

	responses := make([]dto.AccountResponse, 0, len(accounts))
	for _, account := range accounts {
		responses = append(responses, toAccountResponse(account))
	}
	return responses, utils.NewPagination(query.Page, query.PageSize, total), nil
}

// RecordBalance stores what the account really held on a day. Re-recording the same day
// overwrites it, so a corrected count does not leave two conflicting figures behind.
func (s *accountService) RecordBalance(ctx context.Context, userID, accountID int64, request dto.BalanceSnapshotRequest) (dto.BalanceSnapshotResponse, error) {
	account, err := s.mustFind(ctx, userID, accountID)
	if err != nil {
		return dto.BalanceSnapshotResponse{}, err
	}

	asOf, err := utils.ParseDate(request.AsOfDate)
	if err != nil {
		return dto.BalanceSnapshotResponse{}, utils.NewFieldError("Validation failed", map[string]string{
			"as_of_date": err.Error(),
		})
	}

	balance := utils.Round(request.ActualBalance)
	if balance.IsNegative() {
		return dto.BalanceSnapshotResponse{}, utils.NewFieldError("Validation failed", map[string]string{
			"actual_balance": "actual_balance cannot be negative",
		})
	}

	now := time.Now().UTC()
	snapshot := &model.AccountBalanceSnapshot{
		UserID:        userID,
		AccountID:     accountID,
		AsOfDate:      asOf,
		ActualBalance: balance,
		Note:          strings.TrimSpace(request.Note),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.accounts.UpsertSnapshot(ctx, snapshot); err != nil {
		return dto.BalanceSnapshotResponse{}, utils.WrapDomainError(utils.ErrConflict, "Failed to record balance", err)
	}

	snapshot.Account = account
	return toSnapshotResponse(*snapshot), nil
}

func (s *accountService) ListBalances(ctx context.Context, userID, accountID int64) ([]dto.BalanceSnapshotResponse, error) {
	if _, err := s.mustFind(ctx, userID, accountID); err != nil {
		return nil, err
	}

	snapshots, err := s.accounts.ListSnapshots(ctx, userID, accountID, snapshotHistoryLimit)
	if err != nil {
		return nil, utils.WrapDomainError(utils.ErrNotFound, "Failed to list balances", err)
	}

	responses := make([]dto.BalanceSnapshotResponse, 0, len(snapshots))
	for _, snapshot := range snapshots {
		responses = append(responses, toSnapshotResponse(snapshot))
	}
	return responses, nil
}

func (s *accountService) DeleteBalance(ctx context.Context, userID, accountID, id int64) error {
	if _, err := s.mustFind(ctx, userID, accountID); err != nil {
		return err
	}
	if err := s.accounts.DeleteSnapshot(ctx, userID, accountID, id); err != nil {
		return wrapIfInternal(err, "Failed to delete balance")
	}
	return nil
}

func (s *accountService) mustFind(ctx context.Context, userID, id int64) (*model.Account, error) {
	account, err := s.accounts.FindByID(ctx, userID, id)
	if err != nil {
		return nil, utils.WrapDomainError(utils.ErrNotFound, "Failed to load account", err)
	}
	if account == nil {
		return nil, utils.NotFound("Account")
	}
	return account, nil
}

func toAccountResponse(account model.Account) dto.AccountResponse {
	return dto.AccountResponse{
		ID:             account.ID,
		Name:           account.Name,
		AccountType:    string(account.AccountType),
		OpeningBalance: utils.Round(account.OpeningBalance),
		Description:    account.Description,
		IsActive:       account.IsActive,
		CreatedAt:      account.CreatedAt,
		UpdatedAt:      account.UpdatedAt,
	}
}

func toSnapshotResponse(snapshot model.AccountBalanceSnapshot) dto.BalanceSnapshotResponse {
	name := ""
	if snapshot.Account != nil {
		name = snapshot.Account.Name
	}
	return dto.BalanceSnapshotResponse{
		ID:            snapshot.ID,
		AccountID:     snapshot.AccountID,
		AccountName:   name,
		AsOfDate:      utils.FormatDate(snapshot.AsOfDate),
		ActualBalance: utils.Round(snapshot.ActualBalance),
		Note:          snapshot.Note,
		CreatedAt:     snapshot.CreatedAt,
		UpdatedAt:     snapshot.UpdatedAt,
	}
}
