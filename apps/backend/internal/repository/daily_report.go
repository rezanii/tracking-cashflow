package repository

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
)

// AccountFlow is one day's movement on one account. Transfers are kept apart from income and
// expense because moving an allowance into a wallet is neither.
type AccountFlow struct {
	AccountID    int64           `gorm:"column:account_id"`
	IncomeTotal  decimal.Decimal `gorm:"column:income_total"`
	ExpenseTotal decimal.Decimal `gorm:"column:expense_total"`
	TransferIn   decimal.Decimal `gorm:"column:transfer_in"`
	TransferOut  decimal.Decimal `gorm:"column:transfer_out"`
}

// TransferEdge is a single transfer with both ends resolved, which is what turns a row into
// a top-up allocation ("this much went to Dana Cadangan").
type TransferEdge struct {
	ID              int64           `gorm:"column:id"`
	FromAccountID   *int64          `gorm:"column:from_account_id"`
	FromAccountName string          `gorm:"column:from_account_name"`
	FromAccountType string          `gorm:"column:from_account_type"`
	ToAccountID     int64           `gorm:"column:to_account_id"`
	ToAccountName   string          `gorm:"column:to_account_name"`
	ToAccountType   string          `gorm:"column:to_account_type"`
	Amount          decimal.Decimal `gorm:"column:amount"`
	Description     string          `gorm:"column:description"`
}

// LabelledAmount is a grouped total ready to print as a bullet.
type LabelledAmount struct {
	Label  string          `gorm:"column:label"`
	Amount decimal.Decimal `gorm:"column:amount"`
}

type DailyReportRepository interface {
	// OpeningBalance is the cash flow position the day starts from: the accounts' own
	// opening balances are added by the service, this is the movement before the date.
	OpeningBalance(ctx context.Context, userID int64, date time.Time) (decimal.Decimal, error)
	CashFlowExpenses(ctx context.Context, userID int64, date time.Time) ([]LabelledAmount, error)
	AccountFlows(ctx context.Context, userID int64, date time.Time) (map[int64]AccountFlow, error)
	Transfers(ctx context.Context, userID int64, date time.Time) ([]TransferEdge, error)
	// AccountExpenses returns the day's top-level spends on one account with their detail
	// lines attached, so a withdrawal is reported once and still shows what it became.
	AccountExpenses(ctx context.Context, userID, accountID int64, date time.Time) ([]model.Transaction, error)
}

type dailyReportRepository struct {
	db *gorm.DB
}

func NewDailyReportRepository(db *gorm.DB) DailyReportRepository {
	return &dailyReportRepository{db: db}
}

// cashFlowScope matches transactions that belong to the household cash flow: either tied to
// a CASH_FLOW account or to no account at all, which is how rows created before accounts
// existed keep counting.
const cashFlowScope = `(t.account_id IS NULL OR a.account_type = '` + string(model.AccountTypeCashFlow) + `')`

func (r *dailyReportRepository) OpeningBalance(ctx context.Context, userID int64, date time.Time) (decimal.Decimal, error) {
	var row struct {
		Value decimal.Decimal `gorm:"column:value"`
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT COALESCE(SUM(
		           CASE t.transaction_type
		               WHEN 'INCOME'  THEN  t.amount
		               WHEN 'EXPENSE' THEN -t.amount
		               ELSE 0
		           END), 0) AS value
		FROM transactions AS t
		LEFT JOIN accounts AS a ON a.id = t.account_id
		WHERE t.user_id = ?
		  AND t.transaction_date < ?
		  AND t.parent_id IS NULL
		  AND `+cashFlowScope,
		userID, date.Format("2006-01-02"),
	).Scan(&row).Error
	if err != nil {
		return decimal.Zero, err
	}
	return row.Value, nil
}

func (r *dailyReportRepository) CashFlowExpenses(ctx context.Context, userID int64, date time.Time) ([]LabelledAmount, error) {
	var rows []LabelledAmount
	// Ordering by the first row of each category reproduces the order the day was written
	// down in, which is the order a person reads the list back in.
	err := r.db.WithContext(ctx).Raw(`
		SELECT COALESCE(c.name, N'Lain-lain') AS label, SUM(t.amount) AS amount
		FROM transactions AS t
		LEFT JOIN categories AS c ON c.id = t.category_id
		LEFT JOIN accounts   AS a ON a.id = t.account_id
		WHERE t.user_id = ?
		  AND t.transaction_date = ?
		  AND t.transaction_type = 'EXPENSE'
		  AND t.parent_id IS NULL
		  AND `+cashFlowScope+`
		GROUP BY COALESCE(c.name, N'Lain-lain')
		ORDER BY MIN(t.id)`,
		userID, date.Format("2006-01-02"),
	).Scan(&rows).Error
	return rows, err
}

func (r *dailyReportRepository) AccountFlows(ctx context.Context, userID int64, date time.Time) (map[int64]AccountFlow, error) {
	var rows []AccountFlow
	day := date.Format("2006-01-02")
	// The two halves of the UNION are the two ways a transaction touches an account: as its
	// own account, and as the far end of a transfer. Summing them together gives one row per
	// account without a query per account.
	err := r.db.WithContext(ctx).Raw(`
		SELECT account_id,
		       SUM(income_total)  AS income_total,
		       SUM(expense_total) AS expense_total,
		       SUM(transfer_in)   AS transfer_in,
		       SUM(transfer_out)  AS transfer_out
		FROM (
		    SELECT t.account_id AS account_id,
		           CASE WHEN t.transaction_type = 'INCOME'  THEN t.amount ELSE 0 END AS income_total,
		           CASE WHEN t.transaction_type = 'EXPENSE' AND t.parent_id IS NULL THEN t.amount ELSE 0 END AS expense_total,
		           CAST(0 AS DECIMAL(18, 2)) AS transfer_in,
		           CASE WHEN t.transaction_type = 'TRANSFER' THEN t.amount ELSE 0 END AS transfer_out
		    FROM transactions AS t
		    WHERE t.user_id = ? AND t.transaction_date = ? AND t.account_id IS NOT NULL
		    UNION ALL
		    SELECT t.to_account_id,
		           CAST(0 AS DECIMAL(18, 2)),
		           CAST(0 AS DECIMAL(18, 2)),
		           CASE WHEN t.transaction_type = 'TRANSFER' THEN t.amount ELSE 0 END,
		           CAST(0 AS DECIMAL(18, 2))
		    FROM transactions AS t
		    WHERE t.user_id = ? AND t.transaction_date = ? AND t.to_account_id IS NOT NULL
		) AS flows
		GROUP BY account_id`,
		userID, day, userID, day,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	byAccount := make(map[int64]AccountFlow, len(rows))
	for _, row := range rows {
		byAccount[row.AccountID] = row
	}
	return byAccount, nil
}

func (r *dailyReportRepository) Transfers(ctx context.Context, userID int64, date time.Time) ([]TransferEdge, error) {
	var rows []TransferEdge
	err := r.db.WithContext(ctx).Raw(`
		SELECT t.id,
		       t.account_id                        AS from_account_id,
		       COALESCE(src.name, N'')             AS from_account_name,
		       COALESCE(src.account_type, '')      AS from_account_type,
		       t.to_account_id                     AS to_account_id,
		       dest.name                           AS to_account_name,
		       dest.account_type                   AS to_account_type,
		       t.amount                            AS amount,
		       COALESCE(t.description, N'')        AS description
		FROM transactions AS t
		INNER JOIN accounts AS dest ON dest.id = t.to_account_id
		LEFT  JOIN accounts AS src  ON src.id  = t.account_id
		WHERE t.user_id = ?
		  AND t.transaction_date = ?
		  AND t.transaction_type = 'TRANSFER'
		ORDER BY t.id`,
		userID, date.Format("2006-01-02"),
	).Scan(&rows).Error
	return rows, err
}

func (r *dailyReportRepository) AccountExpenses(ctx context.Context, userID, accountID int64, date time.Time) ([]model.Transaction, error) {
	var transactions []model.Transaction
	err := r.db.WithContext(ctx).
		Preload("Category").
		Preload("Children", func(db *gorm.DB) *gorm.DB {
			return db.Order("transactions.id ASC")
		}).
		Where("user_id = ? AND account_id = ? AND transaction_date = ? AND transaction_type = ? AND parent_id IS NULL",
			userID, accountID, date.Format("2006-01-02"), model.TransactionTypeExpense).
		Order("id ASC").
		Find(&transactions).Error
	if err != nil {
		return nil, err
	}
	return transactions, nil
}
