// GN-WAAS Field Officer App — Domain Models
// Mirrors the backend Go types exactly

enum FieldJobStatus {
  queued,
  dispatched,
  enRoute,
  onSite,
  completed,
  failed,
  sos;

  static FieldJobStatus fromString(String s) {
    switch (s.toUpperCase()) {
      case 'QUEUED':      return FieldJobStatus.queued;
      case 'DISPATCHED':  return FieldJobStatus.dispatched;
      case 'EN_ROUTE':    return FieldJobStatus.enRoute;
      case 'ON_SITE':     return FieldJobStatus.onSite;
      case 'COMPLETED':   return FieldJobStatus.completed;
      case 'FAILED':      return FieldJobStatus.failed;
      case 'SOS':         return FieldJobStatus.sos;
      default:            return FieldJobStatus.queued;
    }
  }

  String toApiString() {
    switch (this) {
      case FieldJobStatus.queued:      return 'QUEUED';
      case FieldJobStatus.dispatched:  return 'DISPATCHED';
      case FieldJobStatus.enRoute:     return 'EN_ROUTE';
      case FieldJobStatus.onSite:      return 'ON_SITE';
      case FieldJobStatus.completed:   return 'COMPLETED';
      case FieldJobStatus.failed:      return 'FAILED';
      case FieldJobStatus.sos:         return 'SOS';
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
}

// ─── User ─────────────────────────────────────────────────────────────────────

class User {
  final String id;
  final String email;
  final String fullName;
  final String role;
  final String? badgeNumber;
  final String? districtId;

  const User({
    required this.id,
    required this.email,
    required this.fullName,
    required this.role,
    this.badgeNumber,
    this.districtId,
  });

  factory User.fromJson(Map<String, dynamic> json) => User(
    id:          json['id'] as String,
    email:       json['email'] as String,
    fullName:    json['full_name'] as String,
    role:        json['role'] as String,
    badgeNumber: json['badge_number'] as String?,
    districtId:  json['district_id'] as String?,
  );

  Map<String, dynamic> toJson() => {
    'id':           id,
    'email':        email,
    'full_name':    fullName,
    'role':         role,
    'badge_number': badgeNumber,
    'district_id':  districtId,
  };
}

// ─── FieldJob ─────────────────────────────────────────────────────────────────

class FieldJob {
  final String id;
  final String? jobReference;
  final String auditEventId;
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
  final double? estimatedVarianceGhs;

  FieldJob({
    required this.id,
    this.jobReference,
    required this.auditEventId,
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
  });

  factory FieldJob.fromJson(Map<String, dynamic> json) => FieldJob(
    id:                   json['id'] as String,
    jobReference:         json['job_reference'] as String?,
    auditEventId:         json['audit_event_id'] as String? ?? '',
    accountNumber:        json['account_number'] as String,
    customerName:         json['customer_name'] as String,
    address:              json['address'] as String,
    gpsLat:               (json['gps_lat'] as num).toDouble(),
    gpsLng:               (json['gps_lng'] as num).toDouble(),
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
  };
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
    'ocr_status':       ocrStatus.name.toUpperCase(),
    'officer_notes':    officerNotes,
    'gps_lat':          gpsLat,
    'gps_lng':          gpsLng,
    'gps_accuracy_m':   gpsAccuracyM,
    'photo_urls':       photoUrls,
    'photo_hashes':     photoHashes,
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

// ─── SyncStats ────────────────────────────────────────────────────────────────

class SyncStats {
  final int cachedJobs;
  final int pendingSubmissions;
  final int pendingPhotos;
  final DateTime? lastSyncAt;

  const SyncStats({
    required this.cachedJobs,
    required this.pendingSubmissions,
    required this.pendingPhotos,
    this.lastSyncAt,
  });
}
