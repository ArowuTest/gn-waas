package enums

// AuditStatus represents the lifecycle state of an audit event
type AuditStatus string

const (
	AuditStatusPending            AuditStatus = "PENDING"
	AuditStatusInProgress         AuditStatus = "IN_PROGRESS"
	AuditStatusAwaitingGRA        AuditStatus = "AWAITING_GRA"
	AuditStatusGRAConfirmed       AuditStatus = "GRA_CONFIRMED"
	AuditStatusGRAFailed          AuditStatus = "GRA_FAILED"
	AuditStatusCompleted          AuditStatus = "COMPLETED"
	AuditStatusDisputed           AuditStatus = "DISPUTED"
	AuditStatusEscalated          AuditStatus = "ESCALATED"
	AuditStatusClosed             AuditStatus = "CLOSED"
	AuditStatusPendingCompliance  AuditStatus = "PENDING_COMPLIANCE"
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

// AnomalyType classifies the type of anomaly detected by the Sentinel
// aligned with IWA/AWWA Apparent Loss categories
type AnomalyType string

const (
	// Apparent Loss anomalies (software-detectable)
	AnomalyTypeUnauthorisedConsumption AnomalyType = "UNAUTHORISED_CONSUMPTION"
	AnomalyTypeMeteringInaccuracy      AnomalyType = "METERING_INACCURACY"
	AnomalyTypeDataHandlingError       AnomalyType = "DATA_HANDLING_ERROR"
	AnomalyTypePhantomMeter            AnomalyType = "PHANTOM_METER"
	AnomalyTypeGhostAccount            AnomalyType = "GHOST_ACCOUNT"
	AnomalyTypeCategoryMismatch        AnomalyType = "CATEGORY_MISMATCH"
	AnomalyTypeOutageConsumption       AnomalyType = "OUTAGE_CONSUMPTION"
	AnomalyTypeDistrictImbalance       AnomalyType = "DISTRICT_IMBALANCE"
	AnomalyTypeRationingAnomaly        AnomalyType = "RATIONING_ANOMALY"
	AnomalyTypeShadowBillVariance      AnomalyType = "SHADOW_BILL_VARIANCE"
	AnomalyTypeVATDiscrepancy          AnomalyType = "VAT_DISCREPANCY"
	// Real Loss anomalies (statistical estimates)
	AnomalyTypePhysicalLeak            AnomalyType = "PHYSICAL_LEAK"
	AnomalyTypeNightFlowAnomaly        AnomalyType = "NIGHT_FLOW_ANOMALY"
)

func (a AnomalyType) IsApparentLoss() bool {
	switch a {
	case AnomalyTypeUnauthorisedConsumption, AnomalyTypeMeteringInaccuracy,
		AnomalyTypeDataHandlingError, AnomalyTypePhantomMeter,
		AnomalyTypeGhostAccount, AnomalyTypeCategoryMismatch,
		AnomalyTypeOutageConsumption, AnomalyTypeDistrictImbalance,
		AnomalyTypeRationingAnomaly, AnomalyTypeShadowBillVariance,
		AnomalyTypeVATDiscrepancy:
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

// FraudType classifies the specific fraud pattern detected
type FraudType string

const (
	FraudTypeGhostAccount          FraudType = "GHOST_ACCOUNT"
	FraudTypePhantomMeter          FraudType = "PHANTOM_METER"
	FraudTypeIllegalConnection     FraudType = "ILLEGAL_CONNECTION"
	FraudTypeMeterTampering        FraudType = "METER_TAMPERING"
	FraudTypeReadingCollusion      FraudType = "READING_COLLUSION"
	FraudTypeCategoryDowngrade     FraudType = "CATEGORY_DOWNGRADE"
	FraudTypeVATEvasion            FraudType = "VAT_EVASION"
	FraudTypeOutsideNetworkBilling FraudType = "OUTSIDE_NETWORK_BILLING"
	FraudTypeOutageConsumption     FraudType = "OUTAGE_CONSUMPTION"
)
