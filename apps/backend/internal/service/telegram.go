package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/repository"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

const (
	// pairingCodeLength is long enough that guessing is impractical over a short TTL.
	pairingCodeLength = 8
	// pairingCodeAlphabet leaves out the characters that get misread when retyped (0/O, 1/I).
	pairingCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	// pairingCodeAttempts bounds the retry loop on the unlikely unique-index collision.
	pairingCodeAttempts = 5
)

// helpText is sent for /help and for anything the bot does not recognise.
const helpText = `Perintah yang tersedia:

MEMBACA
/report - laporan cash flow hari ini
/report 2026-09-25 - laporan tanggal tertentu
/saldo - saldo tercatat per akun
/status - status koneksi chat ini

MENCATAT
/saldo <akun> <jumlah> - catat saldo aktual hari ini
/saldo <akun> <jumlah> 2026-09-25 - untuk tanggal tertentu
/catat <akun> <jumlah> <keterangan> - catat pengeluaran
/topup <akun> <jumlah> - alokasi dana ke akun
/topup <akun> <jumlah> dari <akun-sumber> - tentukan sumbernya
/hapus <id> - hapus transaksi yang salah

Jumlah boleh pakai titik atau singkatan: 25000, 25.000, 25rb, 1jt.
Mencatat saldo di tanggal yang sama akan menimpa, jadi salah ketik
cukup dikirim ulang dengan angka yang benar.

LAINNYA
/unlink - lepas koneksi chat ini
/help - pesan ini`

type TelegramService interface {
	// IssuePairingCode returns a short-lived single-use code for the authenticated user.
	IssuePairingCode(ctx context.Context, userID int64) (dto.TelegramPairingCodeResponse, error)
	LinkStatus(ctx context.Context, userID int64) (dto.TelegramLinkResponse, error)
	Unlink(ctx context.Context, userID int64) error
	// SendDailyReport renders the day and delivers it to the user's linked chat.
	SendDailyReport(ctx context.Context, userID int64, date time.Time) (dto.TelegramSendResponse, error)
	// HandleUpdate processes one Telegram update. It never returns an error for a message it
	// simply cannot serve, because the caller's only options are to retry or drop it.
	HandleUpdate(ctx context.Context, update dto.TelegramUpdate)
	Configure(ctx context.Context) error
}

type telegramService struct {
	client   TelegramClient
	links    repository.TelegramRepository
	reports  DailyReportService
	accounts repository.AccountRepository
	// Writes go through the same services the HTTP handlers use, so a command cannot skip
	// the ownership checks or the validation.
	accountWrites     AccountService
	transactionWrites TransactionService
	categories        repository.CategoryRepository
	// codeTTL bounds how long an unused pairing code stays valid.
	codeTTL time.Duration
	// webhookURL and webhookSecret are only set in webhook mode.
	webhookURL    string
	webhookSecret string
	useWebhook    bool

	// botUsername is cached because it never changes for a token and is needed on every
	// pairing request to build the deep link. The mutex guards the lazy fill.
	usernameMu  sync.Mutex
	botUsername string
}

func NewTelegramService(
	client TelegramClient,
	links repository.TelegramRepository,
	reports DailyReportService,
	accounts repository.AccountRepository,
	accountWrites AccountService,
	transactionWrites TransactionService,
	categories repository.CategoryRepository,
	codeTTL time.Duration,
	webhookURL string,
	webhookSecret string,
	useWebhook bool,
) TelegramService {
	return &telegramService{
		client:            client,
		links:             links,
		reports:           reports,
		accounts:          accounts,
		accountWrites:     accountWrites,
		transactionWrites: transactionWrites,
		categories:        categories,
		codeTTL:           codeTTL,
		webhookURL:        webhookURL,
		webhookSecret:     webhookSecret,
		useWebhook:        useWebhook,
	}
}

// Configure registers or clears the webhook so the running mode and Telegram's idea of it
// cannot drift apart.
func (s *telegramService) Configure(ctx context.Context) error {
	username, err := s.client.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("verify bot token: %w", err)
	}
	s.cacheUsername(username)
	slog.Info("telegram bot ready", "username", username, "webhook", s.useWebhook)

	if !s.useWebhook {
		// Polling and a registered webhook are mutually exclusive: Telegram refuses
		// getUpdates while a webhook is set.
		if err := s.client.DeleteWebhook(ctx); err != nil {
			return fmt.Errorf("clear webhook: %w", err)
		}
		return nil
	}
	if s.webhookURL == "" {
		return fmt.Errorf("TELEGRAM_WEBHOOK_URL is required to register a webhook")
	}
	if err := s.client.SetWebhook(ctx, s.webhookURL, s.webhookSecret); err != nil {
		return fmt.Errorf("register webhook: %w", err)
	}
	return nil
}

func (s *telegramService) IssuePairingCode(ctx context.Context, userID int64) (dto.TelegramPairingCodeResponse, error) {
	now := time.Now().UTC()

	var lastErr error
	for attempt := 0; attempt < pairingCodeAttempts; attempt++ {
		code, err := newPairingCode()
		if err != nil {
			return dto.TelegramPairingCodeResponse{}, utils.WrapDomainError(utils.ErrConflict, "Failed to generate pairing code", err)
		}

		pairing := &model.TelegramPairingCode{
			UserID:    userID,
			Code:      code,
			ExpiresAt: now.Add(s.codeTTL),
			CreatedAt: now,
		}
		if err := s.links.CreatePairingCode(ctx, pairing); err != nil {
			lastErr = err
			continue
		}
		response := dto.TelegramPairingCodeResponse{
			Code:        code,
			ExpiresAt:   pairing.ExpiresAt,
			Instruction: fmt.Sprintf("Kirim pesan \"/start %s\" ke bot Telegram", code),
		}
		// The deep link is what turns pairing into one tap. A bot that cannot be reached
		// still yields a usable code, so this failure is not fatal to the request.
		if username := s.resolveBotUsername(ctx); username != "" {
			response.BotUsername = username
			response.DeepLink = "https://t.me/" + username + "?start=" + url.QueryEscape(code)
			response.Instruction = fmt.Sprintf("Buka t.me/%s lalu tekan START, atau kirim \"/start %s\"", username, code)
		}
		return response, nil
	}
	return dto.TelegramPairingCodeResponse{}, utils.WrapDomainError(utils.ErrConflict, "Failed to store pairing code", lastErr)
}

// resolveBotUsername returns the cached username, asking Telegram once if it is not known
// yet. An unreachable bot yields an empty string rather than an error, because a pairing code
// is still useful without the deep link.
func (s *telegramService) resolveBotUsername(ctx context.Context) string {
	s.usernameMu.Lock()
	cached := s.botUsername
	s.usernameMu.Unlock()
	if cached != "" {
		return cached
	}

	username, err := s.client.GetMe(ctx)
	if err != nil {
		slog.Warn("could not resolve the bot username for a deep link", "error", err)
		return ""
	}
	s.cacheUsername(username)
	return username
}

func (s *telegramService) cacheUsername(username string) {
	s.usernameMu.Lock()
	s.botUsername = username
	s.usernameMu.Unlock()
}

func (s *telegramService) LinkStatus(ctx context.Context, userID int64) (dto.TelegramLinkResponse, error) {
	link, err := s.links.FindLinkByUser(ctx, userID)
	if err != nil {
		return dto.TelegramLinkResponse{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to load telegram link", err)
	}
	if link == nil {
		return dto.TelegramLinkResponse{Linked: false}, nil
	}
	linkedAt := link.LinkedAt
	return dto.TelegramLinkResponse{
		Linked:    true,
		ChatID:    link.ChatID,
		Username:  link.Username,
		ChatTitle: link.ChatTitle,
		LinkedAt:  &linkedAt,
	}, nil
}

func (s *telegramService) Unlink(ctx context.Context, userID int64) error {
	if err := s.links.DeleteLinkByUser(ctx, userID); err != nil {
		return wrapIfInternal(err, "Failed to unlink telegram")
	}
	return nil
}

func (s *telegramService) SendDailyReport(ctx context.Context, userID int64, date time.Time) (dto.TelegramSendResponse, error) {
	link, err := s.links.FindLinkByUser(ctx, userID)
	if err != nil {
		return dto.TelegramSendResponse{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to load telegram link", err)
	}
	if link == nil {
		return dto.TelegramSendResponse{}, utils.NewDomainError(utils.ErrValidation, "No Telegram chat is linked to this account")
	}

	report, err := s.reports.Build(ctx, userID, date)
	if err != nil {
		return dto.TelegramSendResponse{}, err
	}

	text := RenderDailyReportMarkdown(report)
	messages := SplitTelegramMessage(text)
	for _, message := range messages {
		if err := s.client.SendMessage(ctx, link.ChatID, message, true); err != nil {
			return dto.TelegramSendResponse{}, utils.WrapDomainError(utils.ErrConflict, "Failed to send Telegram message", err)
		}
	}

	return dto.TelegramSendResponse{
		ChatID:     link.ChatID,
		Messages:   len(messages),
		Characters: len([]rune(text)),
	}, nil
}

func (s *telegramService) HandleUpdate(ctx context.Context, update dto.TelegramUpdate) {
	message := update.Message
	if message == nil {
		message = update.EditedMessage
	}
	if message == nil || message.Chat == nil {
		return
	}
	// A bot talking to a bot is never the user, so it is ignored outright.
	if message.From != nil && message.From.IsBot {
		return
	}

	command, argument := parseCommand(message.Text)
	if command == "" {
		return
	}

	chatID := message.Chat.ID

	if command == "start" {
		s.handleStart(ctx, message, argument)
		return
	}

	link, err := s.links.FindLinkByChat(ctx, chatID)
	if err != nil {
		slog.Error("telegram link lookup failed", "chat_id", chatID, "error", err)
		s.reply(ctx, chatID, "Terjadi kesalahan internal. Coba lagi sebentar.")
		return
	}
	if link == nil {
		// Nothing about the account is revealed to an unlinked chat, not even whether one
		// exists.
		s.reply(ctx, chatID, "Chat ini belum terhubung. Ambil kode pairing di aplikasi, lalu kirim /start <kode>.")
		return
	}

	switch command {
	case "report":
		s.handleReport(ctx, link, argument)
	case "saldo":
		// With no argument this lists balances; with one it records a new one.
		if strings.TrimSpace(argument) == "" {
			s.handleBalances(ctx, link)
			return
		}
		s.handleRecordBalance(ctx, link, argument)
	case "catat":
		s.handleRecordExpense(ctx, link, argument)
	case "topup":
		s.handleTopUp(ctx, link, argument)
	case "hapus":
		s.handleDelete(ctx, link, argument)
	case "status":
		s.reply(ctx, chatID, fmt.Sprintf("Chat ini terhubung sejak %s.", link.LinkedAt.Format("2006-01-02 15:04 UTC")))
	case "unlink":
		if err := s.links.DeleteLinkByChat(ctx, chatID); err != nil {
			slog.Error("telegram unlink failed", "chat_id", chatID, "error", err)
			s.reply(ctx, chatID, "Gagal melepas koneksi. Coba lagi.")
			return
		}
		s.reply(ctx, chatID, "Koneksi dilepas. Kirim /start <kode> untuk menghubungkan lagi.")
	default:
		s.reply(ctx, chatID, helpText)
	}
}

func (s *telegramService) handleStart(ctx context.Context, message *dto.TelegramMessage, argument string) {
	chatID := message.Chat.ID

	code := strings.ToUpper(strings.TrimSpace(argument))
	if code == "" {
		s.reply(ctx, chatID, "Kirim /start <kode> dengan kode pairing dari aplikasi.")
		return
	}

	pairing, err := s.links.FindPairingCode(ctx, code)
	if err != nil {
		slog.Error("pairing code lookup failed", "chat_id", chatID, "error", err)
		s.reply(ctx, chatID, "Terjadi kesalahan internal. Coba lagi sebentar.")
		return
	}
	// A wrong code and an expired code get the same answer, so the reply cannot be used to
	// probe which codes exist.
	if pairing == nil {
		s.reply(ctx, chatID, "Kode tidak valid atau sudah kedaluwarsa.")
		return
	}

	now := time.Now().UTC()
	claimed, err := s.links.ConsumePairingCode(ctx, pairing.ID, now)
	if err != nil {
		slog.Error("pairing code consume failed", "chat_id", chatID, "error", err)
		s.reply(ctx, chatID, "Terjadi kesalahan internal. Coba lagi sebentar.")
		return
	}
	if !claimed {
		s.reply(ctx, chatID, "Kode tidak valid atau sudah kedaluwarsa.")
		return
	}

	link := &model.TelegramLink{
		UserID:    pairing.UserID,
		ChatID:    chatID,
		Username:  chatUsername(message),
		ChatTitle: chatTitle(message.Chat),
		LinkedAt:  now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.links.UpsertLink(ctx, link); err != nil {
		slog.Error("telegram link failed", "chat_id", chatID, "error", err)
		s.reply(ctx, chatID, "Gagal menghubungkan chat. Coba ambil kode baru.")
		return
	}

	slog.Info("telegram chat linked", "chat_id", chatID, "user_id", pairing.UserID)
	s.reply(ctx, chatID, "Chat terhubung. "+helpText)
}

func (s *telegramService) handleReport(ctx context.Context, link *model.TelegramLink, argument string) {
	date := time.Now().UTC()
	if trimmed := strings.TrimSpace(argument); trimmed != "" {
		parsed, err := utils.ParseDate(trimmed)
		if err != nil {
			s.reply(ctx, link.ChatID, "Format tanggal harus YYYY-MM-DD, contoh: /report 2026-09-25")
			return
		}
		date = parsed
	}

	report, err := s.reports.Build(ctx, link.UserID, date)
	if err != nil {
		slog.Error("telegram report build failed", "user_id", link.UserID, "error", err)
		s.reply(ctx, link.ChatID, "Gagal menyusun laporan. Coba lagi sebentar.")
		return
	}

	for _, message := range SplitTelegramMessage(RenderDailyReportMarkdown(report)) {
		if err := s.client.SendMessage(ctx, link.ChatID, message, true); err != nil {
			slog.Error("telegram send failed", "chat_id", link.ChatID, "error", err)
			return
		}
	}
}

func (s *telegramService) handleBalances(ctx context.Context, link *model.TelegramLink) {
	accounts, err := s.accounts.ListByType(ctx, link.UserID, model.AccountTypes()...)
	if err != nil {
		slog.Error("telegram balances failed", "user_id", link.UserID, "error", err)
		s.reply(ctx, link.ChatID, "Gagal memuat saldo. Coba lagi sebentar.")
		return
	}
	if len(accounts) == 0 {
		s.reply(ctx, link.ChatID, "Belum ada akun yang dibuat.")
		return
	}

	snapshots, err := s.accounts.LatestSnapshots(ctx, link.UserID, time.Now().UTC())
	if err != nil {
		slog.Error("telegram snapshots failed", "user_id", link.UserID, "error", err)
		s.reply(ctx, link.ChatID, "Gagal memuat saldo. Coba lagi sebentar.")
		return
	}

	lines := []string{"🏦 " + bold("SALDO PER AKUN"), ""}
	for _, account := range accounts {
		value := "belum dicatat"
		if snapshot, ok := snapshots[account.ID]; ok {
			value = rupiah(snapshot.ActualBalance) + " (" + utils.FormatDate(snapshot.AsOfDate) + ")"
		}
		lines = append(lines, "• "+escapeMarkdownV2(account.Name)+" : "+escapeMarkdownV2(value))
	}

	if err := s.client.SendMessage(ctx, link.ChatID, strings.Join(lines, "\n"), true); err != nil {
		slog.Error("telegram send failed", "chat_id", link.ChatID, "error", err)
	}
}

// reply sends plain text. Plain text needs no escaping, which is what makes it the right
// choice for the error and help paths.
func (s *telegramService) reply(ctx context.Context, chatID int64, text string) {
	if err := s.client.SendMessage(ctx, chatID, text, false); err != nil {
		slog.Error("telegram reply failed", "chat_id", chatID, "error", err)
	}
}

// parseCommand extracts a /command and its argument. Telegram appends @botname in groups,
// so that suffix is stripped before matching.
func parseCommand(text string) (command, argument string) {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "/") {
		return "", ""
	}

	parts := strings.SplitN(trimmed[1:], " ", 2)
	command = strings.ToLower(parts[0])
	if at := strings.Index(command, "@"); at >= 0 {
		command = command[:at]
	}
	if len(parts) == 2 {
		argument = strings.TrimSpace(parts[1])
	}
	return command, argument
}

// newPairingCode uses crypto/rand: a guessable code would hand an account's finances to
// whoever guessed it.
func newPairingCode() (string, error) {
	buffer := make([]byte, pairingCodeLength)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	out := make([]byte, pairingCodeLength)
	for index, value := range buffer {
		out[index] = pairingCodeAlphabet[int(value)%len(pairingCodeAlphabet)]
	}
	return string(out), nil
}

func chatUsername(message *dto.TelegramMessage) string {
	if message.From != nil && message.From.Username != "" {
		return message.From.Username
	}
	if message.Chat != nil {
		return message.Chat.Username
	}
	return ""
}

func chatTitle(chat *dto.TelegramChat) string {
	if chat.Title != "" {
		return chat.Title
	}
	return chat.FirstName
}
