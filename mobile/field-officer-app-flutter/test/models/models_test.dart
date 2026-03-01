// GN-WAAS Flutter — Model Unit Tests

import 'package:flutter_test/flutter_test.dart';
import 'package:gnwaas_field_officer/models/models.dart';

void main() {
  group('FieldJobStatus', () {
    test('fromString parses all valid statuses', () {
      expect(FieldJobStatus.fromString('QUEUED'),     FieldJobStatus.queued);
      expect(FieldJobStatus.fromString('DISPATCHED'), FieldJobStatus.dispatched);
      expect(FieldJobStatus.fromString('EN_ROUTE'),   FieldJobStatus.enRoute);
      expect(FieldJobStatus.fromString('ON_SITE'),    FieldJobStatus.onSite);
      expect(FieldJobStatus.fromString('COMPLETED'),  FieldJobStatus.completed);
      expect(FieldJobStatus.fromString('FAILED'),     FieldJobStatus.failed);
      expect(FieldJobStatus.fromString('SOS'),        FieldJobStatus.sos);
    });

    test('fromString defaults to queued for unknown values', () {
      expect(FieldJobStatus.fromString('UNKNOWN'), FieldJobStatus.queued);
      expect(FieldJobStatus.fromString(''),        FieldJobStatus.queued);
    });

    test('toApiString returns correct uppercase strings', () {
      expect(FieldJobStatus.queued.toApiString(),     'QUEUED');
      expect(FieldJobStatus.dispatched.toApiString(), 'DISPATCHED');
      expect(FieldJobStatus.enRoute.toApiString(),    'EN_ROUTE');
      expect(FieldJobStatus.onSite.toApiString(),     'ON_SITE');
      expect(FieldJobStatus.completed.toApiString(),  'COMPLETED');
      expect(FieldJobStatus.failed.toApiString(),     'FAILED');
      expect(FieldJobStatus.sos.toApiString(),        'SOS');
    });

    test('round-trip: fromString(toApiString()) is identity', () {
      for (final status in FieldJobStatus.values) {
        expect(FieldJobStatus.fromString(status.toApiString()), status);
      }
    });
  });

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

  group('OcrStatus', () {
    test('fromString parses all valid statuses', () {
      expect(OcrStatus.fromString('PENDING'),    OcrStatus.pending);
      expect(OcrStatus.fromString('PROCESSING'), OcrStatus.processing);
      expect(OcrStatus.fromString('SUCCESS'),    OcrStatus.success);
      expect(OcrStatus.fromString('FAILED'),     OcrStatus.failed);
      expect(OcrStatus.fromString('MANUAL'),     OcrStatus.manual);
    });
  });

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
    });

    test('toJson round-trips correctly', () {
      final user = User(
        id:          'user-789',
        email:       'test@gwl.gov.gh',
        fullName:    'Ama Owusu',
        role:        'SUPERVISOR',
        badgeNumber: 'GWL-002',
        districtId:  'district-kumasi',
      );
      final json = user.toJson();
      final restored = User.fromJson(json);
      expect(restored.id,          user.id);
      expect(restored.email,       user.email);
      expect(restored.fullName,    user.fullName);
      expect(restored.role,        user.role);
      expect(restored.badgeNumber, user.badgeNumber);
      expect(restored.districtId,  user.districtId);
    });
  });

  group('FieldJob', () {
    final sampleJson = {
      'id':                     'job-001',
      'job_reference':          'GWL-2026-001',
      'audit_event_id':         'audit-001',
      'account_number':         'ACC-12345',
      'customer_name':          'Kofi Boateng',
      'address':                '12 Independence Ave, Accra',
      'gps_lat':                5.6037,
      'gps_lng':                -0.1870,
      'anomaly_type':           'BILLING_VARIANCE',
      'alert_level':            'HIGH',
      'status':                 'DISPATCHED',
      'scheduled_at':           '2026-03-01T08:00:00Z',
      'estimated_variance_ghs': 450.75,
    };

    test('fromJson parses all fields correctly', () {
      final job = FieldJob.fromJson(sampleJson);
      expect(job.id,                   'job-001');
      expect(job.jobReference,         'GWL-2026-001');
      expect(job.accountNumber,        'ACC-12345');
      expect(job.customerName,         'Kofi Boateng');
      expect(job.gpsLat,               5.6037);
      expect(job.gpsLng,               -0.1870);
      expect(job.alertLevel,           AlertLevel.high);
      expect(job.status,               FieldJobStatus.dispatched);
      expect(job.estimatedVarianceGhs, 450.75);
      expect(job.scheduledAt,          isNotNull);
    });

    test('fromJson handles missing optional fields', () {
      final minimalJson = {
        'id':             'job-002',
        'account_number': 'ACC-99999',
        'customer_name':  'Test Customer',
        'address':        'Test Address',
        'gps_lat':        5.0,
        'gps_lng':        -1.0,
      };
      final job = FieldJob.fromJson(minimalJson);
      expect(job.jobReference,         isNull);
      expect(job.estimatedVarianceGhs, isNull);
      expect(job.scheduledAt,          isNull);
      expect(job.alertLevel,           AlertLevel.medium); // default
      expect(job.status,               FieldJobStatus.queued); // default
    });

    test('toJson round-trips correctly', () {
      final job = FieldJob.fromJson(sampleJson);
      final json = job.toJson();
      expect(json['id'],             'job-001');
      expect(json['alert_level'],    'HIGH');
      expect(json['status'],         'DISPATCHED');
      expect(json['gps_lat'],        5.6037);
    });

    test('status is mutable', () {
      final job = FieldJob.fromJson(sampleJson);
      expect(job.status, FieldJobStatus.dispatched);
      job.status = FieldJobStatus.onSite;
      expect(job.status, FieldJobStatus.onSite);
    });
  });

  group('JobSubmission', () {
    test('toJson serializes all fields', () {
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
      expect(json['ocr_status'],      'SUCCESS');
      expect(json['officer_notes'],   'Meter in good condition');
      expect(json['gps_lat'],         5.6037);
      expect(json['gps_accuracy_m'],  3.5);
      expect(json['photo_urls'],      ['https://cdn.example.com/photo1.jpg']);
      expect(json['photo_hashes'],    ['abc123def456']);
    });
  });

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

  group('SyncStats', () {
    test('constructs correctly', () {
      final now = DateTime.now();
      final stats = SyncStats(
        cachedJobs:         10,
        pendingSubmissions: 3,
        pendingPhotos:      5,
        lastSyncAt:         now,
      );
      expect(stats.cachedJobs,         10);
      expect(stats.pendingSubmissions, 3);
      expect(stats.pendingPhotos,      5);
      expect(stats.lastSyncAt,         now);
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
}
