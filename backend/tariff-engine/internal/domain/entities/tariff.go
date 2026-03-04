package entities

import (
	"time"

	"github.com/google/uuid"
)

// TariffRate represents a PURC-approved tariff rate tier
// All values are stored in the database and admin-configurable
type TariffRate struct {
	ID               uuid.UUID  `db:"id" json:"id"`
	Category         string     `db:"category" json:"category"`
	TierName         string     `db:"tier_name" json:"tier_name"`
	MinVolumeM3      float64    `db:"min_volume_m3" json:"min_volume_m3"`
	MaxVolumeM3      *float64   `db:"max_volume_m3" json:"max_volume_m3"` // nil = unlimited
	RatePerM3        float64    `db:"rate_per_m3" json:"rate_per_m3"`
	ServiceChargeGHS float64    `db:"service_charge_ghs" json:"service_charge_ghs"`
	EffectiveFrom    time.Time  `db:"effective_from" json:"effective_from"`
	EffectiveTo      *time.Time `db:"effective_to" json:"effective_to"`
	ApprovedBy       string     `db:"approved_by" json:"approved_by"`
	RegulatoryRef    string     `db:"regulatory_ref" json:"regulatory_ref"`
	IsActive         bool       `db:"is_active" json:"is_active"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`
}

// VATConfig represents the current VAT configuration
type VATConfig struct {
	ID             uuid.UUID              `db:"id" json:"id"`
	RatePercentage float64                `db:"rate_percentage" json:"rate_percentage"`
	Components     map[string]interface{} `db:"components" json:"components"`
	EffectiveFrom  time.Time              `db:"effective_from" json:"effective_from"`
	EffectiveTo    *time.Time             `db:"effective_to" json:"effective_to"`
	RegulatoryRef  string                 `db:"regulatory_ref" json:"regulatory_ref"`
	IsActive       bool                   `db:"is_active" json:"is_active"`
	CreatedAt      time.Time              `db:"created_at" json:"created_at"`
}

// ShadowBillCalculation is the result of a tariff calculation
// This is the domain object returned by the tariff engine
type ShadowBillCalculation struct {
	AccountID           uuid.UUID `json:"account_id"`
	GWLBillID           uuid.UUID `json:"gwl_bill_id"`
	// FLOW-06 fix: billing period dates are NOT NULL in shadow_bills schema.
	// They are populated from gwl_bills when the tariff service fetches the bill.
	BillingPeriodStart  time.Time `json:"billing_period_start"`
	BillingPeriodEnd    time.Time `json:"billing_period_end"`
	ConsumptionM3       float64   `json:"consumption_m3"`
	CorrectCategory     string    `json:"correct_category"`
	TariffRateID        uuid.UUID `json:"tariff_rate_id"`
	VATConfigID         uuid.UUID `json:"vat_config_id"`

	// Tier breakdown
	Tier1VolumeM3  float64 `json:"tier1_volume_m3"`
	Tier1Rate      float64 `json:"tier1_rate"`
	Tier1AmountGHS float64 `json:"tier1_amount_ghs"`
	Tier2VolumeM3  float64 `json:"tier2_volume_m3"`
	Tier2Rate      float64 `json:"tier2_rate"`
	Tier2AmountGHS float64 `json:"tier2_amount_ghs"`
	ServiceChargeGHS float64 `json:"service_charge_ghs"`

	// Totals
	SubtotalGHS         float64 `json:"subtotal_ghs"`
	VATAmountGHS        float64 `json:"vat_amount_ghs"`
	TotalShadowBillGHS  float64 `json:"total_shadow_bill_ghs"`

	// Variance
	GWLTotalGHS    float64 `json:"gwl_total_ghs"`
	VarianceGHS    float64 `json:"variance_ghs"`
	VariancePct    float64 `json:"variance_pct"`
	IsFlagged      bool    `json:"is_flagged"`
	FlagReason     string  `json:"flag_reason,omitempty"`

	CalculatedAt       time.Time `json:"calculated_at"`
	CalculationVersion string    `json:"calculation_version"`
}

// TariffCalculationRequest is the input to the tariff engine
type TariffCalculationRequest struct {
	AccountID     uuid.UUID `json:"account_id"`
	GWLBillID     uuid.UUID `json:"gwl_bill_id"`
	ConsumptionM3 float64   `json:"consumption_m3"`
	Category      string    `json:"category"`
	GWLTotalGHS   float64   `json:"gwl_total_ghs"`
	BillingDate   time.Time `json:"billing_date"`
}
