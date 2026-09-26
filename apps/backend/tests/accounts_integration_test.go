package tests

import (
	"fmt"
	"net/http"
	"os"
	"testing"
)

// reportDate is the day the worked example below is recorded on. It is fixed rather than
// "today" so the assertions do not depend on when the suite runs.
const reportDate = "2026-09-25"

// createAccount adds an account and returns its id.
func (h *harness) createAccount(name, accountType string) int64 {
	h.t.Helper()

	response, body := h.request(http.MethodPost, "/accounts", map[string]any{
		"name":            name,
		"account_type":    accountType,
		"opening_balance": "0",
	}, true)
	if response.StatusCode != http.StatusCreated {
		h.t.Fatalf("create account status = %d, want 201 (%s)", response.StatusCode, body.Message)
	}

	var created struct {
		ID int64 `json:"id"`
	}
	mustUnmarshal(h.t, body.Data, &created)
	return created.ID
}

// createAccountTransaction posts a transaction tied to accounts, optionally as a detail line
// of another one.
func (h *harness) createAccountTransaction(
	transactionType string,
	categoryID *int64,
	accountID int64,
	toAccountID *int64,
	parentID *int64,
	amount, description, reference string,
) int64 {
	h.t.Helper()

	payload := map[string]any{
		"transaction_date": reportDate,
		"transaction_type": transactionType,
		"amount":           amount,
		"description":      description,
		"reference_number": reference,
		"account_id":       accountID,
	}
	if categoryID != nil {
		payload["category_id"] = *categoryID
	}
	if toAccountID != nil {
		payload["to_account_id"] = *toAccountID
	}
	if parentID != nil {
		payload["parent_id"] = *parentID
	}

	response, body := h.request(http.MethodPost, "/transactions", payload, true)
	if response.StatusCode != http.StatusCreated {
		h.t.Fatalf("create %s status = %d, want 201 (%s)", reference, response.StatusCode, body.Message)
	}

	var created struct {
		ID int64 `json:"id"`
	}
	mustUnmarshal(h.t, body.Data, &created)
	return created.ID
}

func (h *harness) recordBalance(accountID int64, balance string) {
	h.t.Helper()

	response, body := h.request(http.MethodPost, itoaPath("/accounts/%d/balances", accountID), map[string]any{
		"as_of_date":     reportDate,
		"actual_balance": balance,
		"note":           "integration",
	}, true)
	if response.StatusCode != http.StatusCreated {
		h.t.Fatalf("record balance status = %d, want 201 (%s)", response.StatusCode, body.Message)
	}
}

func TestAccountLifecycle(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()
	h.registerAndLogin()

	id := h.createAccount("Dompet Harian", "WALLET")

	t.Run("read back", func(t *testing.T) {
		response, body := h.request(http.MethodGet, itoaPath("/accounts/%d", id), nil, true)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("get status = %d, want 200 (%s)", response.StatusCode, body.Message)
		}
		var account struct {
			Name        string `json:"name"`
			AccountType string `json:"account_type"`
			IsActive    bool   `json:"is_active"`
		}
		mustUnmarshal(t, body.Data, &account)
		if account.Name != "Dompet Harian" || account.AccountType != "WALLET" || !account.IsActive {
			t.Fatalf("unexpected account: %+v", account)
		}
	})

	t.Run("duplicate name is refused", func(t *testing.T) {
		response, _ := h.request(http.MethodPost, "/accounts", map[string]any{
			"name": "Dompet Harian", "account_type": "WALLET", "opening_balance": "0",
		}, true)
		if response.StatusCode != http.StatusConflict {
			t.Fatalf("duplicate status = %d, want 409", response.StatusCode)
		}
	})

	t.Run("unknown type is refused", func(t *testing.T) {
		response, _ := h.request(http.MethodPost, "/accounts", map[string]any{
			"name": "Nonsense", "account_type": "MATTRESS", "opening_balance": "0",
		}, true)
		if response.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("bad type status = %d, want 422", response.StatusCode)
		}
	})

	t.Run("deactivate then delete", func(t *testing.T) {
		response, body := h.request(http.MethodPatch, itoaPath("/accounts/%d/status", id), map[string]any{
			"is_active": false,
		}, true)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status patch = %d, want 200 (%s)", response.StatusCode, body.Message)
		}

		response, body = h.request(http.MethodDelete, itoaPath("/accounts/%d", id), nil, true)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("delete status = %d, want 200 (%s)", response.StatusCode, body.Message)
		}
		response, _ = h.request(http.MethodGet, itoaPath("/accounts/%d", id), nil, true)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("get after delete = %d, want 404", response.StatusCode)
		}
	})
}

// An account in use must not be deletable: the foreign key is NO ACTION, and a silent orphan
// would change every past report.
func TestAccountInUseCannotBeDeleted(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()
	h.registerAndLogin()

	category := h.createCategory("Harian", "EXPENSE")
	accountID := h.createAccount("Dompet", "WALLET")
	h.createAccountTransaction("EXPENSE", &category, accountID, nil, nil, "50000", "Kopi", "IT-USE-1")

	response, body := h.request(http.MethodDelete, itoaPath("/accounts/%d", accountID), nil, true)
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("delete status = %d, want 409 (%s)", response.StatusCode, body.Message)
	}
}

func TestAccountOwnershipIsEnforced(t *testing.T) {
	owner := newHarness(t)
	defer owner.cleanup()
	owner.registerAndLogin()
	accountID := owner.createAccount("Milik Saya", "BANK")

	intruder := newHarness(t)
	defer intruder.cleanup()
	intruder.registerAndLogin()

	// Another user's account must read as absent, not as forbidden: the status code itself
	// should not confirm that the id exists.
	for _, probe := range []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, itoaPath("/accounts/%d", accountID), nil},
		{http.MethodPut, itoaPath("/accounts/%d", accountID), map[string]any{"name": "Dibajak", "account_type": "BANK", "opening_balance": "0"}},
		{http.MethodDelete, itoaPath("/accounts/%d", accountID), nil},
		{http.MethodGet, itoaPath("/accounts/%d/balances", accountID), nil},
	} {
		response, _ := intruder.request(probe.method, probe.path, probe.body, true)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("%s %s = %d, want 404", probe.method, probe.path, response.StatusCode)
		}
	}

	// Attaching someone else's account to a transaction must be refused by validation.
	category := intruder.createCategory("Harian", "EXPENSE")
	response, _ := intruder.request(http.MethodPost, "/transactions", map[string]any{
		"transaction_date": reportDate,
		"transaction_type": "EXPENSE",
		"category_id":      category,
		"amount":           "1000",
		"account_id":       accountID,
	}, true)
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("foreign account status = %d, want 422", response.StatusCode)
	}
}

func TestBalanceSnapshotOverwritesTheSameDay(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()
	h.registerAndLogin()

	accountID := h.createAccount("Bank Utama", "BANK")
	h.recordBalance(accountID, "20000")
	// Re-counting the same day corrects the figure instead of adding a contradictory row.
	h.recordBalance(accountID, "30000")

	response, body := h.request(http.MethodGet, itoaPath("/accounts/%d/balances", accountID), nil, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("list balances status = %d, want 200 (%s)", response.StatusCode, body.Message)
	}

	var snapshots []struct {
		AsOfDate      string `json:"as_of_date"`
		ActualBalance string `json:"actual_balance"`
	}
	mustUnmarshal(t, body.Data, &snapshots)
	if len(snapshots) != 1 {
		t.Fatalf("snapshots = %d, want 1 after re-recording the same day", len(snapshots))
	}
	if snapshots[0].ActualBalance != "30000" {
		t.Fatalf("balance = %s, want 30000", snapshots[0].ActualBalance)
	}
}

// The full worked example over HTTP against SQL Server: the SQL, the service maths and the
// wiring all have to agree for these figures to come out.
func TestDailyCashFlowReportOverHTTP(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()
	h.registerAndLogin()

	cashFlow := h.createAccount("Cash Flow", "CASH_FLOW")
	card := h.createAccount("CC", "CREDIT_CARD")
	bank := h.createAccount("Bank Utama", "BANK")
	wallet := h.createAccount("Dompet Harian", "WALLET")
	savings := h.createAccount("Dana Cadangan", "SAVINGS")

	kpr := h.createCategory("Cicilan Rumah", "EXPENSE")
	living := h.createCategory("Belanja Bulanan", "EXPENSE")
	daily := h.createCategory("Kebutuhan Harian", "EXPENSE")
	billing := h.createCategory("Pembayaran CC", "EXPENSE")
	admin := h.createCategory("Biaya Admin", "EXPENSE")
	incoming := h.createCategory("Dana Masuk", "INCOME")

	// Household cash flow: 1.150.000 + 3.000.000.
	h.createAccountTransaction("EXPENSE", &kpr, cashFlow, nil, nil, "1150000", "Cicilan Rumah", "IT-CF-1")
	h.createAccountTransaction("EXPENSE", &living, cashFlow, nil, nil, "3000000", "Belanja Bulanan", "IT-CF-2")

	// The card bill, and the part taken back into the bank.
	h.createAccountTransaction("EXPENSE", &billing, card, nil, nil, "1750000", "Bayar tagihan", "IT-CC-1")
	h.createAccountTransaction("TRANSFER", nil, card, &bank, nil, "900000", "Ambil kembali", "IT-CC-2")

	// The bank leg: other money in, the fee, and the two allocations out.
	h.createAccountTransaction("INCOME", &incoming, bank, nil, nil, "15000", "Dana masuk lain", "IT-BK-1")
	h.createAccountTransaction("EXPENSE", &admin, bank, nil, nil, "2500", "Admin BI-Fast", "IT-BK-2")
	h.createAccountTransaction("TRANSFER", nil, bank, &savings, nil, "1000000", "Dana Cadangan", "IT-TU-1")
	h.createAccountTransaction("TRANSFER", nil, bank, &wallet, nil, "600000", "Jatah Dompet Harian", "IT-TU-2")

	// The allowance: a withdrawal recorded once, with its parts underneath.
	h.createAccountTransaction("EXPENSE", &daily, wallet, nil, nil, "150000", "Listrik", "IT-LA-1")
	withdrawal := h.createAccountTransaction("EXPENSE", &daily, wallet, nil, nil, "100000", "Tarik Tunai", "IT-LA-2")
	h.createAccountTransaction("EXPENSE", &daily, wallet, nil, &withdrawal, "60000", "Bensin", "IT-LA-2-A")
	h.createAccountTransaction("EXPENSE", &daily, wallet, nil, &withdrawal, "40000", "Jajan anak", "IT-LA-2-B")
	h.createAccountTransaction("EXPENSE", &daily, wallet, nil, nil, "416000", "Pulsa", "IT-LA-3")

	h.recordBalance(wallet, "72500")
	h.recordBalance(bank, "20000")

	response, body := h.request(http.MethodGet, "/reports/daily-cash-flow?date="+reportDate, nil, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("report status = %d, want 200 (%s)", response.StatusCode, body.Message)
	}

	var report struct {
		PeriodLabel      string `json:"period_label"`
		DateLabel        string `json:"date_label"`
		CashFlowExpenses []struct {
			Label  string `json:"label"`
			Amount string `json:"amount"`
		} `json:"cash_flow_expenses"`
		CashFlowTotal string `json:"cash_flow_total"`
		CreditCards   []struct {
			Payments  string `json:"payments"`
			TakenBack string `json:"taken_back"`
			Net       string `json:"net"`
		} `json:"credit_cards"`
		TopUp struct {
			Total       string `json:"total"`
			Allocations []struct {
				Label  string `json:"label"`
				Amount string `json:"amount"`
			} `json:"allocations"`
			Balanced bool `json:"balanced"`
		} `json:"top_up"`
		Wallets []struct {
			Name          string `json:"name"`
			Allotment     string `json:"allotment"`
			RecordedTotal string `json:"recorded_total"`
			Expected      string `json:"expected_remaining"`
			HasActual     bool   `json:"has_actual_balance"`
			Actual        string `json:"actual_balance"`
			Variance      string `json:"variance"`
			Items         []struct {
				Label    string `json:"label"`
				Amount   string `json:"amount"`
				SubItems []struct {
					Label  string `json:"label"`
					Amount string `json:"amount"`
				} `json:"sub_items"`
			} `json:"items"`
		} `json:"wallets"`
		Banks []struct {
			MoneyIn         string `json:"money_in"`
			Fees            string `json:"fees"`
			MoneyOut        string `json:"money_out"`
			Computed        string `json:"computed"`
			Actual          string `json:"actual_balance"`
			PreviousBalance string `json:"previous_balance"`
		} `json:"banks"`
		Reconciliation struct {
			TopUpTotal string `json:"top_up_total"`
			PartsTotal string `json:"parts_total"`
			Difference string `json:"difference"`
			Balanced   bool   `json:"balanced"`
		} `json:"reconciliation"`
	}
	mustUnmarshal(t, body.Data, &report)

	if report.PeriodLabel != "SEPTEMBER 2026" || report.DateLabel != "25/09" {
		t.Fatalf("heading = %q %q", report.PeriodLabel, report.DateLabel)
	}

	t.Run("cash flow groups by category in recorded order", func(t *testing.T) {
		if len(report.CashFlowExpenses) != 2 {
			t.Fatalf("expense lines = %d, want 2: %+v", len(report.CashFlowExpenses), report.CashFlowExpenses)
		}
		if report.CashFlowExpenses[0].Label != "Cicilan Rumah" {
			t.Fatalf("first line = %q, want Cicilan Rumah", report.CashFlowExpenses[0].Label)
		}
		if report.CashFlowTotal != "4150000" {
			t.Fatalf("cash flow total = %s, want 4150000", report.CashFlowTotal)
		}
	})

	t.Run("card payment is netted", func(t *testing.T) {
		if len(report.CreditCards) != 1 {
			t.Fatalf("card sections = %d, want 1", len(report.CreditCards))
		}
		card := report.CreditCards[0]
		if card.Payments != "1750000" || card.TakenBack != "900000" || card.Net != "850000" {
			t.Fatalf("card = %+v, want 1750000 / 900000 / 850000", card)
		}
	})

	t.Run("top-up counts only what was handed out", func(t *testing.T) {
		if report.TopUp.Total != "900000" {
			t.Fatalf("top-up = %s, want 900000 (the card-to-bank move must not count)", report.TopUp.Total)
		}
		if len(report.TopUp.Allocations) != 2 || !report.TopUp.Balanced {
			t.Fatalf("allocations = %+v, balanced = %t", report.TopUp.Allocations, report.TopUp.Balanced)
		}
	})

	t.Run("wallet reconciles against the counted balance", func(t *testing.T) {
		if len(report.Wallets) != 1 {
			t.Fatalf("wallet sections = %d, want 1", len(report.Wallets))
		}
		w := report.Wallets[0]
		if w.Allotment != "600000" {
			t.Fatalf("allotment = %s, want 600000", w.Allotment)
		}
		// Three parents: 502.000 + 100.000 + 416.000. The withdrawal's two parts explain the
		// 100.000 and must not be added to it.
		if len(w.Items) != 3 {
			t.Fatalf("items = %d, want 3: %+v", len(w.Items), w.Items)
		}
		if w.RecordedTotal != "500000" {
			t.Fatalf("recorded = %s, want 500000", w.RecordedTotal)
		}
		if len(w.Items[1].SubItems) != 2 {
			t.Fatalf("withdrawal sub-items = %d, want 2", len(w.Items[1].SubItems))
		}
		if w.Expected != "100000" || !w.HasActual || w.Actual != "72500" || w.Variance != "27500" {
			t.Fatalf("wallet reconciliation = %+v, want 100000 / 72500 / 27500", w)
		}
	})

	t.Run("bank separates the day from the previous balance", func(t *testing.T) {
		if len(report.Banks) != 1 {
			t.Fatalf("bank sections = %d, want 1", len(report.Banks))
		}
		b := report.Banks[0]
		if b.MoneyIn != "915000" || b.Fees != "2500" || b.MoneyOut != "900000" {
			t.Fatalf("bank movement = %+v", b)
		}
		if b.Computed != "12500" || b.PreviousBalance != "7500" {
			t.Fatalf("computed = %s, previous = %s, want 12500 / 7500", b.Computed, b.PreviousBalance)
		}
	})

	t.Run("reconciliation balances", func(t *testing.T) {
		if report.Reconciliation.PartsTotal != "900000" || report.Reconciliation.Difference != "0" {
			t.Fatalf("reconciliation = %+v", report.Reconciliation)
		}
		if !report.Reconciliation.Balanced {
			t.Fatal("reconciliation should balance")
		}
	})
}

// A transfer to itself would create money on both sides of the report, so it is refused.
func TestTransferValidationRejectsImpossibleAccounts(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()
	h.registerAndLogin()

	wallet := h.createAccount("Dompet", "WALLET")
	category := h.createCategory("Harian", "EXPENSE")

	cases := map[string]map[string]any{
		"transfer to itself": {
			"transaction_date": reportDate, "transaction_type": "TRANSFER",
			"amount": "1000", "account_id": wallet, "to_account_id": wallet,
		},
		"destination on an expense": {
			"transaction_date": reportDate, "transaction_type": "EXPENSE", "category_id": category,
			"amount": "1000", "account_id": wallet, "to_account_id": wallet,
		},
		"unknown destination": {
			"transaction_date": reportDate, "transaction_type": "TRANSFER",
			"amount": "1000", "account_id": wallet, "to_account_id": 999999,
		},
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			response, body := h.request(http.MethodPost, "/transactions", payload, true)
			if response.StatusCode != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422 (%s)", response.StatusCode, body.Message)
			}
		})
	}
}

func TestTelegramPairingCodeAndLinkStatus(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()
	h.registerAndLogin()

	t.Run("no chat is linked to a fresh account", func(t *testing.T) {
		response, body := h.request(http.MethodGet, "/telegram/link", nil, true)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", response.StatusCode, body.Message)
		}
		var link struct {
			Linked bool `json:"linked"`
		}
		mustUnmarshal(t, body.Data, &link)
		if link.Linked {
			t.Fatal("a fresh account should have no linked chat")
		}
	})

	t.Run("a pairing code is issued", func(t *testing.T) {
		response, body := h.request(http.MethodPost, "/telegram/pairing-code", nil, true)
		if response.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want 201 (%s)", response.StatusCode, body.Message)
		}
		var code struct {
			Code        string `json:"code"`
			Instruction string `json:"instruction"`
		}
		mustUnmarshal(t, body.Data, &code)
		if len(code.Code) != 8 || code.Instruction == "" {
			t.Fatalf("pairing code = %+v", code)
		}
	})

	t.Run("sending without a linked chat is refused", func(t *testing.T) {
		response, _ := h.request(http.MethodPost, "/telegram/send/daily-report", nil, true)
		if response.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", response.StatusCode)
		}
	})

	t.Run("unlinking a chat that was never linked is a 404", func(t *testing.T) {
		response, _ := h.request(http.MethodDelete, "/telegram/link", nil, true)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", response.StatusCode)
		}
	})
}

// The webhook is publicly reachable, so the secret header is the only thing standing between
// it and a forged update.
func TestTelegramWebhookRequiresTheSecret(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	update := map[string]any{
		"update_id": 1,
		"message": map[string]any{
			"message_id": 1,
			"text":       "/report",
			"chat":       map[string]any{"id": 1, "type": "private"},
		},
	}

	// A wrong or absent secret must never be accepted, whichever mode is configured.
	response, _ := h.request(http.MethodPost, "/telegram/webhook", update, false)
	if response.StatusCode != http.StatusNotFound && response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 404 when the bot is off or 401 when the secret is wrong", response.StatusCode)
	}

	h.request(http.MethodPost, "/telegram/webhook", update, false)
	if response.StatusCode == http.StatusOK {
		t.Fatal("an update without the secret header was accepted")
	}
}

// itoaPath keeps the id formatting in one place rather than at every call site.
func itoaPath(pattern string, id int64) string {
	return fmt.Sprintf(pattern, id)
}

// With TELEGRAM_MODE unset the integration is off, so the webhook must not exist at all: a
// stale secret in the environment should not leave a public endpoint accepting updates.
func TestTelegramWebhookIsClosedWhenTheBotIsOff(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	if mode := os.Getenv("TELEGRAM_MODE"); mode == "webhook" {
		t.Skip("TELEGRAM_MODE=webhook: the endpoint is meant to be open here")
	}

	response, _ := h.request(http.MethodPost, "/telegram/webhook", map[string]any{"update_id": 1}, false)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 while the bot is off", response.StatusCode)
	}
}
