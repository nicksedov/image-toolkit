package helpers

import "github.com/gin-gonic/gin"

// PaginationMode determines how page size is validated.
type PaginationMode int

const (
	// ModeFlexible validates 1 <= pageSize <= MaxPageSizeFlexible, defaults to DefaultPageSize.
	ModeFlexible PaginationMode = iota
	// ModeFixed validates against FixedPageSizes list, defaults to first value.
	ModeFixed
)

// PaginationParams holds parsed pagination parameters.
type PaginationParams struct {
	Page     int
	PageSize int
	Offset   int
}

// PaginationResult holds calculated pagination metadata.
type PaginationResult struct {
	Page        int
	PageSize    int
	TotalPages  int
	HasNextPage bool
	HasPrevPage bool
}

// ParsePagination parses page and pageSize from query parameters.
func ParsePagination(c *gin.Context, mode PaginationMode) PaginationParams {
	page := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "50")

	p, _ := parseInt(page)
	ps, _ := parseInt(pageSizeStr)

	if p < 1 {
		p = 1
	}

	switch mode {
	case ModeFixed:
		isValid := false
		for _, valid := range FixedPageSizes {
			if ps == valid {
				isValid = true
				break
			}
		}
		if !isValid {
			ps = FixedPageSizes[0]
		}
	case ModeFlexible:
		if ps < 1 || ps > MaxPageSizeFlexible {
			ps = DefaultPageSize
		}
	}

	return PaginationParams{
		Page:     p,
		PageSize: ps,
		Offset:   (p - 1) * ps,
	}
}

// CalcPagination calculates pagination metadata from total record count.
func CalcPagination(page, pageSize int, total int64) PaginationResult {
	tp := int(total)
	if tp > 0 {
		tp = (int(total) + pageSize - 1) / pageSize
	}
	if tp < 1 {
		tp = 1
	}
	if page > tp {
		page = tp
	}
	if page < 1 {
		page = 1
	}

	return PaginationResult{
		Page:        page,
		PageSize:    pageSize,
		TotalPages:  tp,
		HasNextPage: page < tp,
		HasPrevPage: page > 1,
	}
}

func parseInt(s string) (int, bool) {
	result := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		result = result*10 + int(ch-'0')
	}
	return result, true
}
