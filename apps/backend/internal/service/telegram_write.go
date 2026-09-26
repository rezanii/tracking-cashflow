package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

// defaultExpenseCategory is where a /catat lands when the chat does not say otherwise. It is
// created on demand so recording from a phone never fails for want of a category, and the
// row can be recategorised later in the web app.
const defaultExpenseCategory = "Kebutuhan Harian"

// amountPattern accepts what a person types on a phone: either plain digits, or digits in
// proper thousand groups, followed by an optional shorthand suffix.
//
// The grouping is validated rather than the separators simply stripped. Stripping blindly
// turns "1.5jt" into 15000000 instead of 1500000 — a tenfold error written straight to the
// ledger. Anything that is not unambiguous thousand grouping is refused, and decimals are not
// accepted at all: nobody types rupiah cents into a chat.
var amountPattern = regexp.MustCompile(`^([0-9]+|[0-9]{1,3}(?:[.,][0-9]{3})+)\s*(rb|ribu|k|jt|juta)?$`)

// amountMultipliers expands the shorthand suffixes.
var amountMultipliers = map[string]int64{
	"":     1,
	"k":    1_000,
	"rb":   1_000,
	"ribu": 1_000,
	"jt":   1_000_000,
	"juta": 1_000_000,
}

// parseChatAmount turns "25.000", "25rb" or "1jt" into a decimal.
func parseChatAmount(raw string) (decimal.Decimal, error) {
	text := strings.ToLower(strings.TrimSpace(raw))

	matches := amountPattern.FindStringSubmatch(text)
	if matches == nil {
		return decimal.Zero, fmt.Errorf("jumlah %q tidak dikenali", raw)
	}

	digits := strings.NewReplacer(".", "", ",", "").Replace(matches[1])
	value, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return decimal.Zero, fmt.Errorf("jumlah %q tidak dikenali", raw)
	}

	multiplier, ok := amountMultipliers[matches[2]]
	if !ok {
		return decimal.Zero, fmt.Errorf("satuan %q tidak dikenali", matches[2])
	}
	return decimal.NewFromInt(value * multiplier), nil
}

// matchAccount finds the account whose name the argument starts with, and returns the rest of
// the text. The longest name wins, so "Dana Cadangan" is not mistaken for "Dana", and account
// names containing spaces need no quoting.
func (s *telegramService) matchAccount(ctx context.Context, userID int64, argument string) (*model.Account, string, error) {
	accounts, err := s.accounts.ListByType(ctx, userID, model.AccountTypes()...)
	if err != nil {
		return nil, "", err
	}

	text := strings.TrimSpace(argument)
	lowered := strings.ToLower(text)

	var best *model.Account
	bestLength := 0
	for index, account := range accounts {
		name := strings.ToLower(account.Name)
		if !strings.HasPrefix(lowered, name) {
			continue
		}
		// The name has to end at a word boundary, otherwise "CC" would match "CCleaner".
		if len(lowered) > len(name) && lowered[len(name)] != ' ' {
			continue
		}
		if len(name) > bestLength {
			best = &accounts[index]
			bestLength = len(name)
		}
	}

	if best == nil {
		return nil, "", fmt.Errorf("akun tidak dikenali")
	}
	return best, strings.TrimSpace(text[bestLength:]), nil
}

// accountNames lists what the user could have meant, so a failed match is actionable.
func (s *telegramService) accountNames(ctx context.Context, userID int64) string {
	accounts, err := s.accounts.ListByType(ctx, userID, model.AccountTypes()...)
	if err != nil || len(accounts) == 0 {
		return ""
	}
	names := make([]string, 0, len(accounts))
	for _, account := range accounts {
		names = append(names, account.Name)
	}
	return strings.Join(names, ", ")
}

// handleRecordBalance stores what an account actually holds: "/saldo Dompet Harian 72500", with an
// optional trailing date.
func (s *telegramService) handleRecordBalance(ctx context.Context, link *model.TelegramLink, argument string) {
	account, rest, err := s.matchAccount(ctx, link.UserID, argument)
	if err != nil {
		s.replyUnknownAccount(ctx, link, "/saldo <akun> <jumlah>")
		return
	}
	if rest == "" {
		s.reply(ctx, link.ChatID, fmt.Sprintf("Jumlahnya berapa? Contoh: /saldo %s 72500", account.Name))
		return
	}

	fields := strings.Fields(rest)
	date := time.Now().UTC()
	// A trailing YYYY-MM-DD records an earlier day; without it the balance is today's.
	if len(fields) > 1 {
		parsed, parseErr := utils.ParseDate(fields[len(fields)-1])
		if parseErr != nil {
			s.reply(ctx, link.ChatID, "Tanggal harus YYYY-MM-DD. Contoh: /saldo "+account.Name+" 72500 2026-09-25")
			return
		}
		date = parsed
		fields = fields[:len(fields)-1]
	}

	amount, err := parseChatAmount(strings.Join(fields, ""))
	if err != nil {
		s.reply(ctx, link.ChatID, err.Error()+". Contoh: /saldo "+account.Name+" 72500")
		return
	}

	if _, err := s.accountWrites.RecordBalance(ctx, link.UserID, account.ID, dto.BalanceSnapshotRequest{
		AsOfDate:      utils.FormatDate(date),
		ActualBalance: amount,
		Note:          "Dicatat dari Telegram",
	}); err != nil {
		s.replyDomainError(ctx, link, err, "Gagal mencatat saldo")
		return
	}

	slog.Info("balance recorded from telegram", "user_id", link.UserID, "account_id", account.ID)

	// The stored figure is echoed back with the variance it produces, so a typo is visible
	// immediately and can be corrected by sending the command again.
	lines := []string{
		"✅ " + bold("Saldo "+account.Name+" dicatat"),
		escapeMarkdownV2(rupiah(amount) + " per " + utils.FormatDate(date)),
	}
	lines = append(lines, s.varianceLines(ctx, link.UserID, date, account.ID)...)
	s.replyMarkdown(ctx, link.ChatID, strings.Join(lines, "\n"))
}

// handleRecordExpense records a spend: "/catat Dompet Harian 25000 kopi".
func (s *telegramService) handleRecordExpense(ctx context.Context, link *model.TelegramLink, argument string) {
	account, rest, err := s.matchAccount(ctx, link.UserID, argument)
	if err != nil {
		s.replyUnknownAccount(ctx, link, "/catat <akun> <jumlah> <keterangan>")
		return
	}

	fields := strings.Fields(rest)
	if len(fields) == 0 {
		s.reply(ctx, link.ChatID, fmt.Sprintf("Jumlahnya berapa? Contoh: /catat %s 25000 kopi", account.Name))
		return
	}

	amount, err := parseChatAmount(fields[0])
	if err != nil {
		s.reply(ctx, link.ChatID, err.Error()+". Contoh: /catat "+account.Name+" 25000 kopi")
		return
	}

	description := strings.TrimSpace(strings.Join(fields[1:], " "))
	if description == "" {
		s.reply(ctx, link.ChatID, "Keterangannya apa? Contoh: /catat "+account.Name+" 25000 kopi")
		return
	}

	categoryID, err := s.defaultCategory(ctx, link.UserID)
	if err != nil {
		slog.Error("default category failed", "user_id", link.UserID, "error", err)
		s.reply(ctx, link.ChatID, "Gagal menyiapkan kategori. Coba lagi sebentar.")
		return
	}

	created, err := s.transactionWrites.Create(ctx, link.UserID, dto.TransactionCreateRequest{
		TransactionDate: utils.FormatDate(time.Now().UTC()),
		TransactionType: string(model.TransactionTypeExpense),
		CategoryID:      &categoryID,
		Amount:          amount,
		Description:     description,
		AccountID:       &account.ID,
	})
	if err != nil {
		s.replyDomainError(ctx, link, err, "Gagal mencatat pengeluaran")
		return
	}

	slog.Info("expense recorded from telegram", "user_id", link.UserID, "transaction_id", created.ID)

	// The id is included because /hapus needs it.
	lines := []string{
		"✅ " + bold("Pengeluaran dicatat"),
		escapeMarkdownV2(description + " · " + rupiah(amount)),
		escapeMarkdownV2(account.Name + " · id " + strconv.FormatInt(created.ID, 10)),
	}
	lines = append(lines, s.varianceLines(ctx, link.UserID, time.Now().UTC(), account.ID)...)
	s.replyMarkdown(ctx, link.ChatID, strings.Join(lines, "\n"))
}

// handleTopUp moves money into an account: "/topup Dompet Harian 600000 dari Bank Utama".
func (s *telegramService) handleTopUp(ctx context.Context, link *model.TelegramLink, argument string) {
	target, rest, err := s.matchAccount(ctx, link.UserID, argument)
	if err != nil {
		s.replyUnknownAccount(ctx, link, "/topup <akun> <jumlah>")
		return
	}

	// "dari <akun>" is optional; the text before it is the amount.
	amountText := rest
	sourceText := ""
	if index := strings.Index(strings.ToLower(rest), "dari "); index >= 0 {
		amountText = strings.TrimSpace(rest[:index])
		sourceText = strings.TrimSpace(rest[index+len("dari "):])
	}

	if amountText == "" {
		s.reply(ctx, link.ChatID, fmt.Sprintf("Jumlahnya berapa? Contoh: /topup %s 600000", target.Name))
		return
	}

	amount, err := parseChatAmount(amountText)
	if err != nil {
		s.reply(ctx, link.ChatID, err.Error()+". Contoh: /topup "+target.Name+" 600000")
		return
	}

	source, err := s.resolveSource(ctx, link.UserID, sourceText, target.ID)
	if err != nil {
		s.reply(ctx, link.ChatID, err.Error())
		return
	}

	created, err := s.transactionWrites.Create(ctx, link.UserID, dto.TransactionCreateRequest{
		TransactionDate: utils.FormatDate(time.Now().UTC()),
		TransactionType: string(model.TransactionTypeTransfer),
		Amount:          amount,
		Description:     "Top-up " + target.Name,
		AccountID:       &source.ID,
		ToAccountID:     &target.ID,
	})
	if err != nil {
		s.replyDomainError(ctx, link, err, "Gagal mencatat top-up")
		return
	}

	slog.Info("top-up recorded from telegram", "user_id", link.UserID, "transaction_id", created.ID)

	lines := []string{
		"✅ " + bold("Top-up dicatat"),
		escapeMarkdownV2(source.Name + " → " + target.Name + " · " + rupiah(amount)),
		escapeMarkdownV2("id " + strconv.FormatInt(created.ID, 10)),
	}
	lines = append(lines, s.varianceLines(ctx, link.UserID, time.Now().UTC(), target.ID)...)
	s.replyMarkdown(ctx, link.ChatID, strings.Join(lines, "\n"))
}

// resolveSource picks where a top-up comes from. Naming it is optional when there is exactly
// one bank account, which is the usual case; otherwise the bot refuses rather than guessing.
func (s *telegramService) resolveSource(ctx context.Context, userID int64, name string, targetID int64) (*model.Account, error) {
	if name != "" {
		source, _, err := s.matchAccount(ctx, userID, name)
		if err != nil {
			return nil, fmt.Errorf("akun sumber %q tidak dikenali. Pilihan: %s", name, s.accountNames(ctx, userID))
		}
		if source.ID == targetID {
			return nil, fmt.Errorf("akun sumber dan tujuan tidak boleh sama")
		}
		return source, nil
	}

	banks, err := s.accounts.ListByType(ctx, userID, model.AccountTypeBank)
	if err != nil {
		return nil, fmt.Errorf("gagal memuat akun sumber")
	}

	candidates := make([]model.Account, 0, len(banks))
	for _, bank := range banks {
		if bank.ID != targetID {
			candidates = append(candidates, bank)
		}
	}

	switch len(candidates) {
	case 1:
		return &candidates[0], nil
	case 0:
		return nil, fmt.Errorf("tidak ada akun bank sebagai sumber. Sebutkan sumbernya: /topup ... dari <akun>")
	default:
		names := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			names = append(names, candidate.Name)
		}
		return nil, fmt.Errorf("ada beberapa akun bank (%s). Sebutkan sumbernya: /topup ... dari <akun>", strings.Join(names, ", "))
	}
}

// handleDelete removes a transaction recorded by mistake.
func (s *telegramService) handleDelete(ctx context.Context, link *model.TelegramLink, argument string) {
	id, err := strconv.ParseInt(strings.TrimSpace(argument), 10, 64)
	if err != nil || id <= 0 {
		s.reply(ctx, link.ChatID, "Sebutkan id transaksinya. Contoh: /hapus 42")
		return
	}

	// Deleting is scoped to the linked user, so an id belonging to somebody else reads as
	// missing rather than being removed.
	if err := s.transactionWrites.Delete(ctx, link.UserID, id); err != nil {
		if domainErr, ok := utils.AsDomainError(err); ok && domainErr.Kind == utils.ErrNotFound {
			s.reply(ctx, link.ChatID, fmt.Sprintf("Transaksi id %d tidak ditemukan.", id))
			return
		}
		s.replyDomainError(ctx, link, err, "Gagal menghapus transaksi")
		return
	}

	slog.Info("transaction deleted from telegram", "user_id", link.UserID, "transaction_id", id)
	s.reply(ctx, link.ChatID, fmt.Sprintf("Transaksi id %d dihapus.", id))
}

// defaultCategory finds or creates the category a chat-recorded expense is filed under.
func (s *telegramService) defaultCategory(ctx context.Context, userID int64) (int64, error) {
	existing, err := s.categories.FindByName(ctx, userID, model.CategoryTypeExpense, defaultExpenseCategory)
	if err != nil {
		return 0, err
	}
	if existing != nil {
		return existing.ID, nil
	}

	now := time.Now().UTC()
	category := &model.Category{
		UserID:      userID,
		Name:        defaultExpenseCategory,
		Type:        model.CategoryTypeExpense,
		Description: "Dibuat otomatis untuk pencatatan dari Telegram",
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.categories.Create(ctx, category); err != nil {
		return 0, err
	}
	return category.ID, nil
}

// varianceLines summarise the account after a write, so the effect of the command is visible
// without asking for the whole report. They are omitted when the account has no section that
// day, which is why this returns a slice rather than a string.
func (s *telegramService) varianceLines(ctx context.Context, userID int64, date time.Time, accountID int64) []string {
	report, err := s.reports.Build(ctx, userID, date)
	if err != nil {
		// A failed summary must not make a successful write look failed.
		slog.Warn("post-write summary failed", "user_id", userID, "error", err)
		return nil
	}

	for _, wallet := range report.Wallets {
		if wallet.AccountID != accountID {
			continue
		}
		lines := []string{
			"",
			escapeMarkdownV2("Jatah " + rupiah(wallet.Allotment) + " · tercatat " + rupiah(wallet.RecordedTotal)),
			escapeMarkdownV2("Sisa menurut catatan " + rupiah(wallet.ExpectedRemaining)),
		}
		if wallet.HasActualBalance {
			if wallet.Variance.IsZero() {
				lines = append(lines, "✅ "+escapeMarkdownV2("Cocok dengan saldo aktual."))
			} else {
				lines = append(lines, "⚠️ "+escapeMarkdownV2("Belum tercatat "+rupiah(wallet.Variance)))
			}
		}
		return lines
	}

	for _, bank := range report.Banks {
		if bank.AccountID != accountID {
			continue
		}
		lines := []string{
			"",
			escapeMarkdownV2("Mutasi hari ini " + rupiah(bank.Computed)),
		}
		if bank.HasActualBalance {
			lines = append(lines, escapeMarkdownV2("Saldo aktual "+rupiah(bank.ActualBalance)+" · saldo sebelumnya "+rupiah(bank.PreviousBalance)))
		}
		return lines
	}
	return nil
}

func (s *telegramService) replyUnknownAccount(ctx context.Context, link *model.TelegramLink, usage string) {
	names := s.accountNames(ctx, link.UserID)
	if names == "" {
		s.reply(ctx, link.ChatID, "Belum ada akun. Buat dulu di aplikasi, lalu coba lagi.")
		return
	}
	s.reply(ctx, link.ChatID, "Akun tidak dikenali.\nFormat: "+usage+"\nAkun yang ada: "+names)
}

// replyDomainError surfaces a validation message the user can act on, and hides anything
// internal behind a generic line.
func (s *telegramService) replyDomainError(ctx context.Context, link *model.TelegramLink, err error, fallback string) {
	domainErr, ok := utils.AsDomainError(err)
	if !ok {
		slog.Error("telegram write failed", "user_id", link.UserID, "error", err)
		s.reply(ctx, link.ChatID, fallback+". Coba lagi sebentar.")
		return
	}

	switch domainErr.Kind {
	case utils.ErrValidation, utils.ErrConflict, utils.ErrNotFound:
		message := domainErr.Message
		for field, reason := range domainErr.Fields {
			message += "\n· " + field + ": " + reason
		}
		s.reply(ctx, link.ChatID, message)
	default:
		slog.Error("telegram write failed", "user_id", link.UserID, "error", err)
		s.reply(ctx, link.ChatID, fallback+". Coba lagi sebentar.")
	}
}

// replyMarkdown sends MarkdownV2. Callers escape their own text, because they decide what is
// bold and what is a value.
func (s *telegramService) replyMarkdown(ctx context.Context, chatID int64, text string) {
	if err := s.client.SendMessage(ctx, chatID, text, true); err != nil {
		slog.Error("telegram reply failed", "chat_id", chatID, "error", err)
	}
}
