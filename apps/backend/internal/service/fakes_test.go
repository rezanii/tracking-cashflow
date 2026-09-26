package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/repository"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

// errBoom stands in for an unexpected driver failure.
var errBoom = errors.New("database unavailable")

// fakeTxManager runs the unit of work inline. The fake repositories ignore the *gorm.DB they
// are handed, so the services can be exercised without a database.
type fakeTxManager struct {
	calls int
}

func (m *fakeTxManager) Run(ctx context.Context, fn func(tx *gorm.DB) error) error {
	m.calls++
	return fn(nil)
}

type fakeUserRepository struct {
	users     map[string]*model.User
	byID      map[int64]*model.User
	nextID    int64
	failOn    string
	createErr error
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{
		users:  map[string]*model.User{},
		byID:   map[int64]*model.User{},
		nextID: 1,
	}
}

func (r *fakeUserRepository) WithTx(*gorm.DB) repository.UserRepository { return r }

func (r *fakeUserRepository) Create(_ context.Context, user *model.User) error {
	if r.createErr != nil {
		return r.createErr
	}
	user.ID = r.nextID
	r.nextID++
	copied := *user
	r.users[user.Email] = &copied
	r.byID[user.ID] = &copied
	return nil
}

func (r *fakeUserRepository) FindByEmail(_ context.Context, email string) (*model.User, error) {
	if r.failOn == "FindByEmail" {
		return nil, errBoom
	}
	user, ok := r.users[email]
	if !ok {
		return nil, nil
	}
	copied := *user
	return &copied, nil
}

func (r *fakeUserRepository) FindByID(_ context.Context, id int64) (*model.User, error) {
	if r.failOn == "FindByID" {
		return nil, errBoom
	}
	user, ok := r.byID[id]
	if !ok {
		return nil, nil
	}
	copied := *user
	return &copied, nil
}

func (r *fakeUserRepository) ExistsByEmail(_ context.Context, email string) (bool, error) {
	if r.failOn == "ExistsByEmail" {
		return false, errBoom
	}
	_, ok := r.users[email]
	return ok, nil
}

type fakeCategoryRepository struct {
	categories map[int64]*model.Category
	nextID     int64
	usage      map[int64]int64
}

func newFakeCategoryRepository() *fakeCategoryRepository {
	return &fakeCategoryRepository{
		categories: map[int64]*model.Category{},
		nextID:     1,
		usage:      map[int64]int64{},
	}
}

// seed inserts a category directly, bypassing the service, so a test can start from a
// known state.
func (r *fakeCategoryRepository) seed(userID int64, name string, categoryType model.CategoryType, active bool) *model.Category {
	category := &model.Category{
		ID:        r.nextID,
		UserID:    userID,
		Name:      name,
		Type:      categoryType,
		IsActive:  active,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	r.categories[category.ID] = category
	r.nextID++
	return category
}

func (r *fakeCategoryRepository) WithTx(*gorm.DB) repository.CategoryRepository { return r }

func (r *fakeCategoryRepository) Create(_ context.Context, category *model.Category) error {
	category.ID = r.nextID
	r.nextID++
	copied := *category
	r.categories[category.ID] = &copied
	return nil
}

func (r *fakeCategoryRepository) Update(_ context.Context, category *model.Category) error {
	existing, ok := r.categories[category.ID]
	if !ok || existing.UserID != category.UserID {
		return utils.NotFound("Category")
	}
	copied := *category
	r.categories[category.ID] = &copied
	return nil
}

func (r *fakeCategoryRepository) Delete(_ context.Context, userID, id int64) error {
	existing, ok := r.categories[id]
	if !ok || existing.UserID != userID {
		return utils.NotFound("Category")
	}
	delete(r.categories, id)
	return nil
}

func (r *fakeCategoryRepository) FindByID(_ context.Context, userID, id int64) (*model.Category, error) {
	existing, ok := r.categories[id]
	// Ownership is part of the lookup: another user's row must read as absent.
	if !ok || existing.UserID != userID {
		return nil, nil
	}
	copied := *existing
	return &copied, nil
}

func (r *fakeCategoryRepository) FindByName(_ context.Context, userID int64, categoryType model.CategoryType, name string) (*model.Category, error) {
	for _, category := range r.categories {
		if category.UserID == userID && category.Type == categoryType && category.Name == name {
			copied := *category
			return &copied, nil
		}
	}
	return nil, nil
}

func (r *fakeCategoryRepository) List(_ context.Context, userID int64, query dto.CategoryListQuery) ([]model.Category, int64, error) {
	var matched []model.Category
	for _, category := range r.categories {
		if category.UserID != userID {
			continue
		}
		if query.Type != "" && string(category.Type) != query.Type {
			continue
		}
		if query.IsActive != nil && category.IsActive != *query.IsActive {
			continue
		}
		if query.Search != "" && !strings.Contains(strings.ToLower(category.Name), strings.ToLower(query.Search)) {
			continue
		}
		matched = append(matched, *category)
	}

	sort.Slice(matched, func(i, j int) bool { return matched[i].Name < matched[j].Name })

	total := int64(len(matched))
	start := utils.Offset(query.Page, query.PageSize)
	if start >= len(matched) {
		return nil, total, nil
	}
	end := start + query.PageSize
	if end > len(matched) {
		end = len(matched)
	}
	return matched[start:end], total, nil
}

func (r *fakeCategoryRepository) CountTransactions(_ context.Context, _, id int64) (int64, error) {
	return r.usage[id], nil
}

func (r *fakeCategoryRepository) SetActive(_ context.Context, userID, id int64, isActive bool) error {
	existing, ok := r.categories[id]
	if !ok || existing.UserID != userID {
		return utils.NotFound("Category")
	}
	existing.IsActive = isActive
	return nil
}

type fakeTransactionRepository struct {
	transactions map[int64]*model.Transaction
	categories   *fakeCategoryRepository
	nextID       int64
	failOn       string
}

func newFakeTransactionRepository(categories *fakeCategoryRepository) *fakeTransactionRepository {
	return &fakeTransactionRepository{
		transactions: map[int64]*model.Transaction{},
		categories:   categories,
		nextID:       1,
	}
}

func (r *fakeTransactionRepository) WithTx(*gorm.DB) repository.TransactionRepository { return r }

func (r *fakeTransactionRepository) Create(_ context.Context, transaction *model.Transaction) error {
	if r.failOn == "Create" {
		return errBoom
	}
	transaction.ID = r.nextID
	r.nextID++
	copied := *transaction
	r.transactions[transaction.ID] = &copied
	return nil
}

func (r *fakeTransactionRepository) Update(_ context.Context, transaction *model.Transaction) error {
	existing, ok := r.transactions[transaction.ID]
	if !ok || existing.UserID != transaction.UserID {
		return utils.NotFound("Transaction")
	}
	copied := *transaction
	r.transactions[transaction.ID] = &copied
	return nil
}

func (r *fakeTransactionRepository) Delete(_ context.Context, userID, id int64) error {
	existing, ok := r.transactions[id]
	if !ok || existing.UserID != userID {
		return utils.NotFound("Transaction")
	}
	delete(r.transactions, id)
	return nil
}

func (r *fakeTransactionRepository) FindByID(_ context.Context, userID, id int64) (*model.Transaction, error) {
	existing, ok := r.transactions[id]
	if !ok || existing.UserID != userID {
		return nil, nil
	}
	copied := *existing
	r.attachCategory(&copied)
	return &copied, nil
}

func (r *fakeTransactionRepository) List(_ context.Context, userID int64, filter dto.TransactionFilter) ([]model.Transaction, int64, error) {
	matched := r.match(userID, filter.DateFrom, filter.DateTo, filter.TransactionType, filter.CategoryID)

	if filter.Search != "" {
		needle := strings.ToLower(filter.Search)
		var filtered []model.Transaction
		for _, transaction := range matched {
			if strings.Contains(strings.ToLower(transaction.Description), needle) ||
				strings.Contains(strings.ToLower(transaction.ReferenceNumber), needle) {
				filtered = append(filtered, transaction)
			}
		}
		matched = filtered
	}

	sort.Slice(matched, func(i, j int) bool {
		if matched[i].TransactionDate.Equal(matched[j].TransactionDate) {
			return matched[i].ID > matched[j].ID
		}
		return matched[i].TransactionDate.After(matched[j].TransactionDate)
	})

	total := int64(len(matched))
	start := utils.Offset(filter.Page, filter.PageSize)
	if start >= len(matched) {
		return nil, total, nil
	}
	end := start + filter.PageSize
	if end > len(matched) {
		end = len(matched)
	}
	return matched[start:end], total, nil
}

func (r *fakeTransactionRepository) ListForReport(_ context.Context, userID int64, filter dto.ReportFilter) ([]model.Transaction, error) {
	if r.failOn == "ListForReport" {
		return nil, errBoom
	}

	matched := r.match(userID, &filter.DateFrom, &filter.DateTo, filter.TransactionType, filter.CategoryID)
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].TransactionDate.Equal(matched[j].TransactionDate) {
			return matched[i].ID < matched[j].ID
		}
		return matched[i].TransactionDate.Before(matched[j].TransactionDate)
	})
	return matched, nil
}

func (r *fakeTransactionRepository) match(
	userID int64,
	from, to *time.Time,
	transactionType string,
	categoryID *int64,
) []model.Transaction {
	var matched []model.Transaction
	for _, transaction := range r.transactions {
		if transaction.UserID != userID {
			continue
		}
		if from != nil && transaction.TransactionDate.Before(*from) {
			continue
		}
		if to != nil && transaction.TransactionDate.After(*to) {
			continue
		}
		if transactionType != "" && string(transaction.TransactionType) != transactionType {
			continue
		}
		if categoryID != nil && (transaction.CategoryID == nil || *transaction.CategoryID != *categoryID) {
			continue
		}
		copied := *transaction
		r.attachCategory(&copied)
		matched = append(matched, copied)
	}
	return matched
}

// attachCategory mirrors the Preload the real repository performs.
func (r *fakeTransactionRepository) attachCategory(transaction *model.Transaction) {
	if transaction.CategoryID == nil || r.categories == nil {
		return
	}
	if category, ok := r.categories.categories[*transaction.CategoryID]; ok {
		copied := *category
		transaction.Category = &copied
	}
}

type fakeReportRepository struct {
	totals    repository.Totals
	byCategor []repository.CategoryTotal
	monthly   []repository.PeriodTotal
	daily     []repository.PeriodTotal
	failOn    string
}

func (r *fakeReportRepository) Totals(context.Context, int64, dto.ReportFilter) (repository.Totals, error) {
	if r.failOn == "Totals" {
		return repository.Totals{}, errBoom
	}
	return r.totals, nil
}

func (r *fakeReportRepository) ExpenseByCategory(context.Context, int64, dto.ReportFilter) ([]repository.CategoryTotal, error) {
	if r.failOn == "ExpenseByCategory" {
		return nil, errBoom
	}
	return r.byCategor, nil
}

func (r *fakeReportRepository) Monthly(context.Context, int64, dto.ReportFilter) ([]repository.PeriodTotal, error) {
	if r.failOn == "Monthly" {
		return nil, errBoom
	}
	return r.monthly, nil
}

func (r *fakeReportRepository) Daily(context.Context, int64, dto.ReportFilter) ([]repository.PeriodTotal, error) {
	if r.failOn == "Daily" {
		return nil, errBoom
	}
	return r.daily, nil
}

func decimalOf(value string) decimal.Decimal {
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func dateOf(value string) time.Time {
	parsed, err := time.Parse(utils.DateLayout, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

// fakeAccountRepository is an in-memory stand-in with the same ownership rules as the real
// repository: a lookup with the wrong user id finds nothing.
type fakeAccountRepository struct {
	accounts  map[int64]*model.Account
	snapshots map[int64]model.AccountBalanceSnapshot
	expenses  map[int64][]model.Transaction
	nextID    int64
	failOn    string
}

func newFakeAccountRepository() *fakeAccountRepository {
	return &fakeAccountRepository{
		accounts:  map[int64]*model.Account{},
		snapshots: map[int64]model.AccountBalanceSnapshot{},
		expenses:  map[int64][]model.Transaction{},
		nextID:    1,
	}
}

func (r *fakeAccountRepository) seed(userID int64, name string, accountType model.AccountType, active bool) *model.Account {
	account := &model.Account{
		ID:          r.nextID,
		UserID:      userID,
		Name:        name,
		AccountType: accountType,
		IsActive:    active,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	r.accounts[account.ID] = account
	r.nextID++
	return account
}

func (r *fakeAccountRepository) seedSnapshot(account *model.Account, asOf time.Time, balance string) {
	r.snapshots[account.ID] = model.AccountBalanceSnapshot{
		UserID:        account.UserID,
		AccountID:     account.ID,
		AsOfDate:      asOf,
		ActualBalance: decimalOf(balance),
	}
}

func (r *fakeAccountRepository) WithTx(*gorm.DB) repository.AccountRepository { return r }

func (r *fakeAccountRepository) Create(_ context.Context, account *model.Account) error {
	if r.failOn == "Create" {
		return errBoom
	}
	account.ID = r.nextID
	r.nextID++
	copied := *account
	r.accounts[account.ID] = &copied
	return nil
}

func (r *fakeAccountRepository) Update(_ context.Context, account *model.Account) error {
	existing, ok := r.accounts[account.ID]
	if !ok || existing.UserID != account.UserID {
		return utils.NotFound("Account")
	}
	copied := *account
	r.accounts[account.ID] = &copied
	return nil
}

func (r *fakeAccountRepository) Delete(_ context.Context, userID, id int64) error {
	existing, ok := r.accounts[id]
	if !ok || existing.UserID != userID {
		return utils.NotFound("Account")
	}
	delete(r.accounts, id)
	return nil
}

func (r *fakeAccountRepository) FindByID(_ context.Context, userID, id int64) (*model.Account, error) {
	if r.failOn == "FindByID" {
		return nil, errBoom
	}
	account, ok := r.accounts[id]
	if !ok || account.UserID != userID {
		return nil, nil
	}
	copied := *account
	return &copied, nil
}

func (r *fakeAccountRepository) FindByName(_ context.Context, userID int64, name string) (*model.Account, error) {
	for _, account := range r.accounts {
		if account.UserID == userID && account.Name == name {
			copied := *account
			return &copied, nil
		}
	}
	return nil, nil
}

func (r *fakeAccountRepository) List(_ context.Context, userID int64, query dto.AccountListQuery) ([]model.Account, int64, error) {
	var matches []model.Account
	for _, account := range r.accounts {
		if account.UserID != userID {
			continue
		}
		if query.AccountType != "" && string(account.AccountType) != query.AccountType {
			continue
		}
		if query.IsActive != nil && account.IsActive != *query.IsActive {
			continue
		}
		matches = append(matches, *account)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Name < matches[j].Name })
	return matches, int64(len(matches)), nil
}

func (r *fakeAccountRepository) ListByType(_ context.Context, userID int64, types ...model.AccountType) ([]model.Account, error) {
	allowed := map[model.AccountType]bool{}
	for _, accountType := range types {
		allowed[accountType] = true
	}

	var matches []model.Account
	for _, account := range r.accounts {
		if account.UserID != userID || !account.IsActive || !allowed[account.AccountType] {
			continue
		}
		matches = append(matches, *account)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
	return matches, nil
}

func (r *fakeAccountRepository) CountTransactions(_ context.Context, _, id int64) (int64, error) {
	return int64(len(r.expenses[id])), nil
}

func (r *fakeAccountRepository) SetActive(_ context.Context, userID, id int64, isActive bool) error {
	account, ok := r.accounts[id]
	if !ok || account.UserID != userID {
		return utils.NotFound("Account")
	}
	account.IsActive = isActive
	return nil
}

func (r *fakeAccountRepository) SumOpeningBalance(_ context.Context, userID int64, types ...model.AccountType) (decimal.Decimal, error) {
	allowed := map[model.AccountType]bool{}
	for _, accountType := range types {
		allowed[accountType] = true
	}

	total := decimal.Zero
	for _, account := range r.accounts {
		if account.UserID != userID || !account.IsActive {
			continue
		}
		if len(types) > 0 && !allowed[account.AccountType] {
			continue
		}
		total = total.Add(account.OpeningBalance)
	}
	return total, nil
}

func (r *fakeAccountRepository) UpsertSnapshot(_ context.Context, snapshot *model.AccountBalanceSnapshot) error {
	snapshot.ID = r.nextID
	r.nextID++
	r.snapshots[snapshot.AccountID] = *snapshot
	return nil
}

func (r *fakeAccountRepository) ListSnapshots(_ context.Context, userID, accountID int64, _ int) ([]model.AccountBalanceSnapshot, error) {
	snapshot, ok := r.snapshots[accountID]
	if !ok || snapshot.UserID != userID {
		return nil, nil
	}
	return []model.AccountBalanceSnapshot{snapshot}, nil
}

func (r *fakeAccountRepository) LatestSnapshots(_ context.Context, userID int64, asOf time.Time) (map[int64]model.AccountBalanceSnapshot, error) {
	if r.failOn == "LatestSnapshots" {
		return nil, errBoom
	}
	out := map[int64]model.AccountBalanceSnapshot{}
	for accountID, snapshot := range r.snapshots {
		if snapshot.UserID != userID || snapshot.AsOfDate.After(asOf) {
			continue
		}
		out[accountID] = snapshot
	}
	return out, nil
}

func (r *fakeAccountRepository) DeleteSnapshot(_ context.Context, userID, accountID, _ int64) error {
	snapshot, ok := r.snapshots[accountID]
	if !ok || snapshot.UserID != userID {
		return utils.NotFound("Balance snapshot")
	}
	delete(r.snapshots, accountID)
	return nil
}

// fakeDailyReportRepository returns pre-built aggregates so the report maths can be tested
// without a database. The real repository's job is the SQL; this fake's job is the shape.
type fakeDailyReportRepository struct {
	openingBalance decimal.Decimal
	cashFlow       []repository.LabelledAmount
	flows          map[int64]repository.AccountFlow
	transfers      []repository.TransferEdge
	expenses       map[int64][]model.Transaction
	failOn         string
}

func newFakeDailyReportRepository() *fakeDailyReportRepository {
	return &fakeDailyReportRepository{
		openingBalance: decimal.Zero,
		flows:          map[int64]repository.AccountFlow{},
		expenses:       map[int64][]model.Transaction{},
	}
}

func (r *fakeDailyReportRepository) OpeningBalance(context.Context, int64, time.Time) (decimal.Decimal, error) {
	if r.failOn == "OpeningBalance" {
		return decimal.Zero, errBoom
	}
	return r.openingBalance, nil
}

func (r *fakeDailyReportRepository) CashFlowExpenses(context.Context, int64, time.Time) ([]repository.LabelledAmount, error) {
	if r.failOn == "CashFlowExpenses" {
		return nil, errBoom
	}
	return r.cashFlow, nil
}

func (r *fakeDailyReportRepository) AccountFlows(context.Context, int64, time.Time) (map[int64]repository.AccountFlow, error) {
	if r.failOn == "AccountFlows" {
		return nil, errBoom
	}
	return r.flows, nil
}

func (r *fakeDailyReportRepository) Transfers(context.Context, int64, time.Time) ([]repository.TransferEdge, error) {
	if r.failOn == "Transfers" {
		return nil, errBoom
	}
	return r.transfers, nil
}

func (r *fakeDailyReportRepository) AccountExpenses(_ context.Context, _, accountID int64, _ time.Time) ([]model.Transaction, error) {
	if r.failOn == "AccountExpenses" {
		return nil, errBoom
	}
	return r.expenses[accountID], nil
}

// fakeTelegramClient records what would have been sent instead of reaching the network.
type fakeTelegramClient struct {
	sent      []string
	chatIDs   []int64
	markdown  []bool
	updates   []dto.TelegramUpdate
	webhookOn bool
	failSend  bool
}

func (c *fakeTelegramClient) SendMessage(_ context.Context, chatID int64, text string, markdown bool) error {
	if c.failSend {
		return errBoom
	}
	c.chatIDs = append(c.chatIDs, chatID)
	c.sent = append(c.sent, text)
	c.markdown = append(c.markdown, markdown)
	return nil
}

func (c *fakeTelegramClient) GetUpdates(context.Context, int64) ([]dto.TelegramUpdate, error) {
	updates := c.updates
	c.updates = nil
	return updates, nil
}

func (c *fakeTelegramClient) SetWebhook(_ context.Context, _, _ string) error {
	c.webhookOn = true
	return nil
}

func (c *fakeTelegramClient) DeleteWebhook(context.Context) error {
	c.webhookOn = false
	return nil
}

func (c *fakeTelegramClient) GetMe(context.Context) (string, error) { return "rezanibot", nil }

// fakeTelegramRepository is an in-memory link and pairing-code store.
type fakeTelegramRepository struct {
	codes  map[string]*model.TelegramPairingCode
	links  map[int64]*model.TelegramLink
	nextID int64
}

func newFakeTelegramRepository() *fakeTelegramRepository {
	return &fakeTelegramRepository{
		codes:  map[string]*model.TelegramPairingCode{},
		links:  map[int64]*model.TelegramLink{},
		nextID: 1,
	}
}

func (r *fakeTelegramRepository) WithTx(*gorm.DB) repository.TelegramRepository { return r }

func (r *fakeTelegramRepository) CreatePairingCode(_ context.Context, code *model.TelegramPairingCode) error {
	if _, exists := r.codes[code.Code]; exists {
		return errBoom
	}
	code.ID = r.nextID
	r.nextID++
	copied := *code
	r.codes[code.Code] = &copied
	return nil
}

func (r *fakeTelegramRepository) FindPairingCode(_ context.Context, code string) (*model.TelegramPairingCode, error) {
	pairing, ok := r.codes[code]
	if !ok {
		return nil, nil
	}
	copied := *pairing
	return &copied, nil
}

func (r *fakeTelegramRepository) ConsumePairingCode(_ context.Context, id int64, usedAt time.Time) (bool, error) {
	for _, pairing := range r.codes {
		if pairing.ID != id {
			continue
		}
		if !pairing.Usable(usedAt) {
			return false, nil
		}
		pairing.UsedAt = &usedAt
		return true, nil
	}
	return false, nil
}

func (r *fakeTelegramRepository) UpsertLink(_ context.Context, link *model.TelegramLink) error {
	for chatID, existing := range r.links {
		if existing.UserID == link.UserID || chatID == link.ChatID {
			delete(r.links, chatID)
		}
	}
	link.ID = r.nextID
	r.nextID++
	copied := *link
	r.links[link.ChatID] = &copied
	return nil
}

func (r *fakeTelegramRepository) FindLinkByUser(_ context.Context, userID int64) (*model.TelegramLink, error) {
	for _, link := range r.links {
		if link.UserID == userID {
			copied := *link
			return &copied, nil
		}
	}
	return nil, nil
}

func (r *fakeTelegramRepository) FindLinkByChat(_ context.Context, chatID int64) (*model.TelegramLink, error) {
	link, ok := r.links[chatID]
	if !ok {
		return nil, nil
	}
	copied := *link
	return &copied, nil
}

func (r *fakeTelegramRepository) DeleteLinkByUser(_ context.Context, userID int64) error {
	for chatID, link := range r.links {
		if link.UserID == userID {
			delete(r.links, chatID)
			return nil
		}
	}
	return utils.NotFound("Telegram link")
}

func (r *fakeTelegramRepository) DeleteLinkByChat(_ context.Context, chatID int64) error {
	if _, ok := r.links[chatID]; !ok {
		return utils.NotFound("Telegram link")
	}
	delete(r.links, chatID)
	return nil
}
