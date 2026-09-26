// Package tests holds API level integration tests. They run against a real Postgres,
// driving the router exactly as HTTP clients do.
//
// The suite skips itself when no database is reachable, so "go test ./..." stays green on a
// machine without the stack running. Set INTEGRATION=1 to turn a skip into a failure.
package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/config"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/repository"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/router"
)

type envelope struct {
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Data    json.RawMessage   `json:"data"`
	Errors  map[string]string `json:"errors"`
}

type harness struct {
	t      *testing.T
	server *httptest.Server
	db     *gorm.DB
	token  string
	userID int64
	email  string
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	// The test binary runs inside tests/, so the backend .env lives one level up.
	_ = godotenv.Load("../.env")

	cfg, err := config.Load()
	if err != nil {
		skipOrFail(t, fmt.Sprintf("configuration not available: %v", err))
	}

	db, err := repository.NewDatabase(cfg)
	if err != nil {
		skipOrFail(t, fmt.Sprintf("database not reachable: %v", err))
	}

	server := httptest.NewServer(router.New(cfg, db))

	h := &harness{t: t, server: server, db: db}
	t.Cleanup(func() {
		h.cleanup()
		server.Close()
		if pool, poolErr := db.DB(); poolErr == nil {
			_ = pool.Close()
		}
	})

	return h
}

func skipOrFail(t *testing.T, reason string) {
	t.Helper()
	if os.Getenv("INTEGRATION") == "1" {
		t.Fatalf("INTEGRATION=1 but %s", reason)
	}
	t.Skipf("skipping integration test: %s", reason)
}

// cleanup removes only the rows this run created, so a developer database keeps its data.
func (h *harness) cleanup() {
	if h.userID == 0 {
		return
	}
	h.db.Exec("DELETE FROM transactions WHERE user_id = ?", h.userID)
	h.db.Exec("DELETE FROM categories WHERE user_id = ?", h.userID)
	h.db.Exec("DELETE FROM users WHERE id = ?", h.userID)
}

func (h *harness) request(method, path string, body any, authenticated bool) (*http.Response, envelope) {
	h.t.Helper()

	var payload *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("encode request body: %v", err)
		}
		payload = bytes.NewReader(encoded)
	} else {
		payload = bytes.NewReader(nil)
	}

	request, err := http.NewRequest(method, h.server.URL+"/api/v1"+path, payload)
	if err != nil {
		h.t.Fatalf("build request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if authenticated && h.token != "" {
		request.Header.Set("Authorization", "Bearer "+h.token)
	}

	response, err := h.server.Client().Do(request)
	if err != nil {
		h.t.Fatalf("%s %s failed: %v", method, path, err)
	}

	var decoded envelope
	if strings.HasPrefix(response.Header.Get("Content-Type"), "application/json") {
		if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
			h.t.Fatalf("decode %s %s response: %v", method, path, err)
		}
	}
	_ = response.Body.Close()

	return response, decoded
}

// registerAndLogin creates a throwaway account so parallel runs never collide.
func (h *harness) registerAndLogin() {
	h.t.Helper()

	h.email = fmt.Sprintf("it-%d@example.com", time.Now().UnixNano())
	password := "Integration123"

	response, body := h.request(http.MethodPost, "/auth/register", map[string]string{
		"name":     "Integration User",
		"email":    h.email,
		"password": password,
	}, false)
	if response.StatusCode != http.StatusCreated {
		h.t.Fatalf("register status = %d, want 201 (%s)", response.StatusCode, body.Message)
	}

	var created struct {
		ID int64 `json:"id"`
	}
	mustUnmarshal(h.t, body.Data, &created)
	h.userID = created.ID

	response, body = h.request(http.MethodPost, "/auth/login", map[string]string{
		"email":    h.email,
		"password": password,
	}, false)
	if response.StatusCode != http.StatusOK {
		h.t.Fatalf("login status = %d, want 200 (%s)", response.StatusCode, body.Message)
	}

	var login struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	mustUnmarshal(h.t, body.Data, &login)
	if login.AccessToken == "" {
		h.t.Fatal("login returned an empty access token")
	}
	if login.TokenType != "Bearer" {
		h.t.Fatalf("token type = %q, want Bearer", login.TokenType)
	}
	h.token = login.AccessToken
}

func (h *harness) createCategory(name, categoryType string) int64 {
	h.t.Helper()

	response, body := h.request(http.MethodPost, "/categories", map[string]string{
		"name": name,
		"type": categoryType,
	}, true)
	if response.StatusCode != http.StatusCreated {
		h.t.Fatalf("create category status = %d, want 201 (%s)", response.StatusCode, body.Message)
	}

	var created struct {
		ID int64 `json:"id"`
	}
	mustUnmarshal(h.t, body.Data, &created)
	return created.ID
}

func (h *harness) createTransaction(date, transactionType string, categoryID *int64, amount, reference string) int64 {
	h.t.Helper()

	payload := map[string]any{
		"transaction_date": date,
		"transaction_type": transactionType,
		"amount":           amount,
		"description":      "integration " + reference,
		"reference_number": reference,
	}
	if categoryID != nil {
		payload["category_id"] = *categoryID
	}

	response, body := h.request(http.MethodPost, "/transactions", payload, true)
	if response.StatusCode != http.StatusCreated {
		h.t.Fatalf("create transaction status = %d, want 201 (%s)", response.StatusCode, body.Message)
	}

	var created struct {
		ID int64 `json:"id"`
	}
	mustUnmarshal(h.t, body.Data, &created)
	return created.ID
}

func mustUnmarshal(t *testing.T, raw json.RawMessage, target any) {
	t.Helper()
	if len(raw) == 0 {
		t.Fatal("response data is empty")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("decode response data: %v", err)
	}
}

func TestAuthFlow(t *testing.T) {
	h := newHarness(t)
	h.registerAndLogin()

	t.Run("me returns the caller", func(t *testing.T) {
		response, body := h.request(http.MethodGet, "/auth/me", nil, true)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", response.StatusCode)
		}
		var user struct {
			ID    int64  `json:"id"`
			Email string `json:"email"`
		}
		mustUnmarshal(t, body.Data, &user)
		if user.ID != h.userID || user.Email != h.email {
			t.Fatalf("me returned %+v, want id %d and %s", user, h.userID, h.email)
		}
	})

	t.Run("duplicate email is a conflict", func(t *testing.T) {
		response, _ := h.request(http.MethodPost, "/auth/register", map[string]string{
			"name":     "Someone Else",
			"email":    h.email,
			"password": "Integration123",
		}, false)
		if response.StatusCode != http.StatusConflict {
			t.Fatalf("status = %d, want 409", response.StatusCode)
		}
	})

	t.Run("wrong password is unauthorized", func(t *testing.T) {
		response, _ := h.request(http.MethodPost, "/auth/login", map[string]string{
			"email":    h.email,
			"password": "WrongPassword1",
		}, false)
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", response.StatusCode)
		}
	})

	t.Run("weak password is rejected", func(t *testing.T) {
		response, body := h.request(http.MethodPost, "/auth/register", map[string]string{
			"name":     "Weak",
			"email":    fmt.Sprintf("weak-%d@example.com", time.Now().UnixNano()),
			"password": "short",
		}, false)
		if response.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", response.StatusCode)
		}
		if _, present := body.Errors["password"]; !present {
			t.Fatalf("errors = %v, want an entry for password", body.Errors)
		}
	})

	t.Run("logout succeeds", func(t *testing.T) {
		response, _ := h.request(http.MethodPost, "/auth/logout", nil, true)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", response.StatusCode)
		}
	})
}

func TestProtectedRoutesRejectBadCredentials(t *testing.T) {
	h := newHarness(t)
	h.registerAndLogin()

	paths := []string{"/auth/me", "/transactions", "/categories", "/dashboard/summary"}

	t.Run("without a token", func(t *testing.T) {
		for _, path := range paths {
			response, _ := h.request(http.MethodGet, path, nil, false)
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("GET %s without a token = %d, want 401", path, response.StatusCode)
			}
		}
	})

	t.Run("with an invalid token", func(t *testing.T) {
		original := h.token
		h.token = "not.a.valid.token"
		defer func() { h.token = original }()

		for _, path := range paths {
			response, _ := h.request(http.MethodGet, path, nil, true)
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("GET %s with a bad token = %d, want 401", path, response.StatusCode)
			}
		}
	})
}

func TestTransactionCRUD(t *testing.T) {
	h := newHarness(t)
	h.registerAndLogin()

	expenseID := h.createCategory("IT Food", "EXPENSE")
	incomeID := h.createCategory("IT Salary", "INCOME")

	var transactionID int64

	t.Run("create", func(t *testing.T) {
		transactionID = h.createTransaction("2026-09-10", "EXPENSE", &expenseID, "125000.50", "IT-CREATE")
		if transactionID == 0 {
			t.Fatal("created transaction has no id")
		}
	})

	t.Run("get", func(t *testing.T) {
		response, body := h.request(http.MethodGet, fmt.Sprintf("/transactions/%d", transactionID), nil, true)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", response.StatusCode)
		}
		var transaction struct {
			Amount       string `json:"amount"`
			CategoryName string `json:"category_name"`
			Date         string `json:"transaction_date"`
		}
		mustUnmarshal(t, body.Data, &transaction)
		// The exact decimal must survive the round trip through Postgres and JSON.
		if transaction.Amount != "125000.5" && transaction.Amount != "125000.50" {
			t.Fatalf("amount = %q, want 125000.50", transaction.Amount)
		}
		if transaction.CategoryName != "IT Food" {
			t.Fatalf("category name = %q, want IT Food", transaction.CategoryName)
		}
		if transaction.Date != "2026-09-10" {
			t.Fatalf("date = %q, want 2026-09-10", transaction.Date)
		}
	})

	t.Run("update", func(t *testing.T) {
		response, body := h.request(http.MethodPut, fmt.Sprintf("/transactions/%d", transactionID), map[string]any{
			"transaction_date": "2026-09-11",
			"transaction_type": "INCOME",
			"category_id":      incomeID,
			"amount":           "250000",
			"description":      "updated",
			"reference_number": "IT-UPDATE",
		}, true)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", response.StatusCode, body.Message)
		}
		var updated struct {
			Type         string `json:"transaction_type"`
			CategoryName string `json:"category_name"`
		}
		mustUnmarshal(t, body.Data, &updated)
		if updated.Type != "INCOME" || updated.CategoryName != "IT Salary" {
			t.Fatalf("updated = %+v, want INCOME and IT Salary", updated)
		}
	})

	t.Run("category type mismatch is rejected", func(t *testing.T) {
		response, body := h.request(http.MethodPost, "/transactions", map[string]any{
			"transaction_date": "2026-09-12",
			"transaction_type": "EXPENSE",
			"category_id":      incomeID,
			"amount":           "1000",
		}, true)
		if response.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", response.StatusCode)
		}
		if _, present := body.Errors["category_id"]; !present {
			t.Fatalf("errors = %v, want an entry for category_id", body.Errors)
		}
	})

	t.Run("transfer stores no category", func(t *testing.T) {
		response, body := h.request(http.MethodPost, "/transactions", map[string]any{
			"transaction_date": "2026-09-13",
			"transaction_type": "TRANSFER",
			"category_id":      expenseID,
			"amount":           "500000",
			"reference_number": "IT-TRANSFER",
		}, true)
		if response.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want 201 (%s)", response.StatusCode, body.Message)
		}
		var created struct {
			CategoryID *int64 `json:"category_id"`
		}
		mustUnmarshal(t, body.Data, &created)
		if created.CategoryID != nil {
			t.Fatalf("category id = %d, want null for a transfer", *created.CategoryID)
		}
	})

	t.Run("delete", func(t *testing.T) {
		response, _ := h.request(http.MethodDelete, fmt.Sprintf("/transactions/%d", transactionID), nil, true)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", response.StatusCode)
		}

		response, _ = h.request(http.MethodGet, fmt.Sprintf("/transactions/%d", transactionID), nil, true)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("status after delete = %d, want 404", response.StatusCode)
		}
	})
}

func TestTransactionOwnershipIsEnforced(t *testing.T) {
	owner := newHarness(t)
	owner.registerAndLogin()
	categoryID := owner.createCategory("IT Owned", "EXPENSE")
	transactionID := owner.createTransaction("2026-09-14", "EXPENSE", &categoryID, "75000", "IT-OWN")

	intruder := newHarness(t)
	intruder.registerAndLogin()

	path := fmt.Sprintf("/transactions/%d", transactionID)

	// Another account must not be able to read, change or remove the row, and the answer is
	// "not found" rather than "forbidden" so ids cannot be probed.
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		response, _ := intruder.request(method, path, nil, true)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("%s as another user = %d, want 404", method, response.StatusCode)
		}
	}

	response, _ := intruder.request(http.MethodPut, path, map[string]any{
		"transaction_date": "2026-09-14",
		"transaction_type": "TRANSFER",
		"amount":           "1",
	}, true)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("PUT as another user = %d, want 404", response.StatusCode)
	}

	// The owner still sees the untouched row.
	response, body := owner.request(http.MethodGet, path, nil, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("owner lost access: status %d (%s)", response.StatusCode, body.Message)
	}
}

func TestTransactionListFilteringAndPaging(t *testing.T) {
	h := newHarness(t)
	h.registerAndLogin()

	expenseID := h.createCategory("IT Transport", "EXPENSE")
	incomeID := h.createCategory("IT Bonus", "INCOME")

	h.createTransaction("2026-08-20", "EXPENSE", &expenseID, "50000", "IT-AUG")
	h.createTransaction("2026-09-02", "EXPENSE", &expenseID, "60000", "IT-SEP-1")
	h.createTransaction("2026-09-03", "EXPENSE", &expenseID, "70000", "IT-SEP-2")
	h.createTransaction("2026-09-04", "INCOME", &incomeID, "900000", "IT-SEP-3")

	readPage := func(t *testing.T, query string) (int, int64) {
		t.Helper()
		response, body := h.request(http.MethodGet, "/transactions"+query, nil, true)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", response.StatusCode, body.Message)
		}
		var page struct {
			Items      []json.RawMessage `json:"items"`
			Pagination struct {
				TotalItems int64 `json:"total_items"`
			} `json:"pagination"`
		}
		mustUnmarshal(t, body.Data, &page)
		return len(page.Items), page.Pagination.TotalItems
	}

	t.Run("date range", func(t *testing.T) {
		_, total := readPage(t, "?date_from=2026-09-01&date_to=2026-09-30")
		if total != 3 {
			t.Fatalf("total = %d, want 3 rows inside September", total)
		}
	})

	t.Run("type filter", func(t *testing.T) {
		_, total := readPage(t, "?transaction_type=INCOME")
		if total != 1 {
			t.Fatalf("total = %d, want 1 income row", total)
		}
	})

	t.Run("category filter", func(t *testing.T) {
		_, total := readPage(t, fmt.Sprintf("?category_id=%d", incomeID))
		if total != 1 {
			t.Fatalf("total = %d, want 1 row for the income category", total)
		}
	})

	t.Run("search", func(t *testing.T) {
		_, total := readPage(t, "?search=IT-SEP")
		if total != 3 {
			t.Fatalf("total = %d, want 3 rows matching the reference prefix", total)
		}
	})

	t.Run("page size", func(t *testing.T) {
		count, total := readPage(t, "?page=1&page_size=2")
		if count != 2 {
			t.Fatalf("items on page = %d, want 2", count)
		}
		if total != 4 {
			t.Fatalf("total = %d, want 4", total)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		count, total := readPage(t, "?date_from=2030-01-01&date_to=2030-12-31")
		if count != 0 || total != 0 {
			t.Fatalf("expected no rows, got %d of %d", count, total)
		}
	})

	t.Run("invalid date is rejected", func(t *testing.T) {
		response, body := h.request(http.MethodGet, "/transactions?date_from=25-09-2026", nil, true)
		if response.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", response.StatusCode)
		}
		if _, present := body.Errors["date_from"]; !present {
			t.Fatalf("errors = %v, want an entry for date_from", body.Errors)
		}
	})
}

func TestCategoryLifecycle(t *testing.T) {
	h := newHarness(t)
	h.registerAndLogin()

	categoryID := h.createCategory("IT Household", "EXPENSE")

	t.Run("duplicate name and type is a conflict", func(t *testing.T) {
		response, _ := h.request(http.MethodPost, "/categories", map[string]string{
			"name": "IT Household",
			"type": "EXPENSE",
		}, true)
		if response.StatusCode != http.StatusConflict {
			t.Fatalf("status = %d, want 409", response.StatusCode)
		}
	})

	t.Run("deactivate then activate", func(t *testing.T) {
		response, body := h.request(http.MethodPatch, fmt.Sprintf("/categories/%d/status", categoryID), map[string]any{
			"is_active": false,
		}, true)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", response.StatusCode, body.Message)
		}
		var updated struct {
			IsActive bool `json:"is_active"`
		}
		mustUnmarshal(t, body.Data, &updated)
		if updated.IsActive {
			t.Fatal("category should be inactive")
		}

		response, _ = h.request(http.MethodPatch, fmt.Sprintf("/categories/%d/status", categoryID), map[string]any{
			"is_active": true,
		}, true)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("reactivate status = %d, want 200", response.StatusCode)
		}
	})

	t.Run("delete is refused while a transaction uses it", func(t *testing.T) {
		h.createTransaction("2026-09-15", "EXPENSE", &categoryID, "45000", "IT-INUSE")

		response, _ := h.request(http.MethodDelete, fmt.Sprintf("/categories/%d", categoryID), nil, true)
		if response.StatusCode != http.StatusConflict {
			t.Fatalf("status = %d, want 409", response.StatusCode)
		}
	})

	t.Run("unused category can be deleted", func(t *testing.T) {
		unusedID := h.createCategory("IT Unused", "EXPENSE")
		response, _ := h.request(http.MethodDelete, fmt.Sprintf("/categories/%d", unusedID), nil, true)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", response.StatusCode)
		}
	})
}

func TestReportsAndExports(t *testing.T) {
	h := newHarness(t)
	h.registerAndLogin()

	salaryID := h.createCategory("IT Payroll", "INCOME")
	kprID := h.createCategory("IT Mortgage", "EXPENSE")

	h.createTransaction("2026-09-05", "INCOME", &salaryID, "15000000", "IT-R-1")
	h.createTransaction("2026-09-06", "EXPENSE", &kprID, "1150000", "IT-R-2")
	// The transfer must not change any total.
	h.createTransaction("2026-09-07", "TRANSFER", nil, "2000000", "IT-R-3")

	query := "?date_from=2026-09-01&date_to=2026-09-30"

	t.Run("summary excludes transfers", func(t *testing.T) {
		response, body := h.request(http.MethodGet, "/reports/summary"+query, nil, true)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", response.StatusCode, body.Message)
		}
		var summary struct {
			TotalIncome  string `json:"total_income"`
			TotalExpense string `json:"total_expense"`
			NetCashFlow  string `json:"net_cash_flow"`
		}
		mustUnmarshal(t, body.Data, &summary)
		if summary.TotalIncome != "15000000" {
			t.Fatalf("total income = %q, want 15000000", summary.TotalIncome)
		}
		if summary.TotalExpense != "1150000" {
			t.Fatalf("total expense = %q, want 1150000", summary.TotalExpense)
		}
		if summary.NetCashFlow != "13850000" {
			t.Fatalf("net cash flow = %q, want 13850000", summary.NetCashFlow)
		}
	})

	t.Run("cash flow carries a running balance", func(t *testing.T) {
		response, body := h.request(http.MethodGet, "/reports/cash-flow"+query, nil, true)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", response.StatusCode, body.Message)
		}
		var report struct {
			Rows []struct {
				Type    string `json:"transaction_type"`
				Income  string `json:"income"`
				Expense string `json:"expense"`
				Balance string `json:"balance"`
			} `json:"rows"`
		}
		mustUnmarshal(t, body.Data, &report)
		if len(report.Rows) != 3 {
			t.Fatalf("rows = %d, want 3", len(report.Rows))
		}
		if report.Rows[0].Balance != "15000000" {
			t.Fatalf("first balance = %q, want 15000000", report.Rows[0].Balance)
		}
		if report.Rows[1].Balance != "13850000" {
			t.Fatalf("second balance = %q, want 13850000", report.Rows[1].Balance)
		}
		// The transfer leaves the balance where it was.
		if report.Rows[2].Balance != "13850000" {
			t.Fatalf("balance after the transfer = %q, want 13850000", report.Rows[2].Balance)
		}
		if report.Rows[2].Income != "0" || report.Rows[2].Expense != "0" {
			t.Fatalf("transfer row = income %q expense %q, want both 0", report.Rows[2].Income, report.Rows[2].Expense)
		}
	})

	t.Run("dashboard summary", func(t *testing.T) {
		response, body := h.request(http.MethodGet, "/dashboard/summary?range=custom&date_from=2026-09-01&date_to=2026-09-30", nil, true)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", response.StatusCode, body.Message)
		}
		var dashboard struct {
			Balance          string `json:"balance"`
			TransactionCount int64  `json:"transaction_count"`
			Trend            []struct {
				Balance string `json:"balance"`
			} `json:"cash_flow_trend"`
		}
		mustUnmarshal(t, body.Data, &dashboard)
		if dashboard.Balance != "13850000" {
			t.Fatalf("balance = %q, want 13850000", dashboard.Balance)
		}
		// The transfer is counted as a transaction even though it moves no net money.
		if dashboard.TransactionCount != 3 {
			t.Fatalf("transaction count = %d, want 3", dashboard.TransactionCount)
		}
		if len(dashboard.Trend) == 0 {
			t.Fatal("cash flow trend is empty")
		}
	})

	t.Run("expense by category", func(t *testing.T) {
		response, body := h.request(http.MethodGet, "/reports/expense-by-category"+query, nil, true)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", response.StatusCode)
		}
		var report struct {
			Total string `json:"total"`
			Rows  []struct {
				CategoryName string `json:"category_name"`
				Percentage   string `json:"percentage"`
			} `json:"rows"`
		}
		mustUnmarshal(t, body.Data, &report)
		if report.Total != "1150000" {
			t.Fatalf("total = %q, want 1150000", report.Total)
		}
		if len(report.Rows) != 1 || report.Rows[0].CategoryName != "IT Mortgage" {
			t.Fatalf("rows = %+v, want one row for IT Mortgage", report.Rows)
		}
		if report.Rows[0].Percentage != "100" {
			t.Fatalf("percentage = %q, want 100", report.Rows[0].Percentage)
		}
	})

	t.Run("monthly", func(t *testing.T) {
		response, body := h.request(http.MethodGet, "/reports/monthly?date_from=2026-01-01&date_to=2026-12-31", nil, true)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", response.StatusCode)
		}
		var report struct {
			Rows []struct {
				Month string `json:"month"`
			} `json:"rows"`
		}
		mustUnmarshal(t, body.Data, &report)
		if len(report.Rows) != 1 || report.Rows[0].Month != "2026-09" {
			t.Fatalf("rows = %+v, want a single 2026-09 row", report.Rows)
		}
	})

	t.Run("missing date range is rejected", func(t *testing.T) {
		response, body := h.request(http.MethodGet, "/reports/cash-flow", nil, true)
		if response.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", response.StatusCode)
		}
		if _, present := body.Errors["date_from"]; !present {
			t.Fatalf("errors = %v, want an entry for date_from", body.Errors)
		}
	})

	t.Run("excel export is a real workbook", func(t *testing.T) {
		content, filename, contentType := h.download(t, "/reports/cash-flow/excel"+query)
		if !bytes.HasPrefix(content, []byte("PK")) {
			t.Fatal("content is not a zip archive, so it cannot be an xlsx file")
		}
		if filename != "cash-flow-2026-09-01-2026-09-30.xlsx" {
			t.Fatalf("filename = %q", filename)
		}
		if !strings.Contains(contentType, "spreadsheetml") {
			t.Fatalf("content type = %q", contentType)
		}
	})

	t.Run("pdf export is a real document", func(t *testing.T) {
		content, filename, contentType := h.download(t, "/reports/cash-flow/pdf"+query)
		if !bytes.HasPrefix(content, []byte("%PDF-")) {
			t.Fatal("content does not start with the PDF header")
		}
		if filename != "cash-flow-2026-09-01-2026-09-30.pdf" {
			t.Fatalf("filename = %q", filename)
		}
		if contentType != "application/pdf" {
			t.Fatalf("content type = %q", contentType)
		}
	})
}

// download fetches a binary export and returns the bytes with the headers a browser needs.
func (h *harness) download(t *testing.T, path string) ([]byte, string, string) {
	t.Helper()

	request, err := http.NewRequest(http.MethodGet, h.server.URL+"/api/v1"+path, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+h.token)

	response, err := h.server.Client().Do(request)
	if err != nil {
		t.Fatalf("GET %s failed: %v", path, err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", path, response.StatusCode)
	}

	buffer := &bytes.Buffer{}
	if _, err := buffer.ReadFrom(response.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}

	disposition := response.Header.Get("Content-Disposition")
	filename := ""
	if start := strings.Index(disposition, `filename="`); start >= 0 {
		rest := disposition[start+len(`filename="`):]
		if end := strings.Index(rest, `"`); end >= 0 {
			filename = rest[:end]
		}
	}

	return buffer.Bytes(), filename, response.Header.Get("Content-Type")
}
