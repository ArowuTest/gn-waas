package enums

// DistrictZoneType classifies a district by its loss profile
// Used for heatmap visualisation and audit prioritisation
type DistrictZoneType string

const (
	DistrictZoneRed    DistrictZoneType = "RED"    // High theft: Loss > 40%
	DistrictZoneYellow DistrictZoneType = "YELLOW" // Physical leaks: Night-flow high, bills consistent
	DistrictZoneGreen  DistrictZoneType = "GREEN"  // Audit-verified: Bulk ≈ Billed
	DistrictZoneGrey   DistrictZoneType = "GREY"   // Insufficient data
)

// SupplyStatus represents the current water supply state of a district
type SupplyStatus string

const (
	SupplyStatusNormal    SupplyStatus = "NORMAL"
	SupplyStatusReduced   SupplyStatus = "REDUCED"   // Rationing in effect
	SupplyStatusOutage    SupplyStatus = "OUTAGE"     // No supply
	SupplyStatusMaintenance SupplyStatus = "MAINTENANCE"
	SupplyStatusUnknown   SupplyStatus = "UNKNOWN"
)

// WaterBalanceComponent maps to IWA/AWWA Water Balance categories
type WaterBalanceComponent string

const (
	// Authorised Consumption
	WBCBilledMetered       WaterBalanceComponent = "BILLED_METERED"
	WBCBilledUnmetered     WaterBalanceComponent = "BILLED_UNMETERED"
	WBCUnbilledMetered     WaterBalanceComponent = "UNBILLED_METERED"
	WBCUnbilledUnmetered   WaterBalanceComponent = "UNBILLED_UNMETERED"

	// Apparent Losses (NRW - software detectable)
	WBCUnauthorisedConsumption WaterBalanceComponent = "UNAUTHORISED_CONSUMPTION"
	WBCMeteringInaccuracies    WaterBalanceComponent = "METERING_INACCURACIES"
	WBCDataHandlingErrors      WaterBalanceComponent = "DATA_HANDLING_ERRORS"

	// Real Losses (NRW - physical)
	WBCMainLeakage         WaterBalanceComponent = "MAIN_LEAKAGE"
	WBCStorageTankOverflow WaterBalanceComponent = "STORAGE_TANK_OVERFLOW"
	WBCServiceConnLeakage  WaterBalanceComponent = "SERVICE_CONNECTION_LEAKAGE"
)

// GRAComplianceStatus tracks the GRA VSDC API signing state
type GRAComplianceStatus string

const (
	GRAStatusPending   GRAComplianceStatus = "PENDING"
	GRAStatusSigned    GRAComplianceStatus = "SIGNED"
	GRAStatusFailed    GRAComplianceStatus = "FAILED"
	GRAStatusRetrying  GRAComplianceStatus = "RETRYING"
	GRAStatusExempt    GRAComplianceStatus = "EXEMPT" // Below VAT threshold
)

// DataConfidenceGrade maps to AWWA Grading Matrix (1-10 scale)
type DataConfidenceGrade int

const (
	DataConfidenceUngraded DataConfidenceGrade = 0
	DataConfidenceVeryLow  DataConfidenceGrade = 1
	DataConfidenceLow      DataConfidenceGrade = 3
	DataConfidenceMedium   DataConfidenceGrade = 5
	DataConfidenceHigh     DataConfidenceGrade = 7
	DataConfidenceVeryHigh DataConfidenceGrade = 9
	DataConfidenceExcellent DataConfidenceGrade = 10
)

// FieldJobStatus tracks the lifecycle of a field audit job
type FieldJobStatus string

const (
	FieldJobStatusAssigned    FieldJobStatus = "ASSIGNED"
	FieldJobStatusDispatched  FieldJobStatus = "DISPATCHED"
	FieldJobStatusEnRoute     FieldJobStatus = "EN_ROUTE"
	FieldJobStatusOnSite      FieldJobStatus = "ON_SITE"
	FieldJobStatusCompleted   FieldJobStatus = "COMPLETED"
	FieldJobStatusFailed      FieldJobStatus = "FAILED"
	FieldJobStatusCancelled   FieldJobStatus = "CANCELLED"
	FieldJobStatusEscalated   FieldJobStatus = "ESCALATED"
)

// OCRStatus tracks the result of OCR meter reading
type OCRStatus string

const (
	OCRStatusSuccess    OCRStatus = "SUCCESS"
	OCRStatusFailed     OCRStatus = "FAILED"
	OCRStatusBlurry     OCRStatus = "BLURRY"
	OCRStatusTampered   OCRStatus = "TAMPERED"
	OCRStatusConflict   OCRStatus = "CONFLICT" // OCR != manual entry
	OCRStatusPending    OCRStatus = "PENDING"
)
