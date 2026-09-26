package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

const testChatID int64 = 987654321

// markdownV2Specials are the characters Telegram rejects when they appear unescaped. The
// asterisk is handled separately because it is also the bold marker.
const markdownV2Specials = "_[]()~`>#+-=|{}.!"

func newTelegramScenario() (TelegramService, *fakeTelegramClient, *fakeTelegramRepository) {
	service, client, links, _, _ := newTelegramWriteScenario()
	return service, client, links
}

// newTelegramWriteScenario also exposes the stores the write commands touch, so a test can
// assert what actually landed rather than only what was replied.
func newTelegramWriteScenario() (TelegramService, *fakeTelegramClient, *fakeTelegramRepository, *fakeAccountRepository, *fakeTransactionRepository) {
	reports, _, accounts := newReportScenario()
	categories := newFakeCategoryRepository()
	transactions := newFakeTransactionRepository(categories)
	client := &fakeTelegramClient{}
	links := newFakeTelegramRepository()

	// The bot writes through the same services the HTTP handlers use.
	accountService := NewAccountService(accounts)
	transactionService := NewTransactionService(transactions, categories, accounts, &fakeTxManager{})

	return NewTelegramService(
		client,
		links,
		reports,
		accounts,
		accountService,
		transactionService,
		categories,
		15*time.Minute,
		"", "", false,
	), client, links, accounts, transactions
}

// A rendered report has to be valid MarkdownV2 or Telegram rejects the whole message, so
// every special character outside a bold marker must carry a backslash.
func TestRenderedReportEscapesEveryMarkdownSpecial(t *testing.T) {
	service, _, _ := newReportScenario()
	report, err := service.Build(context.Background(), ownerID, reportDay)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	message := RenderDailyReportMarkdown(report)
	runes := []rune(message)
	unescapedAsterisks := 0

	for index, char := range runes {
		escaped := index > 0 && runes[index-1] == '\\'
		switch {
		case char == '*':
			if !escaped {
				unescapedAsterisks++
			}
		case char == '\\':
			// A backslash escapes the next rune; nothing to assert about itself.
		case strings.ContainsRune(markdownV2Specials, char):
			if !escaped {
				t.Fatalf("unescaped %q at index %d: %q", char, index, excerpt(runes, index))
			}
		}
	}

	// Bold markers come in pairs. An odd count means an entity was left open, which Telegram
	// rejects just as hard as an unescaped character.
	if unescapedAsterisks%2 != 0 {
		t.Fatalf("unbalanced bold markers: %d unescaped asterisks", unescapedAsterisks)
	}
	// The amounts must survive escaping with their thousand separators intact.
	if !strings.Contains(message, `Rp1\.106\.500`) {
		t.Fatalf("escaped amount is missing from the message:\n%s", message)
	}
}

func TestRenderedReportContainsEverySection(t *testing.T) {
	service, _, _ := newReportScenario()
	report, err := service.Build(context.Background(), ownerID, reportDay)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	message := RenderDailyReportMarkdown(report)
	for _, want := range []string{
		"CASH FLOW SEPTEMBER 2026",
		"SALDO AWAL",
		"PENGELUARAN CASH FLOW",
		"PAYMENT CC",
		"Net Payment CC",
		"TOP\\-UP",
		"DOMPET HARIAN",
		"SISA DOMPET HARIAN",
		"BANK UTAMA",
		"REKONSILIASI TOP\\-UP",
		"STATUS 25/09",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("message is missing %q:\n%s", want, message)
		}
	}
	// The sub-items of the withdrawal have to be visible, otherwise the breakdown is lost.
	if !strings.Contains(message, "Makan siang") || !strings.Contains(message, "Air mineral") {
		t.Fatal("wallet sub-items are missing from the message")
	}
}

func TestSplitTelegramMessageKeepsChunksSendable(t *testing.T) {
	short := "one section only"
	if chunks := SplitTelegramMessage(short); len(chunks) != 1 || chunks[0] != short {
		t.Fatalf("a short message was split: %+v", chunks)
	}

	// Build something well past the limit out of real section blocks.
	var builder strings.Builder
	for section := 0; section < 40; section++ {
		builder.WriteString("\n" + sectionRule + "\n")
		builder.WriteString(strings.Repeat("• baris laporan yang cukup panjang\n", 8))
	}

	chunks := SplitTelegramMessage(builder.String())
	if len(chunks) < 2 {
		t.Fatalf("a long message was not split: %d chunk(s)", len(chunks))
	}
	for index, chunk := range chunks {
		if length := len([]rune(chunk)); length > telegramMaxMessageLength {
			t.Fatalf("chunk %d is %d runes, over the %d limit", index, length, telegramMaxMessageLength)
		}
	}
}

func TestPairingLinksTheChatAndIsSingleUse(t *testing.T) {
	service, client, _ := newTelegramScenario()
	ctx := context.Background()

	issued, err := service.IssuePairingCode(ctx, ownerID)
	if err != nil {
		t.Fatalf("IssuePairingCode returned error: %v", err)
	}
	if len(issued.Code) != pairingCodeLength {
		t.Fatalf("code = %q, want %d characters", issued.Code, pairingCodeLength)
	}

	service.HandleUpdate(ctx, messageUpdate(1, testChatID, "/start "+issued.Code))

	status, err := service.LinkStatus(ctx, ownerID)
	if err != nil {
		t.Fatalf("LinkStatus returned error: %v", err)
	}
	if !status.Linked || status.ChatID != testChatID {
		t.Fatalf("link status = %+v, want linked to %d", status, testChatID)
	}

	// Spending the same code twice must not link a second chat.
	before := len(client.sent)
	service.HandleUpdate(ctx, messageUpdate(2, 555, "/start "+issued.Code))
	if link, _ := service.LinkStatus(ctx, ownerID); link.ChatID != testChatID {
		t.Fatalf("a reused code moved the link to %d", link.ChatID)
	}
	if len(client.sent) <= before {
		t.Fatal("the second attempt was silently ignored instead of being refused")
	}
	if !strings.Contains(client.sent[len(client.sent)-1], "tidak valid") {
		t.Fatalf("second attempt reply = %q, want a refusal", client.sent[len(client.sent)-1])
	}
}

// An unpaired chat must learn nothing: not a report, and not whether an account exists.
func TestUnlinkedChatIsRefusedEveryCommand(t *testing.T) {
	service, client, _ := newTelegramScenario()
	ctx := context.Background()

	for _, command := range []string{"/report", "/saldo", "/status"} {
		client.sent = nil
		service.HandleUpdate(ctx, messageUpdate(1, testChatID, command))

		if len(client.sent) != 1 {
			t.Fatalf("%s produced %d replies, want 1", command, len(client.sent))
		}
		if !strings.Contains(client.sent[0], "belum terhubung") {
			t.Fatalf("%s replied %q, want a not-linked notice", command, client.sent[0])
		}
		if strings.Contains(client.sent[0], "Dompet Harian") || strings.Contains(client.sent[0], "SALDO AWAL") {
			t.Fatalf("%s leaked account data to an unlinked chat: %q", command, client.sent[0])
		}
	}
}

func TestReportCommandSendsTheRenderedReport(t *testing.T) {
	service, client, _ := newTelegramScenario()
	ctx := context.Background()
	linkChat(ctx, t, service)

	client.sent = nil
	service.HandleUpdate(ctx, messageUpdate(3, testChatID, "/report 2026-09-25"))

	if len(client.sent) == 0 {
		t.Fatal("no report was sent")
	}
	if !client.markdown[len(client.markdown)-1] {
		t.Fatal("the report was sent without MarkdownV2")
	}
	if !strings.Contains(client.sent[0], "CASH FLOW SEPTEMBER 2026") {
		t.Fatalf("unexpected report body: %q", client.sent[0])
	}
}

func TestReportCommandRejectsABadDate(t *testing.T) {
	service, client, _ := newTelegramScenario()
	ctx := context.Background()
	linkChat(ctx, t, service)

	client.sent = nil
	service.HandleUpdate(ctx, messageUpdate(4, testChatID, "/report 25-09-2026"))

	if len(client.sent) != 1 || !strings.Contains(client.sent[0], "YYYY-MM-DD") {
		t.Fatalf("replies = %+v, want one format hint", client.sent)
	}
}

// Telegram appends @botname to commands in groups, so the suffix must not break matching.
func TestCommandParsingStripsTheBotSuffix(t *testing.T) {
	cases := map[string][2]string{
		"/report@rezanibot 2026-09-25": {"report", "2026-09-25"},
		"/REPORT":                      {"report", ""},
		"/start ABC123":                {"start", "ABC123"},
		"not a command":                {"", ""},
		"":                             {"", ""},
	}
	for input, want := range cases {
		command, argument := parseCommand(input)
		if command != want[0] || argument != want[1] {
			t.Fatalf("parseCommand(%q) = (%q, %q), want (%q, %q)", input, command, argument, want[0], want[1])
		}
	}
}

func TestSendDailyReportRequiresALinkedChat(t *testing.T) {
	service, _, _ := newTelegramScenario()

	_, err := service.SendDailyReport(context.Background(), ownerID, reportDay)
	if err == nil {
		t.Fatal("SendDailyReport succeeded without a linked chat")
	}
	domainErr, ok := utils.AsDomainError(err)
	if !ok || domainErr.Kind != utils.ErrValidation {
		t.Fatalf("error = %v, want a validation domain error", err)
	}
}

func TestSendDailyReportReportsWhatWasDelivered(t *testing.T) {
	service, client, _ := newTelegramScenario()
	ctx := context.Background()
	linkChat(ctx, t, service)

	client.sent = nil
	result, err := service.SendDailyReport(ctx, ownerID, reportDay)
	if err != nil {
		t.Fatalf("SendDailyReport returned error: %v", err)
	}
	if result.ChatID != testChatID {
		t.Fatalf("chat id = %d, want %d", result.ChatID, testChatID)
	}
	if result.Messages != len(client.sent) {
		t.Fatalf("reported %d messages but sent %d", result.Messages, len(client.sent))
	}
	if result.Characters == 0 {
		t.Fatal("character count was not reported")
	}
}

func TestUnlinkStopsDelivery(t *testing.T) {
	service, _, _ := newTelegramScenario()
	ctx := context.Background()
	linkChat(ctx, t, service)

	if err := service.Unlink(ctx, ownerID); err != nil {
		t.Fatalf("Unlink returned error: %v", err)
	}
	if status, _ := service.LinkStatus(ctx, ownerID); status.Linked {
		t.Fatal("the chat is still linked after unlinking")
	}
	if _, err := service.SendDailyReport(ctx, ownerID, reportDay); err == nil {
		t.Fatal("a report was sent after unlinking")
	}
}

// Polling mode and a registered webhook cannot both be active, so Configure has to clear one.
func TestConfigureClearsTheWebhookInPollingMode(t *testing.T) {
	service, client, _ := newTelegramScenario()
	client.webhookOn = true

	if err := service.Configure(context.Background()); err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	if client.webhookOn {
		t.Fatal("the webhook is still registered while polling")
	}
}

func TestPollerAdvancesPastHandledUpdates(t *testing.T) {
	service, client, _ := newTelegramScenario()
	client.updates = []dto.TelegramUpdate{
		messageUpdate(41, testChatID, "/help"),
		messageUpdate(42, testChatID, "/help"),
	}

	poller := NewTelegramPoller(client, service)
	ctx, cancel := context.WithCancel(context.Background())

	// The loop is stopped as soon as the queued updates have been consumed.
	go func() {
		for poller.offset < 43 {
			time.Sleep(time.Millisecond)
		}
		cancel()
	}()
	poller.Run(ctx)

	if poller.offset != 43 {
		t.Fatalf("offset = %d, want 43 so handled updates are not re-delivered", poller.offset)
	}
}

func linkChat(ctx context.Context, t *testing.T, service TelegramService) {
	t.Helper()
	issued, err := service.IssuePairingCode(ctx, ownerID)
	if err != nil {
		t.Fatalf("IssuePairingCode returned error: %v", err)
	}
	service.HandleUpdate(ctx, messageUpdate(1, testChatID, "/start "+issued.Code))
	status, err := service.LinkStatus(ctx, ownerID)
	if err != nil || !status.Linked {
		t.Fatalf("pairing failed: %+v, %v", status, err)
	}
}

func messageUpdate(updateID, chatID int64, text string) dto.TelegramUpdate {
	return dto.TelegramUpdate{
		UpdateID: updateID,
		Message: &dto.TelegramMessage{
			MessageID: updateID,
			Text:      text,
			Chat:      &dto.TelegramChat{ID: chatID, Type: "private", FirstName: "Reza"},
			From:      &dto.TelegramUser{ID: chatID, Username: "rezani"},
		},
	}
}

// excerpt gives a readable window around a failure so the report does not have to be printed
// in full.
func excerpt(runes []rune, index int) string {
	start := index - 20
	if start < 0 {
		start = 0
	}
	end := index + 20
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[start:end])
}
