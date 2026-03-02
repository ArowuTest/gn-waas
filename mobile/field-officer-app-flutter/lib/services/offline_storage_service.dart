// GN-WAAS Field Officer App — Offline Storage Service
// SQLite-backed offline-first storage with background sync

import 'dart:convert';
import 'package:sqflite/sqflite.dart';
import 'package:path/path.dart' as p;
import 'package:uuid/uuid.dart';
import '../config/app_config.dart';
import '../models/models.dart';

class OfflineStorageService {
  Database? _db;
  static const _uuid = Uuid();

  // ─── Initialisation ────────────────────────────────────────────────────────

  Future<Database> get db async {
    _db ??= await _openDatabase();
    return _db!;
  }

  Future<Database> _openDatabase() async {
    final dbPath = await getDatabasesPath();
    final path   = p.join(dbPath, AppConfig.dbName);

    return openDatabase(
      path,
      version: AppConfig.dbVersion,
      onCreate: _onCreate,
      onUpgrade: _onUpgrade,
    );
  }

  Future<void> _onCreate(Database db, int version) async {
    await db.execute('''
      CREATE TABLE IF NOT EXISTS offline_jobs (
        id                    TEXT PRIMARY KEY,
        job_reference         TEXT,
        audit_event_id        TEXT,
        account_number        TEXT NOT NULL,
        customer_name         TEXT NOT NULL,
        address               TEXT NOT NULL,
        gps_lat               REAL NOT NULL,
        gps_lng               REAL NOT NULL,
        anomaly_type          TEXT,
        alert_level           TEXT NOT NULL DEFAULT 'MEDIUM',
        status                TEXT NOT NULL DEFAULT 'QUEUED',
        scheduled_at          TEXT,
        dispatched_at         TEXT,
        notes                 TEXT,
        estimated_variance_ghs REAL,
        synced_at             TEXT,
        created_at            TEXT NOT NULL DEFAULT (datetime('now')),
        updated_at            TEXT NOT NULL DEFAULT (datetime('now'))
      )
    ''');

    await db.execute('''
      CREATE TABLE IF NOT EXISTS pending_submissions (
        id              TEXT PRIMARY KEY,
        job_id          TEXT NOT NULL,
        submission_json TEXT NOT NULL,
        photo_uris      TEXT NOT NULL DEFAULT '[]',
        status          TEXT NOT NULL DEFAULT 'PENDING',
        retry_count     INTEGER NOT NULL DEFAULT 0,
        last_error      TEXT,
        created_at      TEXT NOT NULL DEFAULT (datetime('now')),
        attempted_at    TEXT,
        FOREIGN KEY (job_id) REFERENCES offline_jobs(id)
      )
    ''');

    await db.execute('''
      CREATE TABLE IF NOT EXISTS offline_photos (
        id            TEXT PRIMARY KEY,
        job_id        TEXT NOT NULL,
        submission_id TEXT,
        local_path    TEXT NOT NULL,
        photo_hash    TEXT NOT NULL,
        gps_lat       REAL NOT NULL,
        gps_lng       REAL NOT NULL,
        gps_accuracy  REAL NOT NULL,
        captured_at   TEXT NOT NULL,
        within_fence  INTEGER NOT NULL DEFAULT 0,
        uploaded      INTEGER NOT NULL DEFAULT 0,
        remote_url    TEXT,
        created_at    TEXT NOT NULL DEFAULT (datetime('now')),
        FOREIGN KEY (job_id) REFERENCES offline_jobs(id)
      )
    ''');

    await db.execute('CREATE INDEX IF NOT EXISTS idx_pending_status ON pending_submissions(status)');
    await db.execute('CREATE INDEX IF NOT EXISTS idx_photos_job ON offline_photos(job_id)');
    await db.execute('CREATE INDEX IF NOT EXISTS idx_jobs_status ON offline_jobs(status)');
  }

  Future<void> _onUpgrade(Database db, int oldVersion, int newVersion) async {
    // Future migrations go here
  }

  // ─── Job Cache ─────────────────────────────────────────────────────────────

  /// Cache jobs fetched from the API using server-wins conflict resolution.
  ///
  /// H2 Fix: Instead of blindly replacing local records (ConflictAlgorithm.replace),
  /// we implement a "server-wins for status, preserve local pending submissions"
  /// strategy:
  ///   - If the local record has a PENDING submission queued, we keep the local
  ///     status and merge only non-status fields from the server.
  ///   - If no pending submission exists, the server version wins entirely.
  ///
  /// This prevents data loss when a job is updated on the server while the
  /// officer is offline and has queued a local status change.
  Future<void> cacheJobs(List<FieldJob> jobs) async {
    final database = await db;

    for (final job in jobs) {
      // Check if there is a pending (unsynced) submission for this job
      final pendingRows = await database.query(
        'pending_submissions',
        where: 'job_id = ? AND status = ?',
        whereArgs: [job.id, 'PENDING'],
        limit: 1,
      );
      final hasPendingSubmission = pendingRows.isNotEmpty;

      // Check if we have a local record with a more recent update
      final localRows = await database.query(
        'offline_jobs',
        where: 'id = ?',
        whereArgs: [job.id],
        limit: 1,
      );

      final serverData = {
        'id':                     job.id,
        'job_reference':          job.jobReference,
        'audit_event_id':         job.auditEventId,
        'account_number':         job.accountNumber,
        'customer_name':          job.customerName,
        'address':                job.address,
        'gps_lat':                job.gpsLat,
        'gps_lng':                job.gpsLng,
        'anomaly_type':           job.anomalyType,
        'alert_level':            job.alertLevel.toApiString(),
        'status':                 job.status.toApiString(),
        'scheduled_at':           job.scheduledAt?.toIso8601String(),
        'dispatched_at':          job.dispatchedAt?.toIso8601String(),
        'notes':                  job.notes,
        'estimated_variance_ghs': job.estimatedVarianceGhs,
        'synced_at':              DateTime.now().toIso8601String(),
        'updated_at':             DateTime.now().toIso8601String(),
      };

      if (localRows.isEmpty) {
        // No local record — insert server version
        await database.insert('offline_jobs', serverData,
            conflictAlgorithm: ConflictAlgorithm.ignore);
      } else if (hasPendingSubmission) {
        // Local pending submission exists — server-wins for metadata only,
        // preserve local status to avoid overwriting the officer's work.
        final localStatus = localRows.first['status'] as String?;
        final mergedData = Map<String, dynamic>.from(serverData);
        if (localStatus != null) {
          mergedData['status'] = localStatus; // preserve local status
        }
        await database.update(
          'offline_jobs',
          mergedData,
          where: 'id = ?',
          whereArgs: [job.id],
        );
      } else {
        // No pending submission — server wins entirely (latest truth)
        await database.update(
          'offline_jobs',
          serverData,
          where: 'id = ?',
          whereArgs: [job.id],
        );
      }
    }
  }

  /// Load cached jobs from SQLite
  Future<List<FieldJob>> loadCachedJobs() async {
    final database = await db;
    final rows = await database.query(
      'offline_jobs',
      orderBy: 'updated_at DESC',
    );
    return rows.map(_rowToFieldJob).toList();
  }

  /// Update a job's status locally
  Future<void> updateJobStatusLocally(String jobId, FieldJobStatus status) async {
    final database = await db;
    await database.update(
      'offline_jobs',
      {
        'status':     status.toApiString(),
        'updated_at': DateTime.now().toIso8601String(),
      },
      where: 'id = ?',
      whereArgs: [jobId],
    );
  }

  // ─── Illegal Connection Reports ───────────────────────────────────────────

  /// Queue an illegal connection report for background sync when offline.
  /// Photo hashes are stored to maintain chain of custody even when offline.
  Future<void> queueIllegalConnectionReport({
    required String connectionType,
    required String severity,
    required String description,
    required double latitude,
    required double longitude,
    required int photoCount,
    List<String> photoHashes = const [],
  }) async {
    final database = await db;
    await database.insert('pending_submissions', {
      'id':              _uuid.v4(),
      'job_id':          'illegal_connection',
      'submission_json': jsonEncode({
        'type': 'ILLEGAL_CONNECTION',
        'connection_type': connectionType,
        'severity': severity,
        'description': description,
        'latitude': latitude,
        'longitude': longitude,
        'photo_count': photoCount,
        // SHA-256 hashes preserved offline for chain-of-custody integrity
        'photo_hashes': photoHashes,
      }),
      'photo_uris':      jsonEncode(<String>[]),
      'status':          'PENDING',
      'retry_count':     0,
      'created_at':      DateTime.now().toIso8601String(),
    });
  }

  // ─── Pending Submissions ───────────────────────────────────────────────────

  /// Queue a job submission for background sync
  Future<String> queueSubmission(
    String jobId,
    JobSubmission submission,
    List<String> photoUris,
  ) async {
    final database = await db;
    final id = _uuid.v4();
    await database.insert('pending_submissions', {
      'id':              id,
      'job_id':          jobId,
      'submission_json': jsonEncode(submission.toJson()),
      'photo_uris':      jsonEncode(photoUris),
      'status':          'PENDING',
      'retry_count':     0,
      'created_at':      DateTime.now().toIso8601String(),
    });
    return id;
  }

  /// Get all pending submissions
  Future<List<PendingSubmission>> getPendingSubmissions() async {
    final database = await db;
    final rows = await database.query(
      'pending_submissions',
      where: 'status IN (?, ?)',
      whereArgs: ['PENDING', 'FAILED'],
      orderBy: 'created_at ASC',
    );
    return rows.map(_rowToPendingSubmission).toList();
  }

  /// Mark a submission as done
  Future<void> markSubmissionDone(String submissionId) async {
    final database = await db;
    await database.update(
      'pending_submissions',
      {'status': 'DONE', 'attempted_at': DateTime.now().toIso8601String()},
      where: 'id = ?',
      whereArgs: [submissionId],
    );
  }

  /// Mark a submission as failed with error
  Future<void> markSubmissionFailed(String submissionId, String error) async {
    final database = await db;
    await database.rawUpdate('''
      UPDATE pending_submissions
      SET status = 'FAILED',
          last_error = ?,
          retry_count = retry_count + 1,
          attempted_at = ?
      WHERE id = ?
    ''', [error, DateTime.now().toIso8601String(), submissionId]);
  }

  // ─── Photos ────────────────────────────────────────────────────────────────

  /// Save a captured photo record
  Future<void> savePhoto(String jobId, MeterPhoto photo, {String? submissionId}) async {
    final database = await db;
    await database.insert('offline_photos', {
      'id':            _uuid.v4(),
      'job_id':        jobId,
      'submission_id': submissionId,
      'local_path':    photo.localPath,
      'photo_hash':    photo.hash,
      'gps_lat':       photo.gpsLat,
      'gps_lng':       photo.gpsLng,
      'gps_accuracy':  photo.gpsAccuracyM,
      'captured_at':   photo.capturedAt.toIso8601String(),
      'within_fence':  photo.withinFence ? 1 : 0,
      'uploaded':      0,
      'created_at':    DateTime.now().toIso8601String(),
    });
  }

  /// Get photos for a job
  Future<List<MeterPhoto>> getPhotosForJob(String jobId) async {
    final database = await db;
    final rows = await database.query(
      'offline_photos',
      where: 'job_id = ?',
      whereArgs: [jobId],
      orderBy: 'captured_at ASC',
    );
    return rows.map(_rowToMeterPhoto).toList();
  }

  // ─── Sync Stats ────────────────────────────────────────────────────────────

  Future<SyncStats> getSyncStats() async {
    final database = await db;

    final jobCount = Sqflite.firstIntValue(
      await database.rawQuery('SELECT COUNT(*) FROM offline_jobs'),
    ) ?? 0;

    final pendingCount = Sqflite.firstIntValue(
      await database.rawQuery(
        "SELECT COUNT(*) FROM pending_submissions WHERE status IN ('PENDING','FAILED')",
      ),
    ) ?? 0;

    final photoCount = Sqflite.firstIntValue(
      await database.rawQuery(
        'SELECT COUNT(*) FROM offline_photos WHERE uploaded = 0',
      ),
    ) ?? 0;

    final lastSyncRow = await database.rawQuery(
      'SELECT MAX(synced_at) as last FROM offline_jobs',
    );
    final lastSyncStr = lastSyncRow.first['last'] as String?;

    return SyncStats(
      cachedJobs:          jobCount,
      pendingSubmissions:  pendingCount,
      pendingPhotos:       photoCount,
      lastSyncAt:          lastSyncStr != null ? DateTime.tryParse(lastSyncStr) : null,
    );
  }

  /// Clear all cached data (on logout)
  Future<void> clearAll() async {
    final database = await db;
    await database.delete('offline_photos');
    await database.delete('pending_submissions');
    await database.delete('offline_jobs');
  }

  // ─── Row mappers ───────────────────────────────────────────────────────────

  FieldJob _rowToFieldJob(Map<String, dynamic> row) => FieldJob(
    id:                   row['id'] as String,
    jobReference:         row['job_reference'] as String?,
    auditEventId:         row['audit_event_id'] as String? ?? '',
    accountNumber:        row['account_number'] as String,
    customerName:         row['customer_name'] as String,
    address:              row['address'] as String,
    gpsLat:               row['gps_lat'] as double,
    gpsLng:               row['gps_lng'] as double,
    anomalyType:          row['anomaly_type'] as String? ?? 'UNKNOWN',
    alertLevel:           AlertLevel.fromString(row['alert_level'] as String),
    status:               FieldJobStatus.fromString(row['status'] as String),
    scheduledAt:          row['scheduled_at'] != null
                            ? DateTime.tryParse(row['scheduled_at'] as String)
                            : null,
    dispatchedAt:         row['dispatched_at'] != null
                            ? DateTime.tryParse(row['dispatched_at'] as String)
                            : null,
    notes:                row['notes'] as String?,
    estimatedVarianceGhs: row['estimated_variance_ghs'] as double?,
  );

  PendingSubmission _rowToPendingSubmission(Map<String, dynamic> row) {
    final submissionJson = jsonDecode(row['submission_json'] as String) as Map<String, dynamic>;
    final photoUris = (jsonDecode(row['photo_uris'] as String) as List<dynamic>)
        .map((e) => e as String)
        .toList();

    return PendingSubmission(
      id:         row['id'] as String,
      jobId:      row['job_id'] as String,
      submission: JobSubmission(
        jobId:          submissionJson['job_id'] as String,
        ocrReadingM3:   (submissionJson['ocr_reading_m3'] as num).toDouble(),
        ocrConfidence:  (submissionJson['ocr_confidence'] as num).toDouble(),
        ocrStatus:      OcrStatus.fromString(submissionJson['ocr_status'] as String),
        officerNotes:   submissionJson['officer_notes'] as String,
        gpsLat:         (submissionJson['gps_lat'] as num).toDouble(),
        gpsLng:         (submissionJson['gps_lng'] as num).toDouble(),
        gpsAccuracyM:   (submissionJson['gps_accuracy_m'] as num).toDouble(),
        photoUrls:      (submissionJson['photo_urls'] as List<dynamic>).cast<String>(),
        photoHashes:    (submissionJson['photo_hashes'] as List<dynamic>).cast<String>(),
      ),
      photoUris:  photoUris,
      status:     row['status'] as String,
      retryCount: row['retry_count'] as int,
      lastError:  row['last_error'] as String?,
      attemptedAt: row['attempted_at'] != null
                    ? DateTime.tryParse(row['attempted_at'] as String)
                    : null,
    );
  }

  MeterPhoto _rowToMeterPhoto(Map<String, dynamic> row) => MeterPhoto(
    localPath:    row['local_path'] as String,
    hash:         row['photo_hash'] as String,
    gpsLat:       row['gps_lat'] as double,
    gpsLng:       row['gps_lng'] as double,
    gpsAccuracyM: row['gps_accuracy'] as double,
    capturedAt:   DateTime.parse(row['captured_at'] as String),
    withinFence:  (row['within_fence'] as int) == 1,
    remoteUrl:    row['remote_url'] as String?,
  );
}
