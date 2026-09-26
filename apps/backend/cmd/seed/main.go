// Command seed loads development data: one user, a starter category set and a month of
// sample transactions so the dashboard and the reports have something to show.
//
// It is idempotent and refuses to run when APP_ENV is production.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/config"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/repository"
)

const (
	defaultSeedEmail = "admin@example.com"
	seedPassword     = "Admin123!"
	seedName         = "Admin Example"
)

// seedEmail is overridable so the demo data can be attached to an account that already
// exists — a real one on a hosted database, where creating a second account with a known
// password would be worse than pointless.
func seedEmail() string {
	if email := strings.TrimSpace(os.Getenv("SEED_EMAIL")); email != "" {
		return email
	}
	return defaultSeedEmail
}

type seedCategory struct {
	Name        string
	Type        model.CategoryType
	Description string
}

type seedTransaction struct {
	DayOfMonth      int
	Type            model.TransactionType
	CategoryName    string
	Amount          string
	Description     string
	ReferenceNumber string
}

func main() {
	if err := run(); err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	if cfg.IsProduction() {
		return errors.New("seeding is refused when APP_ENV is production")
	}

	db, err := repository.NewDatabase(cfg)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer closePool(db)

	ctx := context.Background()

	// One transaction for the whole seed: a partial load would leave categories without
	// the transactions that reference them.
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err := ensureUser(tx)
		if err != nil {
			return err
		}

		categories, err := ensureCategories(tx, user.ID)
		if err != nil {
			return err
		}

		if err := ensureTransactions(tx, user.ID, categories); err != nil {
			return err
		}

		accounts, err := ensureAccounts(tx, user.ID)
		if err != nil {
			return err
		}

		// The demo day sits on today's date so a bare /report in Telegram returns the full
		// report without an argument.
		now := time.Now().UTC()
		demoDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		if err := ensureDemoTransactions(tx, user.ID, categories, accounts, demoDate); err != nil {
			return err
		}
		return ensureBalanceSnapshots(tx, user.ID, accounts, demoDate)
	})
	if err != nil {
		return err
	}

	if seedEmail() == defaultSeedEmail {
		slog.Info("seed complete", "email", defaultSeedEmail, "password", seedPassword)
		slog.Warn("the seeded credentials are for development only")
	} else {
		slog.Info("seed complete", "email", seedEmail(), "note", "existing account, password untouched")
	}
	return nil
}

func ensureUser(tx *gorm.DB) (*model.User, error) {
	email := seedEmail()

	var existing model.User
	err := tx.Where("email = ?", email).Take(&existing).Error
	if err == nil {
		slog.Info("user already present", "email", email)
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("look up seed user: %w", err)
	}

	// An account named explicitly is expected to exist already. Creating it with the known
	// development password instead would hand out a credential nobody asked for.
	if email != defaultSeedEmail {
		return nil, fmt.Errorf("SEED_EMAIL %q does not exist; create the account first", email)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(seedPassword), 12)
	if err != nil {
		return nil, fmt.Errorf("hash seed password: %w", err)
	}

	now := time.Now().UTC()
	user := &model.User{
		Name:         seedName,
		Email:        email,
		PasswordHash: string(hash),
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := tx.Create(user).Error; err != nil {
		return nil, fmt.Errorf("create seed user: %w", err)
	}
	slog.Info("user created", "email", email)
	return user, nil
}

func ensureCategories(tx *gorm.DB, userID int64) (map[string]int64, error) {
	definitions := []seedCategory{
		{Name: "Salary", Type: model.CategoryTypeIncome, Description: "Monthly salary"},
		{Name: "Bonus", Type: model.CategoryTypeIncome, Description: "Performance bonus"},
		{Name: "Investment", Type: model.CategoryTypeIncome, Description: "Dividends and interest"},
		{Name: "Food", Type: model.CategoryTypeExpense, Description: "Groceries and eating out"},
		{Name: "Transport", Type: model.CategoryTypeExpense, Description: "Fuel, parking and fares"},
		{Name: "Cicilan Rumah", Type: model.CategoryTypeExpense, Description: "Mortgage instalment"},
		{Name: "Household", Type: model.CategoryTypeExpense, Description: "Utilities and upkeep"},
		{Name: "Shopping", Type: model.CategoryTypeExpense, Description: "Clothing and goods"},
		{Name: "Entertainment", Type: model.CategoryTypeExpense, Description: "Leisure"},
		{Name: "Other", Type: model.CategoryTypeExpense, Description: "Uncategorised spending"},
	}
	definitions = append(definitions, demoCategories()...)

	ids := make(map[string]int64, len(definitions))
	now := time.Now().UTC()

	for _, definition := range definitions {
		var existing model.Category
		err := tx.Where("user_id = ? AND type = ? AND name = ?", userID, definition.Type, definition.Name).
			Take(&existing).Error
		if err == nil {
			ids[definition.Name] = existing.ID
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("look up category %s: %w", definition.Name, err)
		}

		category := &model.Category{
			UserID:      userID,
			Name:        definition.Name,
			Type:        definition.Type,
			Description: definition.Description,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := tx.Create(category).Error; err != nil {
			return nil, fmt.Errorf("create category %s: %w", definition.Name, err)
		}
		ids[definition.Name] = category.ID
	}

	slog.Info("categories ready", "count", len(ids))
	return ids, nil
}

func ensureTransactions(tx *gorm.DB, userID int64, categories map[string]int64) error {
	definitions := []seedTransaction{
		{DayOfMonth: 1, Type: model.TransactionTypeExpense, CategoryName: "Cicilan Rumah", Amount: "1150000", Description: "Cicilan Rumah instalment", ReferenceNumber: "Cicilan Rumah-001"},
		{DayOfMonth: 2, Type: model.TransactionTypeExpense, CategoryName: "Household", Amount: "485000", Description: "Electricity and water", ReferenceNumber: "UTL-001"},
		{DayOfMonth: 3, Type: model.TransactionTypeExpense, CategoryName: "Food", Amount: "100000", Description: "Weekly groceries", ReferenceNumber: "FOOD-001"},
		{DayOfMonth: 5, Type: model.TransactionTypeIncome, CategoryName: "Salary", Amount: "15000000", Description: "Monthly salary", ReferenceNumber: "SAL-001"},
		{DayOfMonth: 6, Type: model.TransactionTypeExpense, CategoryName: "Transport", Amount: "175000", Description: "Fuel", ReferenceNumber: "TRP-001"},
		{DayOfMonth: 8, Type: model.TransactionTypeExpense, CategoryName: "Food", Amount: "210000", Description: "Dinner with family", ReferenceNumber: "FOOD-002"},
		{DayOfMonth: 10, Type: model.TransactionTypeTransfer, Amount: "2000000", Description: "Move to savings account", ReferenceNumber: "TRF-001"},
		{DayOfMonth: 12, Type: model.TransactionTypeExpense, CategoryName: "Shopping", Amount: "640000", Description: "Clothing", ReferenceNumber: "SHP-001"},
		{DayOfMonth: 15, Type: model.TransactionTypeIncome, CategoryName: "Bonus", Amount: "2500000", Description: "Quarterly bonus", ReferenceNumber: "BON-001"},
		{DayOfMonth: 17, Type: model.TransactionTypeExpense, CategoryName: "Entertainment", Amount: "150000", Description: "Cinema", ReferenceNumber: "ENT-001"},
		{DayOfMonth: 19, Type: model.TransactionTypeExpense, CategoryName: "Food", Amount: "280000", Description: "Weekly groceries", ReferenceNumber: "FOOD-003"},
		{DayOfMonth: 21, Type: model.TransactionTypeExpense, CategoryName: "Transport", Amount: "95000", Description: "Ride hailing", ReferenceNumber: "TRP-002"},
		{DayOfMonth: 23, Type: model.TransactionTypeIncome, CategoryName: "Investment", Amount: "450000", Description: "Mutual fund dividend", ReferenceNumber: "INV-001"},
		{DayOfMonth: 25, Type: model.TransactionTypeExpense, CategoryName: "Household", Amount: "375000", Description: "Internet and phone", ReferenceNumber: "UTL-002"},
		{DayOfMonth: 27, Type: model.TransactionTypeExpense, CategoryName: "Other", Amount: "125000", Description: "Miscellaneous", ReferenceNumber: "OTH-001"},
	}

	now := time.Now().UTC()
	base := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	created := 0

	for _, definition := range definitions {
		date := base.AddDate(0, 0, definition.DayOfMonth-1)

		var count int64
		if err := tx.Model(&model.Transaction{}).
			Where("user_id = ? AND reference_number = ?", userID, definition.ReferenceNumber).
			Count(&count).Error; err != nil {
			return fmt.Errorf("look up transaction %s: %w", definition.ReferenceNumber, err)
		}
		if count > 0 {
			continue
		}

		amount, err := decimal.NewFromString(definition.Amount)
		if err != nil {
			return fmt.Errorf("parse amount for %s: %w", definition.ReferenceNumber, err)
		}

		var categoryID *int64
		if definition.Type.RequiresCategory() {
			id, ok := categories[definition.CategoryName]
			if !ok {
				return fmt.Errorf("seed category %s is missing", definition.CategoryName)
			}
			categoryID = &id
		}

		transaction := &model.Transaction{
			UserID:          userID,
			TransactionDate: date,
			TransactionType: definition.Type,
			CategoryID:      categoryID,
			Amount:          amount,
			Description:     definition.Description,
			ReferenceNumber: definition.ReferenceNumber,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := tx.Omit("Category").Create(transaction).Error; err != nil {
			return fmt.Errorf("create transaction %s: %w", definition.ReferenceNumber, err)
		}
		created++
	}

	slog.Info("transactions ready", "created", created, "total", len(definitions))
	return nil
}

func closePool(db *gorm.DB) {
	pool, err := db.DB()
	if err != nil {
		return
	}
	_ = pool.Close()
}
