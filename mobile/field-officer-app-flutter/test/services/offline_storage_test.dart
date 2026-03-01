// GN-WAAS Flutter — Offline Storage Service Tests
// Uses sqflite_common_ffi for in-memory SQLite testing

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

  // ─── Helper ───────────────────────────────────────────────────────────────

  FieldJob makeJob({
    String id = 'job-001',
    FieldJobStatus status = FieldJobStatus.queued,
    AlertLevel alertLevel = AlertLevel.medium,
  }) => FieldJob(
    id:            id,
    jobReference:  'GWL-2026-$id',
    auditEventId:  'audit-$id',
    accountNumber: 'ACC-$id',
    customerName:  'Test Customer $id',
    address:       '1 Test Street, Accra',
    gpsLat:        5.6037,
    gpsLng:        -0.1870,
    anomalyType:   'BILLING_VARIANCE',
    alertLevel:    alertLevel,
    status:        status,
    estimatedVarianceGhs: 250.0,
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

    test('preserves all job fields', () async {
      final job = makeJob(alertLevel: AlertLevel.critical);
      await storage.cacheJobs([job]);
      final cached = await storage.loadCachedJobs();
      final restored = cached.first;
      expect(restored.id,                   job.id);
      expect(restored.accountNumber,        job.accountNumber);
      expect(restored.customerName,         job.customerName);
      expect(restored.alertLevel,           AlertLevel.critical);
      expect(restored.estimatedVarianceGhs, 250.0);
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

  // ─── Pending Submissions ──────────────────────────────────────────────────

  group('queueSubmission', () {
    test('queues a submission and returns an ID', () async {
      await storage.cacheJobs([makeJob()]);
      final id = await storage.queueSubmission(
        'job-001', makeSubmission('job-001'), ['/tmp/photo.jpg'],
      );
      expect(id, isNotEmpty);
    });

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
      expect(restored.submission.officerNotes,  'Test notes');
      expect(restored.photoUris.length,         2);
    });
  });

  group('markSubmissionDone', () {
    test('removes submission from pending list', () async {
      await storage.cacheJobs([makeJob()]);
      final id = await storage.queueSubmission(
        'job-001', makeSubmission('job-001'), [],
      );
      await storage.markSubmissionDone(id);
      final pending = await storage.getPendingSubmissions();
      expect(pending, isEmpty);
    });
  });

  group('markSubmissionFailed', () {
    test('increments retry count and sets error', () async {
      await storage.cacheJobs([makeJob()]);
      final id = await storage.queueSubmission(
        'job-001', makeSubmission('job-001'), [],
      );
      await storage.markSubmissionFailed(id, 'Network timeout');
      final pending = await storage.getPendingSubmissions();
      expect(pending.first.retryCount, 1);
      expect(pending.first.lastError,  'Network timeout');
      expect(pending.first.status,     'FAILED');
    });

    test('increments retry count on multiple failures', () async {
      await storage.cacheJobs([makeJob()]);
      final id = await storage.queueSubmission(
        'job-001', makeSubmission('job-001'), [],
      );
      await storage.markSubmissionFailed(id, 'Error 1');
      await storage.markSubmissionFailed(id, 'Error 2');
      await storage.markSubmissionFailed(id, 'Error 3');
      final pending = await storage.getPendingSubmissions();
      expect(pending.first.retryCount, 3);
      expect(pending.first.lastError,  'Error 3');
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

    test('does not return photos from other jobs', () async {
      await storage.cacheJobs([
        makeJob(id: 'job-001'),
        makeJob(id: 'job-002'),
      ]);
      await storage.savePhoto('job-001', makePhoto('job-001'));
      final photos = await storage.getPhotosForJob('job-002');
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
      expect(stats.lastSyncAt,         isNull);
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

    test('does not count done submissions as pending', () async {
      await storage.cacheJobs([makeJob()]);
      final id = await storage.queueSubmission(
        'job-001', makeSubmission('job-001'), [],
      );
      await storage.markSubmissionDone(id);
      final stats = await storage.getSyncStats();
      expect(stats.pendingSubmissions, 0);
    });
  });

  // ─── Clear All ────────────────────────────────────────────────────────────

  group('clearAll', () {
    test('removes all data', () async {
      await storage.cacheJobs([makeJob()]);
      await storage.queueSubmission('job-001', makeSubmission('job-001'), []);
      await storage.savePhoto('job-001', makePhoto('job-001'));
      await storage.clearAll();
      expect(await storage.loadCachedJobs(),          isEmpty);
      expect(await storage.getPendingSubmissions(),   isEmpty);
      expect(await storage.getPhotosForJob('job-001'), isEmpty);
    });
  });
}
