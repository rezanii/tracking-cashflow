package service

import (
	"bytes"
	"context"
	"fmt"

	"github.com/xuri/excelize/v2"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

const (
	sheetSummary      = "Summary"
	sheetTransactions = "Transactions"
	// currencyFormat renders 1150000 as "Rp 1.150.000" for a reader whose Excel is set to
	// Indonesian, and "Rp 1,150,000" for one set to English.
	//
	// Excel's format codes are locale-neutral: "," always means the thousands separator and
	// "." the decimal point, whatever the reader's locale, and Excel substitutes the
	// separators that locale uses when it displays the cell. Writing the Indonesian
	// separators literally — "#.##0,00" — is read as a decimal point followed by a stray
	// thousands separator, which is why the cell came out as "Rp 1150000,000".
	//
	// No decimals, matching the PDF. The stored value keeps its two, so a cell is rounded for
	// display only.
	currencyFormat = `"Rp" #,##0`
	dateFormat     = "dd/mm/yyyy"
)

// CashFlowExcel builds a two sheet workbook: a summary of the period and the transaction
// detail with a running balance.
func (s *reportService) CashFlowExcel(ctx context.Context, userID int64, filter dto.ReportFilter) ([]byte, string, error) {
	report, err := s.CashFlow(ctx, userID, filter)
	if err != nil {
		return nil, "", err
	}

	file := excelize.NewFile()
	defer func() {
		_ = file.Close()
	}()

	styles, err := newExcelStyles(file)
	if err != nil {
		return nil, "", utils.WrapDomainError(utils.ErrValidation, "Failed to prepare workbook styles", err)
	}

	if err := writeSummarySheet(file, styles, report); err != nil {
		return nil, "", utils.WrapDomainError(utils.ErrValidation, "Failed to write summary sheet", err)
	}
	if err := writeTransactionSheet(file, styles, report); err != nil {
		return nil, "", utils.WrapDomainError(utils.ErrValidation, "Failed to write transaction sheet", err)
	}

	// excelize creates a default "Sheet1"; the report only needs the two named sheets.
	if index, err := file.GetSheetIndex("Sheet1"); err == nil && index >= 0 {
		file.DeleteSheet("Sheet1")
	}
	if index, err := file.GetSheetIndex(sheetSummary); err == nil {
		file.SetActiveSheet(index)
	}

	buffer := &bytes.Buffer{}
	if err := file.Write(buffer); err != nil {
		return nil, "", utils.WrapDomainError(utils.ErrValidation, "Failed to encode workbook", err)
	}

	filename := fmt.Sprintf("cash-flow-%s-%s.xlsx", report.Period.From, report.Period.To)
	return buffer.Bytes(), filename, nil
}

type excelStyles struct {
	title    int
	header   int
	label    int
	currency int
	date     int
	total    int
	totalNum int
}

func newExcelStyles(file *excelize.File) (excelStyles, error) {
	var styles excelStyles
	var err error

	if styles.title, err = file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14},
	}); err != nil {
		return styles, err
	}

	if styles.header, err = file.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1F4E78"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    thinBorder(),
	}); err != nil {
		return styles, err
	}

	if styles.label, err = file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	}); err != nil {
		return styles, err
	}

	if styles.currency, err = file.NewStyle(&excelize.Style{
		CustomNumFmt: strPtr(currencyFormat),
		Border:       thinBorder(),
	}); err != nil {
		return styles, err
	}

	if styles.date, err = file.NewStyle(&excelize.Style{
		CustomNumFmt: strPtr(dateFormat),
		Alignment:    &excelize.Alignment{Horizontal: "center"},
		Border:       thinBorder(),
	}); err != nil {
		return styles, err
	}

	if styles.total, err = file.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true},
		Fill:   excelize.Fill{Type: "pattern", Color: []string{"DCE6F1"}, Pattern: 1},
		Border: thinBorder(),
	}); err != nil {
		return styles, err
	}

	if styles.totalNum, err = file.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"DCE6F1"}, Pattern: 1},
		CustomNumFmt: strPtr(currencyFormat),
		Border:       thinBorder(),
	}); err != nil {
		return styles, err
	}

	return styles, nil
}

func writeSummarySheet(file *excelize.File, styles excelStyles, report dto.CashFlowResponse) error {
	if _, err := file.NewSheet(sheetSummary); err != nil {
		return err
	}

	rows := [][]any{
		{"FINANCIAL REPORT"},
		{},
		{"Period", fmt.Sprintf("%s to %s", report.Period.From, report.Period.To)},
		{"Total Income", report.TotalIncome.InexactFloat64()},
		{"Total Expense", report.TotalExpense.InexactFloat64()},
		{"Net Cash Flow", report.NetCashFlow.InexactFloat64()},
		{"Transaction Count", len(report.Rows)},
	}

	for index, row := range rows {
		cell, err := excelize.CoordinatesToCellName(1, index+1)
		if err != nil {
			return err
		}
		if err := file.SetSheetRow(sheetSummary, cell, &row); err != nil {
			return err
		}
	}

	if err := file.SetCellStyle(sheetSummary, "A1", "A1", styles.title); err != nil {
		return err
	}
	if err := file.SetCellStyle(sheetSummary, "A3", "A7", styles.label); err != nil {
		return err
	}
	if err := file.SetCellStyle(sheetSummary, "B4", "B6", styles.currency); err != nil {
		return err
	}
	if err := file.SetColWidth(sheetSummary, "A", "A", 22); err != nil {
		return err
	}
	return file.SetColWidth(sheetSummary, "B", "B", 32)
}

func writeTransactionSheet(file *excelize.File, styles excelStyles, report dto.CashFlowResponse) error {
	if _, err := file.NewSheet(sheetTransactions); err != nil {
		return err
	}

	headers := []any{"Date", "Type", "Category", "Description", "Reference", "Income", "Expense", "Balance"}
	if err := file.SetSheetRow(sheetTransactions, "A1", &headers); err != nil {
		return err
	}
	if err := file.SetCellStyle(sheetTransactions, "A1", "H1", styles.header); err != nil {
		return err
	}

	for index, row := range report.Rows {
		rowNumber := index + 2
		values := []any{
			row.TransactionDate,
			row.TransactionType,
			row.CategoryName,
			row.Description,
			row.ReferenceNumber,
			row.Income.InexactFloat64(),
			row.Expense.InexactFloat64(),
			row.Balance.InexactFloat64(),
		}
		cell, err := excelize.CoordinatesToCellName(1, rowNumber)
		if err != nil {
			return err
		}
		if err := file.SetSheetRow(sheetTransactions, cell, &values); err != nil {
			return err
		}
		if err := file.SetCellStyle(sheetTransactions, fmt.Sprintf("A%d", rowNumber), fmt.Sprintf("A%d", rowNumber), styles.date); err != nil {
			return err
		}
		if err := file.SetCellStyle(sheetTransactions, fmt.Sprintf("F%d", rowNumber), fmt.Sprintf("H%d", rowNumber), styles.currency); err != nil {
			return err
		}
	}

	totalRow := len(report.Rows) + 2
	totals := []any{"TOTAL", "", "", "", "", report.TotalIncome.InexactFloat64(), report.TotalExpense.InexactFloat64(), report.NetCashFlow.InexactFloat64()}
	totalCell, err := excelize.CoordinatesToCellName(1, totalRow)
	if err != nil {
		return err
	}
	if err := file.SetSheetRow(sheetTransactions, totalCell, &totals); err != nil {
		return err
	}
	if err := file.SetCellStyle(sheetTransactions, fmt.Sprintf("A%d", totalRow), fmt.Sprintf("E%d", totalRow), styles.total); err != nil {
		return err
	}
	if err := file.SetCellStyle(sheetTransactions, fmt.Sprintf("F%d", totalRow), fmt.Sprintf("H%d", totalRow), styles.totalNum); err != nil {
		return err
	}

	widths := map[string]float64{"A": 12, "B": 12, "C": 20, "D": 38, "E": 16, "F": 18, "G": 18, "H": 18}
	for column, width := range widths {
		if err := file.SetColWidth(sheetTransactions, column, column, width); err != nil {
			return err
		}
	}

	// Freeze the header so the table stays readable while scrolling a long period.
	return file.SetPanes(sheetTransactions, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})
}

func thinBorder() []excelize.Border {
	sides := []string{"left", "top", "right", "bottom"}
	borders := make([]excelize.Border, 0, len(sides))
	for _, side := range sides {
		borders = append(borders, excelize.Border{Type: side, Color: "B7B7B7", Style: 1})
	}
	return borders
}

func strPtr(value string) *string {
	return &value
}
