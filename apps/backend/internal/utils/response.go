package utils

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

type Envelope struct {
	Success bool              `json:"success"`
	Message string            `json:"message"`
	Data    any               `json:"data,omitempty"`
	Errors  map[string]string `json:"errors,omitempty"`
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

type PagedData struct {
	Items      any        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

func NewPagination(page, pageSize int, totalItems int64) Pagination {
	totalPages := 0
	if pageSize > 0 {
		totalPages = int((totalItems + int64(pageSize) - 1) / int64(pageSize))
	}
	return Pagination{Page: page, PageSize: pageSize, TotalItems: totalItems, TotalPages: totalPages}
}

func WriteJSON(w http.ResponseWriter, status int, body Envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func OK(w http.ResponseWriter, message string, data any) {
	WriteJSON(w, http.StatusOK, Envelope{Success: true, Message: message, Data: data})
}

func Created(w http.ResponseWriter, message string, data any) {
	WriteJSON(w, http.StatusCreated, Envelope{Success: true, Message: message, Data: data})
}

func Error(w http.ResponseWriter, status int, message string, fields map[string]string) {
	WriteJSON(w, status, Envelope{Success: false, Message: message, Errors: fields})
}

// WriteError maps a domain error onto an HTTP status. Anything unrecognised is logged in
// full and reported as a generic 500 so driver and schema details never leave the server.
func WriteError(w http.ResponseWriter, err error) {
	domainErr, ok := AsDomainError(err)
	if !ok {
		slog.Error("unhandled error", "error", err)
		Error(w, http.StatusInternalServerError, "Internal server error", nil)
		return
	}

	if cause := domainErr.Cause(); cause != nil {
		slog.Error("request failed", "message", domainErr.Message, "error", cause)
	}

	switch {
	case errors.Is(domainErr.Kind, ErrValidation):
		Error(w, http.StatusUnprocessableEntity, domainErr.Message, domainErr.Fields)
	case errors.Is(domainErr.Kind, ErrNotFound):
		Error(w, http.StatusNotFound, domainErr.Message, nil)
	case errors.Is(domainErr.Kind, ErrConflict):
		Error(w, http.StatusConflict, domainErr.Message, nil)
	case errors.Is(domainErr.Kind, ErrUnauthorized):
		Error(w, http.StatusUnauthorized, domainErr.Message, nil)
	case errors.Is(domainErr.Kind, ErrForbidden):
		Error(w, http.StatusForbidden, domainErr.Message, nil)
	default:
		slog.Error("unmapped domain error", "message", domainErr.Message)
		Error(w, http.StatusInternalServerError, "Internal server error", nil)
	}
}
