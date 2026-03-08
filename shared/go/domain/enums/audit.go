package enums

// AuditStatus represents the lifecycle state of an audit event
type AuditStatus string

const (
	AuditStatusPending           AuditStatus = "PENDING"
	AuditStatusInProgress        AuditStatus = "IN_PROGRESS"
	AuditStatusAwaitingGRA       AuditStatus = "AWAITING_GRA"
	AuditStatusGRAConfirmed      AuditStatus = "GRA_CONFIRMED"
	AuditStatusGRAFailed         AuditStatus = "GRA_FAILED"
	AuditStatusCompleted         AuditStatus = "COMPLETED"
	AuditStatusDisputed          AuditStatus = "DISPUTED"
	AuditStatusEscalated         AuditStatus = "ESCALATED"
	AuditStatusClosed            AuditStatus = "CLOSED"
	AuditStatusPendingCompliance AuditStatus = "PENDING_COMPLIANCE"
)

func (a AuditStatus) IsValid() bool {
	switch a {
	case AuditStatusPending, AuditStatusInProgress, AuditStatusAwaitingGRA,
		AuditStatusGRAConfirmed, AuditStatusGRAFailed, AuditStatusCompleted,
		AuditStatusDisputed, AuditStatusEscalated, AuditStatusClosed,
		AuditStatusPendingCompliance:
		return true
	}
	return false
}

// AnomalyType classifies the type of anomaly detected by the Sentinel.
//
// GN-WAAS is a REVENUE LEAKAGE detection system. Every anomaly type is
// classified into one of three leakage categories:
//
//	REVENUE_LEAKAGE — GWL is under-collecting money (primary mission)
//	COMPLIANCE      — GWL is over-billing or violating PURC rules
//	DATA_QUALITY    — Needs field verification before classification
type AnomalyType string

const (
	// ── REVENUE_LEAKAGE anomalies ─────────────────────────────────────────────
	// GWL is delivering water but not collecting the correct amount.

	// ShadowBillVariance: GWL bill < shadow bill by >threshold%.
	// Cause: meter reading manipulation, tariff misapplication.
	// GHS impact: shadow_bill - gwl_bill per month.
	AnomalyTypeShadowBillVariance AnomalyType = "SHADOW_BILL_VARIANCE"

	// CategoryMismatch: Account registered as RESIDENTIAL but consuming at
	// commercial volumes/patterns. GWL under-collects the tariff difference.
	// GHS impact: (commercial_rate - residential_rate) × monthly_volume.
	AnomalyTypeCategoryMismatch AnomalyType = "CATEGORY_MISMATCH"

	// PhantomMeter: Meter exists in billing system but not physically present.
	// Readings are fabricated, typically low to avoid suspicion.
	// GHS impact: shadow_bill - gwl_bill per month.
	AnomalyTypePhantomMeter AnomalyType = "PHANTOM_METER"

	// DistrictImbalance: Bulk meter input >> sum of all household bills.
	// After subtracting estimated real losses, the remainder is unregistered
	// consumption — addresses consuming water with no billing account.
	// GHS impact: unaccounted_m3 × district_avg_tariff per month.
	AnomalyTypeDistrictImbalance AnomalyType = "DISTRICT_IMBALANCE"

	// UnmeteredConsumption: Field officer confirmed real address, water flowing,
	// but no meter installed. GWL is delivering water and collecting nothing.
	// Fix: install meter, register account, back-bill estimated consumption.
	// GHS impact: district_avg_consumption × tariff × months_unmetered.
	AnomalyTypeUnmeteredConsumption AnomalyType = "UNMETERED_CONSUMPTION"

	// MeteringInaccuracy: Meter is present but recording incorrectly (faulty,
	// tampered, or degraded). Under-recording = revenue leakage.
	AnomalyTypeMeteringInaccuracy AnomalyType = "METERING_INACCURACY"

	// UnauthorisedConsumption: Illegal connection confirmed by field officer.
	// Water is being consumed with no account and no meter.
	AnomalyTypeUnauthorisedConsumption AnomalyType = "UNAUTHORISED_CONSUMPTION"

	// VATDiscrepancy: VAT not applied or applied at wrong rate.
	AnomalyTypeVATDiscrepancy AnomalyType = "VAT_DISCREPANCY"

	// ── DATA_QUALITY anomalies ────────────────────────────────────────────────
	// These require field verification before they can be classified as
	// revenue leakage or dismissed. They do NOT have a GHS value until confirmed.

	// AddressUnverified: Account GPS coordinates fall outside the GWL pipe
	// network boundary. This is a DATA QUALITY screening signal — it could be:
	//   (a) Wrong GPS coordinates in GWL's records (most common)
	//   (b) Real address in a recently connected area (stale GIS data)
	//   (c) Real address with no meter installed → becomes UNMETERED_CONSUMPTION
	//   (d) Fake address created by GWL staff → becomes FRAUDULENT_ACCOUNT
	// Severity: LOW. Action: dispatch field officer to verify.
	// GHS impact: NONE until field verification outcome is recorded.
	AnomalyTypeAddressUnverified AnomalyType = "ADDRESS_UNVERIFIED"

	// DataHandlingError: Billing data inconsistency, likely a data entry error.
	AnomalyTypeDataHandlingError AnomalyType = "DATA_HANDLING_ERROR"

	// RationingAnomaly: Consumption pattern inconsistent with supply schedule.
	AnomalyTypeRationingAnomaly AnomalyType = "RATIONING_ANOMALY"

	// ── COMPLIANCE anomalies ──────────────────────────────────────────────────
	// GWL is over-billing customers or violating PURC rules.
	// These are NOT revenue leakage — they are compliance violations.
	// GN-WAAS reports these to PURC/GRA but they do not generate recovery events.

	// OutageConsumption: GWL billed a customer during a confirmed supply outage.
	// The customer was charged for water they did not receive.
	// This is a PURC compliance violation, not revenue leakage to GWL.
	AnomalyTypeOutageConsumption AnomalyType = "OUTAGE_CONSUMPTION"

	// ── INTERNAL FRAUD (escalation only) ─────────────────────────────────────
	// Confirmed by field verification. Escalated to GWL management + GRA.
	// These are NOT revenue recovery events — they are internal fraud cases.

	// FraudulentAccount: Field officer confirmed address does not exist.
	// A GWL staff member created a fake account and is collecting cash payments.
	// This is GWL internal embezzlement, not billing revenue leakage.
	// Action: escalate to GWL management + GRA. Do NOT create recovery event.
	AnomalyTypeFraudulentAccount AnomalyType = "FRAUDULENT_ACCOUNT"

	// Real Loss anomalies (statistical estimates — physical infrastructure)
	AnomalyTypePhysicalLeak     AnomalyType = "PHYSICAL_LEAK"
	AnomalyTypeNightFlowAnomaly AnomalyType = "NIGHT_FLOW_ANOMALY"
)

// LeakageCategory classifies whether an anomaly represents revenue leakage,
// a compliance violation, or a data quality issue needing verification.
type LeakageCategory string

const (
	LeakageCategoryRevenue    LeakageCategory = "REVENUE_LEAKAGE"
	LeakageCategoryCompliance LeakageCategory = "COMPLIANCE"
	LeakageCategoryDataQuality LeakageCategory = "DATA_QUALITY"
)

// GetLeakageCategory returns the leakage category for an anomaly type.
func (a AnomalyType) GetLeakageCategory() LeakageCategory {
	switch a {
	case AnomalyTypeShadowBillVariance, AnomalyTypeCategoryMismatch,
		AnomalyTypePhantomMeter, AnomalyTypeDistrictImbalance,
		AnomalyTypeUnmeteredConsumption, AnomalyTypeMeteringInaccuracy,
		AnomalyTypeUnauthorisedConsumption, AnomalyTypeVATDiscrepancy:
		return LeakageCategoryRevenue
	case AnomalyTypeOutageConsumption:
		return LeakageCategoryCompliance
	default:
		return LeakageCategoryDataQuality
	}
}

// IsRevenuLeakage returns true if this anomaly type represents money GWL
// should be collecting but isn't.
func (a AnomalyType) IsRevenuLeakage() bool {
	return a.GetLeakageCategory() == LeakageCategoryRevenue
}

// IsApparentLoss returns true for IWA/AWWA apparent loss categories.
// Used for water balance reporting.
func (a AnomalyType) IsApparentLoss() bool {
	switch a {
	case AnomalyTypeUnauthorisedConsumption, AnomalyTypeMeteringInaccuracy,
		AnomalyTypeDataHandlingError, AnomalyTypePhantomMeter,
		AnomalyTypeCategoryMismatch, AnomalyTypeOutageConsumption,
		AnomalyTypeDistrictImbalance, AnomalyTypeRationingAnomaly,
		AnomalyTypeShadowBillVariance, AnomalyTypeVATDiscrepancy,
		AnomalyTypeUnmeteredConsumption, AnomalyTypeAddressUnverified,
		AnomalyTypeFraudulentAccount:
		return true
	}
	return false
}

// AlertLevel represents the severity of a flagged anomaly
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "INFO"
	AlertLevelLow      AlertLevel = "LOW"
	AlertLevelMedium   AlertLevel = "MEDIUM"
	AlertLevelHigh     AlertLevel = "HIGH"
	AlertLevelCritical AlertLevel = "CRITICAL"
)

func (a AlertLevel) IsValid() bool {
	switch a {
	case AlertLevelInfo, AlertLevelLow, AlertLevelMedium, AlertLevelHigh, AlertLevelCritical:
		return true
	}
	return false
}

// FraudType classifies the specific fraud or leakage pattern detected
type FraudType string

const (
	// Revenue leakage fraud types
	FraudTypePhantomMeter          FraudType = "PHANTOM_METER"
	FraudTypeIllegalConnection     FraudType = "ILLEGAL_CONNECTION"
	FraudTypeMeterTampering        FraudType = "METER_TAMPERING"
	FraudTypeReadingCollusion      FraudType = "READING_COLLUSION"
	FraudTypeCategoryDowngrade     FraudType = "CATEGORY_DOWNGRADE"
	FraudTypeVATEvasion            FraudType = "VAT_EVASION"
	FraudTypeUnmeteredAddress      FraudType = "UNMETERED_ADDRESS"
	FraudTypeUnregisteredConnection FraudType = "UNREGISTERED_CONNECTION"

	// Internal GWL fraud (escalation only, not revenue recovery)
	FraudTypeFraudulentAccount     FraudType = "FRAUDULENT_ACCOUNT"
	FraudTypeOutsideNetworkBilling FraudType = "OUTSIDE_NETWORK_BILLING" // legacy, maps to DATA_QUALITY

	// Compliance violations
	FraudTypeOutageConsumption FraudType = "OUTAGE_CONSUMPTION"

	// Deprecated: use AnomalyTypeAddressUnverified instead
	// Kept for backward compatibility with existing DB records
	FraudTypeGhostAccount FraudType = "GHOST_ACCOUNT" // deprecated
)

// FieldJobOutcome represents the structured finding from a field officer visit.
// This drives the auto-escalation logic in the audit handler.
type FieldJobOutcome string

const (
	// Meter outcomes
	FieldJobOutcomeMeterFoundOK       FieldJobOutcome = "METER_FOUND_OK"
	FieldJobOutcomeMeterFoundTampered FieldJobOutcome = "METER_FOUND_TAMPERED"
	FieldJobOutcomeMeterFoundFaulty   FieldJobOutcome = "METER_FOUND_FAULTY"
	// No meter → revenue leakage → recommend installation
	FieldJobOutcomeMeterNotFoundInstall FieldJobOutcome = "METER_NOT_FOUND_INSTALL"

	// Address outcomes
	FieldJobOutcomeAddressValidUnregistered FieldJobOutcome = "ADDRESS_VALID_UNREGISTERED"
	// Address doesn't exist → GWL internal fraud
	FieldJobOutcomeAddressInvalid    FieldJobOutcome = "ADDRESS_INVALID"
	FieldJobOutcomeAddressDemolished FieldJobOutcome = "ADDRESS_DEMOLISHED"
	FieldJobOutcomeAccessDenied      FieldJobOutcome = "ACCESS_DENIED"

	// Category outcomes
	FieldJobOutcomeCategoryConfirmedCorrect  FieldJobOutcome = "CATEGORY_CONFIRMED_CORRECT"
	FieldJobOutcomeCategoryMismatchConfirmed FieldJobOutcome = "CATEGORY_MISMATCH_CONFIRMED"

	// Other
	FieldJobOutcomeDuplicateMeter          FieldJobOutcome = "DUPLICATE_METER"
	FieldJobOutcomeIllegalConnectionFound  FieldJobOutcome = "ILLEGAL_CONNECTION_FOUND"
)

// IsRevenuLeakageOutcome returns true if this field outcome confirms revenue leakage.
// These outcomes trigger auto-creation of a revenue_recovery_event.
func (o FieldJobOutcome) IsRevenuLeakageOutcome() bool {
	switch o {
	case FieldJobOutcomeMeterNotFoundInstall,
		FieldJobOutcomeAddressValidUnregistered,
		FieldJobOutcomeCategoryMismatchConfirmed,
		FieldJobOutcomeMeterFoundTampered,
		FieldJobOutcomeMeterFoundFaulty,
		FieldJobOutcomeDuplicateMeter,
		FieldJobOutcomeIllegalConnectionFound:
		return true
	}
	return false
}

// IsInternalFraudOutcome returns true if this outcome indicates GWL staff fraud.
// These outcomes are escalated to GWL management + GRA, NOT revenue recovery.
func (o FieldJobOutcome) IsInternalFraudOutcome() bool {
	return o == FieldJobOutcomeAddressInvalid
}
