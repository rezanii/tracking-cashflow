package service

import (
	"strings"

	"github.com/shopspring/decimal"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
)

const (
	// telegramMaxMessageLength is the Bot API limit for a text message.
	telegramMaxMessageLength = 4096
	// telegramChunkBudget leaves room so a split never lands exactly on the limit.
	telegramChunkBudget = 3900
	// sectionRule separates the blocks of the report.
	sectionRule = "━━━━━━━━━━━━━━"
)

// markdownV2Escapes lists every character Telegram requires to be escaped in MarkdownV2
// body text. An unescaped "." or "-" inside an amount is enough to make the API reject the
// whole message, so nothing reaches Telegram unescaped.
var markdownV2Escapes = strings.NewReplacer(
	`\`, `\\`,
	"_", `\_`,
	"*", `\*`,
	"[", `\[`,
	"]", `\]`,
	"(", `\(`,
	")", `\)`,
	"~", `\~`,
	"`", "\\`",
	">", `\>`,
	"#", `\#`,
	"+", `\+`,
	"-", `\-`,
	"=", `\=`,
	"|", `\|`,
	"{", `\{`,
	"}", `\}`,
	".", `\.`,
	"!", `\!`,
)

func escapeMarkdownV2(text string) string {
	return markdownV2Escapes.Replace(text)
}

// bold wraps already-escaped text. The asterisks are added after escaping, otherwise they
// would be escaped themselves and print literally.
func bold(text string) string {
	return "*" + escapeMarkdownV2(text) + "*"
}

// rupiah renders an amount the way the report reads it: no space after Rp, dots between
// thousands.
func rupiah(amount decimal.Decimal) string {
	return strings.Replace(formatRupiah(amount), "Rp ", "Rp", 1)
}

func boldRupiah(amount decimal.Decimal) string {
	return bold(rupiah(amount))
}

// reportBuilder collects the message as lines, so section helpers stay readable.
type reportBuilder struct {
	lines []string
}

func (b *reportBuilder) add(line string) { b.lines = append(b.lines, line) }

func (b *reportBuilder) blank() { b.lines = append(b.lines, "") }

// section starts a new block with the rule and an icon heading.
func (b *reportBuilder) section(icon, title string) {
	b.add(sectionRule)
	b.add(icon + " " + bold(title))
	b.blank()
}

func (b *reportBuilder) bullet(label string, amount decimal.Decimal) {
	b.add("• " + escapeMarkdownV2(label) + " : " + boldRupiah(amount))
}

func (b *reportBuilder) subBullet(label string, amount decimal.Decimal) {
	b.add("  ◦ " + escapeMarkdownV2(label) + " : " + escapeMarkdownV2(rupiah(amount)))
}

func (b *reportBuilder) String() string {
	return strings.Join(b.lines, "\n")
}

// RenderDailyReportMarkdown turns the report into MarkdownV2. Every section is skipped when
// it has nothing to say, so a quiet day produces a short message instead of a page of zeros.
func RenderDailyReportMarkdown(report dto.DailyCashFlowReport) string {
	b := &reportBuilder{}

	b.add("📊 " + bold("CASH FLOW "+report.PeriodLabel))
	b.add("📅 " + bold(report.DateLabel))
	b.blank()

	b.section("💰", "SALDO AWAL")
	b.add(bold(report.OpeningBalanceLabel))
	b.add(boldRupiah(report.OpeningBalance))
	b.blank()

	if len(report.CashFlowExpenses) > 0 {
		b.section("💸", "PENGELUARAN CASH FLOW")
		for _, line := range report.CashFlowExpenses {
			b.bullet(line.Label, line.Amount)
		}
		b.blank()
		b.add("➡️ " + bold("Total Pengeluaran") + " : " + boldRupiah(report.CashFlowTotal))
		b.blank()
	}

	for _, card := range report.CreditCards {
		b.section("💳", "PAYMENT CC")
		b.bullet("Bayar "+card.Name, card.Payments)
		b.bullet("Ambil kembali / Top-up", card.TakenBack)
		b.blank()
		b.add("➡️ " + bold("Net Payment "+card.Name))
		// The arithmetic is spelled out because the net figure is the one that gets
		// questioned, and showing both operands answers it without a second message.
		b.add(escapeMarkdownV2(rupiah(card.Payments) + " − " + rupiah(card.TakenBack)))
		b.add(escapeMarkdownV2("= ") + boldRupiah(card.Net))
		b.blank()
	}

	if !report.TopUp.Total.IsZero() {
		b.section("💰", "TOP-UP")
		b.add(bold("Total Top-up") + " : " + boldRupiah(report.TopUp.Total))
		b.blank()
		if len(report.TopUp.Allocations) > 0 {
			b.add(escapeMarkdownV2("Alokasi:"))
			b.blank()
			parts := make([]string, 0, len(report.TopUp.Allocations))
			for _, allocation := range report.TopUp.Allocations {
				b.bullet(allocation.Label, allocation.Amount)
				parts = append(parts, rupiah(allocation.Amount))
			}
			b.blank()
			b.add("➡️ " + escapeMarkdownV2(strings.Join(parts, " + ")))
			b.add(escapeMarkdownV2("= ") + boldRupiah(report.TopUp.AllocationTotal) + balanceMark(report.TopUp.Balanced))
			b.blank()
		}
	}

	for _, wallet := range report.Wallets {
		b.section("🏦", strings.ToUpper(wallet.Name))
		b.add(bold("Jatah "+wallet.Name) + " : " + boldRupiah(wallet.Allotment))
		b.blank()

		if len(wallet.Items) > 0 {
			b.add(escapeMarkdownV2("Transaksi yang tercatat:"))
			b.blank()
			for _, item := range wallet.Items {
				b.bullet(item.Label, item.Amount)
				for _, sub := range item.SubItems {
					b.subBullet(sub.Label, sub.Amount)
				}
			}
			b.blank()
		}
		b.add("➡️ " + bold("Total transaksi tercatat"))
		b.add(boldRupiah(wallet.RecordedTotal))
		b.blank()

		b.section("💰", "SISA "+strings.ToUpper(wallet.Name))
		b.add(escapeMarkdownV2("Jatah " + wallet.Name))
		b.add(escapeMarkdownV2(rupiah(wallet.Allotment)))
		b.blank()
		b.add(escapeMarkdownV2("Dikurangi transaksi"))
		b.add(escapeMarkdownV2(rupiah(wallet.RecordedTotal)))
		b.blank()
		b.add("➡️ " + bold("Sisa menurut catatan"))
		b.add(boldRupiah(wallet.ExpectedRemaining))
		b.blank()

		if wallet.HasActualBalance {
			b.add(escapeMarkdownV2("Namun saldo aktual yang ada:"))
			b.add(boldRupiah(wallet.ActualBalance))
			b.blank()
			if wallet.Variance.IsZero() {
				b.add("✅ " + escapeMarkdownV2("Tidak ada selisih: seluruh transaksi sudah tercatat."))
			} else {
				b.add(escapeMarkdownV2("Sehingga terdapat selisih:"))
				b.blank()
				b.add(escapeMarkdownV2(rupiah(wallet.ExpectedRemaining) + " − " + rupiah(wallet.ActualBalance)))
				b.add(escapeMarkdownV2("= ") + boldRupiah(wallet.Variance))
				b.blank()
				b.add("➡️ " + bold(rupiah(wallet.Variance)+" = transaksi/pengeluaran yang belum tercatat atau belum teridentifikasi"))
			}
			b.blank()
		}
	}

	for _, bank := range report.Banks {
		b.section("🏦", strings.ToUpper(bank.Name))
		b.add(escapeMarkdownV2("Mutasi yang tercatat:"))
		b.blank()
		b.bullet("Dana masuk", bank.MoneyIn)
		if !bank.Fees.IsZero() {
			b.bullet("Biaya admin", bank.Fees)
		}
		b.bullet("Dana keluar", bank.MoneyOut)
		b.blank()
		b.add(escapeMarkdownV2("Perhitungan:"))
		b.blank()
		b.add(escapeMarkdownV2(rupiah(bank.MoneyIn) + " − " + rupiah(bank.Fees) + " − " + rupiah(bank.MoneyOut)))
		b.add(escapeMarkdownV2("= ") + boldRupiah(bank.Computed))
		b.blank()

		if bank.HasActualBalance {
			b.add(escapeMarkdownV2("Saldo " + bank.Name + " saat ini:"))
			b.add(boldRupiah(bank.ActualBalance))
			b.blank()
			b.add(escapeMarkdownV2("Karena "+rupiah(bank.ActualBalance)+" juga dipengaruhi saldo sebelumnya, ") +
				bold(rupiah(bank.ActualBalance)+" tidak dianggap sebagai potongan jatah") +
				escapeMarkdownV2("."))
			b.blank()
			b.add(escapeMarkdownV2("Selisih terhadap mutasi tersebut:"))
			b.blank()
			b.add(escapeMarkdownV2(rupiah(bank.ActualBalance) + " − " + rupiah(bank.Computed)))
			b.add(escapeMarkdownV2("= ") + boldRupiah(bank.PreviousBalance))
			b.blank()
			b.add("➡️ " + bold(rupiah(bank.PreviousBalance)+" merupakan saldo sebelumnya yang sudah ada di "+bank.Name) + escapeMarkdownV2("."))
			b.blank()
		}
	}

	if !report.Reconciliation.TopUpTotal.IsZero() {
		b.section("📊", "REKONSILIASI TOP-UP")
		b.add(bold("Total Top-up") + " : " + boldRupiah(report.Reconciliation.TopUpTotal))
		b.blank()
		parts := make([]string, 0, len(report.Reconciliation.Parts))
		for _, part := range report.Reconciliation.Parts {
			b.add("• " + escapeMarkdownV2(part.Label+" : "+rupiah(part.Amount)))
			parts = append(parts, rupiah(part.Amount))
		}
		b.blank()
		b.add(escapeMarkdownV2("Perhitungan:"))
		b.blank()
		b.add(escapeMarkdownV2(strings.Join(parts, " + ")))
		b.add(escapeMarkdownV2("= ") + boldRupiah(report.Reconciliation.PartsTotal) + balanceMark(report.Reconciliation.Balanced))
		b.blank()
		b.add("➡️ " + bold("Selisih Top-up") + " : " + boldRupiah(report.Reconciliation.Difference))
		b.blank()
	}

	if len(report.Status) > 0 {
		b.section("📌", "STATUS "+report.DateLabel)
		for _, line := range report.Status {
			b.add(line.Icon + " " + bold(line.Label) + " : " + boldRupiah(line.Amount))
		}
		b.add(sectionRule)
	}

	return b.String()
}

// balanceMark flags a section that adds up, which is the point of printing the arithmetic.
func balanceMark(balanced bool) string {
	if balanced {
		return " ✅"
	}
	return " ⚠️"
}

// SplitTelegramMessage cuts a long report at section boundaries so no message exceeds the
// Bot API limit. A single section longer than the budget is emitted whole and left for the
// API to reject rather than being cut through a Markdown entity, which would break parsing.
func SplitTelegramMessage(text string) []string {
	if len([]rune(text)) <= telegramMaxMessageLength {
		return []string{text}
	}

	blocks := strings.Split(text, "\n"+sectionRule)
	var messages []string
	current := blocks[0]

	for _, block := range blocks[1:] {
		candidate := current + "\n" + sectionRule + block
		if len([]rune(candidate)) > telegramChunkBudget {
			messages = append(messages, current)
			current = sectionRule + block
			continue
		}
		current = candidate
	}
	if strings.TrimSpace(current) != "" {
		messages = append(messages, current)
	}
	return messages
}
