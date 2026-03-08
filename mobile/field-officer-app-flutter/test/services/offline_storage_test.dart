// GN-WAAS Flutter — Offline Storage Service Tests
// Uses sqflite_common_ffi for in-memory SQLite testing
// Covers v3 schema: leakage fields, outcome fields, pending_outcomes table

import 'package:flutter_test/flutter_test.dart';
import 'package:sqflite_common_ffi/sqflite_ffi.dart';
import 'package:gnwaas_field_officer/models/models.dart';
import 'package:gnwaas_field_officer/services/offline_storage_service.dart';

void main() {
  setUpAll(() {
    // Use in-memory SQLite for tests
    sqfliteFfiInit();
    databaseFactory = databaseFactoryFfi;
  });

  late OfflineStorageService storage;

  setUp(() {
    storage = OfflineStorageService();
  });

  tearDown(() async {
    final db = await storage.db;
    final path = db.path;
    await db.close();
    if (path != inMemoryDatabasePath) {
      await deleteDatabase(path);
    }
  });

  // ─── Helpers ──────────────────────────────────────────────────────────────

  FieldJob makeJob({
    String id = 'job-001',
    FieldJobStatus status = FieldJobStatus.queued,
    AlertLevel alertLevel = AlertLevel.medium,
    double? monthlyLeakageGhs,
    LeakageCategory? leakageCategory,
    FieldJobOutcome? outcome,
  }) => FieldJob(
    id:                   id,
    jobReference:         'GWL-2026-$id',
    auditEventId:         'audit-$id',
    accountNumber:        'ACC-$id',
    customerName:         'Test Customer $id',
    address:              '1 Test Street, Accra',
    gpsLat:               5.6037,
    gpsLng:               -0.1870,
    anomalyType:          'BILLING_VARIANCE',
    alertLevel:           alertLevel,
    status:               status,
    estimatedVarianceGhs: 250.0,
    monthlyLeakageGhs:    monthlyLeakageGhs,
    leakageCategory:      leakageCategory,
    outcome:              outcome,
  );

  JobSubmission makeSubmission(String jobId) => JobSubmission(
    jobId:         jobId,
    ocrReadingM3:  12.345,
    ocrConfidence: 0.95,
    ocrStatus:     OcrStatus.success,
    officerNotes:  'Test notes',
    gpsLat:        5.6037,
    gpsLng:        -0.1870,
    gpsAccuracyM:  3.5,
    photoUrls:     [],
    photoHashes:   ['abc123'],
  );

  MeterPhoto makePhoto(String jobId) => MeterPhoto(
    localPath:    '/tmp/photo_$jobId.jpg',
    hash:         'sha256_$jobId',
    gpsLat:       5.6037,
    gpsLng:       -0.1870,
    gpsAccuracyM: 3.5,
    capturedAt:   DateTime.now(),
    withinFence:  true,
  );

  FieldJobOutcomeRequest makeOutcomeRequest() => const FieldJobOutcomeRequest(
    outcome:          FieldJobOutcome.meterFoundTampered,
    outcomeNotes:     'Seal broken',
    meterFound:       true,
    addressConfirmed: true,
    recommendedAction: 'Replace meter',
  );

  // ─── Database Initialization ──────────────────────────────────────────────

  group('Database initialization', () {
    test('creates database and tables successfully', () async {
      final db = await storage.db;
      expect(db.isOpen, isTrue);
    });

    test('returns same instance on repeated calls', () async {
      final db1 = await storage.db;
      final db2 = await storage.db;
      expect(identical(db1, db2), isTrue);
    });

    test('creates pending_outcomes table (v3)', () async {
      final db = await storage.db;
      final tables = await db.rawQuery(
        "SELECT name FROM sqlite_master WHERE type='table' AND name='pending_outcomes'",
      );
      expect(tables, isNotEmpty);
    });
  });

  // ─── Job Cache ────────────────────────────────────────────────────────────

  group('cacheJobs', () {
    test('caches a single job', () async {
      await storage.cacheJobs([makeJob()]);
      final cached = await storage.loadCachedJobs();
      expect(cached.length, 1);
      expect(cached.first.id, 'job-001');
    });

    test('caches multiple jobs', () async {
      await storage.cacheJobs([
        makeJob(id: 'job-001'),
        makeJob(id: 'job-002'),
        makeJob(id: 'job-003'),
      ]);
      final cached = await storage.loadCachedJobs();
      expect(cached.length, 3);
    });

    test('replaces existing job on conflict', () async {
      await storage.cacheJobs([makeJob(status: FieldJobStatus.queued)]);
      await storage.cacheJobs([makeJob(status: FieldJobStatus.dispatched)]);
      final cached = await storage.loadCachedJobs();
      expect(cached.length, 1);
      expect(cached.first.status, FieldJobStatus.dispatched);
    });

    test('preserves v3 leakage fields', () async {
      final job = makeJob(
        monthlyLeakageGhs: 350.75,
        leakageCategory:   LeakageCategory.revenueLeakage,
      );
      await storage.cacheJobs([job]);
      final cached = await storage.loadCachedJobs();
      final restored = cached.first;
      expect(restored.monthlyLeakageGhs, 350.75);
      expect(restored.leakageCategory,   LeakageCategory.revenueLeakage);
    });

    test('preserves v3 outcome fields', () async {
      final job = makeJob(
        status:  FieldJobStatus.outcomeRecorded,
        outcome: FieldJobOutcome.meterFoundTampered,
      );
      await storage.cacheJobs([job]);
      final cached = await storage.loadCachedJobs();
      final restored = cached.first;
      expect(restored.status,  FieldJobStatus.outcomeRecorded);
      expect(restored.outcome, FieldJobOutcome.meterFoundTampered);
    });
  });

  group('loadCachedJobs', () {
    test('returns empty list when no jobs cached', () async {
      final jobs = await storage.loadCachedJobs();
      expect(jobs, isEmpty);
    });
  });

  group('updateJobStatusLocally', () {
    test('updates job status', () async {
      await storage.cacheJobs([makeJob(status: FieldJobStatus.queued)]);
      await storage.updateJobStatusLocally('job-001', FieldJobStatus.onSite);
      final cached = await storage.loadCachedJobs();
      expect(cached.first.status, FieldJobStatus.onSite);
    });

    test('does not affect other jobs', () async {
      await storage.cacheJobs([
        makeJob(id: 'job-001', status: FieldJobStatus.queued),
        makeJob(id: 'job-002', status: FieldJobStatus.queued),
      ]);
      await storage.updateJobStatusLocally('job-001', FieldJobStatus.completed);
      final cached = await storage.loadCachedJobs();
      final job2 = cached.firstWhere((j) => j.id == 'job-002');
      expect(job2.status, FieldJobStatus.queued);
    });
  });

  group('updateJobOutcomeLocally (v3)', () {
    test('updates outcome fields and status', () async {
      await storage.cacheJobs([makeJob(status: FieldJobStatus.completed)]);
      await storage.updateJobOutcomeLocally('job-001', makeOutcomeRequest());
      final cached = await storage.loadCachedJobs();
      final job = cached.first;
      expect(job.outcome,          FieldJobOutcome.meterFoundTampered);
      expect(job.outcomeNotes,     'Seal broken');
      expect(job.meterFound,       isTrue);
      expect(job.addressConfirmed, isTrue);
      expect(job.status,           FieldJobStatus.outcomeRecorded);
      expect(job.outcomeRecordedAt, isNotNull);
    });
  });

  // ─── Pending Submissions ──────────────────────────────────────────────────

  group('queueSubmission', () {
    test('queued submission appears in getPendingSubmissions', () async {
      await storage.cacheJobs([makeJob()]);
      await storage.queueSubmission(
        'job-001', makeSubmission('job-001'), ['/tmp/photo.jpg'],
      );
      final pending = await storage.getPendingSubmissions();
      expect(pending.length, 1);
      expect(pending.first.jobId, 'job-001');
      expect(pending.first.status, 'PENDING');
    });

    test('submission data is preserved correctly', () async {
      await storage.cacheJobs([makeJob()]);
      await storage.queueSubmission(
        'job-001', makeSubmission('job-001'), ['/tmp/p1.jpg', '/tmp/p2.jpg'],
      );
      final pending = await storage.getPendingSubmissions();
      final restored = pending.first;
      expect(restored.submission.ocrReadingM3,  12.345);
      expect(restored.submission.ocrConfidence, 0.95);
      expect(restored.submission.ocrStatus,     OcrStatus.success);
      expect(restored.photoUris.length,         2);
    });
  });

  group('markSubmissionDone', () {
    test('removes submission from pending list', () async {
      await storage.cacheJobs([makeJob()]);
      await storage.queueSubmission('job-001', makeSubmission('job-001'), []);
      final pending = await storage.getPendingSubmissions();
      await storage.markSubmissionDone(pending.first.id);
      final afterDone = await storage.getPendingSubmissions();
      expect(afterDone, isEmpty);
    });
  });

  group('markSubmissionFailed', () {
    test('increments retry count and records error', () async {
      await storage.cacheJobs([makeJob()]);
      await storage.queueSubmission('job-001', makeSubmission('job-001'), []);
      final pending = await storage.getPendingSubmissions();
      await storage.markSubmissionFailed(pending.first.id, 'Network timeout');
      final after = await storage.getPendingSubmissions();
      expect(after.first.retryCount, 1);
      expect(after.first.lastError,  'Network timeout');
      // Status stays PENDING so sync service retries (retry_count < 5 guard prevents infinite loops)
      expect(after.first.status, 'PENDING');
    });
  });

  // ─── Pending Outcomes (v3) ────────────────────────────────────────────────

  group('queueOutcome (v3)', () {
    test('queues an outcome and updates local job', () async {
      await storage.cacheJobs([makeJob(status: FieldJobStatus.completed)]);
      await storage.queueOutcome('job-001', makeOutcomeRequest());

      final pending = await storage.getPendingOutcomes();
      expect(pending.length, 1);
      expect(pending.first.jobId, 'job-001');
      expect(pending.first.outcomeRequest.outcome, FieldJobOutcome.meterFoundTampered);

      // Local job should be updated immediately
      final cached = await storage.loadCachedJobs();
      expect(cached.first.outcome, FieldJobOutcome.meterFoundTampered);
      expect(cached.first.status,  FieldJobStatus.outcomeRecorded);
    });

    test('outcome request data is preserved correctly', () async {
      await storage.cacheJobs([makeJob()]);
      await storage.queueOutcome('job-001', makeOutcomeRequest());
      final pending = await storage.getPendingOutcomes();
      final restored = pending.first.outcomeRequest;
      expect(restored.outcome,           FieldJobOutcome.meterFoundTampered);
      expect(restored.outcomeNotes,      'Seal broken');
      expect(restored.meterFound,        isTrue);
      expect(restored.addressConfirmed,  isTrue);
      expect(restored.recommendedAction, 'Replace meter');
    });
  });

  group('markOutcomeDone (v3)', () {
    test('removes outcome from pending list', () async {
      await storage.cacheJobs([makeJob()]);
      await storage.queueOutcome('job-001', makeOutcomeRequest());
      final pending = await storage.getPendingOutcomes();
      await storage.markOutcomeDone(pending.first.id);
      final afterDone = await storage.getPendingOutcomes();
      expect(afterDone, isEmpty);
    });
  });

  group('markOutcomeFailed (v3)', () {
    test('increments retry count', () async {
      await storage.cacheJobs([makeJob()]);
      await storage.queueOutcome('job-001', makeOutcomeRequest());
      final pending = await storage.getPendingOutcomes();
      await storage.markOutcomeFailed(pending.first.id, 'Server error');
      final after = await storage.getPendingOutcomes();
      expect(after.first.retryCount, 1);
      expect(after.first.lastError,  'Server error');
    });
  });

  // ─── Photos ───────────────────────────────────────────────────────────────

  group('savePhoto / getPhotosForJob', () {
    test('saves and retrieves a photo', () async {
      await storage.cacheJobs([makeJob()]);
      await storage.savePhoto('job-001', makePhoto('job-001'));
      final photos = await storage.getPhotosForJob('job-001');
      expect(photos.length, 1);
      expect(photos.first.hash,        'sha256_job-001');
      expect(photos.first.withinFence, isTrue);
    });

    test('returns empty list for job with no photos', () async {
      await storage.cacheJobs([makeJob()]);
      final photos = await storage.getPhotosForJob('job-001');
      expect(photos, isEmpty);
    });
  });

  // ─── Sync Stats ───────────────────────────────────────────────────────────

  group('getSyncStats', () {
    test('returns zeros when database is empty', () async {
      final stats = await storage.getSyncStats();
      expect(stats.cachedJobs,         0);
      expect(stats.pendingSubmissions, 0);
      expect(stats.pendingPhotos,      0);
      expect(stats.pendingOutcomes,    0);
      expect(stats.lastSyncAt,         isNull);
    });

    test('counts pending outcomes correctly (v3)', () async {
      await storage.cacheJobs([makeJob()]);
      await storage.queueOutcome('job-001', makeOutcomeRequest());
      final stats = await storage.getSyncStats();
      expect(stats.pendingOutcomes, 1);
    });

    test('counts cached jobs correctly', () async {
      await storage.cacheJobs([makeJob(id: 'job-001'), makeJob(id: 'job-002')]);
      final stats = await storage.getSyncStats();
      expect(stats.cachedJobs, 2);
    });

    test('counts pending submissions correctly', () async {
      await storage.cacheJobs([makeJob()]);
      await storage.queueSubmission('job-001', makeSubmission('job-001'), []);
      await storage.queueSubmission('job-001', makeSubmission('job-001'), []);
      final stats = await storage.getSyncStats();
      expect(stats.pendingSubmissions, 2);
    });
  });
}
