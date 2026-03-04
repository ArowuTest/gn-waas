package domain

import (
	"time"

	"github.com/google/uuid"
)

// ============================================================
// AUDIT EVENT DOMAIN ENTITIES
// ============================================================

// AuditEvent represents a full audit lifecycle record
type AuditEvent struct {
	ID                    uuid.UUID  `db:"id" json:"id"`
	AuditReference        string     `db:"audit_reference" json:"audit_reference"`
	AccountID             uuid.UUID  `db:"account_id" json:"account_id"`
	DistrictID            uuid.UUID  `db:"district_id" json:"district_id"`
	AnomalyFlagID         *uuid.UUID `db:"anomaly_flag_id" json:"anomaly_flag_id,omitempty"`
	Status                string     `db:"status" json:"status"`
	AssignedOfficerID     *uuid.UUID `db:"assigned_officer_id" json:"assigned_officer_id,omitempty"`
	AssignedSupervisorID  *uuid.UUID `db:"assigned_supervisor_id" json:"assigned_supervisor_id,omitempty"`
	AssignedAt            *time.Time `db:"assigned_at" json:"assigned_at,omitempty"`
	DueDate               *time.Time `db:"due_date" json:"due_date,omitempty"`
	FieldJobID            *uuid.UUID `db:"field_job_id" json:"field_job_id,omitempty"`
	MeterPhotoURL         *string    `db:"meter_photo_url" json:"meter_photo_url,omitempty"`
	SurroundingsPhotoURL  *string    `db:"surroundings_photo_url" json:"surroundings_photo_url,omitempty"`
	OCRReadingValue       *float64   `db:"ocr_reading_value" json:"ocr_reading_value,omitempty"`
	ManualReadingValue    *float64   `db:"manual_reading_value" json:"manual_reading_value,omitempty"`
	OCRStatus             *string    `db:"ocr_status" json:"ocr_status,omitempty"`
	GPSLatitude           *float64   `db:"gps_latitude" json:"gps_latitude,omitempty"`
	GPSLongitude          *float64   `db:"gps_longitude" json:"gps_longitude,omitempty"`
	GPSPrecisionM         *float64   `db:"gps_precision_m" json:"gps_precision_m,omitempty"`
	TamperEvidenceDetected bool      `db:"tamper_evidence_detected" json:"tamper_evidence_detected"`
	TamperEvidenceURL     *string    `db:"tamper_evidence_url" json:"tamper_evidence_url,omitempty"`
	GRAStatus             string     `db:"gra_status" json:"gra_status"`
	GRASDCId              *string    `db:"gra_sdc_id" json:"gra_sdc_id,omitempty"`
	GRAQRCodeURL          *string    `db:"gra_qr_code_url" json:"gra_qr_code_url,omitempty"`
	GRASignedAt           *time.Time `db:"gra_signed_at" json:"gra_signed_at,omitempty"`
	GWLBilledGHS          *float64   `db:"gwl_billed_ghs" json:"gwl_billed_ghs,omitempty"`
	ShadowBillGHS         *float64   `db:"shadow_bill_ghs" json:"shadow_bill_ghs,omitempty"`
	VariancePct           *float64   `db:"variance_pct" json:"variance_pct,omitempty"`
	ConfirmedLossGHS      *float64   `db:"confirmed_loss_ghs" json:"confirmed_loss_ghs,omitempty"`
	RecoveryInvoiceGHS    *float64   `db:"recovery_invoice_ghs" json:"recovery_invoice_ghs,omitempty"`
	SuccessFeeGHS         *float64   `db:"success_fee_ghs" json:"success_fee_ghs,omitempty"`
	IsLocked              bool       `db:"is_locked" json:"is_locked"`
	LockedAt              *time.Time `db:"locked_at" json:"locked_at,omitempty"`
	Notes                 *string    `db:"notes" json:"notes,omitempty"`
	CreatedAt             time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at" json:"updated_at"`
}

// FieldJob represents a field officer dispatch job
type FieldJob struct {
	ID                    uuid.UUID  `db:"id" json:"id"`
	JobReference          string     `db:"job_reference" json:"job_reference"`
	AuditEventID          *uuid.UUID `db:"audit_event_id" json:"audit_event_id,omitempty"`
	AccountID             uuid.UUID  `db:"account_id" json:"account_id"`
	DistrictID            uuid.UUID  `db:"district_id" json:"district_id"`
	AssignedOfficerID     *uuid.UUID `db:"assigned_officer_id" json:"assigned_officer_id,omitempty"`
	Status                string     `db:"status" json:"status"`
	IsBlindAudit          bool       `db:"is_blind_audit" json:"is_blind_audit"`
	TargetGPSLat          float64    `db:"target_gps_lat" json:"target_gps_lat"`
	TargetGPSLng          float64    `db:"target_gps_lng" json:"target_gps_lng"`
	GPSFenceRadiusM       float64    `db:"gps_fence_radius_m" json:"gps_fence_radius_m"`
	DispatchedAt          *time.Time `db:"dispatched_at" json:"dispatched_at,omitempty"`
	ArrivedAt             *time.Time `db:"arrived_at" json:"arrived_at,omitempty"`
	CompletedAt           *time.Time `db:"completed_at" json:"completed_at,omitempty"`
	OfficerGPSLat         *float64   `db:"officer_gps_lat" json:"officer_gps_lat,omitempty"`
	OfficerGPSLng         *float64   `db:"officer_gps_lng" json:"officer_gps_lng,omitempty"`
	GPSVerified           *bool      `db:"gps_verified" json:"gps_verified,omitempty"`
	BiometricVerified     bool       `db:"biometric_verified" json:"biometric_verified"`
	Priority              int        `db:"priority" json:"priority"`
	RequiresSecurityEscort bool      `db:"requires_security_escort" json:"requires_security_escort"`
	SOSTriggered          bool       `db:"sos_triggered" json:"sos_triggered"`
	SOSTriggeredAt        *time.Time `db:"sos_triggered_at" json:"sos_triggered_at,omitempty"`
	Notes                 *string    `db:"notes" json:"notes,omitempty"`
	CreatedAt             time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at" json:"updated_at"`
}

// FieldJobEvidence holds the evidence submitted by a field officer after meter capture
type FieldJobEvidence struct {
	OCRReadingValue float64
	OCRConfidence   float64
	OCRStatus       string
	Notes           string
	GPSLat          float64
	GPSLng          float64
	GPSAccuracyM    float64
	PhotoURLs       []string
	PhotoHashes     []string
}

// User represents a GN-WAAS system user
type User struct {
	ID           uuid.UUID  `db:"id" json:"id"`
	Email        string     `db:"email" json:"email"`
	FullName     string     `db:"full_name" json:"full_name"`
	PhoneNumber  string     `db:"phone_number" json:"phone_number"`
	Role         string     `db:"role" json:"role"`
	Status       string     `db:"status" json:"status"`
	Organisation string     `db:"organisation" json:"organisation"`
	EmployeeID   string     `db:"employee_id" json:"employee_id"`
	DistrictID   *uuid.UUID `db:"district_id" json:"district_id,omitempty"`
	KeycloakID   *string    `db:"keycloak_id" json:"keycloak_id,omitempty"`
	LastLoginAt  *time.Time `db:"last_login_at" json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
}

// District represents a GWL service district
type District struct {
	ID                  uuid.UUID `db:"id" json:"id"`
	DistrictCode        string    `db:"district_code" json:"district_code"`
	DistrictName        string    `db:"district_name" json:"district_name"`
	Region              string    `db:"region" json:"region"`
	PopulationEstimate  int       `db:"population_estimate" json:"population_estimate"`
	TotalConnections    int       `db:"total_connections" json:"total_connections"`
	SupplyStatus        string    `db:"supply_status" json:"supply_status"`
	ZoneType            string    `db:"zone_type" json:"zone_type"`
	GeographicZone      string    `db:"geographic_zone" json:"geographic_zone"`
	LossRatioPct        *float64  `db:"loss_ratio_pct" json:"loss_ratio_pct,omitempty"`
	DataConfidenceGrade *int      `db:"data_confidence_grade" json:"data_confidence_grade,omitempty"`
	IsPilotDistrict     bool      `db:"is_pilot_district" json:"is_pilot_district"`
	IsActive            bool      `db:"is_active" json:"is_active"`
	// GPS coordinates for DMA map rendering (nullable — not all districts have GPS set)
	GPSLatitude         *float64  `db:"gps_latitude" json:"gps_latitude,omitempty"`
	GPSLongitude        *float64  `db:"gps_longitude" json:"gps_longitude,omitempty"`
	CreatedAt           time.Time `db:"created_at" json:"created_at"`
}

// WaterAccount represents a GWL customer account
type WaterAccount struct {
	ID                   uuid.UUID  `db:"id" json:"id"`
	GWLAccountNumber     string     `db:"gwl_account_number" json:"gwl_account_number"`
	AccountHolderName    string     `db:"account_holder_name" json:"account_holder_name"`
	AccountHolderTIN     *string    `db:"account_holder_tin" json:"account_holder_tin,omitempty"`
	Category             string     `db:"category" json:"category"`
	Status               string     `db:"status" json:"status"`
	DistrictID           uuid.UUID  `db:"district_id" json:"district_id"`
	MeterNumber          string     `db:"meter_number" json:"meter_number"`
	AddressLine1         string     `db:"address_line1" json:"address_line1"`
	GPSLatitude          float64    `db:"gps_latitude" json:"gps_latitude"`
	GPSLongitude         float64    `db:"gps_longitude" json:"gps_longitude"`
	IsWithinNetwork      *bool      `db:"is_within_network" json:"is_within_network,omitempty"`
	MonthlyAvgConsumption float64   `db:"monthly_avg_consumption" json:"monthly_avg_consumption"`
	IsPhantomFlagged     bool       `db:"is_phantom_flagged" json:"is_phantom_flagged"`
	CreatedAt            time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt            time.Time  `db:"updated_at" json:"updated_at"`
}

// SystemConfig represents a configurable system parameter
type SystemConfig struct {
	ID          uuid.UUID `db:"id" json:"id"`
	ConfigKey   string    `db:"config_key" json:"config_key"`
	ConfigValue string    `db:"config_value" json:"config_value"`
	ConfigType  string    `db:"config_type" json:"config_type"`
	Description string    `db:"description" json:"description"`
	Category    string    `db:"category" json:"category"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// RecoveryRecord represents a revenue recovery event
type RecoveryRecord struct {
	ID                  uuid.UUID `db:"id" json:"id"`
	AuditEventID        uuid.UUID `db:"audit_event_id" json:"audit_event_id"`
	AccountID           uuid.UUID `db:"account_id" json:"account_id"`
	DistrictID          uuid.UUID `db:"district_id" json:"district_id"`
	OriginalGWLBillGHS  float64   `db:"original_gwl_bill_ghs" json:"original_gwl_bill_ghs"`
	CorrectedBillGHS    float64   `db:"corrected_bill_ghs" json:"corrected_bill_ghs"`
	RecoveredAmountGHS  float64   `db:"recovered_amount_ghs" json:"recovered_amount_ghs"`
	RecoveryDate        time.Time `db:"recovery_date" json:"recovery_date"`
	SuccessFeeRatePct   float64   `db:"success_fee_rate_pct" json:"success_fee_rate_pct"`
	SuccessFeeGHS       float64   `db:"success_fee_ghs" json:"success_fee_ghs"`
	FeePaid             bool      `db:"fee_paid" json:"fee_paid"`
	CreatedAt           time.Time `db:"created_at" json:"created_at"`
}
