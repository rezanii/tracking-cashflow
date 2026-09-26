package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/model"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/repository"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

// indonesianMonths names the period heading. The report is read in Indonesian, so the month
// is not taken from time.Month().String().
var indonesianMonths = [...]string{
	"JANUARI", "FEBRUARI", "MARET", "APRIL", "MEI", "JUNI",
	"JULI", "AGUSTUS", "SEPTEMBER", "OKTOBER", "NOVEMBER", "DESEMBER",
}

type DailyReportService interface {
	// Build assembles the whole day: cash flow, card payments, top-up, wallets, banks and
	// the reconciliation that ties them together.
	Build(ctx context.Context, userID int64, date time.Time) (dto.DailyCashFlowReport, error)
}

type dailyReportService struct {
	daily    repository.DailyReportRepository
	accounts repository.AccountRepository
}

func NewDailyReportService(daily repository.DailyReportRepository, accounts repository.AccountRepository) DailyReportService {
	return &dailyReportService{daily: daily, accounts: accounts}
}

func (s *dailyReportService) Build(ctx context.Context, userID int64, date time.Time) (dto.DailyCashFlowReport, error) {
	accounts, err := s.accounts.ListByType(ctx, userID, model.AccountTypes()...)
	if err != nil {
		return dto.DailyCashFlowReport{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to load accounts", err)
	}

	flows, err := s.daily.AccountFlows(ctx, userID, date)
	if err != nil {
		return dto.DailyCashFlowReport{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to load account movement", err)
	}

	transfers, err := s.daily.Transfers(ctx, userID, date)
	if err != nil {
		return dto.DailyCashFlowReport{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to load transfers", err)
	}

	snapshots, err := s.accounts.LatestSnapshots(ctx, userID, date)
	if err != nil {
		return dto.DailyCashFlowReport{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to load observed balances", err)
	}

	expenses, err := s.daily.CashFlowExpenses(ctx, userID, date)
	if err != nil {
		return dto.DailyCashFlowReport{}, utils.WrapDomainError(utils.ErrNotFound, "Failed to load expenses", err)
	}

	opening, err := s.openingBalance(ctx, userID, date)
	if err != nil {
		return dto.DailyCashFlowReport{}, err
	}

	report := dto.DailyCashFlowReport{
		Date:                utils.FormatDate(date),
		PeriodLabel:         fmt.Sprintf("%s %d", indonesianMonths[int(date.Month())-1], date.Year()),
		DateLabel:           date.Format("02/01"),
		OpeningBalanceLabel: "Saldo Awal Cash Flow",
		OpeningBalance:      opening,
	}

	report.CashFlowExpenses, report.CashFlowTotal = cashFlowLines(expenses)
	report.CreditCards = creditCardSections(accounts, flows)
	report.TopUp = topUpSection(transfers)

	report.Wallets, err = s.walletSections(ctx, userID, date, accounts, flows, snapshots)
	if err != nil {
		return dto.DailyCashFlowReport{}, err
	}

	report.Banks = bankSections(accounts, flows, snapshots)
	report.Reconciliation = reconciliation(report.TopUp, report.Wallets, transfers)
	report.Status = statusLines(report)

	return report, nil
}

// openingBalance is what the cash flow starts the day with: the accounts' configured opening
// balances plus every cash flow movement recorded before the report date.
func (s *dailyReportService) openingBalance(ctx context.Context, userID int64, date time.Time) (decimal.Decimal, error) {
	configured, err := s.accounts.SumOpeningBalance(ctx, userID, model.AccountTypeCashFlow)
	if err != nil {
		return decimal.Zero, utils.WrapDomainError(utils.ErrNotFound, "Failed to load opening balances", err)
	}
	movement, err := s.daily.OpeningBalance(ctx, userID, date)
	if err != nil {
		return decimal.Zero, utils.WrapDomainError(utils.ErrNotFound, "Failed to load prior movement", err)
	}
	return utils.Round(configured.Add(movement)), nil
}

func cashFlowLines(rows []repository.LabelledAmount) ([]dto.AmountLine, decimal.Decimal) {
	lines := make([]dto.AmountLine, 0, len(rows))
	total := decimal.Zero
	for _, row := range rows {
		amount := utils.Round(row.Amount)
		lines = append(lines, dto.AmountLine{Label: row.Label, Amount: amount})
		total = total.Add(amount)
	}
	return lines, utils.Round(total)
}

// creditCardSections nets what was paid to the card against what was taken back off it. The
// money taken back is what funds the top-up, so it is not a second expense.
func creditCardSections(accounts []model.Account, flows map[int64]repository.AccountFlow) []dto.CreditCardSection {
	var sections []dto.CreditCardSection
	for _, account := range accounts {
		if account.AccountType != model.AccountTypeCreditCard {
			continue
		}
		flow := flows[account.ID]
		payments := utils.Round(flow.ExpenseTotal)
		takenBack := utils.Round(flow.TransferOut)
		if payments.IsZero() && takenBack.IsZero() {
			continue
		}
		sections = append(sections, dto.CreditCardSection{
			AccountID: account.ID,
			Name:      account.Name,
			Payments:  payments,
			TakenBack: takenBack,
			Net:       utils.Round(payments.Sub(takenBack)),
		})
	}
	return sections
}

// topUpSection sums the transfers that handed money out and lists where each part went.
// Destinations that are spent (wallets) and set aside (savings) both count as allocations.
func topUpSection(transfers []repository.TransferEdge) dto.TopUpSection {
	section := dto.TopUpSection{
		Total:           decimal.Zero,
		AllocationTotal: decimal.Zero,
		Allocations:     []dto.AmountLine{},
	}

	// index keeps the first-seen order so the allocation list reads in the order it happened
	// rather than in map order.
	index := map[string]int{}
	for _, transfer := range transfers {
		destination := model.AccountType(transfer.ToAccountType)
		if destination != model.AccountTypeWallet && destination != model.AccountTypeSavings {
			continue
		}

		amount := utils.Round(transfer.Amount)
		section.Total = section.Total.Add(amount)

		if position, seen := index[transfer.ToAccountName]; seen {
			section.Allocations[position].Amount = utils.Round(section.Allocations[position].Amount.Add(amount))
			continue
		}
		index[transfer.ToAccountName] = len(section.Allocations)
		section.Allocations = append(section.Allocations, dto.AmountLine{
			Label:  transfer.ToAccountName,
			Amount: amount,
		})
	}

	for _, allocation := range section.Allocations {
		section.AllocationTotal = section.AllocationTotal.Add(allocation.Amount)
	}
	section.Total = utils.Round(section.Total)
	section.AllocationTotal = utils.Round(section.AllocationTotal)
	section.Balanced = section.Total.Equal(section.AllocationTotal)
	return section
}

func (s *dailyReportService) walletSections(
	ctx context.Context,
	userID int64,
	date time.Time,
	accounts []model.Account,
	flows map[int64]repository.AccountFlow,
	snapshots map[int64]model.AccountBalanceSnapshot,
) ([]dto.WalletSection, error) {
	var sections []dto.WalletSection
	for _, account := range accounts {
		if account.AccountType != model.AccountTypeWallet {
			continue
		}

		transactions, err := s.daily.AccountExpenses(ctx, userID, account.ID, date)
		if err != nil {
			return nil, utils.WrapDomainError(utils.ErrNotFound, "Failed to load wallet transactions", err)
		}

		flow := flows[account.ID]
		allotment := utils.Round(flow.TransferIn)
		if allotment.IsZero() && len(transactions) == 0 {
			continue
		}

		items := make([]dto.WalletItem, 0, len(transactions))
		recorded := decimal.Zero
		for _, transaction := range transactions {
			amount := utils.Round(transaction.Amount)
			// Only the parent counts towards the recorded total; its children explain that
			// same money and would double it.
			recorded = recorded.Add(amount)

			subItems := make([]dto.AmountLine, 0, len(transaction.Children))
			for _, child := range transaction.Children {
				subItems = append(subItems, dto.AmountLine{
					Label:  transactionLabel(child),
					Amount: utils.Round(child.Amount),
				})
			}
			items = append(items, dto.WalletItem{
				Label:    transactionLabel(transaction),
				Amount:   amount,
				SubItems: subItems,
			})
		}

		recorded = utils.Round(recorded)
		expected := utils.Round(allotment.Sub(recorded))

		section := dto.WalletSection{
			AccountID:         account.ID,
			Name:              account.Name,
			Allotment:         allotment,
			Items:             items,
			RecordedTotal:     recorded,
			ExpectedRemaining: expected,
			ActualBalance:     decimal.Zero,
			Variance:          decimal.Zero,
		}

		if snapshot, ok := snapshots[account.ID]; ok {
			section.HasActualBalance = true
			section.ActualBalance = utils.Round(snapshot.ActualBalance)
			section.Variance = utils.Round(expected.Sub(section.ActualBalance))
		}
		sections = append(sections, section)
	}
	return sections, nil
}

func bankSections(
	accounts []model.Account,
	flows map[int64]repository.AccountFlow,
	snapshots map[int64]model.AccountBalanceSnapshot,
) []dto.BankSection {
	var sections []dto.BankSection
	for _, account := range accounts {
		if account.AccountType != model.AccountTypeBank {
			continue
		}

		flow := flows[account.ID]
		moneyIn := utils.Round(flow.IncomeTotal.Add(flow.TransferIn))
		fees := utils.Round(flow.ExpenseTotal)
		moneyOut := utils.Round(flow.TransferOut)
		if moneyIn.IsZero() && fees.IsZero() && moneyOut.IsZero() {
			continue
		}

		computed := utils.Round(moneyIn.Sub(fees).Sub(moneyOut))
		section := dto.BankSection{
			AccountID:       account.ID,
			Name:            account.Name,
			MoneyIn:         moneyIn,
			Fees:            fees,
			MoneyOut:        moneyOut,
			Computed:        computed,
			ActualBalance:   decimal.Zero,
			PreviousBalance: decimal.Zero,
		}

		if snapshot, ok := snapshots[account.ID]; ok {
			section.HasActualBalance = true
			section.ActualBalance = utils.Round(snapshot.ActualBalance)
			// What is left over after this day's own movement was already there, which is
			// why the balance is not read as a deduction from the allowance.
			section.PreviousBalance = utils.Round(section.ActualBalance.Sub(computed))
		}
		sections = append(sections, section)
	}
	return sections
}

// reconciliation proves the top-up is fully accounted for. Money either went into savings,
// was spent and written down, was spent without being written down, or is still in hand.
func reconciliation(topUp dto.TopUpSection, wallets []dto.WalletSection, transfers []repository.TransferEdge) dto.ReconciliationSection {
	section := dto.ReconciliationSection{
		TopUpTotal: topUp.Total,
		Parts:      []dto.AmountLine{},
		PartsTotal: decimal.Zero,
	}

	savings := map[string]decimal.Decimal{}
	var savingsOrder []string
	for _, transfer := range transfers {
		if model.AccountType(transfer.ToAccountType) != model.AccountTypeSavings {
			continue
		}
		if _, seen := savings[transfer.ToAccountName]; !seen {
			savingsOrder = append(savingsOrder, transfer.ToAccountName)
			savings[transfer.ToAccountName] = decimal.Zero
		}
		savings[transfer.ToAccountName] = savings[transfer.ToAccountName].Add(utils.Round(transfer.Amount))
	}
	for _, name := range savingsOrder {
		section.Parts = append(section.Parts, dto.AmountLine{Label: name, Amount: utils.Round(savings[name])})
	}

	for _, wallet := range wallets {
		section.Parts = append(section.Parts, dto.AmountLine{
			Label:  "Transaksi " + wallet.Name,
			Amount: wallet.RecordedTotal,
		})
		if wallet.HasActualBalance {
			section.Parts = append(section.Parts,
				dto.AmountLine{Label: "Transaksi belum tercatat", Amount: wallet.Variance},
				dto.AmountLine{Label: "Saldo aktual " + wallet.Name, Amount: wallet.ActualBalance},
			)
			continue
		}
		section.Parts = append(section.Parts, dto.AmountLine{
			Label:  "Sisa " + wallet.Name,
			Amount: wallet.ExpectedRemaining,
		})
	}

	for _, part := range section.Parts {
		section.PartsTotal = section.PartsTotal.Add(part.Amount)
	}
	section.PartsTotal = utils.Round(section.PartsTotal)
	section.Difference = utils.Round(section.TopUpTotal.Sub(section.PartsTotal))
	section.Balanced = section.Difference.IsZero()
	return section
}

func statusLines(report dto.DailyCashFlowReport) []dto.StatusLine {
	lines := []dto.StatusLine{}

	if !report.CashFlowTotal.IsZero() {
		lines = append(lines, dto.StatusLine{Icon: "💸", Label: "Pengeluaran Cash Flow", Amount: report.CashFlowTotal})
	}
	for _, card := range report.CreditCards {
		lines = append(lines, dto.StatusLine{Icon: "💳", Label: "Net Payment " + card.Name, Amount: card.Net})
	}
	if !report.TopUp.Total.IsZero() {
		lines = append(lines, dto.StatusLine{Icon: "💰", Label: "Top-up", Amount: report.TopUp.Total})
		for _, allocation := range report.TopUp.Allocations {
			lines = append(lines, dto.StatusLine{Icon: "💰", Label: allocation.Label, Amount: allocation.Amount})
		}
	}
	for _, wallet := range report.Wallets {
		lines = append(lines, dto.StatusLine{Icon: "💸", Label: "Transaksi " + wallet.Name + " tercatat", Amount: wallet.RecordedTotal})
		if wallet.HasActualBalance {
			lines = append(lines,
				dto.StatusLine{Icon: "⚠️", Label: "Transaksi belum tercatat", Amount: wallet.Variance},
				dto.StatusLine{Icon: "💰", Label: "Saldo Aktual " + wallet.Name, Amount: wallet.ActualBalance},
			)
			continue
		}
		lines = append(lines, dto.StatusLine{Icon: "💰", Label: "Sisa " + wallet.Name, Amount: wallet.ExpectedRemaining})
	}
	for _, bank := range report.Banks {
		if !bank.HasActualBalance {
			continue
		}
		lines = append(lines, dto.StatusLine{Icon: "🏦", Label: "Saldo " + bank.Name, Amount: bank.ActualBalance})
	}
	return lines
}

// transactionLabel prefers the description, because that is what was actually written down
// for the item; the category is the fallback for a row recorded without one.
func transactionLabel(transaction model.Transaction) string {
	if description := strings.TrimSpace(transaction.Description); description != "" {
		return description
	}
	if name := transaction.CategoryName(); name != "" {
		return name
	}
	return "Lain-lain"
}
