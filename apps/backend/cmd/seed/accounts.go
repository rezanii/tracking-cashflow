package main

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
)

// The demo day is seeded on today's date so a bare /report in Telegram shows the full report
// with no argument. Every other seeded transaction sits on an earlier day of the month, so
// the day's own figures are exactly the ones defined here.
//
// The figures are invented, and deliberately so: this data is committed to a public
// repository, and a sample must never be somebody's real finances. They are chosen to make
// every section of the daily report non-trivial — a wallet that comes up short, a bank
// carrying a balance from before, and a reconciliation that still lands on zero.
const demoReferencePrefix = "DEMO-"

type seedAccount struct {
	Name        string
	Type        model.AccountType
	Description string
	// ActualBalance is the balance counted in the real world on the demo day. An empty
	// string means nobody counted it, and the report then stops at the expected figure
	// instead of reporting a variance.
	ActualBalance string
}

// seedDemoTransaction describes one row of the demo day. Children are detail lines of the
// row above them: their amounts explain the parent and are not added to it.
type seedDemoTransaction struct {
	Reference   string
	Type        model.TransactionType
	Category    string
	Account     string
	ToAccount   string
	Amount      string
	Description string
	Children    []seedDemoTransaction
}

func demoAccounts() []seedAccount {
	return []seedAccount{
		{Name: "Cash Flow", Type: model.AccountTypeCashFlow, Description: "Household budget"},
		{Name: "Kartu Kredit", Type: model.AccountTypeCreditCard, Description: "Credit card"},
		{Name: "Bank Utama", Type: model.AccountTypeBank, Description: "Bank account", ActualBalance: "20000"},
		{Name: "Dompet Harian", Type: model.AccountTypeWallet, Description: "Daily allowance wallet", ActualBalance: "72500"},
		{Name: "Dana Cadangan", Type: model.AccountTypeSavings, Description: "Emergency fund"},
	}
}

// demoCategories are the categories the demo day needs on top of the generic ones.
func demoCategories() []seedCategory {
	return []seedCategory{
		{Name: "Cicilan Rumah", Type: model.CategoryTypeExpense, Description: "Mortgage instalment"},
		{Name: "Cicilan Motor", Type: model.CategoryTypeExpense, Description: "Vehicle instalment"},
		{Name: "Listrik & Air", Type: model.CategoryTypeExpense, Description: "Utilities"},
		{Name: "Internet", Type: model.CategoryTypeExpense, Description: "Internet and phone"},
		{Name: "Belanja Bulanan", Type: model.CategoryTypeExpense, Description: "Monthly groceries"},
		{Name: "Kebutuhan Harian", Type: model.CategoryTypeExpense, Description: "Daily wallet spending"},
		{Name: "Pembayaran CC", Type: model.CategoryTypeExpense, Description: "Credit card bill"},
		{Name: "Biaya Admin", Type: model.CategoryTypeExpense, Description: "Bank charges"},
		{Name: "Dana Masuk", Type: model.CategoryTypeIncome, Description: "Incoming funds"},
	}
}

func demoTransactions() []seedDemoTransaction {
	return []seedDemoTransaction{
		// Household cash flow for the day: 5.000.000 in total.
		{Reference: "DEMO-CF-01", Type: model.TransactionTypeExpense, Category: "Cicilan Rumah", Account: "Cash Flow", Amount: "1200000", Description: "Cicilan Rumah"},
		{Reference: "DEMO-CF-02", Type: model.TransactionTypeExpense, Category: "Cicilan Motor", Account: "Cash Flow", Amount: "800000", Description: "Cicilan Motor"},
		{Reference: "DEMO-CF-03", Type: model.TransactionTypeExpense, Category: "Listrik & Air", Account: "Cash Flow", Amount: "450000", Description: "Listrik & Air"},
		{Reference: "DEMO-CF-04", Type: model.TransactionTypeExpense, Category: "Internet", Account: "Cash Flow", Amount: "350000", Description: "Internet"},
		{Reference: "DEMO-CF-05", Type: model.TransactionTypeExpense, Category: "Belanja Bulanan", Account: "Cash Flow", Amount: "2200000", Description: "Belanja Bulanan"},

		// The card bill, and the part of it taken back into the bank. The money taken back is
		// a transfer, not a second expense, which is what makes the net figure meaningful:
		// 1.750.000 paid - 900.000 taken back = 850.000.
		{Reference: "DEMO-CC-01", Type: model.TransactionTypeExpense, Category: "Pembayaran CC", Account: "Kartu Kredit", Amount: "1750000", Description: "Bayar tagihan"},
		{Reference: "DEMO-CC-02", Type: model.TransactionTypeTransfer, Account: "Kartu Kredit", ToAccount: "Bank Utama", Amount: "900000", Description: "Ambil kembali / Top-up"},

		// The bank leg: other money arriving, the transfer fee, and the two allocations out.
		// 915.000 in - 2.500 fee - 900.000 out = 12.500 of movement today.
		{Reference: "DEMO-BK-01", Type: model.TransactionTypeIncome, Category: "Dana Masuk", Account: "Bank Utama", Amount: "15000", Description: "Bunga tabungan"},
		{Reference: "DEMO-BK-02", Type: model.TransactionTypeExpense, Category: "Biaya Admin", Account: "Bank Utama", Amount: "2500", Description: "Biaya transfer"},
		{Reference: "DEMO-TU-01", Type: model.TransactionTypeTransfer, Account: "Bank Utama", ToAccount: "Dana Cadangan", Amount: "300000", Description: "Alokasi Dana Cadangan"},
		{Reference: "DEMO-TU-02", Type: model.TransactionTypeTransfer, Account: "Bank Utama", ToAccount: "Dompet Harian", Amount: "600000", Description: "Jatah Dompet Harian"},

		// What the allowance was spent on. The withdrawal is recorded once and its parts are
		// listed underneath, so the recorded total stays 500.000 rather than 700.000.
		{Reference: "DEMO-DH-01", Type: model.TransactionTypeExpense, Category: "Kebutuhan Harian", Account: "Dompet Harian", Amount: "150000", Description: "Bensin"},
		{
			Reference: "DEMO-DH-02", Type: model.TransactionTypeExpense, Category: "Kebutuhan Harian", Account: "Dompet Harian", Amount: "200000", Description: "Tarik Tunai",
			Children: []seedDemoTransaction{
				{Reference: "DEMO-DH-02-A", Type: model.TransactionTypeExpense, Category: "Kebutuhan Harian", Account: "Dompet Harian", Amount: "45000", Description: "Makan siang"},
				{Reference: "DEMO-DH-02-B", Type: model.TransactionTypeExpense, Category: "Kebutuhan Harian", Account: "Dompet Harian", Amount: "25000", Description: "Kopi"},
				{Reference: "DEMO-DH-02-C", Type: model.TransactionTypeExpense, Category: "Kebutuhan Harian", Account: "Dompet Harian", Amount: "10000", Description: "Parkir"},
				{Reference: "DEMO-DH-02-D", Type: model.TransactionTypeExpense, Category: "Kebutuhan Harian", Account: "Dompet Harian", Amount: "20000", Description: "Jajan anak"},
				{Reference: "DEMO-DH-02-E", Type: model.TransactionTypeExpense, Category: "Kebutuhan Harian", Account: "Dompet Harian", Amount: "100000", Description: "Sisa tunai"},
			},
		},
		{Reference: "DEMO-DH-03", Type: model.TransactionTypeExpense, Category: "Kebutuhan Harian", Account: "Dompet Harian", Amount: "100000", Description: "Pulsa"},
		{Reference: "DEMO-DH-04", Type: model.TransactionTypeExpense, Category: "Kebutuhan Harian", Account: "Dompet Harian", Amount: "50000", Description: "Obat"},
	}
}

func ensureAccounts(tx *gorm.DB, userID int64) (map[string]int64, error) {
	ids := map[string]int64{}
	now := time.Now().UTC()
	created := 0

	for _, definition := range demoAccounts() {
		var existing model.Account
		err := tx.Where("user_id = ? AND name = ?", userID, definition.Name).Take(&existing).Error
		if err == nil {
			ids[definition.Name] = existing.ID
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("look up account %s: %w", definition.Name, err)
		}

		account := &model.Account{
			UserID:         userID,
			Name:           definition.Name,
			AccountType:    definition.Type,
			OpeningBalance: decimal.Zero,
			Description:    definition.Description,
			IsActive:       true,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := tx.Create(account).Error; err != nil {
			return nil, fmt.Errorf("create account %s: %w", definition.Name, err)
		}
		ids[definition.Name] = account.ID
		created++
	}

	slog.Info("accounts ready", "created", created, "total", len(ids))
	return ids, nil
}

// ensureBalanceSnapshots records what the wallet and the bank actually held on the demo day.
// Without these the report cannot show a variance, because there is nothing to compare the
// recorded transactions against.
func ensureBalanceSnapshots(tx *gorm.DB, userID int64, accounts map[string]int64, date time.Time) error {
	created := 0
	now := time.Now().UTC()

	for _, definition := range demoAccounts() {
		if definition.ActualBalance == "" {
			continue
		}
		accountID, ok := accounts[definition.Name]
		if !ok {
			return fmt.Errorf("seed account %s is missing", definition.Name)
		}

		var count int64
		if err := tx.Model(&model.AccountBalanceSnapshot{}).
			Where("account_id = ? AND as_of_date = ?", accountID, date.Format("2006-01-02")).
			Count(&count).Error; err != nil {
			return fmt.Errorf("look up balance for %s: %w", definition.Name, err)
		}
		if count > 0 {
			continue
		}

		balance, err := decimal.NewFromString(definition.ActualBalance)
		if err != nil {
			return fmt.Errorf("parse balance for %s: %w", definition.Name, err)
		}

		snapshot := &model.AccountBalanceSnapshot{
			UserID:        userID,
			AccountID:     accountID,
			AsOfDate:      date,
			ActualBalance: balance,
			Note:          "Seeded demo balance",
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := tx.Omit("Account").Create(snapshot).Error; err != nil {
			return fmt.Errorf("create balance for %s: %w", definition.Name, err)
		}
		created++
	}

	slog.Info("balance snapshots ready", "created", created)
	return nil
}

func ensureDemoTransactions(
	tx *gorm.DB,
	userID int64,
	categories map[string]int64,
	accounts map[string]int64,
	date time.Time,
) error {
	created := 0
	for _, definition := range demoTransactions() {
		parentID, inserted, err := insertDemoTransaction(tx, userID, categories, accounts, date, definition, nil)
		if err != nil {
			return err
		}
		if inserted {
			created++
		}

		for _, child := range definition.Children {
			// The parent id is passed down, which is what turns the child into a detail line
			// rather than a second spend of the same money.
			_, childInserted, err := insertDemoTransaction(tx, userID, categories, accounts, date, child, &parentID)
			if err != nil {
				return err
			}
			if childInserted {
				created++
			}
		}
	}

	slog.Info("demo transactions ready", "created", created, "date", date.Format("2006-01-02"))
	return nil
}

func insertDemoTransaction(
	tx *gorm.DB,
	userID int64,
	categories map[string]int64,
	accounts map[string]int64,
	date time.Time,
	definition seedDemoTransaction,
	parentID *int64,
) (int64, bool, error) {
	var existing model.Transaction
	err := tx.Where("user_id = ? AND reference_number = ?", userID, definition.Reference).Take(&existing).Error
	if err == nil {
		return existing.ID, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, false, fmt.Errorf("look up demo transaction %s: %w", definition.Reference, err)
	}

	amount, err := decimal.NewFromString(definition.Amount)
	if err != nil {
		return 0, false, fmt.Errorf("parse amount for %s: %w", definition.Reference, err)
	}

	var categoryID *int64
	if definition.Type.RequiresCategory() {
		id, ok := categories[definition.Category]
		if !ok {
			return 0, false, fmt.Errorf("seed category %s is missing", definition.Category)
		}
		categoryID = &id
	}

	accountID, ok := accounts[definition.Account]
	if !ok {
		return 0, false, fmt.Errorf("seed account %s is missing", definition.Account)
	}

	var toAccountID *int64
	if definition.ToAccount != "" {
		id, ok := accounts[definition.ToAccount]
		if !ok {
			return 0, false, fmt.Errorf("seed account %s is missing", definition.ToAccount)
		}
		toAccountID = &id
	}

	now := time.Now().UTC()
	transaction := &model.Transaction{
		UserID:          userID,
		TransactionDate: date,
		TransactionType: definition.Type,
		CategoryID:      categoryID,
		Amount:          amount,
		Description:     definition.Description,
		ReferenceNumber: definition.Reference,
		AccountID:       &accountID,
		ToAccountID:     toAccountID,
		ParentID:        parentID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := tx.Omit("Category", "Account", "ToAccount", "Children").Create(transaction).Error; err != nil {
		return 0, false, fmt.Errorf("create demo transaction %s: %w", definition.Reference, err)
	}
	return transaction.ID, true, nil
}
