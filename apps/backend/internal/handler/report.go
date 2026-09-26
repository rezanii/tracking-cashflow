package handler

import (
	"fmt"
	"net/http"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/middleware"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/service"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
)

type ReportHandler struct {
	reports service.ReportService
	daily   service.DailyReportService
}

func NewReportHandler(reports service.ReportService, daily service.DailyReportService) *ReportHandler {
	return &ReportHandler{reports: reports, daily: daily}
}

// DailyCashFlow returns the structured daily report: cash flow, card payments, top-up,
// wallets, banks and the reconciliation that ties them together.
//
//	@Summary		Daily cash flow report
//	@Description	The same report the Telegram bot sends, as JSON. Sections with nothing to say are
//	@Description	omitted, and a wallet or bank only reports a variance once a balance has been
//	@Description	recorded for it.
//	@Tags			Reports
//	@Produce		json
//	@Security		BearerAuth
//	@Param			date	query		string	false	"Report date, YYYY-MM-DD. Defaults to today."
//	@Success		200		{object}	utils.Envelope{data=dto.DailyCashFlowReport}
//	@Failure		401		{object}	utils.Envelope
//	@Failure		422		{object}	utils.Envelope
//	@Router			/reports/daily-cash-flow [get]
func (h *ReportHandler) DailyCashFlow(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserID(r.Context())
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	date, err := reportDate(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	report, err := h.daily.Build(r.Context(), userID, date)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Data retrieved successfully", report)
}

// Dashboard returns the figures and chart series behind the dashboard.
//
//	@Summary		Dashboard summary
//	@Tags			Reports
//	@Produce		json
//	@Security		BearerAuth
//	@Param			range		query		string	false	"today, week, month, year or custom"	default(month)
//	@Param			date_from	query		string	false	"Required when range is custom"
//	@Param			date_to		query		string	false	"Required when range is custom"
//	@Success		200			{object}	utils.Envelope{data=dto.DashboardSummaryResponse}
//	@Failure		401			{object}	utils.Envelope
//	@Router			/dashboard/summary [get]
func (h *ReportHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserID(r.Context())
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	filter, err := dashboardFilter(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	summary, err := h.reports.Dashboard(r.Context(), userID, filter)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Data retrieved successfully", summary)
}

// Summary returns period totals.
//
//	@Summary		Period summary
//	@Tags			Reports
//	@Produce		json
//	@Security		BearerAuth
//	@Param			date_from			query		string	true	"Start date, YYYY-MM-DD"
//	@Param			date_to				query		string	true	"End date, YYYY-MM-DD"
//	@Param			category_id			query		int		false	"Category id"
//	@Param			transaction_type	query		string	false	"INCOME, EXPENSE or TRANSFER"
//	@Success		200					{object}	utils.Envelope{data=dto.SummaryResponse}
//	@Failure		422					{object}	utils.Envelope
//	@Router			/reports/summary [get]
func (h *ReportHandler) Summary(w http.ResponseWriter, r *http.Request) {
	userID, filter, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	summary, err := h.reports.Summary(r.Context(), userID, filter)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Data retrieved successfully", summary)
}

// CashFlow returns the cash flow rows with a running balance.
//
//	@Summary		Cash flow report
//	@Tags			Reports
//	@Produce		json
//	@Security		BearerAuth
//	@Param			date_from			query		string	true	"Start date, YYYY-MM-DD"
//	@Param			date_to				query		string	true	"End date, YYYY-MM-DD"
//	@Param			category_id			query		int		false	"Category id"
//	@Param			transaction_type	query		string	false	"INCOME, EXPENSE or TRANSFER"
//	@Success		200					{object}	utils.Envelope{data=dto.CashFlowResponse}
//	@Failure		422					{object}	utils.Envelope
//	@Router			/reports/cash-flow [get]
func (h *ReportHandler) CashFlow(w http.ResponseWriter, r *http.Request) {
	userID, filter, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	report, err := h.reports.CashFlow(r.Context(), userID, filter)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Data retrieved successfully", report)
}

// ExpenseByCategory returns expense totals grouped by category.
//
//	@Summary		Expense by category
//	@Tags			Reports
//	@Produce		json
//	@Security		BearerAuth
//	@Param			date_from	query		string	true	"Start date, YYYY-MM-DD"
//	@Param			date_to		query		string	true	"End date, YYYY-MM-DD"
//	@Param			category_id	query		int		false	"Category id"
//	@Success		200			{object}	utils.Envelope{data=dto.ExpenseByCategoryResponse}
//	@Failure		422			{object}	utils.Envelope
//	@Router			/reports/expense-by-category [get]
func (h *ReportHandler) ExpenseByCategory(w http.ResponseWriter, r *http.Request) {
	userID, filter, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	report, err := h.reports.ExpenseByCategory(r.Context(), userID, filter)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Data retrieved successfully", report)
}

// Monthly returns one row per month in the period.
//
//	@Summary		Monthly report
//	@Tags			Reports
//	@Produce		json
//	@Security		BearerAuth
//	@Param			date_from			query		string	true	"Start date, YYYY-MM-DD"
//	@Param			date_to				query		string	true	"End date, YYYY-MM-DD"
//	@Param			category_id			query		int		false	"Category id"
//	@Param			transaction_type	query		string	false	"INCOME, EXPENSE or TRANSFER"
//	@Success		200					{object}	utils.Envelope{data=dto.MonthlyResponse}
//	@Failure		422					{object}	utils.Envelope
//	@Router			/reports/monthly [get]
func (h *ReportHandler) Monthly(w http.ResponseWriter, r *http.Request) {
	userID, filter, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	report, err := h.reports.Monthly(r.Context(), userID, filter)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	utils.OK(w, "Data retrieved successfully", report)
}

// CashFlowExcel streams the cash flow report as a workbook.
//
//	@Summary		Cash flow report as Excel
//	@Tags			Reports
//	@Produce		application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
//	@Security		BearerAuth
//	@Param			date_from			query	string	true	"Start date, YYYY-MM-DD"
//	@Param			date_to				query	string	true	"End date, YYYY-MM-DD"
//	@Param			category_id			query	int		false	"Category id"
//	@Param			transaction_type	query	string	false	"INCOME, EXPENSE or TRANSFER"
//	@Success		200					{file}	binary
//	@Failure		422					{object}	utils.Envelope
//	@Router			/reports/cash-flow/excel [get]
func (h *ReportHandler) CashFlowExcel(w http.ResponseWriter, r *http.Request) {
	userID, filter, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	content, filename, err := h.reports.CashFlowExcel(r.Context(), userID, filter)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	writeDownload(w, content, filename, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
}

// CashFlowPDF streams the cash flow report as a PDF.
//
//	@Summary		Cash flow report as PDF
//	@Tags			Reports
//	@Produce		application/pdf
//	@Security		BearerAuth
//	@Param			date_from			query	string	true	"Start date, YYYY-MM-DD"
//	@Param			date_to				query	string	true	"End date, YYYY-MM-DD"
//	@Param			category_id			query	int		false	"Category id"
//	@Param			transaction_type	query	string	false	"INCOME, EXPENSE or TRANSFER"
//	@Success		200					{file}	binary
//	@Failure		422					{object}	utils.Envelope
//	@Router			/reports/cash-flow/pdf [get]
func (h *ReportHandler) CashFlowPDF(w http.ResponseWriter, r *http.Request) {
	userID, filter, err := h.scope(r)
	if err != nil {
		utils.WriteError(w, err)
		return
	}

	content, filename, err := h.reports.CashFlowPDF(r.Context(), userID, filter)
	if err != nil {
		utils.WriteError(w, err)
		return
	}
	writeDownload(w, content, filename, "application/pdf")
}

func (h *ReportHandler) scope(r *http.Request) (int64, dto.ReportFilter, error) {
	userID, err := middleware.UserID(r.Context())
	if err != nil {
		return 0, dto.ReportFilter{}, err
	}
	filter, err := reportFilter(r)
	if err != nil {
		return 0, dto.ReportFilter{}, err
	}
	return userID, filter, nil
}

// writeDownload sends a generated file. Content-Length is set so the browser can show
// progress, and the filename is quoted for names containing a space.
func writeDownload(w http.ResponseWriter, content []byte, filename, contentType string) {
	header := w.Header()
	header.Set("Content-Type", contentType)
	header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	header.Set("Content-Length", fmt.Sprintf("%d", len(content)))
	header.Set("Cache-Control", "no-store")
	header.Set("Access-Control-Expose-Headers", "Content-Disposition")

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(content); err != nil {
		utils.Error(w, http.StatusInternalServerError, "Failed to stream report", nil)
	}
}
