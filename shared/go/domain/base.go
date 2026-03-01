package domain

import (
	"time"

	"github.com/google/uuid"
)

// BaseEntity provides common fields for all domain entities
type BaseEntity struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// AuditableEntity extends BaseEntity with audit trail fields
type AuditableEntity struct {
	BaseEntity
	CreatedBy uuid.UUID  `json:"created_by" db:"created_by"`
	UpdatedBy uuid.UUID  `json:"updated_by" db:"updated_by"`
	Version   int        `json:"version" db:"version"` // Optimistic locking
}

// GeoCoordinate represents a GPS location
type GeoCoordinate struct {
	Latitude  float64 `json:"latitude" db:"latitude"`
	Longitude float64 `json:"longitude" db:"longitude"`
	Precision float64 `json:"precision" db:"precision"` // GPS accuracy in metres
}

// MoneyAmount represents a monetary value in GHS
type MoneyAmount struct {
	Amount   float64 `json:"amount" db:"amount"`
	Currency string  `json:"currency" db:"currency"` // Always "GHS"
}

func NewMoneyGHS(amount float64) MoneyAmount {
	return MoneyAmount{Amount: amount, Currency: "GHS"}
}

// WaterVolume represents a water measurement in cubic metres
type WaterVolume struct {
	CubicMetres float64 `json:"cubic_metres" db:"cubic_metres"`
}

// DateRange represents a time period for audit/billing cycles
type DateRange struct {
	StartDate time.Time `json:"start_date" db:"start_date"`
	EndDate   time.Time `json:"end_date" db:"end_date"`
}

func (d DateRange) DurationDays() int {
	return int(d.EndDate.Sub(d.StartDate).Hours() / 24)
}
