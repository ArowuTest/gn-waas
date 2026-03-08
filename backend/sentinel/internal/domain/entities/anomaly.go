package entities

import (
	"time"

	"github.com/google/uuid"
)

// AnomalyFlag is the core output of the Sentinel engine.
// Fields match the anomaly_flags table schema (migrations 004 + 031).
type AnomalyFlag struct {
	ID                  uuid.UUID              `json:"id"`
	AccountID           *uuid.UUID             `json:"account_id,omitempty"`
	DistrictID          uuid.UUID              `json:"district_id"`
	AnomalyType         string                 `json:"anomaly_type"`
	AlertLevel          string                 `json:"alert_level"`
	FraudType           string                 `json:"fraud_type,omitempty"`
	Title               string                 `json:"title"`
	Description         string                 `json:"description"`
	EstimatedLossGHS    float64                `json:"estimated_loss_ghs,omitempty"`
	BillingPeriodStart  *time.Time             `json:"billing_period_start,omitempty"`
	BillingPeriodEnd    *time.Time             `json:"billing_period_end,omitempty"`
	ShadowBillID        *uuid.UUID             `json:"shadow_bill_id,omitempty"`
	GWLBillID           *uuid.UUID             `json:"gwl_bill_id,omitempty"`
	EvidenceData        map[string]interface{} `json:"evidence_data"`
	Status              string                 `json:"status"`
	SentinelVersion     string                 `json:"sentinel_version"`
	DetectionHash       string                 `json:"detection_hash,omitempty"`
	CreatedAt           time.Time              `json:"created_at"`

	// Revenue leakage fields (migration 031)
	// These are written to dedicated DB columns so the pipeline view and
	// dashboard can aggregate GHS values without parsing JSONB.
	LeakageCategory       string  `json:"leakage_category,omitempty"`
	MonthlyLeakageGHS     float64 `json:"monthly_leakage_ghs,omitempty"`
	AnnualisedLeakageGHS  float64 `json:"annualised_leakage_ghs,omitempty"`
}

// GWLBillingRecord mirrors the GWL billing data
type GWLBillingRecord struct {
	ID                  uuid.UUID `json:"id"`
	GWLBillID           string    `json:"gwl_bill_id"`
	AccountID           uuid.UUID `json:"account_id"`
	DistrictID          uuid.UUID `json:"district_id"`
	BillingPeriodStart  time.Time `json:"billing_period_start"`
	BillingPeriodEnd    time.Time `json:"billing_period_end"`
	ConsumptionM3       float64   `json:"consumption_m3"`
	GWLCategory         string    `json:"gwl_category"`
	GWLAmountGHS        float64   `json:"gwl_amount_ghs"`
	GWLVatGHS           float64   `json:"gwl_vat_ghs"`
	GWLTotalGHS         float64   `json:"gwl_total_ghs"`
}

// ShadowBillResult is the output from the tariff engine
type ShadowBillResult struct {
	ID                 uuid.UUID `json:"id"`
	CorrectCategory    string    `json:"correct_category"`
	TotalShadowBillGHS float64   `json:"total_shadow_bill_ghs"`
	VATAmountGHS       float64   `json:"vat_amount_ghs"`
	VarianceGHS        float64   `json:"variance_ghs"`
	VariancePct        float64   `json:"variance_pct"`
}

// WaterAccount is the account entity used by Sentinel
type WaterAccount struct {
	ID                    uuid.UUID  `json:"id"`
	GWLAccountNumber      string     `json:"gwl_account_number"`
	DistrictID            uuid.UUID  `json:"district_id"`
	Category              string     `json:"category"`
	GPSLatitude           float64    `json:"gps_latitude"`
	GPSLongitude          float64    `json:"gps_longitude"`
	IsWithinNetwork       *bool      `json:"is_within_network"`
	NetworkCheckDate      *time.Time `json:"network_check_date"`
	MonthlyAvgConsumption float64    `json:"monthly_avg_consumption"`
	IsPhantomFlagged      bool       `json:"is_phantom_flagged"`
}

// District is the district entity used by Sentinel
type District struct {
	ID           uuid.UUID `json:"id"`
	DistrictCode string    `json:"district_code"`
	DistrictName string    `json:"district_name"`
	Region       string    `json:"region"`
}

// SupplySchedule represents the water supply schedule for an area
type SupplySchedule struct {
	DistrictID        uuid.UUID  `json:"district_id"`
	SupplyDaysPerWeek int        `json:"supply_days_per_week"`
	EffectiveFrom     time.Time  `json:"effective_from"`
	EffectiveTo       *time.Time `json:"effective_to"`
}

// WaterBalanceSummary is the read model for the latest water balance record
type WaterBalanceSummary struct {
	DistrictID                  interface{} `json:"district_id"`
	PeriodStart                 interface{} `json:"period_start"`
	PeriodEnd                   interface{} `json:"period_end"`
	SystemInputM3               float64     `json:"system_input_m3"`
	TotalAuthorisedM3           float64     `json:"total_authorised_m3"`
	TotalApparentLossesM3       float64     `json:"total_apparent_losses_m3"`
	TotalRealLossesM3           float64     `json:"total_real_losses_m3"`
	NRWPercent                  float64     `json:"nrw_percent"`
	ILI                         float64     `json:"ili"`
	IWAGrade                    string      `json:"iwa_grade"`
	EstimatedRevenueRecoveryGHS float64     `json:"estimated_revenue_recovery_ghs"`
	DataConfidenceScore         float64     `json:"data_confidence_score"`
	ComputedAt                  interface{} `json:"computed_at"`
}

// TariffRate holds a single PURC tariff tier loaded from the database.
// Sentinel loads these at startup to avoid hardcoded rates.
type TariffRate struct {
	Category       string  `json:"category"`
	TierName       string  `json:"tier_name"`
	MinVolumeM3    float64 `json:"min_volume_m3"`
	MaxVolumeM3    float64 `json:"max_volume_m3"` // 0 = no upper limit
	RatePerM3      float64 `json:"rate_per_m3"`
	ServiceCharge  float64 `json:"service_charge_ghs"`
}

// TariffConfig holds all active tariff rates and the VAT rate.
// Loaded from the database at sentinel startup; refreshed every 24h.
type TariffConfig struct {
	Rates      []TariffRate `json:"rates"`
	VATRate    float64      `json:"vat_rate"`    // e.g. 20.0 (percent)
	LoadedAt   time.Time    `json:"loaded_at"`
}

// BlendedRateForCategory returns the blended (average) rate per m3 for a category.
// Used for leakage estimates when tiered calculation is not possible.
func (tc *TariffConfig) BlendedRateForCategory(category string) float64 {
	var total, count float64
	for _, r := range tc.Rates {
		if r.Category == category {
			total += r.RatePerM3
			count++
		}
	}
	if count == 0 {
		// Fallback: PURC 2026 residential tier-2 (conservative estimate)
		return 10.8320
	}
	return total / count
}

// VATMultiplier returns 1 + (VATRate/100), e.g. 1.20 for 20% VAT.
func (tc *TariffConfig) VATMultiplier() float64 {
	if tc.VATRate <= 0 {
		return 1.20 // PURC 2026 default
	}
	return 1.0 + (tc.VATRate / 100.0)
}
