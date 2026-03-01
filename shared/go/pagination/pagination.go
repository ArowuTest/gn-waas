package pagination

import (
	"math"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// Params holds pagination parameters
type Params struct {
	Page     int `json:"page" query:"page"`
	PageSize int `json:"page_size" query:"page_size"`
}

// Normalize ensures pagination params are within valid bounds
func (p *Params) Normalize() {
	if p.Page < 1 {
		p.Page = DefaultPage
	}
	if p.PageSize < 1 {
		p.PageSize = DefaultPageSize
	}
	if p.PageSize > MaxPageSize {
		p.PageSize = MaxPageSize
	}
}

// Offset returns the SQL OFFSET value
func (p *Params) Offset() int {
	return (p.Page - 1) * p.PageSize
}

// Limit returns the SQL LIMIT value
func (p *Params) Limit() int {
	return p.PageSize
}

// Result wraps a paginated response
type Result[T any] struct {
	Data       []T  `json:"data"`
	Total      int  `json:"total"`
	Page       int  `json:"page"`
	PageSize   int  `json:"page_size"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// NewResult creates a paginated result
func NewResult[T any](data []T, total int, params Params) *Result[T] {
	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}
	return &Result[T]{
		Data:       data,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
		HasNext:    params.Page < totalPages,
		HasPrev:    params.Page > 1,
	}
}
