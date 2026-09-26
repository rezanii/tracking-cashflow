package service

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
)

func TestParseChatAmountAcceptsWhatAPersonTypes(t *testing.T) {
	valid := map[string]string{
		"25000":     "25000",
		"25.000":    "25000",
		"25,000":    "25000",
		"600.000": "600000",
		"25rb":      "25000",
		"25 rb":     "25000",
		"25ribu":    "25000",
		"25k":       "25000",
		"1jt":       "1000000",
		"2 juta":    "2000000",
		"1JT":       "1000000",
	}
	for input, want := range valid {
		got, err := parseChatAmount(input)
		if err != nil {
			t.Fatalf("parseChatAmount(%q) returned error: %v", input, err)
		}
		if !got.Equal(decimalOf(want)) {
			t.Fatalf("parseChatAmount(%q) = %s, want %s", input, got, want)
		}
	}

	// Anything ambiguous or non-numeric is refused rather than guessed at, because a wrong
	// guess here writes a wrong amount to the ledger.
	for _, input := range []string{"", "kopi", "-25000", "25.0.0.0rb x", "abc123", "25 apel", "1.5jt"} {
		if got, err := parseChatAmount(input); err == nil {
			t.Fatalf("parseChatAmount(%q) = %s, want an error", input, got)
		}
	}
}

func TestMatchAccountPrefersTheLongestName(t *testing.T) {
	service, _, _, accounts, _ := newTelegramWriteScenario()
	// "Dana" would otherwise swallow "Dana Cadangan".
	accounts.seed(ownerID, "Dana", model.AccountTypeWallet, true)

	impl, ok := service.(*telegramService)
	if !ok {
		t.Fatal("expected the concrete service")
	}
	ctx := context.Background()

	cases := map[string][2]string{
		"Dana Cadangan 1000000":     {"Dana Cadangan", "1000000"},
		"Dana 5000":                {"Dana", "5000"},
		"Dompet Harian 25000 kopi":       {"Dompet Harian", "25000 kopi"},
		"dompet harian 25000":            {"Dompet Harian", "25000"},
		"Bank Utama 20000 2026-09-25": {"Bank Utama", "20000 2026-09-25"},
	}
	for input, want := range cases {
		account, rest, err := impl.matchAccount(ctx, ownerID, input)
		if err != nil {
			t.Fatalf("matchAccount(%q) returned error: %v", input, err)
		}
		if account.Name != want[0] || rest != want[1] {
			t.Fatalf("matchAccount(%q) = (%q, %q), want (%q, %q)", input, account.Name, rest, want[0], want[1])
		}
	}

	if _, _, err := impl.matchAccount(ctx, ownerID, "Rekening Lain 5000"); err == nil {
		t.Fatal("an unknown account name was matched")
	}
	// Another user's accounts must not be reachable by name.
	if _, _, err := impl.matchAccount(ctx, intruderID, "Dompet Harian 25000"); err == nil {
		t.Fatal("matched an account belonging to a different user")
	}
}

func TestSaldoCommandRecordsTheBalance(t *testing.T) {
	service, client, _, accounts, _ := newTelegramWriteScenario()
	ctx := context.Background()
	linkChat(ctx, t, service)

	client.sent = nil
	service.HandleUpdate(ctx, messageUpdate(10, testChatID, "/saldo Dompet Harian 72.500"))

	snapshot, ok := accounts.snapshots[walletAccountID]
	if !ok {
		t.Fatal("no balance was stored")
	}
	if !snapshot.ActualBalance.Equal(decimalOf("72500")) {
		t.Fatalf("stored balance = %s, want 72500", snapshot.ActualBalance)
	}

	if len(client.sent) != 1 {
		t.Fatalf("replies = %d, want 1", len(client.sent))
	}
	// The reply has to show the stored figure and the variance, so a typo is visible at once.
	reply := client.sent[0]
	for _, want := range []string{"54\\.165", "tercatat", "Belum tercatat"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("reply is missing %q:\n%s", want, reply)
		}
	}
}

func TestSaldoCommandAcceptsATrailingDate(t *testing.T) {
	service, client, _, accounts, _ := newTelegramWriteScenario()
	ctx := context.Background()
	linkChat(ctx, t, service)

	client.sent = nil
	service.HandleUpdate(ctx, messageUpdate(11, testChatID, "/saldo Bank Utama 20000 2026-09-25"))

	snapshot, ok := accounts.snapshots[bankAccountID]
	if !ok {
		t.Fatal("no balance was stored")
	}
	if got := snapshot.AsOfDate.Format("2006-01-02"); got != "2026-09-25" {
		t.Fatalf("as_of_date = %s, want 2026-09-25", got)
	}
	if !snapshot.ActualBalance.Equal(decimalOf("20000")) {
		t.Fatalf("stored balance = %s, want 20000", snapshot.ActualBalance)
	}
}

// Re-sending the command is how a typo is corrected, so the second write must replace the
// first rather than pile up.
func TestSaldoCommandCorrectsATypo(t *testing.T) {
	service, _, _, accounts, _ := newTelegramWriteScenario()
	ctx := context.Background()
	linkChat(ctx, t, service)

	service.HandleUpdate(ctx, messageUpdate(12, testChatID, "/saldo Dompet Harian 725000"))
	service.HandleUpdate(ctx, messageUpdate(13, testChatID, "/saldo Dompet Harian 72500"))

	if got := accounts.snapshots[walletAccountID].ActualBalance; !got.Equal(decimalOf("72500")) {
		t.Fatalf("stored balance = %s, want 72500 after the correction", got)
	}
}

func TestSaldoWithoutArgumentStillLists(t *testing.T) {
	service, client, _, accounts, _ := newTelegramWriteScenario()
	ctx := context.Background()
	linkChat(ctx, t, service)

	before := len(accounts.snapshots)
	client.sent = nil
	service.HandleUpdate(ctx, messageUpdate(14, testChatID, "/saldo"))

	if len(accounts.snapshots) != before {
		t.Fatal("a bare /saldo wrote something")
	}
	if len(client.sent) != 1 || !strings.Contains(client.sent[0], "SALDO PER AKUN") {
		t.Fatalf("reply = %+v, want the balance list", client.sent)
	}
}

func TestCatatCommandRecordsAnExpense(t *testing.T) {
	service, client, _, _, transactions := newTelegramWriteScenario()
	ctx := context.Background()
	linkChat(ctx, t, service)

	client.sent = nil
	service.HandleUpdate(ctx, messageUpdate(15, testChatID, "/catat Dompet Harian 25rb kopi pagi"))

	var stored *model.Transaction
	for _, transaction := range transactions.transactions {
		stored = transaction
	}
	if stored == nil {
		t.Fatal("no transaction was stored")
	}
	if stored.TransactionType != model.TransactionTypeExpense {
		t.Fatalf("type = %s, want EXPENSE", stored.TransactionType)
	}
	if !stored.Amount.Equal(decimalOf("25000")) {
		t.Fatalf("amount = %s, want 25000", stored.Amount)
	}
	if stored.Description != "kopi pagi" {
		t.Fatalf("description = %q, want kopi pagi", stored.Description)
	}
	if stored.AccountID == nil || *stored.AccountID != walletAccountID {
		t.Fatalf("account = %v, want the wallet", stored.AccountID)
	}
	// A category is mandatory in the database, so one has to be resolved without asking.
	if stored.CategoryID == nil {
		t.Fatal("no category was attached")
	}

	if len(client.sent) != 1 || !strings.Contains(client.sent[0], "id ") {
		t.Fatalf("reply = %+v, want a confirmation carrying the id for /hapus", client.sent)
	}
}

func TestCatatCommandRejectsIncompleteInput(t *testing.T) {
	service, client, _, _, transactions := newTelegramWriteScenario()
	ctx := context.Background()
	linkChat(ctx, t, service)

	for _, command := range []string{"/catat", "/catat Dompet Harian", "/catat Dompet Harian kopi", "/catat Dompet Harian 25000"} {
		client.sent = nil
		before := len(transactions.transactions)
		service.HandleUpdate(ctx, messageUpdate(16, testChatID, command))

		if len(transactions.transactions) != before {
			t.Fatalf("%q wrote a transaction although it is incomplete", command)
		}
		if len(client.sent) != 1 {
			t.Fatalf("%q produced %d replies, want 1 hint", command, len(client.sent))
		}
	}
}

func TestTopUpCommandRecordsATransfer(t *testing.T) {
	service, client, _, _, transactions := newTelegramWriteScenario()
	ctx := context.Background()
	linkChat(ctx, t, service)

	client.sent = nil
	// Bank Utama is the only bank account, so naming the source is optional.
	service.HandleUpdate(ctx, messageUpdate(17, testChatID, "/topup Dompet Harian 600.000"))

	var stored *model.Transaction
	for _, transaction := range transactions.transactions {
		stored = transaction
	}
	if stored == nil {
		t.Fatal("no transfer was stored")
	}
	if stored.TransactionType != model.TransactionTypeTransfer {
		t.Fatalf("type = %s, want TRANSFER", stored.TransactionType)
	}
	if stored.CategoryID != nil {
		t.Fatal("a transfer must carry no category")
	}
	if stored.AccountID == nil || *stored.AccountID != bankAccountID {
		t.Fatalf("source = %v, want the bank account", stored.AccountID)
	}
	if stored.ToAccountID == nil || *stored.ToAccountID != walletAccountID {
		t.Fatalf("destination = %v, want the wallet", stored.ToAccountID)
	}
	if !stored.Amount.Equal(decimalOf("600000")) {
		t.Fatalf("amount = %s, want 600000", stored.Amount)
	}
}

func TestTopUpCommandHonoursAnExplicitSource(t *testing.T) {
	service, _, _, accounts, transactions := newTelegramWriteScenario()
	ctx := context.Background()
	linkChat(ctx, t, service)

	second := accounts.seed(ownerID, "BCA", model.AccountTypeBank, true)

	service.HandleUpdate(ctx, messageUpdate(18, testChatID, "/topup Dompet Harian 500000 dari BCA"))

	var stored *model.Transaction
	for _, transaction := range transactions.transactions {
		stored = transaction
	}
	if stored == nil || stored.AccountID == nil || *stored.AccountID != second.ID {
		t.Fatalf("source = %v, want BCA (%d)", stored, second.ID)
	}
}

// With more than one bank account the bot must ask rather than pick one, because guessing
// would put the money in the wrong place silently.
func TestTopUpRefusesToGuessBetweenSeveralBanks(t *testing.T) {
	service, client, _, accounts, transactions := newTelegramWriteScenario()
	ctx := context.Background()
	linkChat(ctx, t, service)

	accounts.seed(ownerID, "BCA", model.AccountTypeBank, true)

	client.sent = nil
	before := len(transactions.transactions)
	service.HandleUpdate(ctx, messageUpdate(19, testChatID, "/topup Dompet Harian 500000"))

	if len(transactions.transactions) != before {
		t.Fatal("a transfer was written although the source was ambiguous")
	}
	if len(client.sent) != 1 || !strings.Contains(client.sent[0], "dari") {
		t.Fatalf("reply = %+v, want a request to name the source", client.sent)
	}
}

func TestHapusCommandDeletesAndRefusesForeignRows(t *testing.T) {
	service, client, _, _, transactions := newTelegramWriteScenario()
	ctx := context.Background()
	linkChat(ctx, t, service)

	service.HandleUpdate(ctx, messageUpdate(20, testChatID, "/catat Dompet Harian 25000 kopi"))
	var id int64
	for storedID := range transactions.transactions {
		id = storedID
	}

	client.sent = nil
	service.HandleUpdate(ctx, messageUpdate(21, testChatID, "/hapus "+strconv.FormatInt(id, 10)))
	if _, still := transactions.transactions[id]; still {
		t.Fatal("the transaction was not deleted")
	}

	// An id that does not belong to the linked user must read as missing.
	client.sent = nil
	service.HandleUpdate(ctx, messageUpdate(22, testChatID, "/hapus 999999"))
	if len(client.sent) != 1 || !strings.Contains(client.sent[0], "tidak ditemukan") {
		t.Fatalf("reply = %+v, want a not-found notice", client.sent)
	}

	client.sent = nil
	service.HandleUpdate(ctx, messageUpdate(23, testChatID, "/hapus abc"))
	if len(client.sent) != 1 || !strings.Contains(client.sent[0], "id") {
		t.Fatalf("reply = %+v, want a usage hint", client.sent)
	}
}

// Every write command must be refused outright when the chat is not paired.
func TestWriteCommandsRequireAPairedChat(t *testing.T) {
	service, client, _, accounts, transactions := newTelegramWriteScenario()
	ctx := context.Background()

	for _, command := range []string{
		"/saldo Dompet Harian 72500",
		"/catat Dompet Harian 25000 kopi",
		"/topup Dompet Harian 600000",
		"/hapus 1",
	} {
		client.sent = nil
		snapshotsBefore := len(accounts.snapshots)
		transactionsBefore := len(transactions.transactions)

		service.HandleUpdate(ctx, messageUpdate(30, testChatID, command))

		if len(accounts.snapshots) != snapshotsBefore || len(transactions.transactions) != transactionsBefore {
			t.Fatalf("%q wrote data from an unlinked chat", command)
		}
		if len(client.sent) != 1 || !strings.Contains(client.sent[0], "belum terhubung") {
			t.Fatalf("%q replied %+v, want a not-linked notice", command, client.sent)
		}
	}
}
