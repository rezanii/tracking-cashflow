package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/repository"
)

// reportDay is the day the scenario below is recorded on.
var reportDay = time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)

// Account ids used by the scenario, matching the order accounts are seeded in.
const (
	cashFlowAccountID int64 = 1
	cardAccountID     int64 = 2
	bankAccountID     int64 = 3
	walletAccountID   int64 = 4
	savingsAccountID  int64 = 5
)

// newReportScenario builds the worked example end to end: a household day, a card bill with
// part of it taken back, a top-up split between savings and an allowance, a wallet whose
// counted balance is short, and a bank account carrying a previous balance.
func newReportScenario() (DailyReportService, *fakeDailyReportRepository, *fakeAccountRepository) {
	accounts := newFakeAccountRepository()
	cashFlow := accounts.seed(ownerID, "Cash Flow", model.AccountTypeCashFlow, true)
	card := accounts.seed(ownerID, "CC", model.AccountTypeCreditCard, true)
	bank := accounts.seed(ownerID, "Bank Utama", model.AccountTypeBank, true)
	wallet := accounts.seed(ownerID, "Dompet Harian", model.AccountTypeWallet, true)
	savings := accounts.seed(ownerID, "Dana Cadangan", model.AccountTypeSavings, true)

	// The ids the scenario's aggregates are keyed by have to match what was just seeded.
	for id, want := range map[int64]int64{
		cashFlow.ID: cashFlowAccountID,
		card.ID:     cardAccountID,
		bank.ID:     bankAccountID,
		wallet.ID:   walletAccountID,
		savings.ID:  savingsAccountID,
	} {
		if id != want {
			panic("scenario account ids drifted")
		}
	}

	accounts.seedSnapshot(wallet, reportDay, "72500")
	accounts.seedSnapshot(bank, reportDay, "20000")

	daily := newFakeDailyReportRepository()
	daily.openingBalance = decimalOf("8500000")
	daily.cashFlow = []repository.LabelledAmount{
		{Label: "Cicilan Rumah", Amount: decimalOf("1150000")},
		{Label: "Cicilan Motor", Amount: decimalOf("1000000")},
		{Label: "Listrik & Air", Amount: decimalOf("1000000")},
		{Label: "Internet", Amount: decimalOf("1000000")},
		{Label: "Belanja Bulanan", Amount: decimalOf("3000000")},
		{Label: "Tabungan Anak", Amount: decimalOf("1000000")},
	}
	daily.flows = map[int64]repository.AccountFlow{
		cardAccountID: {
			AccountID:    cardAccountID,
			ExpenseTotal: decimalOf("1750000"),
			TransferOut:  decimalOf("900000"),
			IncomeTotal:  decimalOf("0"),
			TransferIn:   decimalOf("0"),
		},
		bankAccountID: {
			AccountID:    bankAccountID,
			IncomeTotal:  decimalOf("15000"),
			TransferIn:   decimalOf("900000"),
			ExpenseTotal: decimalOf("2500"),
			TransferOut:  decimalOf("900000"),
		},
		walletAccountID: {
			AccountID:    walletAccountID,
			TransferIn:   decimalOf("600000"),
			ExpenseTotal: decimalOf("500000"),
			IncomeTotal:  decimalOf("0"),
			TransferOut:  decimalOf("0"),
		},
	}
	daily.transfers = []repository.TransferEdge{
		{ID: 1, FromAccountID: &card.ID, FromAccountName: "CC", FromAccountType: "CREDIT_CARD",
			ToAccountID: bank.ID, ToAccountName: "Bank Utama", ToAccountType: "BANK", Amount: decimalOf("900000")},
		{ID: 2, FromAccountID: &bank.ID, FromAccountName: "Bank Utama", FromAccountType: "BANK",
			ToAccountID: savings.ID, ToAccountName: "Dana Cadangan", ToAccountType: "SAVINGS", Amount: decimalOf("1000000")},
		{ID: 3, FromAccountID: &bank.ID, FromAccountName: "Bank Utama", FromAccountType: "BANK",
			ToAccountID: wallet.ID, ToAccountName: "Dompet Harian", ToAccountType: "WALLET", Amount: decimalOf("600000")},
	}
	daily.expenses[walletAccountID] = []model.Transaction{
		{ID: 10, Amount: decimalOf("150000"), Description: "Listrik"},
		{ID: 11, Amount: decimalOf("100000"), Description: "Tarik Tunai", Children: []model.Transaction{
			{ID: 12, Amount: decimalOf("25000"), Description: "Makan siang"},
			{ID: 13, Amount: decimalOf("30000"), Description: "Bensin"},
			{ID: 14, Amount: decimalOf("10000"), Description: "Parkir"},
			{ID: 15, Amount: decimalOf("10000"), Description: "Jajan anak"},
			{ID: 16, Amount: decimalOf("25000"), Description: "Sisa tunai"},
		}},
		{ID: 17, Amount: decimalOf("29000"), Description: "Jajan", Children: []model.Transaction{
			{ID: 18, Amount: decimalOf("20000"), Description: "Makan siang"},
			{ID: 19, Amount: decimalOf("7500"), Description: "Kopi"},
			{ID: 20, Amount: decimalOf("1500"), Description: "Air mineral"},
		}},
		{ID: 21, Amount: decimalOf("100000"), Description: "Pulsa"},
		{ID: 22, Amount: decimalOf("50000"), Description: "Obat"},
	}

	return NewDailyReportService(daily, accounts), daily, accounts
}

func TestDailyReportReproducesTheWorkedExample(t *testing.T) {
	service, _, _ := newReportScenario()

	report, err := service.Build(context.Background(), ownerID, reportDay)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	t.Run("heading", func(t *testing.T) {
		if report.PeriodLabel != "SEPTEMBER 2026" {
			t.Fatalf("period = %q, want SEPTEMBER 2026", report.PeriodLabel)
		}
		if report.DateLabel != "25/09" {
			t.Fatalf("date label = %q, want 25/09", report.DateLabel)
		}
	})

	t.Run("cash flow", func(t *testing.T) {
		if !report.OpeningBalance.Equal(decimalOf("8500000")) {
			t.Fatalf("opening balance = %s, want 8500000", report.OpeningBalance)
		}
		if len(report.CashFlowExpenses) != 6 {
			t.Fatalf("expense lines = %d, want 6", len(report.CashFlowExpenses))
		}
		// The order is the order the day was recorded in, not alphabetical.
		if report.CashFlowExpenses[0].Label != "Cicilan Rumah" || report.CashFlowExpenses[5].Label != "Tabungan Anak" {
			t.Fatalf("expense order drifted: %+v", report.CashFlowExpenses)
		}
		if !report.CashFlowTotal.Equal(decimalOf("5000000")) {
			t.Fatalf("cash flow total = %s, want 5000000", report.CashFlowTotal)
		}
	})

	t.Run("payment cc nets the money taken back", func(t *testing.T) {
		if len(report.CreditCards) != 1 {
			t.Fatalf("card sections = %d, want 1", len(report.CreditCards))
		}
		card := report.CreditCards[0]
		if !card.Payments.Equal(decimalOf("1750000")) {
			t.Fatalf("payments = %s, want 1750000", card.Payments)
		}
		if !card.TakenBack.Equal(decimalOf("900000")) {
			t.Fatalf("taken back = %s, want 900000", card.TakenBack)
		}
		if !card.Net.Equal(decimalOf("850000")) {
			t.Fatalf("net = %s, want 850000", card.Net)
		}
	})

	t.Run("top-up only counts wallet and savings destinations", func(t *testing.T) {
		// The transfer from the card into the bank moves the money but hands nothing out, so
		// counting it would double the top-up.
		if !report.TopUp.Total.Equal(decimalOf("900000")) {
			t.Fatalf("top-up total = %s, want 900000", report.TopUp.Total)
		}
		if len(report.TopUp.Allocations) != 2 {
			t.Fatalf("allocations = %d, want 2", len(report.TopUp.Allocations))
		}
		if !report.TopUp.Balanced {
			t.Fatalf("allocations %s do not add up to %s", report.TopUp.AllocationTotal, report.TopUp.Total)
		}
	})

	t.Run("wallet variance is the unrecorded spending", func(t *testing.T) {
		if len(report.Wallets) != 1 {
			t.Fatalf("wallet sections = %d, want 1", len(report.Wallets))
		}
		wallet := report.Wallets[0]
		if !wallet.Allotment.Equal(decimalOf("600000")) {
			t.Fatalf("allotment = %s, want 600000", wallet.Allotment)
		}
		// Five recorded items; the withdrawal's own parts must not be added on top.
		if len(wallet.Items) != 5 {
			t.Fatalf("items = %d, want 5", len(wallet.Items))
		}
		if len(wallet.Items[1].SubItems) != 5 {
			t.Fatalf("withdrawal sub-items = %d, want 5", len(wallet.Items[1].SubItems))
		}
		if !wallet.RecordedTotal.Equal(decimalOf("500000")) {
			t.Fatalf("recorded total = %s, want 500000 (children must not be counted twice)", wallet.RecordedTotal)
		}
		if !wallet.ExpectedRemaining.Equal(decimalOf("100000")) {
			t.Fatalf("expected remaining = %s, want 100000", wallet.ExpectedRemaining)
		}
		if !wallet.HasActualBalance || !wallet.ActualBalance.Equal(decimalOf("72500")) {
			t.Fatalf("actual balance = %s (has=%t), want 72500", wallet.ActualBalance, wallet.HasActualBalance)
		}
		if !wallet.Variance.Equal(decimalOf("27500")) {
			t.Fatalf("variance = %s, want 27500", wallet.Variance)
		}
	})

	t.Run("bank separates this day from what was already there", func(t *testing.T) {
		if len(report.Banks) != 1 {
			t.Fatalf("bank sections = %d, want 1", len(report.Banks))
		}
		bank := report.Banks[0]
		if !bank.MoneyIn.Equal(decimalOf("915000")) {
			t.Fatalf("money in = %s, want 915000", bank.MoneyIn)
		}
		if !bank.Fees.Equal(decimalOf("2500")) {
			t.Fatalf("fees = %s, want 2500", bank.Fees)
		}
		if !bank.MoneyOut.Equal(decimalOf("900000")) {
			t.Fatalf("money out = %s, want 900000", bank.MoneyOut)
		}
		if !bank.Computed.Equal(decimalOf("12500")) {
			t.Fatalf("computed = %s, want 12500", bank.Computed)
		}
		if !bank.PreviousBalance.Equal(decimalOf("7500")) {
			t.Fatalf("previous balance = %s, want 7500", bank.PreviousBalance)
		}
	})

	t.Run("reconciliation balances to zero", func(t *testing.T) {
		if !report.Reconciliation.PartsTotal.Equal(decimalOf("900000")) {
			t.Fatalf("parts total = %s, want 900000", report.Reconciliation.PartsTotal)
		}
		if !report.Reconciliation.Difference.IsZero() {
			t.Fatalf("difference = %s, want 0", report.Reconciliation.Difference)
		}
		if !report.Reconciliation.Balanced {
			t.Fatal("reconciliation should balance")
		}
	})

	t.Run("status recaps every figure", func(t *testing.T) {
		if len(report.Status) == 0 {
			t.Fatal("status is empty")
		}
		labels := map[string]bool{}
		for _, line := range report.Status {
			labels[line.Label] = true
		}
		for _, want := range []string{"Top-up", "Dana Cadangan", "Transaksi Dompet Harian tercatat", "Transaksi belum tercatat", "Saldo Aktual Dompet Harian", "Saldo Bank Utama"} {
			if !labels[want] {
				t.Fatalf("status is missing %q: %+v", want, labels)
			}
		}
	})
}

// A wallet nobody counted must not invent a variance: the report stops at what the records
// say, and the reconciliation uses the expected remainder instead.
func TestDailyReportWithoutAnObservedBalanceReportsNoVariance(t *testing.T) {
	service, _, accounts := newReportScenario()
	accounts.snapshots = map[int64]model.AccountBalanceSnapshot{}

	report, err := service.Build(context.Background(), ownerID, reportDay)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	wallet := report.Wallets[0]
	if wallet.HasActualBalance {
		t.Fatal("wallet should not claim an observed balance")
	}
	if !wallet.Variance.IsZero() {
		t.Fatalf("variance = %s, want 0 when nothing was counted", wallet.Variance)
	}
	if !report.Banks[0].PreviousBalance.IsZero() {
		t.Fatalf("bank previous balance = %s, want 0 when nothing was counted", report.Banks[0].PreviousBalance)
	}
	// 1.000.000 to savings + 500.000 recorded + 100.000 expected remainder = 900.000.
	if !report.Reconciliation.Balanced {
		t.Fatalf("reconciliation should still balance: %s vs %s",
			report.Reconciliation.PartsTotal, report.Reconciliation.TopUpTotal)
	}
}

// A day with nothing recorded must still produce a valid report rather than an error or a
// page of zero-value sections.
func TestDailyReportOnAQuietDay(t *testing.T) {
	accounts := newFakeAccountRepository()
	accounts.seed(ownerID, "Cash Flow", model.AccountTypeCashFlow, true)
	service := NewDailyReportService(newFakeDailyReportRepository(), accounts)

	report, err := service.Build(context.Background(), ownerID, reportDay)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if len(report.CashFlowExpenses) != 0 || len(report.Wallets) != 0 || len(report.Banks) != 0 {
		t.Fatalf("a quiet day produced sections: %+v", report)
	}
	if !report.TopUp.Total.IsZero() {
		t.Fatalf("top-up = %s, want 0", report.TopUp.Total)
	}

	// The renderer must still produce something sendable.
	message := RenderDailyReportMarkdown(report)
	if !strings.Contains(message, "SALDO AWAL") {
		t.Fatalf("quiet-day message lost its heading: %q", message)
	}
}

func TestDailyReportPropagatesRepositoryFailure(t *testing.T) {
	for _, failure := range []string{"OpeningBalance", "CashFlowExpenses", "AccountFlows", "Transfers", "AccountExpenses"} {
		t.Run(failure, func(t *testing.T) {
			service, daily, _ := newReportScenario()
			daily.failOn = failure

			if _, err := service.Build(context.Background(), ownerID, reportDay); err == nil {
				t.Fatalf("Build succeeded although %s failed", failure)
			}
		})
	}
}
