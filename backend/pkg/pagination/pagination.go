// Package pagination provides standard pagination helpers.
package pagination

import (
	"strconv"

	"github.com/labstack/echo/v4"
)

const (
	DefaultPage  = 1
	DefaultLimit = 20
	MaxLimit     = 100
)

// Params holds pagination parameters.
type Params struct {
	Page  int
	Limit int
}

// Offset calculates the SQL OFFSET.
func (p Params) Offset() int {
	return (p.Page - 1) * p.Limit
}

// Meta is the pagination metadata returned in responses.
type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// Response is the standard paginated response format.
type Response[T any] struct {
	Data []T  `json:"data"`
	Meta Meta `json:"meta"`
}

// NewMeta creates pagination metadata.
func NewMeta(p Params, total int64) Meta {
	totalPages := int(total) / p.Limit
	if int(total)%p.Limit != 0 {
		totalPages++
	}
	return Meta{
		Page:       p.Page,
		Limit:      p.Limit,
		Total:      total,
		TotalPages: totalPages,
	}
}

// FromContext parses pagination params from Echo context.
func FromContext(c echo.Context) Params {
	page := parseInt(c.QueryParam("page"), DefaultPage)
	limit := parseInt(c.QueryParam("limit"), DefaultLimit)

	if page < 1 {
		page = DefaultPage
	}
	if limit < 1 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	return Params{Page: page, Limit: limit}
}

func parseInt(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}

// NewResponse creates a paginated response.
func NewResponse[T any](data []T, params Params, total int64) Response[T] {
	if data == nil {
		data = []T{}
	}
	return Response[T]{
		Data: data,
		Meta: NewMeta(params, total),
	}
}
