// GN-WAAS Flutter — Model Unit Tests
// Covers all enums and models including migration 031/032 additions

import 'package:flutter_test/flutter_test.dart';
import 'package:gnwaas_field_officer/models/models.dart';

void main() {
  // ─── FieldJobStatus ────────────────────────────────────────────────────────

  group('FieldJobStatus', () {
    test('fromString parses all valid statuses', () {
      expect(FieldJobStatus.fromString('QUEUED'),           FieldJobStatus.queued);
      expect(FieldJobStatus.fromString('ASSIGNED'),         FieldJobStatus.assigned);
      expect(FieldJobStatus.fromString('DISPATCHED'),       FieldJobStatus.dispatched);
      expect(FieldJobStatus.fromString('EN_ROUTE'),         FieldJobStatus.enRoute);
      expect(FieldJobStatus.fromString('ON_SITE'),          FieldJobStatus.onSite);
      expect(FieldJobStatus.fromString('COMPLETED'),        FieldJobStatus.completed);
      expect(FieldJobStatus.fromString('FAILED'),           FieldJobStatus.failed);
      expect(FieldJobStatus.fromString('SOS'),              FieldJobStatus.sos);
      expect(FieldJobStatus.fromString('OUTCOME_RECORDED'), FieldJobStatus.outcomeRecorded);
    });

    test('fromString defaults to queued for unknown values', () {
      expect(FieldJobStatus.fromString('UNKNOWN'), FieldJobStatus.queued);
      expect(FieldJobStatus.fromString(''),        FieldJobStatus.queued);
    });

    test('toApiString returns correct uppercase strings', () {
      expect(FieldJobStatus.queued.toApiString(),          'QUEUED');
      expect(FieldJobStatus.dispatched.toApiString(),      'DISPATCHED');
      expect(FieldJobStatus.enRoute.toApiString(),         'EN_ROUTE');
      expect(FieldJobStatus.onSite.toApiString(),          'ON_SITE');
      expect(FieldJobStatus.completed.toApiString(),       'COMPLETED');
      expect(FieldJobStatus.failed.toApiString(),          'FAILED');
      expect(FieldJobStatus.sos.toApiString(),             'SOS');
      expect(FieldJobStatus.outcomeRecorded.toApiString(), 'OUTCOME_RECORDED');
    });

    test('round-trip: fromString(toApiString()) is identity', () {
      for (final status in FieldJobStatus.values) {
        expect(FieldJobStatus.fromString(status.toApiString()), status);
      }
    });

    test('displayLabel is human-readable', () {
      expect(FieldJobStatus.enRoute.displayLabel,         'En Route');
      expect(FieldJobStatus.onSite.displayLabel,          'On Site');
      expect(FieldJobStatus.outcomeRecorded.displayLabel, 'Outcome Recorded');
    });
  });

  // ─── FieldJobOutcome ───────────────────────────────────────────────────────

  group('FieldJobOutcome', () {
    test('fromString parses all valid outcomes', () {
      expect(FieldJobOutcome.fromString('METER_FOUND_OK'),              FieldJobOutcome.meterFoundOk);
      expect(FieldJobOutcome.fromString('METER_FOUND_TAMPERED'),        FieldJobOutcome.meterFoundTampered);
      expect(FieldJobOutcome.fromString('METER_FOUND_FAULTY'),          FieldJobOutcome.meterFoundFaulty);
      expect(FieldJobOutcome.fromString('METER_NOT_FOUND_INSTALL'),     FieldJobOutcome.meterNotFoundInstall);
      expect(FieldJobOutcome.fromString('ADDRESS_VALID_UNREGISTERED'),  FieldJobOutcome.addressValidUnregistered);
      expect(FieldJobOutcome.fromString('ADDRESS_INVALID'),             FieldJobOutcome.addressInvalid);
      expect(FieldJobOutcome.fromString('ADDRESS_DEMOLISHED'),          FieldJobOutcome.addressDemolished);
      expect(FieldJobOutcome.fromString('ACCESS_DENIED'),               FieldJobOutcome.accessDenied);
      expect(FieldJobOutcome.fromString('CATEGORY_CONFIRMED_CORRECT'),  FieldJobOutcome.categoryConfirmedCorrect);
      expect(FieldJobOutcome.fromString('CATEGORY_MISMATCH_CONFIRMED'), FieldJobOutcome.categoryMismatchConfirmed);
      expect(FieldJobOutcome.fromString('DUPLICATE_METER'),             FieldJobOutcome.duplicateMeter);
      expect(FieldJobOutcome.fromString('ILLEGAL_CONNECTION_FOUND'),    FieldJobOutcome.illegalConnectionFound);
    });

    test('fromString returns null for unknown/null values', () {
      expect(FieldJobOutcome.fromString(null),      isNull);
      expect(FieldJobOutcome.fromString('UNKNOWN'), isNull);
      expect(FieldJobOutcome.fromString(''),        isNull);
    });

    test('round-trip: fromString(toApiString()) is identity', () {
      for (final outcome in FieldJobOutcome.values) {
        expect(FieldJobOutcome.fromString(outcome.toApiString()), outcome);
      }
    });

    test('confirmsLeakage is true for revenue-impacting outcomes', () {
      expect(FieldJobOutcome.meterFoundTampered.confirmsLeakage,       isTrue);
      expect(FieldJobOutcome.meterFoundFaulty.confirmsLeakage,         isTrue);
      expect(FieldJobOutcome.meterNotFoundInstall.confirmsLeakage,     isTrue);
      expect(FieldJobOutcome.addressValidUnregistered.confirmsLeakage, isTrue);
      expect(FieldJobOutcome.categoryMismatchConfirmed.confirmsLeakage,isTrue);
      expect(FieldJobOutcome.illegalConnectionFound.confirmsLeakage,   isTrue);
      expect(FieldJobOutcome.duplicateMeter.confirmsLeakage,           isTrue);
    });

    test('confirmsLeakage is false for non-leakage outcomes', () {
      expect(FieldJobOutcome.meterFoundOk.confirmsLeakage,            isFalse);
      expect(FieldJobOutcome.accessDenied.confirmsLeakage,            isFalse);
      expect(FieldJobOutcome.categoryConfirmedCorrect.confirmsLeakage, isFalse);
      expect(FieldJobOutcome.addressDemolished.confirmsLeakage,       isFalse);
    });

    test('requiresEscalation is true only for ADDRESS_INVALID', () {
      expect(FieldJobOutcome.addressInvalid.requiresEscalation, isTrue);
      for (final outcome in FieldJobOutcome.values) {
        if (outcome != FieldJobOutcome.addressInvalid) {
          expect(outcome.requiresEscalation, isFalse,
              reason: '${outcome.toApiString()} should not require escalation');
        }
      }
    });

    test('displayLabel is non-empty for all outcomes', () {
      for (final outcome in FieldJobOutcome.values) {
        expect(outcome.displayLabel, isNotEmpty);
      }
    });
  });

  // ─── LeakageCategory ──────────────────────────────────────────────────────

  group('LeakageCategory', () {
    test('fromString parses all valid categories', () {
      expect(LeakageCategory.fromString('REVENUE_LEAKAGE'), LeakageCategory.revenueLeakage);
      expect(LeakageCategory.fromString('COMPLIANCE'),      LeakageCategory.compliance);
      expect(LeakageCategory.fromString('DATA_QUALITY'),    LeakageCategory.dataQuality);
    });

    test('fromString defaults to dataQuality for unknown', () {
      expect(LeakageCategory.fromString(null),      LeakageCategory.dataQuality);
      expect(LeakageCategory.fromString('UNKNOWN'), LeakageCategory.dataQuality);
    });

    test('round-trip', () {
      for (final cat in LeakageCategory.values) {
        expect(LeakageCategory.fromString(cat.toApiString()), cat);
      }
    });
  });

  // ─── AlertLevel ───────────────────────────────────────────────────────────

  group('AlertLevel', () {
    test('fromString parses all valid levels', () {
      expect(AlertLevel.fromString('CRITICAL'), AlertLevel.critical);
      expect(AlertLevel.fromString('HIGH'),     AlertLevel.high);
      expect(AlertLevel.fromString('MEDIUM'),   AlertLevel.medium);
      expect(AlertLevel.fromString('LOW'),      AlertLevel.low);
      expect(AlertLevel.fromString('INFO'),     AlertLevel.info);
    });

    test('fromString defaults to info for unknown', () {
      expect(AlertLevel.fromString('UNKNOWN'), AlertLevel.info);
    });

    test('toApiString returns uppercase', () {
      expect(AlertLevel.critical.toApiString(), 'CRITICAL');
      expect(AlertLevel.high.toApiString(),     'HIGH');
      expect(AlertLevel.medium.toApiString(),   'MEDIUM');
      expect(AlertLevel.low.toApiString(),      'LOW');
      expect(AlertLevel.info.toApiString(),     'INFO');
    });
  });

  // ─── OcrStatus ────────────────────────────────────────────────────────────

  group('OcrStatus', () {
    test('fromString parses all valid statuses', () {
      expect(OcrStatus.fromString('PENDING'),    OcrStatus.pending);
      expect(OcrStatus.fromString('PROCESSING'), OcrStatus.processing);
      expect(OcrStatus.fromString('SUCCESS'),    OcrStatus.success);
      expect(OcrStatus.fromString('FAILED'),     OcrStatus.failed);
      expect(OcrStatus.fromString('MANUAL'),     OcrStatus.manual);
    });

    test('toApiString returns uppercase', () {
      expect(OcrStatus.success.toApiString(), 'SUCCESS');
      expect(OcrStatus.manual.toApiString(),  'MANUAL');
    });
  });

  // ─── User ─────────────────────────────────────────────────────────────────

  group('User', () {
    test('fromJson parses all fields', () {
      final json = {
        'id':           'user-123',
        'email':        'officer@gwl.gov.gh',
        'full_name':    'Kwame Mensah',
        'role':         'FIELD_OFFICER',
        'badge_number': 'GWL-001',
        'district_id':  'district-accra',
      };
      final user = User.fromJson(json);
      expect(user.id,          'user-123');
      expect(user.email,       'officer@gwl.gov.gh');
      expect(user.fullName,    'Kwame Mensah');
      expect(user.role,        'FIELD_OFFICER');
      expect(user.badgeNumber, 'GWL-001');
      expect(user.districtId,  'district-accra');
    });

    test('fromJson handles optional null fields', () {
      final json = {
        'id':        'user-456',
        'email':     'test@test.com',
        'full_name': 'Test User',
        'role':      'FIELD_OFFICER',
      };
      final user = User.fromJson(json);
      expect(user.badgeNumber, isNull);
      expect(user.districtId,  isNull);
      expect(user.lastLoginAt, isNull);
    });

    test('fromJson normalises name field', () {
      final json = {
        'id':    'user-789',
        'email': 'test@test.com',
        'name':  'Legacy Name Field', // backend may return 'name' not 'full_name'
        'role':  'FIELD_OFFICER',
      };
      final user = User.fromJson(json);
      expect(user.fullName, 'Legacy Name Field');
    });
  });

  // ─── FieldJob ─────────────────────────────────────────────────────────────

  group('FieldJob', () {
    test('fromJson parses all v3 fields', () {
      final json = {
        'id':                     'job-001',
        'job_reference':          'GWL-2026-001',
        'audit_event_id':         'audit-001',
        'account_number':         'ACC-001',
        'customer_name':          'Kwame Mensah',
        'address':                '1 Independence Ave, Accra',
        'gps_lat':                5.6037,
        'gps_lng':                -0.1870,
        'anomaly_type':           'SHADOW_BILL_VARIANCE',
        'alert_level':            'HIGH',
        'status':                 'ON_SITE',
        'estimated_variance_ghs': 150.0,
        // v3 fields
        'monthly_leakage_ghs':    250.50,
        'annualised_leakage_ghs': 3006.0,
        'leakage_category':       'REVENUE_LEAKAGE',
        'outcome':                'METER_FOUND_TAMPERED',
        'outcome_notes':          'Meter seal broken',
        'meter_found':            true,
        'address_confirmed':      true,
        'recommended_action':     'Replace meter and seal',
        'outcome_recorded_at':    '2026-03-08T10:30:00Z',
      };
      final job = FieldJob.fromJson(json);

      expect(job.id,                   'job-001');
      expect(job.jobReference,         'GWL-2026-001');
      expect(job.auditEventId,         'audit-001');
      expect(job.status,               FieldJobStatus.onSite);
      expect(job.alertLevel,           AlertLevel.high);
      // v3 fields
      expect(job.monthlyLeakageGhs,    250.50);
      expect(job.annualisedLeakageGhs, 3006.0);
      expect(job.leakageCategory,      LeakageCategory.revenueLeakage);
      expect(job.outcome,              FieldJobOutcome.meterFoundTampered);
      expect(job.outcomeNotes,         'Meter seal broken');
      expect(job.meterFound,           isTrue);
      expect(job.addressConfirmed,     isTrue);
      expect(job.recommendedAction,    'Replace meter and seal');
      expect(job.outcomeRecordedAt,    isNotNull);
    });

    test('fromJson handles null v3 fields gracefully', () {
      final json = {
        'id':            'job-002',
        'account_number': 'ACC-002',
        'customer_name':  'Test Customer',
        'address':        '2 Test St',
        'gps_lat':        5.6037,
        'gps_lng':        -0.1870,
        'alert_level':    'MEDIUM',
        'status':         'QUEUED',
      };
      final job = FieldJob.fromJson(json);

      expect(job.monthlyLeakageGhs,    isNull);
      expect(job.annualisedLeakageGhs, isNull);
      expect(job.leakageCategory,      isNull);
      expect(job.outcome,              isNull);
      expect(job.meterFound,           isNull);
      expect(job.addressConfirmed,     isNull);
      expect(job.auditEventId,         isNull);
    });

    test('primaryLeakageGhs prefers monthlyLeakageGhs over estimatedVarianceGhs', () {
      final job = FieldJob(
        id:                   'job-003',
        accountNumber:        'ACC-003',
        customerName:         'Test',
        address:              '3 Test St',
        gpsLat:               5.0,
        gpsLng:               -0.1,
        anomalyType:          'BILLING_VARIANCE',
        alertLevel:           AlertLevel.medium,
        status:               FieldJobStatus.queued,
        estimatedVarianceGhs: 100.0,
        monthlyLeakageGhs:    250.0,
      );
      expect(job.primaryLeakageGhs, 250.0);
    });

    test('primaryLeakageGhs falls back to estimatedVarianceGhs when monthly is null', () {
      final job = FieldJob(
        id:                   'job-004',
        accountNumber:        'ACC-004',
        customerName:         'Test',
        address:              '4 Test St',
        gpsLat:               5.0,
        gpsLng:               -0.1,
        anomalyType:          'BILLING_VARIANCE',
        alertLevel:           AlertLevel.medium,
        status:               FieldJobStatus.queued,
        estimatedVarianceGhs: 100.0,
      );
      expect(job.primaryLeakageGhs, 100.0);
    });

    test('hasOutcome is true when outcome is set', () {
      final job = FieldJob(
        id:           'job-005',
        accountNumber: 'ACC-005',
        customerName:  'Test',
        address:       '5 Test St',
        gpsLat:        5.0,
        gpsLng:        -0.1,
        anomalyType:   'BILLING_VARIANCE',
        alertLevel:    AlertLevel.medium,
        status:        FieldJobStatus.completed,
        outcome:       FieldJobOutcome.meterFoundOk,
      );
      expect(job.hasOutcome, isTrue);
    });

    test('needsOutcome is true for completed job without outcome', () {
      final job = FieldJob(
        id:           'job-006',
        accountNumber: 'ACC-006',
        customerName:  'Test',
        address:       '6 Test St',
        gpsLat:        5.0,
        gpsLng:        -0.1,
        anomalyType:   'BILLING_VARIANCE',
        alertLevel:    AlertLevel.medium,
        status:        FieldJobStatus.completed,
      );
      expect(job.needsOutcome, isTrue);
    });

    test('needsOutcome is false when outcome already recorded', () {
      final job = FieldJob(
        id:           'job-007',
        accountNumber: 'ACC-007',
        customerName:  'Test',
        address:       '7 Test St',
        gpsLat:        5.0,
        gpsLng:        -0.1,
        anomalyType:   'BILLING_VARIANCE',
        alertLevel:    AlertLevel.medium,
        status:        FieldJobStatus.outcomeRecorded,
        outcome:       FieldJobOutcome.meterFoundTampered,
      );
      expect(job.needsOutcome, isFalse);
    });

    test('anomalyTypeDescription returns human-readable text', () {
      final job = FieldJob(
        id:           'job-008',
        accountNumber: 'ACC-008',
        customerName:  'Test',
        address:       '8 Test St',
        gpsLat:        5.0,
        gpsLng:        -0.1,
        anomalyType:   'ADDRESS_UNVERIFIED',
        alertLevel:    AlertLevel.medium,
        status:        FieldJobStatus.queued,
      );
      expect(job.anomalyTypeDescription, contains('GPS'));
    });
  });

  // ─── FieldJobOutcomeRequest ───────────────────────────────────────────────

  group('FieldJobOutcomeRequest', () {
    test('toJson includes all non-null fields', () {
      final request = FieldJobOutcomeRequest(
        outcome:            FieldJobOutcome.meterFoundTampered,
        outcomeNotes:       'Seal broken',
        meterFound:         true,
        addressConfirmed:   true,
        recommendedAction:  'Replace meter',
        estimatedMonthlyM3: 12.5,
      );
      final json = request.toJson();
      expect(json['outcome'],               'METER_FOUND_TAMPERED');
      expect(json['outcome_notes'],         'Seal broken');
      expect(json['meter_found'],           isTrue);
      expect(json['address_confirmed'],     isTrue);
      expect(json['recommended_action'],    'Replace meter');
      expect(json['estimated_monthly_m3'],  12.5);
    });

    test('toJson omits null/empty optional fields', () {
      final request = FieldJobOutcomeRequest(
        outcome: FieldJobOutcome.accessDenied,
      );
      final json = request.toJson();
      expect(json['outcome'],              'ACCESS_DENIED');
      expect(json.containsKey('outcome_notes'),        isFalse);
      expect(json.containsKey('meter_found'),          isFalse);
      expect(json.containsKey('address_confirmed'),    isFalse);
      expect(json.containsKey('recommended_action'),   isFalse);
      expect(json.containsKey('estimated_monthly_m3'), isFalse);
    });
  });

  // ─── JobSubmission ────────────────────────────────────────────────────────

  group('JobSubmission', () {
    test('toJson uses toApiString() for ocr_status', () {
      final submission = JobSubmission(
        jobId:         'job-001',
        ocrReadingM3:  12.345,
        ocrConfidence: 0.95,
        ocrStatus:     OcrStatus.success,
        officerNotes:  'Meter in good condition',
        gpsLat:        5.6037,
        gpsLng:        -0.1870,
        gpsAccuracyM:  3.5,
        photoUrls:     ['https://cdn.example.com/photo1.jpg'],
        photoHashes:   ['abc123def456'],
      );

      final json = submission.toJson();
      expect(json['job_id'],          'job-001');
      expect(json['ocr_reading_m3'],  12.345);
      expect(json['ocr_confidence'],  0.95);
      expect(json['ocr_status'],      'SUCCESS'); // must be uppercase API string
      expect(json['officer_notes'],   'Meter in good condition');
      expect(json['gps_lat'],         5.6037);
      expect(json['gps_accuracy_m'],  3.5);
      expect(json['photo_urls'],      ['https://cdn.example.com/photo1.jpg']);
      expect(json['photo_hashes'],    ['abc123def456']);
    });
  });

  // ─── OcrResult ────────────────────────────────────────────────────────────

  group('OcrResult', () {
    test('fromJson parses correctly', () {
      final json = {
        'reading_m3':  15.678,
        'confidence':  0.92,
        'status':      'SUCCESS',
        'raw_text':    '015.678',
      };
      final result = OcrResult.fromJson(json);
      expect(result.readingM3,  15.678);
      expect(result.confidence, 0.92);
      expect(result.status,     OcrStatus.success);
      expect(result.rawText,    '015.678');
    });
  });

  // ─── SyncStats ────────────────────────────────────────────────────────────

  group('SyncStats', () {
    test('constructs correctly with all fields', () {
      final now = DateTime.now();
      final stats = SyncStats(
        cachedJobs:         10,
        pendingSubmissions: 3,
        pendingPhotos:      5,
        pendingOutcomes:    2,
        lastSyncAt:         now,
      );
      expect(stats.cachedJobs,         10);
      expect(stats.pendingSubmissions, 3);
      expect(stats.pendingPhotos,      5);
      expect(stats.pendingOutcomes,    2);
      expect(stats.lastSyncAt,         now);
    });

    test('pendingOutcomes defaults to 0', () {
      const stats = SyncStats(
        cachedJobs:         5,
        pendingSubmissions: 1,
        pendingPhotos:      2,
      );
      expect(stats.pendingOutcomes, 0);
    });

    test('lastSyncAt can be null', () {
      const stats = SyncStats(
        cachedJobs:         0,
        pendingSubmissions: 0,
        pendingPhotos:      0,
      );
      expect(stats.lastSyncAt, isNull);
    });
  });

  // ─── MobileConfig ─────────────────────────────────────────────────────────

  group('MobileConfig', () {
    test('fromJson parses all fields', () {
      final json = {
        'geofence_radius_m':          150.0,
        'require_biometric':          false,
        'blind_audit_default':        true,
        'require_surroundings_photo': true,
        'max_photo_age_minutes':      10,
        'ocr_conflict_tolerance_pct': 3.0,
        'sync_interval_seconds':      60,
        'max_jobs_per_officer':       8,
        'app_min_version':            '1.0.0',
        'app_latest_version':         '1.1.0',
        'force_update':               false,
        'maintenance_mode':           false,
        'maintenance_message':        '',
      };
      final config = MobileConfig.fromJson(json);
      expect(config.geofenceRadiusM,    150.0);
      expect(config.requireBiometric,   isFalse);
      expect(config.syncIntervalSeconds, 60);
      expect(config.appLatestVersion,   '1.1.0');
    });

    test('defaults are sensible', () {
      const config = MobileConfig.defaults;
      // Default changed from 100.0 → 10.0 so the GPS lock stays meaningful
      // when the config endpoint is unreachable on first launch.
      expect(config.geofenceRadiusM,    10.0);
      expect(config.requireBiometric,   isTrue);
      expect(config.maintenanceMode,    isFalse);
    });

    test('round-trip toJson/fromJson', () {
      const original = MobileConfig(
        geofenceRadiusM: 200.0,
        requireBiometric: false,
        syncIntervalSeconds: 45,
      );
      final restored = MobileConfig.fromJson(original.toJson());
      expect(restored.geofenceRadiusM,    200.0);
      expect(restored.requireBiometric,   isFalse);
      expect(restored.syncIntervalSeconds, 45);
    });
  });
}
