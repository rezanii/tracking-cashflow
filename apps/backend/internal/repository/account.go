package repository

import (
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

type AccountRepository interface {
	WithTx(tx *gorm.DB) AccountRepository
	Create(ctx context.Context, account *model.Account) error
	Update(ctx context.Context, account *model.Account) error
	Delete(ctx context.Context, userID, id int64) error
	FindByID(ctx context.Context, userID, id int64) (*model.Account, error)
	FindByName(ctx context.Context, userID int64, name string) (*model.Account, error)
	List(ctx context.Context, userID int64, query dto.AccountListQuery) ([]model.Account, int64, error)
	ListByType(ctx context.Context, userID int64, types ...model.AccountType) ([]model.Account, error)
	CountTransactions(ctx context.Context, userID, id int64) (int64, error)
	SetActive(ctx context.Context, userID, id int64, isActive bool) error
	SumOpeningBalance(ctx context.Context, userID int64, types ...model.AccountType) (decimal.Decimal, error)

	UpsertSnapshot(ctx context.Context, snapshot *model.AccountBalanceSnapshot) error
	ListSnapshots(ctx context.Context, userID, accountID int64, limit int) ([]model.AccountBalanceSnapshot, error)
	// LatestSnapshots returns, per account, the most recent observed balance on or before
	// the given date, keyed by account id. A day with no count simply has no entry.
	LatestSnapshots(ctx context.Context, userID int64, asOf time.Time) (map[int64]model.AccountBalanceSnapshot, error)
	DeleteSnapshot(ctx context.Context, userID, accountID, id int64) error
}

type accountRepository struct {
	db *gorm.DB
}

func NewAccountRepository(db *gorm.DB) AccountRepository {
	return &accountRepository{db: db}
}

func (r *accountRepository) WithTx(tx *gorm.DB) AccountRepository {
	return &accountRepository{db: tx}
}

func (r *accountRepository) Create(ctx context.Context, account *model.Account) error {
	return r.db.WithContext(ctx).Create(account).Error
}

func (r *accountRepository) Update(ctx context.Context, account *model.Account) error {
	result := r.db.WithContext(ctx).
		Model(&model.Account{}).
		Where("id = ? AND user_id = ?", account.ID, account.UserID).
		Updates(map[string]any{
			"name":            account.Name,
			"account_type":    account.AccountType,
			"opening_balance": account.OpeningBalance,
			"description":     account.Description,
			"updated_at":      account.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return utils.NotFound("Account")
	}
	return nil
}

func (r *accountRepository) Delete(ctx context.Context, userID, id int64) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Account{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return utils.NotFound("Account")
	}
	return nil
}

func (r *accountRepository) FindByID(ctx context.Context, userID, id int64) (*model.Account, error) {
	var account model.Account
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Take(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *accountRepository) FindByName(ctx context.Context, userID int64, name string) (*model.Account, error) {
	var account model.Account
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND name = ?", userID, name).
		Take(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &account, nil
}

var accountSortColumns = map[string]string{
	"name":            "name",
	"account_type":    "account_type",
	"opening_balance": "opening_balance",
	"created_at":      "created_at",
}

func (r *accountRepository) List(ctx context.Context, userID int64, query dto.AccountListQuery) ([]model.Account, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.Account{}).Where("user_id = ?", userID)

	if query.AccountType != "" {
		base = base.Where("account_type = ?", query.AccountType)
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

	column, direction := utils.NormalizeSort(query.SortBy, query.SortDir, accountSortColumns, "name")

	var accounts []model.Account
	err := base.
		Order(column + " " + direction).
		Order("id DESC").
		Limit(query.PageSize).
		Offset(utils.Offset(query.Page, query.PageSize)).
		Find(&accounts).Error
	if err != nil {
		return nil, 0, err
	}
	return accounts, total, nil
}

// ListByType returns active accounts of the given types, ordered by name so the report
// sections come out in a stable order from one day to the next.
func (r *accountRepository) ListByType(ctx context.Context, userID int64, types ...model.AccountType) ([]model.Account, error) {
	if len(types) == 0 {
		return nil, nil
	}
	var accounts []model.Account
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND is_active = 1 AND account_type IN ?", userID, types).
		Order("account_type ASC").
		Order("name ASC").
		Find(&accounts).Error
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

// CountTransactions counts every reference to the account, including the far side of a
// transfer, so a used account cannot be deleted from under a report.
func (r *accountRepository) CountTransactions(ctx context.Context, userID, id int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Transaction{}).
		Where("user_id = ? AND (account_id = ? OR to_account_id = ?)", userID, id, id).
		Count(&count).Error
	return count, err
}

func (r *accountRepository) SetActive(ctx context.Context, userID, id int64, isActive bool) error {
	result := r.db.WithContext(ctx).
		Model(&model.Account{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{"is_active": isActive, "updated_at": gorm.Expr("SYSUTCDATETIME()")})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return utils.NotFound("Account")
	}
	return nil
}

func (r *accountRepository) SumOpeningBalance(ctx context.Context, userID int64, types ...model.AccountType) (decimal.Decimal, error) {
	var row struct {
		Value decimal.Decimal `gorm:"column:value"`
	}
	query := r.db.WithContext(ctx).
		Model(&model.Account{}).
		Select("COALESCE(SUM(opening_balance), 0) AS value").
		Where("user_id = ? AND is_active = 1", userID)
	if len(types) > 0 {
		query = query.Where("account_type IN ?", types)
	}
	if err := query.Scan(&row).Error; err != nil {
		return decimal.Zero, err
	}
	return row.Value, nil
}

// UpsertSnapshot keeps one observed balance per account per day: re-counting a wallet
// corrects the figure instead of adding a second, contradictory row.
//
// This is an explicit update-then-insert rather than clause.OnConflict, because GORM's
// SQL Server driver ignores that clause and emits a plain INSERT, which the unique index
// then rejects.
func (r *accountRepository) UpsertSnapshot(ctx context.Context, snapshot *model.AccountBalanceSnapshot) error {
	day := snapshot.AsOfDate.Format("2006-01-02")

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.AccountBalanceSnapshot{}).
			Where("user_id = ? AND account_id = ? AND as_of_date = ?", snapshot.UserID, snapshot.AccountID, day).
			Updates(map[string]any{
				"actual_balance": snapshot.ActualBalance,
				"note":           snapshot.Note,
				"updated_at":     snapshot.UpdatedAt,
			})
		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected > 0 {
			// Read the row back so the caller gets the stored id rather than a zero value.
			return tx.Where("user_id = ? AND account_id = ? AND as_of_date = ?", snapshot.UserID, snapshot.AccountID, day).
				Take(snapshot).Error
		}
		return tx.Omit("Account").Create(snapshot).Error
	})
}

func (r *accountRepository) ListSnapshots(ctx context.Context, userID, accountID int64, limit int) ([]model.AccountBalanceSnapshot, error) {
	var snapshots []model.AccountBalanceSnapshot
	err := r.db.WithContext(ctx).
		Preload("Account").
		Where("user_id = ? AND account_id = ?", userID, accountID).
		Order("as_of_date DESC").
		Limit(limit).
		Find(&snapshots).Error
	if err != nil {
		return nil, err
	}
	return snapshots, nil
}

func (r *accountRepository) LatestSnapshots(ctx context.Context, userID int64, asOf time.Time) (map[int64]model.AccountBalanceSnapshot, error) {
	var snapshots []model.AccountBalanceSnapshot
	// ROW_NUMBER picks the newest row per account in one pass, rather than a query per
	// account in the report builder.
	err := r.db.WithContext(ctx).
		Raw(`
			SELECT id, user_id, account_id, as_of_date, actual_balance, note, created_at, updated_at
			FROM (
			    SELECT s.*,
			           ROW_NUMBER() OVER (PARTITION BY s.account_id ORDER BY s.as_of_date DESC, s.id DESC) AS rn
			    FROM account_balance_snapshots AS s
			    WHERE s.user_id = ? AND s.as_of_date <= ?
			) AS ranked
			WHERE ranked.rn = 1`,
			userID, asOf.Format("2006-01-02"),
		).Scan(&snapshots).Error
	if err != nil {
		return nil, err
	}

	byAccount := make(map[int64]model.AccountBalanceSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		byAccount[snapshot.AccountID] = snapshot
	}
	return byAccount, nil
}

func (r *accountRepository) DeleteSnapshot(ctx context.Context, userID, accountID, id int64) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND account_id = ?", id, userID, accountID).
		Delete(&model.AccountBalanceSnapshot{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return utils.NotFound("Balance snapshot")
	}
	return nil
}
