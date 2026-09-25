package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/rezanii/tracking-cashflow/apps/backend/internal/dto"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/utils"
	"github.com/rezanii/tracking-cashflow/apps/backend/internal/validator"
)

// maxBodyBytes caps a request body so a large payload cannot exhaust memory.
const maxBodyBytes = 1 << 20

// decode reads and validates a JSON body. Unknown fields are rejected so a typo in a client
// payload surfaces as an error instead of being silently ignored.
func decode[T any](r *http.Request, v *validator.Validator, target *T) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return utils.NewDomainError(utils.ErrValidation, "Request body is required")
		}
		return utils.WrapDomainError(utils.ErrValidation, "Request body is not valid JSON", err)
	}
	return v.Struct(target)
}

func idParam(r *http.Request, name string) (int64, error) {
	raw := chi.URLParam(r, name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, utils.NewFieldError("Validation failed", map[string]string{name: name + " must be a positive integer"})
	}
	return id, nil
}

func queryInt(r *http.Request, name string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func queryInt64Ptr(r *http.Request, name string) (*int64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return nil, utils.NewFieldError("Validation failed", map[string]string{name: name + " must be a positive integer"})
	}
	return &value, nil
}

func queryBoolPtr(r *http.Request, name string) (*bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, utils.NewFieldError("Validation failed", map[string]string{name: name + " must be true or false"})
	}
	return &value, nil
}

func queryDatePtr(r *http.Request, name string) (*time.Time, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, nil
	}
	parsed, err := utils.ParseDate(raw)
	if err != nil {
		return nil, utils.NewFieldError("Validation failed", map[string]string{name: err.Error()})
	}
	return &parsed, nil
}

// queryTransactionType validates the enum here rather than letting an unknown value reach
// the query, where it would silently return an empty result set.
func queryTransactionType(r *http.Request, name string) (string, error) {
	raw := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get(name)))
	switch raw {
	case "", "INCOME", "EXPENSE", "TRANSFER":
		return raw, nil
	default:
		return "", utils.NewFieldError("Validation failed", map[string]string{
			name: name + " must be one of: INCOME, EXPENSE, TRANSFER",
		})
	}
}

func queryCategoryType(r *http.Request, name string) (string, error) {
	raw := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get(name)))
	switch raw {
	case "", "INCOME", "EXPENSE":
		return raw, nil
	default:
		return "", utils.NewFieldError("Validation failed", map[string]string{
			name: name + " must be one of: INCOME, EXPENSE",
		})
	}
}

// reportFilter resolves the shared report query. date_from and date_to are mandatory so a
// report can never accidentally scan the whole history.
func reportFilter(r *http.Request) (dto.ReportFilter, error) {
	from, err := queryDatePtr(r, "date_from")
	if err != nil {
		return dto.ReportFilter{}, err
	}
	to, err := queryDatePtr(r, "date_to")
	if err != nil {
		return dto.ReportFilter{}, err
	}

	fields := map[string]string{}
	if from == nil {
		fields["date_from"] = "date_from is required"
	}
	if to == nil {
		fields["date_to"] = "date_to is required"
	}
	if len(fields) > 0 {
		return dto.ReportFilter{}, utils.NewFieldError("Validation failed", fields)
	}
	if to.Before(*from) {
		return dto.ReportFilter{}, utils.NewFieldError("Validation failed", map[string]string{
			"date_to": "date_to must not be earlier than date_from",
		})
	}

	categoryID, err := queryInt64Ptr(r, "category_id")
	if err != nil {
		return dto.ReportFilter{}, err
	}
	transactionType, err := queryTransactionType(r, "transaction_type")
	if err != nil {
		return dto.ReportFilter{}, err
	}

	return dto.ReportFilter{
		DateFrom:        *from,
		DateTo:          *to,
		CategoryID:      categoryID,
		TransactionType: transactionType,
	}, nil
}

// dashboardFilter accepts either a named range or an explicit custom range.
func dashboardFilter(r *http.Request) (dto.ReportFilter, error) {
	name := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("range")))
	if name == "" {
		name = "month"
	}

	if name != "custom" {
		from, to, ok := utils.ResolveRange(name)
		if !ok {
			return dto.ReportFilter{}, utils.NewFieldError("Validation failed", map[string]string{
				"range": "range must be one of: today, week, month, year, custom",
			})
		}
		return dto.ReportFilter{DateFrom: from, DateTo: to}, nil
	}
	return reportFilter(r)
}
