package service

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/shopspring/decimal"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

// The detail table has seven columns, so the page is landscape to keep the description
// and both amount columns readable.
var pdfColumnWidths = []float64{22, 20, 34, 78, 30, 30, 32}

var pdfColumnHeaders = []string{"Date", "Type", "Category", "Description", "Income", "Expense", "Balance"}

func (s *reportService) CashFlowPDF(ctx context.Context, userID int64, filter dto.ReportFilter) ([]byte, string, error) {
	report, err := s.CashFlow(ctx, userID, filter)
	if err != nil {
		return nil, "", err
	}

	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetTitle("Financial Report", false)
	pdf.SetAutoPageBreak(true, 15)
	pdf.SetMargins(10, 12, 10)

	// The header is repeated on every page, including pages created by the auto break.
	pdf.SetHeaderFunc(func() {
		if pdf.PageNo() == 1 {
			return
		}
		pdf.SetY(12)
		writeTableHeader(pdf)
	})
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 6, fmt.Sprintf("Page %d", pdf.PageNo()), "", 0, "R", false, 0, "")
	})

	pdf.AddPage()
	writePDFTitle(pdf, report)
	writeTableHeader(pdf)
	writePDFRows(pdf, report)
	writePDFTotals(pdf, report)

	buffer := &bytes.Buffer{}
	if err := pdf.Output(buffer); err != nil {
		return nil, "", utils.WrapDomainError(utils.ErrValidation, "Failed to encode PDF", err)
	}

	filename := fmt.Sprintf("cash-flow-%s-%s.pdf", report.Period.From, report.Period.To)
	return buffer.Bytes(), filename, nil
}

func writePDFTitle(pdf *fpdf.Fpdf, report dto.CashFlowResponse) {
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 9, "FINANCIAL REPORT", "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, fmt.Sprintf("Period: %s - %s", longDate(report.Period.From), longDate(report.Period.To)), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	pdf.SetFont("Helvetica", "B", 10)
	summary := [][2]string{
		{"Total Income", formatRupiah(report.TotalIncome)},
		{"Total Expense", formatRupiah(report.TotalExpense)},
		{"Net Cash Flow", formatRupiah(report.NetCashFlow)},
		{"Transactions", fmt.Sprintf("%d", len(report.Rows))},
	}
	for _, item := range summary {
		pdf.SetFont("Helvetica", "", 10)
		pdf.CellFormat(40, 6, item[0], "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(60, 6, item[1], "", 1, "L", false, 0, "")
	}
	pdf.Ln(3)
}

func writeTableHeader(pdf *fpdf.Fpdf) {
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(31, 78, 120)
	pdf.SetTextColor(255, 255, 255)
	for index, header := range pdfColumnHeaders {
		align := "L"
		if index >= 4 {
			align = "R"
		}
		pdf.CellFormat(pdfColumnWidths[index], 8, header, "1", 0, align, true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetTextColor(0, 0, 0)
}

func writePDFRows(pdf *fpdf.Fpdf, report dto.CashFlowResponse) {
	pdf.SetFont("Helvetica", "", 8)

	if len(report.Rows) == 0 {
		total := 0.0
		for _, width := range pdfColumnWidths {
			total += width
		}
		pdf.CellFormat(total, 8, "No transactions in this period", "1", 1, "C", false, 0, "")
		return
	}

	fill := false
	for _, row := range report.Rows {
		pdf.SetFillColor(245, 247, 250)
		values := []string{
			formatDisplayDate(row.TransactionDate),
			row.TransactionType,
			truncate(row.CategoryName, 20),
			truncate(row.Description, 48),
			formatRupiah(row.Income),
			formatRupiah(row.Expense),
			formatRupiah(row.Balance),
		}
		for index, value := range values {
			align := "L"
			if index >= 4 {
				align = "R"
			}
			pdf.CellFormat(pdfColumnWidths[index], 7, value, "1", 0, align, fill, 0, "")
		}
		pdf.Ln(-1)
		fill = !fill
	}
}

func writePDFTotals(pdf *fpdf.Fpdf, report dto.CashFlowResponse) {
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(220, 230, 241)

	labelWidth := pdfColumnWidths[0] + pdfColumnWidths[1] + pdfColumnWidths[2] + pdfColumnWidths[3]
	pdf.CellFormat(labelWidth, 8, "TOTAL", "1", 0, "L", true, 0, "")
	pdf.CellFormat(pdfColumnWidths[4], 8, formatRupiah(report.TotalIncome), "1", 0, "R", true, 0, "")
	pdf.CellFormat(pdfColumnWidths[5], 8, formatRupiah(report.TotalExpense), "1", 0, "R", true, 0, "")
	pdf.CellFormat(pdfColumnWidths[6], 8, formatRupiah(report.NetCashFlow), "1", 1, "R", true, 0, "")
}

// formatRupiah renders an amount as Rp 1.150.000 using dots as thousand separators.
func formatRupiah(amount decimal.Decimal) string {
	rounded := amount.Round(0)
	negative := rounded.IsNegative()
	digits := rounded.Abs().String()

	var builder strings.Builder
	for index, char := range digits {
		if index > 0 && (len(digits)-index)%3 == 0 {
			builder.WriteByte('.')
		}
		builder.WriteRune(char)
	}

	if negative {
		return "-Rp " + builder.String()
	}
	return "Rp " + builder.String()
}

func formatDisplayDate(value string) string {
	parsed, err := time.Parse(utils.DateLayout, value)
	if err != nil {
		return value
	}
	return parsed.Format("02/01/2006")
}

func longDate(value string) string {
	parsed, err := time.Parse(utils.DateLayout, value)
	if err != nil {
		return value
	}
	return parsed.Format("02 January 2006")
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}
