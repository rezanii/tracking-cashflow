package utils

import "strings"

const (
	defaultPage     = 1
	defaultPageSize = 10
	maxPageSize     = 100
)

// NormalizePaging clamps client-supplied paging so a request cannot ask for an unbounded page.
func NormalizePaging(page, pageSize int) (int, int) {
	if page < 1 {
		page = defaultPage
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

func Offset(page, pageSize int) int {
	return (page - 1) * pageSize
}

// NormalizeSort maps a client sort field onto an allow list of columns. Anything unknown
// falls back to the default, which keeps the value out of the SQL string entirely.
func NormalizeSort(sortBy, sortDir string, allowed map[string]string, defaultColumn string) (string, string) {
	column, ok := allowed[strings.ToLower(strings.TrimSpace(sortBy))]
	if !ok {
		column = defaultColumn
	}
	direction := "DESC"
	if strings.EqualFold(strings.TrimSpace(sortDir), "asc") {
		direction = "ASC"
	}
	return column, direction
}
