package entities

import (
	"time"

	"github.com/google/uuid"
)

// AnomalyFlag is the core output of the Sentinel engine
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
	ID                   uuid.UUID  `json:"id"`
	GWLAccountNumber     string     `json:"gwl_account_number"`
	DistrictID           uuid.UUID  `json:"district_id"`
	Category             string     `json:"category"`
	GPSLatitude          float64    `json:"gps_latitude"`
	GPSLongitude         float64    `json:"gps_longitude"`
	IsWithinNetwork      *bool      `json:"is_within_network"`
	NetworkCheckDate     *time.Time `json:"network_check_date"`
	MonthlyAvgConsumption float64   `json:"monthly_avg_consumption"`
	IsPhantomFlagged     bool       `json:"is_phantom_flagged"`
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
	DistrictID        uuid.UUID `json:"district_id"`
	SupplyDaysPerWeek int       `json:"supply_days_per_week"`
	EffectiveFrom     time.Time `json:"effective_from"`
	EffectiveTo       *time.Time `json:"effective_to"`
}
