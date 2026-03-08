// GN-WAAS Field Officer App — Domain Models
// Mirrors the backend Go types exactly (migrations 004 + 031 + 032)

/// FieldJobStatus mirrors the field_job_status PostgreSQL enum exactly.
/// Values: QUEUED, ASSIGNED, DISPATCHED, EN_ROUTE, ON_SITE,
///         COMPLETED, FAILED, CANCELLED, ESCALATED, SOS, OUTCOME_RECORDED
enum FieldJobStatus {
  queued,
  assigned,
  dispatched,
  enRoute,
  onSite,
  completed,
  failed,
  cancelled,
  escalated,
  sos,
  outcomeRecorded; // Added in migration 031

  static FieldJobStatus fromString(String s) {
    switch (s.toUpperCase()) {
      case 'QUEUED':           return FieldJobStatus.queued;
      case 'ASSIGNED':         return FieldJobStatus.assigned;
      case 'DISPATCHED':       return FieldJobStatus.dispatched;
      case 'EN_ROUTE':         return FieldJobStatus.enRoute;
      case 'ON_SITE':          return FieldJobStatus.onSite;
      case 'COMPLETED':        return FieldJobStatus.completed;
      case 'FAILED':           return FieldJobStatus.failed;
      case 'CANCELLED':        return FieldJobStatus.cancelled;
      case 'ESCALATED':        return FieldJobStatus.escalated;
      case 'SOS':              return FieldJobStatus.sos;
      case 'OUTCOME_RECORDED': return FieldJobStatus.outcomeRecorded;
      default:                 return FieldJobStatus.queued;
    }
  }

  String toApiString() {
    switch (this) {
      case FieldJobStatus.queued:          return 'QUEUED';
      case FieldJobStatus.assigned:        return 'ASSIGNED';
      case FieldJobStatus.dispatched:      return 'DISPATCHED';
      case FieldJobStatus.enRoute:         return 'EN_ROUTE';
      case FieldJobStatus.onSite:          return 'ON_SITE';
      case FieldJobStatus.completed:       return 'COMPLETED';
      case FieldJobStatus.failed:          return 'FAILED';
      case FieldJobStatus.cancelled:       return 'CANCELLED';
      case FieldJobStatus.escalated:       return 'ESCALATED';
      case FieldJobStatus.sos:             return 'SOS';
      case FieldJobStatus.outcomeRecorded: return 'OUTCOME_RECORDED';
    }
  }

  /// Human-readable label for display in the UI
  String get displayLabel {
    switch (this) {
      case FieldJobStatus.queued:          return 'Queued';
      case FieldJobStatus.assigned:        return 'Assigned';
      case FieldJobStatus.dispatched:      return 'Dispatched';
      case FieldJobStatus.enRoute:         return 'En Route';
      case FieldJobStatus.onSite:          return 'On Site';
      case FieldJobStatus.completed:       return 'Completed';
      case FieldJobStatus.failed:          return 'Failed';
      case FieldJobStatus.cancelled:       return 'Cancelled';
      case FieldJobStatus.escalated:       return 'Escalated';
      case FieldJobStatus.sos:             return 'SOS';
      case FieldJobStatus.outcomeRecorded: return 'Outcome Recorded';
    }
  }
}

/// FieldJobOutcome mirrors the field_job_outcome PostgreSQL enum (migration 031).
/// Structured outcomes from field officer visits — drives auto-escalation logic.
enum FieldJobOutcome {
  // Meter-related outcomes
  meterFoundOk,               // Meter present, reading taken, all good
  meterFoundTampered,         // Meter present but physically tampered
  meterFoundFaulty,           // Meter present but not recording correctly
  meterNotFoundInstall,       // No meter, address valid → recommend installation
  // Address-related outcomes
  addressValidUnregistered,   // Real address, consuming water, no GWL account
  addressInvalid,             // Address does not exist → fraudulent account
  addressDemolished,          // Property demolished, account should be closed
  accessDenied,               // Could not access property, reschedule
  // Category outcomes
  categoryConfirmedCorrect,   // Registered category matches actual use
  categoryMismatchConfirmed,  // Confirmed commercial use, billed as residential
  // Other
  duplicateMeter,             // Two accounts sharing one physical meter
  illegalConnectionFound;     // Illegal tap/bypass found

  static FieldJobOutcome? fromString(String? s) {
    if (s == null) return null;
    switch (s.toUpperCase()) {
      case 'METER_FOUND_OK':               return FieldJobOutcome.meterFoundOk;
      case 'METER_FOUND_TAMPERED':         return FieldJobOutcome.meterFoundTampered;
      case 'METER_FOUND_FAULTY':           return FieldJobOutcome.meterFoundFaulty;
      case 'METER_NOT_FOUND_INSTALL':      return FieldJobOutcome.meterNotFoundInstall;
      case 'ADDRESS_VALID_UNREGISTERED':   return FieldJobOutcome.addressValidUnregistered;
      case 'ADDRESS_INVALID':              return FieldJobOutcome.addressInvalid;
      case 'ADDRESS_DEMOLISHED':           return FieldJobOutcome.addressDemolished;
      case 'ACCESS_DENIED':                return FieldJobOutcome.accessDenied;
      case 'CATEGORY_CONFIRMED_CORRECT':   return FieldJobOutcome.categoryConfirmedCorrect;
      case 'CATEGORY_MISMATCH_CONFIRMED':  return FieldJobOutcome.categoryMismatchConfirmed;
      case 'DUPLICATE_METER':              return FieldJobOutcome.duplicateMeter;
      case 'ILLEGAL_CONNECTION_FOUND':     return FieldJobOutcome.illegalConnectionFound;
      default:                             return null;
    }
  }

  String toApiString() {
    switch (this) {
      case FieldJobOutcome.meterFoundOk:              return 'METER_FOUND_OK';
      case FieldJobOutcome.meterFoundTampered:        return 'METER_FOUND_TAMPERED';
      case FieldJobOutcome.meterFoundFaulty:          return 'METER_FOUND_FAULTY';
      case FieldJobOutcome.meterNotFoundInstall:      return 'METER_NOT_FOUND_INSTALL';
      case FieldJobOutcome.addressValidUnregistered:  return 'ADDRESS_VALID_UNREGISTERED';
      case FieldJobOutcome.addressInvalid:            return 'ADDRESS_INVALID';
      case FieldJobOutcome.addressDemolished:         return 'ADDRESS_DEMOLISHED';
      case FieldJobOutcome.accessDenied:              return 'ACCESS_DENIED';
      case FieldJobOutcome.categoryConfirmedCorrect:  return 'CATEGORY_CONFIRMED_CORRECT';
      case FieldJobOutcome.categoryMismatchConfirmed: return 'CATEGORY_MISMATCH_CONFIRMED';
      case FieldJobOutcome.duplicateMeter:            return 'DUPLICATE_METER';
      case FieldJobOutcome.illegalConnectionFound:    return 'ILLEGAL_CONNECTION_FOUND';
    }
  }

  /// Human-readable label for display in the UI
  String get displayLabel {
    switch (this) {
      case FieldJobOutcome.meterFoundOk:              return 'Meter Found — OK';
      case FieldJobOutcome.meterFoundTampered:        return 'Meter Tampered';
      case FieldJobOutcome.meterFoundFaulty:          return 'Meter Faulty';
      case FieldJobOutcome.meterNotFoundInstall:      return 'No Meter — Install Required';
      case FieldJobOutcome.addressValidUnregistered:  return 'Address Valid — Unregistered';
      case FieldJobOutcome.addressInvalid:            return 'Address Does Not Exist';
      case FieldJobOutcome.addressDemolished:         return 'Property Demolished';
      case FieldJobOutcome.accessDenied:              return 'Access Denied — Reschedule';
      case FieldJobOutcome.categoryConfirmedCorrect:  return 'Category Correct';
      case FieldJobOutcome.categoryMismatchConfirmed: return 'Category Mismatch Confirmed';
      case FieldJobOutcome.duplicateMeter:            return 'Duplicate Meter Found';
      case FieldJobOutcome.illegalConnectionFound:    return 'Illegal Connection Found';
    }
  }

  /// Whether this outcome confirms revenue leakage (triggers pipeline advance)
  bool get confirmsLeakage {
    switch (this) {
      case FieldJobOutcome.meterNotFoundInstall:
      case FieldJobOutcome.addressValidUnregistered:
      case FieldJobOutcome.categoryMismatchConfirmed:
      case FieldJobOutcome.illegalConnectionFound:
      case FieldJobOutcome.meterFoundTampered:
      case FieldJobOutcome.meterFoundFaulty:
      case FieldJobOutcome.duplicateMeter:
        return true;
      default:
        return false;
    }
  }

  /// Whether this outcome requires escalation to management
  bool get requiresEscalation {
    return this == FieldJobOutcome.addressInvalid; // Fraudulent account
  }
}

/// LeakageCategory mirrors the leakage_category PostgreSQL enum (migration 031).
enum LeakageCategory {
  revenueLeakage,
  compliance,
  dataQuality;

  static LeakageCategory fromString(String? s) {
    switch (s?.toUpperCase()) {
      case 'REVENUE_LEAKAGE': return LeakageCategory.revenueLeakage;
      case 'COMPLIANCE':      return LeakageCategory.compliance;
      case 'DATA_QUALITY':    return LeakageCategory.dataQuality;
      default:                return LeakageCategory.dataQuality;
    }
  }

  String toApiString() {
    switch (this) {
      case LeakageCategory.revenueLeakage: return 'REVENUE_LEAKAGE';
      case LeakageCategory.compliance:     return 'COMPLIANCE';
      case LeakageCategory.dataQuality:    return 'DATA_QUALITY';
    }
  }

  String get displayLabel {
    switch (this) {
      case LeakageCategory.revenueLeakage: return 'Revenue Leakage';
      case LeakageCategory.compliance:     return 'Compliance';
      case LeakageCategory.dataQuality:    return 'Data Quality';
    }
  }
}

enum AlertLevel {
  critical,
  high,
  medium,
  low,
  info;

  static AlertLevel fromString(String s) {
    switch (s.toUpperCase()) {
      case 'CRITICAL': return AlertLevel.critical;
      case 'HIGH':     return AlertLevel.high;
      case 'MEDIUM':   return AlertLevel.medium;
      case 'LOW':      return AlertLevel.low;
      default:         return AlertLevel.info;
    }
  }

  String toApiString() => name.toUpperCase();
}

enum OcrStatus {
  pending,
  processing,
  success,
  failed,
  manual;

  static OcrStatus fromString(String s) {
    switch (s.toUpperCase()) {
      case 'PENDING':    return OcrStatus.pending;
      case 'PROCESSING': return OcrStatus.processing;
      case 'SUCCESS':    return OcrStatus.success;
      case 'FAILED':     return OcrStatus.failed;
      case 'MANUAL':     return OcrStatus.manual;
      default:           return OcrStatus.pending;
    }
  }

  String toApiString() => name.toUpperCase();
}

// ─── User ─────────────────────────────────────────────────────────────────────

class User {
  final String id;
  final String email;
  final String fullName;
  final String role;
  final String? badgeNumber;
  final String? districtId;
  final String? districtName;
  final String? phoneNumber;
  final DateTime? lastLoginAt;

  const User({
    required this.id,
    required this.email,
    required this.fullName,
    required this.role,
    this.badgeNumber,
    this.districtId,
    this.districtName,
    this.phoneNumber,
    this.lastLoginAt,
  });

  factory User.fromJson(Map<String, dynamic> json) => User(
    id:           json['id'] as String,
    email:        json['email'] as String,
    fullName:     (json['full_name'] ?? json['name'] ?? '') as String,
    role:         json['role'] as String? ?? 'FIELD_OFFICER',
    badgeNumber:  json['badge_number'] as String?,
    districtId:   json['district_id'] as String?,
    districtName: json['district_name'] as String?,
    phoneNumber:  json['phone_number'] as String?,
    lastLoginAt:  json['last_login_at'] != null
                    ? DateTime.tryParse(json['last_login_at'] as String)
                    : null,
  );

  Map<String, dynamic> toJson() => {
    'id':            id,
    'email':         email,
    'full_name':     fullName,
    'role':          role,
    'badge_number':  badgeNumber,
    'district_id':   districtId,
    'district_name': districtName,
    'phone_number':  phoneNumber,
  };
}

// ─── FieldJob ─────────────────────────────────────────────────────────────────

class FieldJob {
  final String id;
  final String? jobReference;
  final String? auditEventId;   // nullable — not all jobs have an audit event
  final String accountNumber;
  final String customerName;
  final String address;
  final double gpsLat;
  final double gpsLng;
  final String anomalyType;
  final AlertLevel alertLevel;
  FieldJobStatus status;
  final DateTime? scheduledAt;
  final DateTime? dispatchedAt;
  final String? notes;

  // Legacy field — kept for backward compatibility
  final double? estimatedVarianceGhs;

  // Revenue leakage fields (migration 031)
  // These are the primary financial metrics shown to the officer
  final double? monthlyLeakageGhs;
  final double? annualisedLeakageGhs;
  final LeakageCategory? leakageCategory;

  // Field outcome fields (migration 031)
  // Populated after the officer records what they found on-site
  FieldJobOutcome? outcome;
  String? outcomeNotes;
  bool? meterFound;
  bool? addressConfirmed;
  String? recommendedAction;
  DateTime? outcomeRecordedAt;

  FieldJob({
    required this.id,
    this.jobReference,
    this.auditEventId,
    required this.accountNumber,
    required this.customerName,
    required this.address,
    required this.gpsLat,
    required this.gpsLng,
    required this.anomalyType,
    required this.alertLevel,
    required this.status,
    this.scheduledAt,
    this.dispatchedAt,
    this.notes,
    this.estimatedVarianceGhs,
    this.monthlyLeakageGhs,
    this.annualisedLeakageGhs,
    this.leakageCategory,
    this.outcome,
    this.outcomeNotes,
    this.meterFound,
    this.addressConfirmed,
    this.recommendedAction,
    this.outcomeRecordedAt,
  });

  factory FieldJob.fromJson(Map<String, dynamic> json) => FieldJob(
    id:                   json['id'] as String,
    jobReference:         json['job_reference'] as String?,
    auditEventId:         json['audit_event_id'] as String?,
    accountNumber:        json['account_number'] as String? ?? '',
    customerName:         json['customer_name'] as String? ?? '',
    address:              json['address'] as String? ?? '',
    gpsLat:               (json['gps_lat'] as num?)?.toDouble() ?? 0.0,
    gpsLng:               (json['gps_lng'] as num?)?.toDouble() ?? 0.0,
    anomalyType:          json['anomaly_type'] as String? ?? 'UNKNOWN',
    alertLevel:           AlertLevel.fromString(json['alert_level'] as String? ?? 'MEDIUM'),
    status:               FieldJobStatus.fromString(json['status'] as String? ?? 'QUEUED'),
    scheduledAt:          json['scheduled_at'] != null
                            ? DateTime.tryParse(json['scheduled_at'] as String)
                            : null,
    dispatchedAt:         json['dispatched_at'] != null
                            ? DateTime.tryParse(json['dispatched_at'] as String)
                            : null,
    notes:                json['notes'] as String?,
    estimatedVarianceGhs: json['estimated_variance_ghs'] != null
                            ? (json['estimated_variance_ghs'] as num).toDouble()
                            : null,
    // Revenue leakage fields (migration 031)
    monthlyLeakageGhs:    json['monthly_leakage_ghs'] != null
                            ? (json['monthly_leakage_ghs'] as num).toDouble()
                            : null,
    annualisedLeakageGhs: json['annualised_leakage_ghs'] != null
                            ? (json['annualised_leakage_ghs'] as num).toDouble()
                            : null,
    leakageCategory:      json['leakage_category'] != null
                            ? LeakageCategory.fromString(json['leakage_category'] as String)
                            : null,
    // Field outcome fields (migration 031)
    outcome:              FieldJobOutcome.fromString(json['outcome'] as String?),
    outcomeNotes:         json['outcome_notes'] as String?,
    meterFound:           json['meter_found'] as bool?,
    addressConfirmed:     json['address_confirmed'] as bool?,
    recommendedAction:    json['recommended_action'] as String?,
    outcomeRecordedAt:    json['outcome_recorded_at'] != null
                            ? DateTime.tryParse(json['outcome_recorded_at'] as String)
                            : null,
  );

  Map<String, dynamic> toJson() => {
    'id':                     id,
    'job_reference':          jobReference,
    'audit_event_id':         auditEventId,
    'account_number':         accountNumber,
    'customer_name':          customerName,
    'address':                address,
    'gps_lat':                gpsLat,
    'gps_lng':                gpsLng,
    'anomaly_type':           anomalyType,
    'alert_level':            alertLevel.toApiString(),
    'status':                 status.toApiString(),
    'scheduled_at':           scheduledAt?.toIso8601String(),
    'dispatched_at':          dispatchedAt?.toIso8601String(),
    'notes':                  notes,
    'estimated_variance_ghs': estimatedVarianceGhs,
    'monthly_leakage_ghs':    monthlyLeakageGhs,
    'annualised_leakage_ghs': annualisedLeakageGhs,
    'leakage_category':       leakageCategory?.toApiString(),
    'outcome':                outcome?.toApiString(),
    'outcome_notes':          outcomeNotes,
    'meter_found':            meterFound,
    'address_confirmed':      addressConfirmed,
    'recommended_action':     recommendedAction,
    'outcome_recorded_at':    outcomeRecordedAt?.toIso8601String(),
  };

  /// The primary GHS amount to display — prefers monthly_leakage_ghs over legacy estimated_variance_ghs
  double? get primaryLeakageGhs => monthlyLeakageGhs ?? estimatedVarianceGhs;

  /// Whether the officer has already recorded an outcome for this job
  bool get hasOutcome => outcome != null;

  /// Whether this job needs an outcome recorded (completed but no outcome yet)
  bool get needsOutcome =>
      (status == FieldJobStatus.completed || status == FieldJobStatus.onSite) &&
      outcome == null;

  /// Human-readable anomaly type description for field officers
  String get anomalyTypeDescription {
    switch (anomalyType.toUpperCase()) {
      case 'SHADOW_BILL_VARIANCE':
        return 'Billing discrepancy — shadow bill differs from GWL bill';
      case 'CATEGORY_MISMATCH':
        return 'Category mismatch — may be commercial billed as residential';
      case 'PHANTOM_METER':
        return 'Phantom meter — meter registered but GPS outside network';
      case 'DISTRICT_IMBALANCE':
        return 'District imbalance — bulk meter vs household billing gap';
      case 'ADDRESS_UNVERIFIED':
        return 'Address unverified — GPS outside network boundary, needs field check';
      case 'UNMETERED_CONSUMPTION':
        return 'Unmetered consumption — water flowing, no meter, no billing';
      case 'FRAUDULENT_ACCOUNT':
        return 'Fraudulent account — address confirmed non-existent';
      case 'OUTAGE_CONSUMPTION':
        return 'Outage billing — customer billed during supply outage (PURC violation)';
      case 'METERING_INACCURACY':
        return 'Meter inaccuracy — meter reading inconsistent with consumption';
      case 'UNAUTHORISED_CONSUMPTION':
        return 'Unauthorised consumption — illegal connection or bypass';
      case 'VAT_DISCREPANCY':
        return 'VAT discrepancy — incorrect VAT applied to bill';
      default:
        return anomalyType.replaceAll('_', ' ').toLowerCase();
    }
  }
}

// ─── MeterPhoto ───────────────────────────────────────────────────────────────

class MeterPhoto {
  final String localPath;
  final String hash;
  final double gpsLat;
  final double gpsLng;
  final double gpsAccuracyM;
  final DateTime capturedAt;
  final bool withinFence;
  String? remoteUrl;

  MeterPhoto({
    required this.localPath,
    required this.hash,
    required this.gpsLat,
    required this.gpsLng,
    required this.gpsAccuracyM,
    required this.capturedAt,
    required this.withinFence,
    this.remoteUrl,
  });
}

// ─── OcrResult ────────────────────────────────────────────────────────────────

class OcrResult {
  final double readingM3;
  final double confidence;
  final OcrStatus status;
  final String rawText;

  const OcrResult({
    required this.readingM3,
    required this.confidence,
    required this.status,
    required this.rawText,
  });

  factory OcrResult.fromJson(Map<String, dynamic> json) => OcrResult(
    readingM3:  (json['reading_m3'] as num).toDouble(),
    confidence: (json['confidence'] as num).toDouble(),
    status:     OcrStatus.fromString(json['status'] as String),
    rawText:    json['raw_text'] as String,
  );
}

// ─── JobSubmission ────────────────────────────────────────────────────────────

class JobSubmission {
  final String jobId;
  final double ocrReadingM3;
  final double ocrConfidence;
  final OcrStatus ocrStatus;
  final String officerNotes;
  final double gpsLat;
  final double gpsLng;
  final double gpsAccuracyM;
  final List<String> photoUrls;
  final List<String> photoHashes;

  const JobSubmission({
    required this.jobId,
    required this.ocrReadingM3,
    required this.ocrConfidence,
    required this.ocrStatus,
    required this.officerNotes,
    required this.gpsLat,
    required this.gpsLng,
    required this.gpsAccuracyM,
    required this.photoUrls,
    required this.photoHashes,
  });

  Map<String, dynamic> toJson() => {
    'job_id':           jobId,
    'ocr_reading_m3':   ocrReadingM3,
    'ocr_confidence':   ocrConfidence,
    'ocr_status':       ocrStatus.toApiString(), // use toApiString() not name.toUpperCase()
    'officer_notes':    officerNotes,
    'gps_lat':          gpsLat,
    'gps_lng':          gpsLng,
    'gps_accuracy_m':   gpsAccuracyM,
    'photo_urls':       photoUrls,
    'photo_hashes':     photoHashes,
  };
}

// ─── FieldJobOutcomeRequest ───────────────────────────────────────────────────
// Sent to PATCH /field-jobs/:id/outcome

class FieldJobOutcomeRequest {
  final FieldJobOutcome outcome;
  final String? outcomeNotes;
  final bool? meterFound;
  final bool? addressConfirmed;
  final String? recommendedAction;
  final double? estimatedMonthlyM3; // For UNMETERED_CONSUMPTION back-billing

  const FieldJobOutcomeRequest({
    required this.outcome,
    this.outcomeNotes,
    this.meterFound,
    this.addressConfirmed,
    this.recommendedAction,
    this.estimatedMonthlyM3,
  });

  Map<String, dynamic> toJson() => {
    'outcome':               outcome.toApiString(),
    if (outcomeNotes != null && outcomeNotes!.isNotEmpty)
      'outcome_notes':       outcomeNotes,
    if (meterFound != null)
      'meter_found':         meterFound,
    if (addressConfirmed != null)
      'address_confirmed':   addressConfirmed,
    if (recommendedAction != null && recommendedAction!.isNotEmpty)
      'recommended_action':  recommendedAction,
    if (estimatedMonthlyM3 != null && estimatedMonthlyM3! > 0)
      'estimated_monthly_m3': estimatedMonthlyM3,
  };
}

// ─── PendingSubmission (offline queue) ───────────────────────────────────────

class PendingSubmission {
  final String id;
  final String jobId;
  final JobSubmission submission;
  final List<String> photoUris;
  String status; // PENDING | UPLOADING | FAILED | DONE
  int retryCount;
  String? lastError;
  DateTime? attemptedAt;

  PendingSubmission({
    required this.id,
    required this.jobId,
    required this.submission,
    required this.photoUris,
    this.status = 'PENDING',
    this.retryCount = 0,
    this.lastError,
    this.attemptedAt,
  });
}

// ─── PendingOutcome (offline outcome queue) ───────────────────────────────────
// When the officer records an outcome while offline, it's queued here.

class PendingOutcome {
  final String id;
  final String jobId;
  final FieldJobOutcomeRequest outcomeRequest;
  String status; // PENDING | FAILED | DONE
  int retryCount;
  String? lastError;
  DateTime createdAt;

  PendingOutcome({
    required this.id,
    required this.jobId,
    required this.outcomeRequest,
    this.status = 'PENDING',
    this.retryCount = 0,
    this.lastError,
    DateTime? createdAt,
  }) : createdAt = createdAt ?? DateTime.now();
}

// ─── SyncStats ────────────────────────────────────────────────────────────────

class SyncStats {
  final int cachedJobs;
  final int pendingSubmissions;
  final int pendingPhotos;
  final int pendingOutcomes;
  final DateTime? lastSyncAt;

  const SyncStats({
    required this.cachedJobs,
    required this.pendingSubmissions,
    required this.pendingPhotos,
    this.pendingOutcomes = 0,
    this.lastSyncAt,
  });
}

// ─── MobileConfig ─────────────────────────────────────────────────────────────
// Defined here so it can be used by providers without circular imports.
// Full implementation is in remote_config_service.dart.

class MobileConfig {
  final double geofenceRadiusM;
  final bool requireBiometric;
  final bool blindAuditDefault;
  final bool requireSurroundingsPhoto;
  final int maxPhotoAgeMinutes;
  final double ocrConflictTolerancePct;
  final int syncIntervalSeconds;
  final int maxJobsPerOfficer;
  final String appMinVersion;
  final String appLatestVersion;
  final bool forceUpdate;
  final bool maintenanceMode;
  final String maintenanceMessage;

  const MobileConfig({
    this.geofenceRadiusM          = 100.0,
    this.requireBiometric         = true,
    this.blindAuditDefault        = true,
    this.requireSurroundingsPhoto = true,
    this.maxPhotoAgeMinutes       = 5,
    this.ocrConflictTolerancePct  = 2.0,
    this.syncIntervalSeconds      = 30,
    this.maxJobsPerOfficer        = 5,
    this.appMinVersion            = '1.0.0',
    this.appLatestVersion         = '1.0.0',
    this.forceUpdate              = false,
    this.maintenanceMode          = false,
    this.maintenanceMessage       = '',
  });

  static const MobileConfig defaults = MobileConfig();

  factory MobileConfig.fromJson(Map<String, dynamic> json) => MobileConfig(
    geofenceRadiusM:          (json['geofence_radius_m']          as num?)?.toDouble() ?? 100.0,
    requireBiometric:         json['require_biometric']           as bool?  ?? true,
    blindAuditDefault:        json['blind_audit_default']         as bool?  ?? true,
    requireSurroundingsPhoto: json['require_surroundings_photo']  as bool?  ?? true,
    maxPhotoAgeMinutes:       json['max_photo_age_minutes']       as int?   ?? 5,
    ocrConflictTolerancePct:  (json['ocr_conflict_tolerance_pct'] as num?)?.toDouble() ?? 2.0,
    syncIntervalSeconds:      json['sync_interval_seconds']       as int?   ?? 30,
    maxJobsPerOfficer:        json['max_jobs_per_officer']        as int?   ?? 5,
    appMinVersion:            json['app_min_version']             as String? ?? '1.0.0',
    appLatestVersion:         json['app_latest_version']          as String? ?? '1.0.0',
    forceUpdate:              json['force_update']                as bool?  ?? false,
    maintenanceMode:          json['maintenance_mode']            as bool?  ?? false,
    maintenanceMessage:       json['maintenance_message']         as String? ?? '',
  );

  Map<String, dynamic> toJson() => {
    'geofence_radius_m':           geofenceRadiusM,
    'require_biometric':           requireBiometric,
    'blind_audit_default':         blindAuditDefault,
    'require_surroundings_photo':  requireSurroundingsPhoto,
    'max_photo_age_minutes':       maxPhotoAgeMinutes,
    'ocr_conflict_tolerance_pct':  ocrConflictTolerancePct,
    'sync_interval_seconds':       syncIntervalSeconds,
    'max_jobs_per_officer':        maxJobsPerOfficer,
    'app_min_version':             appMinVersion,
    'app_latest_version':          appLatestVersion,
    'force_update':                forceUpdate,
    'maintenance_mode':            maintenanceMode,
    'maintenance_message':         maintenanceMessage,
  };
}
