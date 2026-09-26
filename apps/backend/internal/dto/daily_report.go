package dto

import "github.com/shopspring/decimal"

// AmountLine is a labelled figure: one bullet in the report.
type AmountLine struct {
	Label  string          `json:"label" example:"Cicilan Rumah"`
	Amount decimal.Decimal `json:"amount" swaggertype:"string" example:"1150000.00"`
}

// WalletItem is one recorded spend from a wallet. SubItems break a single recorded amount
// into what it was actually spent on, which is why a cash withdrawal can be listed once and
// still show its parts.
type WalletItem struct {
	Label    string          `json:"label" example:"Tarik Tunai"`
	Amount   decimal.Decimal `json:"amount" swaggertype:"string" example:"100000.00"`
	SubItems []AmountLine    `json:"sub_items"`
}

// WalletSection reconciles an allowance: what was handed over, what was written down, and
// what is actually left.
type WalletSection struct {
	AccountID int64  `json:"account_id" example:"3"`
	Name      string `json:"name" example:"Dompet Harian"`
	// Allotment is the total topped up into this wallet on the report date.
	Allotment decimal.Decimal `json:"allotment" swaggertype:"string" example:"600000.00"`
	Items     []WalletItem    `json:"items"`
	// RecordedTotal sums Items, counting a parent once and never its children.
	RecordedTotal decimal.Decimal `json:"recorded_total" swaggertype:"string" example:"500000.00"`
	// ExpectedRemaining is Allotment - RecordedTotal: what should still be there.
	ExpectedRemaining decimal.Decimal `json:"expected_remaining" swaggertype:"string" example:"100000.00"`
	// HasActualBalance is false when nobody counted the wallet, in which case the report
	// stops at ExpectedRemaining instead of inventing a variance.
	HasActualBalance bool            `json:"has_actual_balance" example:"true"`
	ActualBalance    decimal.Decimal `json:"actual_balance" swaggertype:"string" example:"72500.00"`
	// Variance is ExpectedRemaining - ActualBalance: spending that was never written down.
	Variance decimal.Decimal `json:"variance" swaggertype:"string" example:"27500.00"`
}

// BankSection is a mutation summary for one bank account.
type BankSection struct {
	AccountID int64           `json:"account_id" example:"4"`
	Name      string          `json:"name" example:"Bank Utama"`
	MoneyIn   decimal.Decimal `json:"money_in" swaggertype:"string" example:"915000.00"`
	// Fees are direct charges on the account, such as a transfer fee.
	Fees     decimal.Decimal `json:"fees" swaggertype:"string" example:"2500.00"`
	MoneyOut decimal.Decimal `json:"money_out" swaggertype:"string" example:"900000.00"`
	// Computed is MoneyIn - Fees - MoneyOut: the day's own movement.
	Computed         decimal.Decimal `json:"computed" swaggertype:"string" example:"12500.00"`
	HasActualBalance bool            `json:"has_actual_balance" example:"true"`
	ActualBalance    decimal.Decimal `json:"actual_balance" swaggertype:"string" example:"20000.00"`
	// PreviousBalance is ActualBalance - Computed: what was already sitting there, which is
	// why the actual balance is not treated as a deduction from the wallet.
	PreviousBalance decimal.Decimal `json:"previous_balance" swaggertype:"string" example:"7500.00"`
}

// CreditCardSection nets a bill payment against money taken back off the card.
type CreditCardSection struct {
	AccountID int64           `json:"account_id" example:"5"`
	Name      string          `json:"name" example:"Bayar tagihan"`
	Payments  decimal.Decimal `json:"payments" swaggertype:"string" example:"1750000.00"`
	// TakenBack is money moved off the card into another account, the top-up source.
	TakenBack decimal.Decimal `json:"taken_back" swaggertype:"string" example:"900000.00"`
	Net       decimal.Decimal `json:"net" swaggertype:"string" example:"850000.00"`
}

// TopUpSection is the money handed out on the report date and where it went.
type TopUpSection struct {
	Total       decimal.Decimal `json:"total" swaggertype:"string" example:"900000.00"`
	Allocations []AmountLine    `json:"allocations"`
	// AllocationTotal sums Allocations; when it equals Total the section is balanced.
	AllocationTotal decimal.Decimal `json:"allocation_total" swaggertype:"string" example:"900000.00"`
	Balanced        bool            `json:"balanced" example:"true"`
}

// ReconciliationSection proves the top-up is fully accounted for: set aside, spent,
// unrecorded and still in hand must add back up to what was handed out.
type ReconciliationSection struct {
	TopUpTotal decimal.Decimal `json:"top_up_total" swaggertype:"string" example:"900000.00"`
	Parts      []AmountLine    `json:"parts"`
	PartsTotal decimal.Decimal `json:"parts_total" swaggertype:"string" example:"900000.00"`
	Difference decimal.Decimal `json:"difference" swaggertype:"string" example:"0.00"`
	Balanced   bool            `json:"balanced" example:"true"`
}

// StatusLine is one line of the closing recap. Icon travels with the line so the renderer
// stays a formatter and does not have to know what each figure means.
type StatusLine struct {
	Icon   string          `json:"icon" example:"💰"`
	Label  string          `json:"label" example:"Top-up"`
	Amount decimal.Decimal `json:"amount" swaggertype:"string" example:"900000.00"`
}

// DailyCashFlowReport is the whole report for one day, in the order it is presented.
type DailyCashFlowReport struct {
	// Date is the day the report covers.
	Date string `json:"date" example:"2026-09-25"`
	// PeriodLabel is the month heading, already localised, e.g. "SEPTEMBER 2026".
	PeriodLabel string `json:"period_label" example:"SEPTEMBER 2026"`
	// DateLabel is the short day heading, e.g. "25/09".
	DateLabel string `json:"date_label" example:"25/09"`

	OpeningBalanceLabel string          `json:"opening_balance_label" example:"Saldo Awal Cash Flow"`
	OpeningBalance      decimal.Decimal `json:"opening_balance" swaggertype:"string" example:"8500000.00"`

	CashFlowExpenses []AmountLine    `json:"cash_flow_expenses"`
	CashFlowTotal    decimal.Decimal `json:"cash_flow_total" swaggertype:"string" example:"5000000.00"`

	CreditCards []CreditCardSection `json:"credit_cards"`
	TopUp       TopUpSection        `json:"top_up"`
	Wallets     []WalletSection     `json:"wallets"`
	Banks       []BankSection       `json:"banks"`

	Reconciliation ReconciliationSection `json:"reconciliation"`
	Status         []StatusLine          `json:"status"`
}

// TelegramSendResponse reports what was delivered.
type TelegramSendResponse struct {
	ChatID   int64 `json:"chat_id" example:"123456789"`
	Messages int   `json:"messages" example:"1"`
	// Characters is the rendered length before splitting, useful when a report grows near
	// the 4096-character limit.
	Characters int `json:"characters" example:"2480"`
}
