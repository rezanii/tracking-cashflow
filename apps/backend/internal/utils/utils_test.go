package utils

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestBalance(t *testing.T) {
	cases := []struct {
		name    string
		income  string
		expense string
		want    string
	}{
		{name: "positive", income: "15000000", expense: "8500000", want: "6500000"},
		{name: "negative", income: "1000000", expense: "2500000", want: "-1500000"},
		{name: "zero", income: "0", expense: "0", want: "0"},
		// A float64 pipeline would report 0.09999999999999998 here.
		{name: "no floating point drift", income: "0.3", expense: "0.2", want: "0.1"},
		{name: "rounds to two places", income: "10.005", expense: "0", want: "10.01"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			income := mustDecimal(t, testCase.income)
			expense := mustDecimal(t, testCase.expense)
			want := mustDecimal(t, testCase.want)

			got := Balance(income, expense)
			if !got.Equal(want) {
				t.Fatalf("Balance(%s, %s) = %s, want %s", testCase.income, testCase.expense, got, want)
			}
		})
	}
}

func TestBalanceSumsWithoutDrift(t *testing.T) {
	// A hundred additions of 0.01 must land exactly on 1.00.
	total := Zero()
	cent := mustDecimal(t, "0.01")
	for i := 0; i < 100; i++ {
		total = Round(total.Add(cent))
	}
	if want := mustDecimal(t, "1.00"); !total.Equal(want) {
		t.Fatalf("sum of 100 cents = %s, want %s", total, want)
	}
}

func TestNormalizePaging(t *testing.T) {
	cases := []struct {
		name         string
		page         int
		pageSize     int
		wantPage     int
		wantPageSize int
	}{
		{name: "defaults", page: 0, pageSize: 0, wantPage: 1, wantPageSize: 10},
		{name: "negatives", page: -5, pageSize: -2, wantPage: 1, wantPageSize: 10},
		{name: "kept", page: 3, pageSize: 25, wantPage: 3, wantPageSize: 25},
		{name: "capped", page: 1, pageSize: 5000, wantPage: 1, wantPageSize: 100},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			page, pageSize := NormalizePaging(testCase.page, testCase.pageSize)
			if page != testCase.wantPage || pageSize != testCase.wantPageSize {
				t.Fatalf("NormalizePaging(%d, %d) = (%d, %d), want (%d, %d)",
					testCase.page, testCase.pageSize, page, pageSize, testCase.wantPage, testCase.wantPageSize)
			}
		})
	}
}

func TestOffset(t *testing.T) {
	if got := Offset(1, 10); got != 0 {
		t.Fatalf("Offset(1, 10) = %d, want 0", got)
	}
	if got := Offset(4, 25); got != 75 {
		t.Fatalf("Offset(4, 25) = %d, want 75", got)
	}
}

func TestNormalizeSortRejectsUnknownColumn(t *testing.T) {
	allowed := map[string]string{"transaction_date": "transaction_date", "amount": "amount"}

	// An injection attempt must never reach the SQL string.
	column, direction := NormalizeSort("amount; DROP TABLE users", "asc", allowed, "transaction_date")
	if column != "transaction_date" {
		t.Fatalf("column = %q, want the default transaction_date", column)
	}
	if direction != "ASC" {
		t.Fatalf("direction = %q, want ASC", direction)
	}

	column, direction = NormalizeSort("AMOUNT", "whatever", allowed, "transaction_date")
	if column != "amount" {
		t.Fatalf("column = %q, want amount", column)
	}
	if direction != "DESC" {
		t.Fatalf("direction = %q, want DESC as the default", direction)
	}
}

func TestParseDate(t *testing.T) {
	parsed, err := ParseDate("2026-09-25")
	if err != nil {
		t.Fatalf("ParseDate returned error: %v", err)
	}
	if got := FormatDate(parsed); got != "2026-09-25" {
		t.Fatalf("round trip = %q, want 2026-09-25", got)
	}
	if parsed.Location() != time.UTC {
		t.Fatalf("location = %v, want UTC", parsed.Location())
	}

	for _, invalid := range []string{"25/09/2026", "2026-13-01", "", "2026-09-25T00:00:00Z"} {
		if _, err := ParseDate(invalid); err == nil {
			t.Fatalf("ParseDate(%q) should have failed", invalid)
		}
	}
}

func TestResolveRange(t *testing.T) {
	for _, name := range []string{"today", "week", "month", "year"} {
		from, to, ok := ResolveRange(name)
		if !ok {
			t.Fatalf("ResolveRange(%q) reported not ok", name)
		}
		if to.Before(from) {
			t.Fatalf("ResolveRange(%q) returned to %s before from %s", name, to, from)
		}
	}

	if _, _, ok := ResolveRange("decade"); ok {
		t.Fatal("ResolveRange(\"decade\") should report not ok")
	}
}

func TestResolveRangeMonthCoversWholeMonth(t *testing.T) {
	from, to, ok := ResolveRange("month")
	if !ok {
		t.Fatal("ResolveRange(\"month\") reported not ok")
	}
	if from.Day() != 1 {
		t.Fatalf("month range starts on day %d, want 1", from.Day())
	}
	if to.AddDate(0, 0, 1).Day() != 1 {
		t.Fatalf("month range ends on %s, which is not the last day of the month", FormatDate(to))
	}
}

func TestNewPagination(t *testing.T) {
	pagination := NewPagination(2, 10, 125)
	if pagination.TotalPages != 13 {
		t.Fatalf("TotalPages = %d, want 13", pagination.TotalPages)
	}

	empty := NewPagination(1, 10, 0)
	if empty.TotalPages != 0 {
		t.Fatalf("TotalPages for no rows = %d, want 0", empty.TotalPages)
	}
}

func TestTokenManagerRoundTrip(t *testing.T) {
	manager := NewTokenManager("test-secret-value-for-unit-tests", time.Hour)

	token, expiresAt, err := manager.Generate(42, "user@example.com")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if !expiresAt.After(time.Now().UTC()) {
		t.Fatalf("expiresAt %s is not in the future", expiresAt)
	}

	claims, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	userID, err := claims.UserID()
	if err != nil {
		t.Fatalf("UserID returned error: %v", err)
	}
	if userID != 42 {
		t.Fatalf("UserID = %d, want 42", userID)
	}
	if claims.Email != "user@example.com" {
		t.Fatalf("Email = %q, want user@example.com", claims.Email)
	}
}

func TestTokenManagerRejectsForeignSecret(t *testing.T) {
	issuer := NewTokenManager("the-real-secret-value-used-to-sign", time.Hour)
	attacker := NewTokenManager("a-different-secret-value-entirely", time.Hour)

	token, _, err := issuer.Generate(1, "user@example.com")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if _, err := attacker.Parse(token); err == nil {
		t.Fatal("a token signed with another secret must not validate")
	}
}

func TestTokenManagerRejectsExpiredToken(t *testing.T) {
	manager := NewTokenManager("test-secret-value-for-unit-tests", -time.Minute)

	token, _, err := manager.Generate(1, "user@example.com")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if _, err := manager.Parse(token); err == nil {
		t.Fatal("an expired token must not validate")
	}
}

func TestTokenManagerRejectsGarbage(t *testing.T) {
	manager := NewTokenManager("test-secret-value-for-unit-tests", time.Hour)
	for _, token := range []string{"", "not-a-token", "a.b.c"} {
		if _, err := manager.Parse(token); err == nil {
			t.Fatalf("Parse(%q) should have failed", token)
		}
	}
}

func mustDecimal(t *testing.T, value string) decimal.Decimal {
	t.Helper()
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		t.Fatalf("invalid decimal %q: %v", value, err)
	}
	return parsed
}
