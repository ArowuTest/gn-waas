// GN-WAAS Field Officer App — Offline Storage Service
// SQLite-backed offline-first storage with background sync
//
// DB Version History:
//   v1 — Initial schema (offline_jobs, pending_submissions, offline_photos)
//   v2 — Added indexes for performance
//   v3 — Added leakage fields + outcome fields to offline_jobs (migration 031)
//        Added pending_outcomes table for offline outcome recording

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
      version: AppConfig.dbVersion, // v3
      onCreate: _onCreate,
      onUpgrade: _onUpgrade,
    );
  }

  Future<void> _onCreate(Database db, int version) async {
    // ── offline_jobs (v3 schema) ──────────────────────────────────────────
    await db.execute('''
      CREATE TABLE IF NOT EXISTS offline_jobs (
        id                      TEXT PRIMARY KEY,
        job_reference           TEXT,
        audit_event_id          TEXT,
        account_number          TEXT NOT NULL,
        customer_name           TEXT NOT NULL,
        address                 TEXT NOT NULL,
        gps_lat                 REAL NOT NULL,
        gps_lng                 REAL NOT NULL,
        anomaly_type            TEXT,
        alert_level             TEXT NOT NULL DEFAULT 'MEDIUM',
        status                  TEXT NOT NULL DEFAULT 'QUEUED',
        scheduled_at            TEXT,
        dispatched_at           TEXT,
        notes                   TEXT,
        estimated_variance_ghs  REAL,
        -- v3: revenue leakage fields (migration 031)
        monthly_leakage_ghs     REAL,
        annualised_leakage_ghs  REAL,
        leakage_category        TEXT,
        -- v3: field outcome fields (migration 031)
        outcome                 TEXT,
        outcome_notes           TEXT,
        meter_found             INTEGER,
        address_confirmed       INTEGER,
        recommended_action      TEXT,
        outcome_recorded_at     TEXT,
        synced_at               TEXT,
        created_at              TEXT NOT NULL DEFAULT (datetime('now')),
        updated_at              TEXT NOT NULL DEFAULT (datetime('now'))
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

    // v3: pending_outcomes table for offline outcome recording
    await db.execute('''
      CREATE TABLE IF NOT EXISTS pending_outcomes (
        id              TEXT PRIMARY KEY,
        job_id          TEXT NOT NULL,
        outcome_json    TEXT NOT NULL,
        status          TEXT NOT NULL DEFAULT 'PENDING',
        retry_count     INTEGER NOT NULL DEFAULT 0,
        last_error      TEXT,
        created_at      TEXT NOT NULL DEFAULT (datetime('now')),
        attempted_at    TEXT,
        FOREIGN KEY (job_id) REFERENCES offline_jobs(id)
      )
    ''');

    await db.execute('CREATE INDEX IF NOT EXISTS idx_pending_status ON pending_submissions(status)');
    await db.execute('CREATE INDEX IF NOT EXISTS idx_photos_job ON offline_photos(job_id)');
    await db.execute('CREATE INDEX IF NOT EXISTS idx_jobs_status ON offline_jobs(status)');
    await db.execute('CREATE INDEX IF NOT EXISTS idx_outcomes_status ON pending_outcomes(status)');
    await db.execute('CREATE INDEX IF NOT EXISTS idx_outcomes_job ON pending_outcomes(job_id)');

    // pending_illegal_reports table (v3)
    await db.execute('''
      CREATE TABLE IF NOT EXISTS pending_illegal_reports (
        id           TEXT PRIMARY KEY,
        job_id       TEXT,
        report_json  TEXT NOT NULL,
        status       TEXT NOT NULL DEFAULT 'PENDING',
        retry_count  INTEGER NOT NULL DEFAULT 0,
        last_error   TEXT,
        created_at   TEXT NOT NULL
      )
    ''');
    await db.execute('CREATE INDEX IF NOT EXISTS idx_illegal_status ON pending_illegal_reports(status)');
  }

  Future<void> _onUpgrade(Database db, int oldVersion, int newVersion) async {
    // v1 → v2: indexes only (no schema change)
    // v2 → v3: add leakage + outcome columns to offline_jobs, add pending_outcomes table
    if (oldVersion < 3) {
      // Add new columns to offline_jobs (SQLite ALTER TABLE only supports ADD COLUMN)
      final alterStatements = [
        'ALTER TABLE offline_jobs ADD COLUMN monthly_leakage_ghs REAL',
        'ALTER TABLE offline_jobs ADD COLUMN annualised_leakage_ghs REAL',
        'ALTER TABLE offline_jobs ADD COLUMN leakage_category TEXT',
        'ALTER TABLE offline_jobs ADD COLUMN outcome TEXT',
        'ALTER TABLE offline_jobs ADD COLUMN outcome_notes TEXT',
        'ALTER TABLE offline_jobs ADD COLUMN meter_found INTEGER',
        'ALTER TABLE offline_jobs ADD COLUMN address_confirmed INTEGER',
        'ALTER TABLE offline_jobs ADD COLUMN recommended_action TEXT',
        'ALTER TABLE offline_jobs ADD COLUMN outcome_recorded_at TEXT',
      ];

      for (final sql in alterStatements) {
        try {
          await db.execute(sql);
        } catch (_) {
          // Column may already exist if partial migration ran — ignore
        }
      }

      // Create pending_illegal_reports table
      await db.execute('''
        CREATE TABLE IF NOT EXISTS pending_illegal_reports (
          id           TEXT PRIMARY KEY,
          job_id       TEXT,
          report_json  TEXT NOT NULL,
          status       TEXT NOT NULL DEFAULT 'PENDING',
          retry_count  INTEGER NOT NULL DEFAULT 0,
          last_error   TEXT,
          created_at   TEXT NOT NULL
        )
      ''');
      try {
        await db.execute('CREATE INDEX IF NOT EXISTS idx_illegal_status ON pending_illegal_reports(status)');
      } catch (_) {}

      // Create pending_outcomes table
      await db.execute('''
        CREATE TABLE IF NOT EXISTS pending_outcomes (
          id              TEXT PRIMARY KEY,
          job_id          TEXT NOT NULL,
          outcome_json    TEXT NOT NULL,
          status          TEXT NOT NULL DEFAULT 'PENDING',
          retry_count     INTEGER NOT NULL DEFAULT 0,
          last_error      TEXT,
          created_at      TEXT NOT NULL DEFAULT (datetime('now')),
          attempted_at    TEXT,
          FOREIGN KEY (job_id) REFERENCES offline_jobs(id)
        )
      ''');

      await db.execute('CREATE INDEX IF NOT EXISTS idx_outcomes_status ON pending_outcomes(status)');
      await db.execute('CREATE INDEX IF NOT EXISTS idx_outcomes_job ON pending_outcomes(job_id)');
    }
  }

  // ─── Job Cache ─────────────────────────────────────────────────────────────

  /// Cache jobs fetched from the API using server-wins conflict resolution.
  ///
  /// H2 Fix: Instead of blindly replacing local records (ConflictAlgorithm.replace),
  /// we implement a "server-wins for status, preserve local pending submissions"
  /// strategy:
  ///   - If the local record has a PENDING submission queued, we keep the local
  ///     status and merge only non-status fields from the server.
  ///   - If no pending submission exists, the server record wins entirely.
  Future<void> cacheJobs(List<FieldJob> jobs) async {
    final database = await db;
    final batch = database.batch();

    for (final job in jobs) {
      // Check if there's a pending submission for this job
      final pending = await database.query(
        'pending_submissions',
        where: 'job_id = ? AND status = ?',
        whereArgs: [job.id, 'PENDING'],
        limit: 1,
      );

      if (pending.isNotEmpty) {
        // Preserve local status — only update non-status fields
        batch.update(
          'offline_jobs',
          _jobToRow(job, preserveStatus: true),
          where: 'id = ?',
          whereArgs: [job.id],
          conflictAlgorithm: ConflictAlgorithm.ignore,
        );
        // Insert if not exists
        batch.insert(
          'offline_jobs',
          _jobToRow(job),
          conflictAlgorithm: ConflictAlgorithm.ignore,
        );
      } else {
        // Server wins — full replace
        batch.insert(
          'offline_jobs',
          _jobToRow(job),
          conflictAlgorithm: ConflictAlgorithm.replace,
        );
      }
    }

    await batch.commit(noResult: true);
  }

  Future<List<FieldJob>> loadCachedJobs() async {
    final database = await db;
    final rows = await database.query(
      'offline_jobs',
      orderBy: 'created_at DESC',
    );
    return rows.map(_rowToJob).toList();
  }

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

  /// Update a job's outcome fields locally after recording an outcome.
  Future<void> updateJobOutcomeLocally(
    String jobId,
    FieldJobOutcomeRequest request,
  ) async {
    final database = await db;
    await database.update(
      'offline_jobs',
      {
        'outcome':              request.outcome.toApiString(),
        'outcome_notes':        request.outcomeNotes,
        'meter_found':          request.meterFound == null ? null : (request.meterFound! ? 1 : 0),
        'address_confirmed':    request.addressConfirmed == null ? null : (request.addressConfirmed! ? 1 : 0),
        'recommended_action':   request.recommendedAction,
        'outcome_recorded_at':  DateTime.now().toIso8601String(),
        'status':               FieldJobStatus.outcomeRecorded.toApiString(),
        'updated_at':           DateTime.now().toIso8601String(),
      },
      where: 'id = ?',
      whereArgs: [jobId],
    );
  }

  // ─── Pending Submissions ───────────────────────────────────────────────────

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

  Future<List<PendingSubmission>> getPendingSubmissions() async {
    final database = await db;
    final rows = await database.query(
      'pending_submissions',
      where: 'status = ? AND retry_count < 5',
      whereArgs: ['PENDING'],
      orderBy: 'created_at ASC',
    );
    return rows.map(_rowToPendingSubmission).toList();
  }

  Future<void> markSubmissionDone(String id) async {
    final database = await db;
    await database.update(
      'pending_submissions',
      {'status': 'DONE', 'attempted_at': DateTime.now().toIso8601String()},
      where: 'id = ?',
      whereArgs: [id],
    );
  }

  Future<void> markSubmissionFailed(String id, String error) async {
    final database = await db;
    // Keep status PENDING so the sync service retries it (retry_count < 5 guard prevents infinite loops)
    await database.rawUpdate('''
      UPDATE pending_submissions
      SET retry_count = retry_count + 1,
          last_error = ?,
          attempted_at = ?
      WHERE id = ?
    ''', [error, DateTime.now().toIso8601String(), id]);
  }

  // ─── Pending Outcomes ──────────────────────────────────────────────────────

  /// Queue an outcome for sync when connectivity is restored.
  Future<void> queueOutcome(
    String jobId,
    FieldJobOutcomeRequest request,
  ) async {
    final database = await db;
    await database.insert('pending_outcomes', {
      'id':           _uuid.v4(),
      'job_id':       jobId,
      'outcome_json': jsonEncode(request.toJson()),
      'status':       'PENDING',
      'retry_count':  0,
      'created_at':   DateTime.now().toIso8601String(),
    });
    // Also update local job record so the UI reflects the outcome immediately
    await updateJobOutcomeLocally(jobId, request);
  }

  Future<List<PendingOutcome>> getPendingOutcomes() async {
    final database = await db;
    final rows = await database.query(
      'pending_outcomes',
      where: 'status = ? AND retry_count < 5',
      whereArgs: ['PENDING'],
      orderBy: 'created_at ASC',
    );
    return rows.map(_rowToPendingOutcome).toList();
  }

  Future<void> markOutcomeDone(String id) async {
    final database = await db;
    await database.update(
      'pending_outcomes',
      {'status': 'DONE', 'attempted_at': DateTime.now().toIso8601String()},
      where: 'id = ?',
      whereArgs: [id],
    );
  }

  Future<void> markOutcomeFailed(String id, String error) async {
    final database = await db;
    // Keep status PENDING so the sync service retries it (retry_count < 5 guard prevents infinite loops)
    await database.rawUpdate('''
      UPDATE pending_outcomes
      SET retry_count = retry_count + 1,
          last_error = ?,
          attempted_at = ?
      WHERE id = ?
    ''', [error, DateTime.now().toIso8601String(), id]);
  }

  // ─── Illegal Connection Reports (offline queue) ───────────────────────────

  /// MOB-03 fix: Retrieve pending illegal connection reports for sync.
  /// Returns reports with status=PENDING and retry_count < 5.
  Future<List<Map<String, dynamic>>> getPendingIllegalReports() async {
    final database = await db;
    final rows = await database.query(
      'pending_illegal_reports',
      where: 'status = ? AND retry_count < 5',
      whereArgs: ['PENDING'],
      orderBy: 'created_at ASC',
    );
    return rows.map((r) => Map<String, dynamic>.from(r)).toList();
  }

  Future<void> markIllegalReportDone(String id) async {
    final database = await db;
    await database.update(
      'pending_illegal_reports',
      {'status': 'DONE', 'attempted_at': DateTime.now().toIso8601String()},
      where: 'id = ?',
      whereArgs: [id],
    );
  }

  Future<void> markIllegalReportFailed(String id, String error) async {
    final database = await db;
    // Keep status PENDING so the sync service retries (retry_count < 5 guard)
    await database.rawUpdate(
      'UPDATE pending_illegal_reports SET retry_count = retry_count + 1, attempted_at = ? WHERE id = ?',
      [DateTime.now().toIso8601String(), id],
    );
  }

  // ─── Photos ────────────────────────────────────────────────────────────────

  Future<void> savePhoto(String jobId, MeterPhoto photo) async {
    final database = await db;
    await database.insert('offline_photos', {
      'id':           _uuid.v4(),
      'job_id':       jobId,
      'local_path':   photo.localPath,
      'photo_hash':   photo.hash,
      'gps_lat':      photo.gpsLat,
      'gps_lng':      photo.gpsLng,
      'gps_accuracy': photo.gpsAccuracyM,
      'captured_at':  photo.capturedAt.toIso8601String(),
      'within_fence': photo.withinFence ? 1 : 0,
      'uploaded':     0,
    });
  }

  Future<List<MeterPhoto>> getPhotosForJob(String jobId) async {
    final database = await db;
    final rows = await database.query(
      'offline_photos',
      where: 'job_id = ?',
      whereArgs: [jobId],
    );
    return rows.map(_rowToPhoto).toList();
  }

  // ─── Sync Stats ────────────────────────────────────────────────────────────

  Future<SyncStats> getSyncStats() async {
    final database = await db;

    final jobCount = Sqflite.firstIntValue(
      await database.rawQuery('SELECT COUNT(*) FROM offline_jobs'),
    ) ?? 0;

    final pendingCount = Sqflite.firstIntValue(
      await database.rawQuery(
        "SELECT COUNT(*) FROM pending_submissions WHERE status = 'PENDING'",
      ),
    ) ?? 0;

    final photoCount = Sqflite.firstIntValue(
      await database.rawQuery(
        'SELECT COUNT(*) FROM offline_photos WHERE uploaded = 0',
      ),
    ) ?? 0;

    final outcomeCount = Sqflite.firstIntValue(
      await database.rawQuery(
        "SELECT COUNT(*) FROM pending_outcomes WHERE status = 'PENDING'",
      ),
    ) ?? 0;

    final lastSyncRow = await database.rawQuery(
      'SELECT MAX(synced_at) as last_sync FROM offline_jobs',
    );
    final lastSyncStr = lastSyncRow.first['last_sync'] as String?;

    return SyncStats(
      cachedJobs:         jobCount,
      pendingSubmissions: pendingCount,
      pendingPhotos:      photoCount,
      pendingOutcomes:    outcomeCount,
      lastSyncAt:         lastSyncStr != null ? DateTime.tryParse(lastSyncStr) : null,
    );
  }

  // ─── Row Mappers ───────────────────────────────────────────────────────────

  Map<String, dynamic> _jobToRow(FieldJob job, {bool preserveStatus = false}) {
    final row = <String, dynamic>{
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
      'scheduled_at':           job.scheduledAt?.toIso8601String(),
      'dispatched_at':          job.dispatchedAt?.toIso8601String(),
      'notes':                  job.notes,
      'estimated_variance_ghs': job.estimatedVarianceGhs,
      // v3 fields
      'monthly_leakage_ghs':    job.monthlyLeakageGhs,
      'annualised_leakage_ghs': job.annualisedLeakageGhs,
      'leakage_category':       job.leakageCategory?.toApiString(),
      'outcome':                job.outcome?.toApiString(),
      'outcome_notes':          job.outcomeNotes,
      'meter_found':            job.meterFound == null ? null : (job.meterFound! ? 1 : 0),
      'address_confirmed':      job.addressConfirmed == null ? null : (job.addressConfirmed! ? 1 : 0),
      'recommended_action':     job.recommendedAction,
      'outcome_recorded_at':    job.outcomeRecordedAt?.toIso8601String(),
      'updated_at':             DateTime.now().toIso8601String(),
    };
    if (!preserveStatus) {
      row['status'] = job.status.toApiString();
    }
    return row;
  }

  FieldJob _rowToJob(Map<String, dynamic> row) => FieldJob(
    id:                   row['id'] as String,
    jobReference:         row['job_reference'] as String?,
    auditEventId:         row['audit_event_id'] as String?,
    accountNumber:        row['account_number'] as String,
    customerName:         row['customer_name'] as String,
    address:              row['address'] as String,
    gpsLat:               row['gps_lat'] as double,
    gpsLng:               row['gps_lng'] as double,
    anomalyType:          row['anomaly_type'] as String? ?? 'UNKNOWN',
    alertLevel:           AlertLevel.fromString(row['alert_level'] as String? ?? 'MEDIUM'),
    status:               FieldJobStatus.fromString(row['status'] as String? ?? 'QUEUED'),
    scheduledAt:          row['scheduled_at'] != null
                            ? DateTime.tryParse(row['scheduled_at'] as String)
                            : null,
    dispatchedAt:         row['dispatched_at'] != null
                            ? DateTime.tryParse(row['dispatched_at'] as String)
                            : null,
    notes:                row['notes'] as String?,
    estimatedVarianceGhs: row['estimated_variance_ghs'] as double?,
    // v3 fields
    monthlyLeakageGhs:    row['monthly_leakage_ghs'] as double?,
    annualisedLeakageGhs: row['annualised_leakage_ghs'] as double?,
    leakageCategory:      row['leakage_category'] != null
                            ? LeakageCategory.fromString(row['leakage_category'] as String)
                            : null,
    outcome:              FieldJobOutcome.fromString(row['outcome'] as String?),
    outcomeNotes:         row['outcome_notes'] as String?,
    meterFound:           row['meter_found'] != null ? (row['meter_found'] as int) == 1 : null,
    addressConfirmed:     row['address_confirmed'] != null ? (row['address_confirmed'] as int) == 1 : null,
    recommendedAction:    row['recommended_action'] as String?,
    outcomeRecordedAt:    row['outcome_recorded_at'] != null
                            ? DateTime.tryParse(row['outcome_recorded_at'] as String)
                            : null,
  );

  PendingSubmission _rowToPendingSubmission(Map<String, dynamic> row) {
    final submissionJson = jsonDecode(row['submission_json'] as String) as Map<String, dynamic>;
    final photoUris = (jsonDecode(row['photo_uris'] as String) as List<dynamic>)
        .cast<String>();

    return PendingSubmission(
      id:          row['id'] as String,
      jobId:       row['job_id'] as String,
      submission:  JobSubmission(
        jobId:         submissionJson['job_id'] as String,
        ocrReadingM3:  (submissionJson['ocr_reading_m3'] as num).toDouble(),
        ocrConfidence: (submissionJson['ocr_confidence'] as num).toDouble(),
        ocrStatus:     OcrStatus.fromString(submissionJson['ocr_status'] as String? ?? 'MANUAL'),
        officerNotes:  submissionJson['officer_notes'] as String? ?? '',
        gpsLat:        (submissionJson['gps_lat'] as num).toDouble(),
        gpsLng:        (submissionJson['gps_lng'] as num).toDouble(),
        gpsAccuracyM:  (submissionJson['gps_accuracy_m'] as num).toDouble(),
        photoUrls:     (submissionJson['photo_urls'] as List<dynamic>?)?.cast<String>() ?? [],
        photoHashes:   (submissionJson['photo_hashes'] as List<dynamic>?)?.cast<String>() ?? [],
      ),
      photoUris:   photoUris,
      status:      row['status'] as String,
      retryCount:  row['retry_count'] as int,
      lastError:   row['last_error'] as String?,
      attemptedAt: row['attempted_at'] != null
                     ? DateTime.tryParse(row['attempted_at'] as String)
                     : null,
    );
  }

  PendingOutcome _rowToPendingOutcome(Map<String, dynamic> row) {
    final outcomeJson = jsonDecode(row['outcome_json'] as String) as Map<String, dynamic>;
    return PendingOutcome(
      id:             row['id'] as String,
      jobId:          row['job_id'] as String,
      outcomeRequest: FieldJobOutcomeRequest(
        outcome:            FieldJobOutcome.fromString(outcomeJson['outcome'] as String?)!,
        outcomeNotes:       outcomeJson['outcome_notes'] as String?,
        meterFound:         outcomeJson['meter_found'] as bool?,
        addressConfirmed:   outcomeJson['address_confirmed'] as bool?,
        recommendedAction:  outcomeJson['recommended_action'] as String?,
        estimatedMonthlyM3: outcomeJson['estimated_monthly_m3'] != null
                              ? (outcomeJson['estimated_monthly_m3'] as num).toDouble()
                              : null,
      ),
      status:     row['status'] as String,
      retryCount: row['retry_count'] as int,
      lastError:  row['last_error'] as String?,
      createdAt:  DateTime.tryParse(row['created_at'] as String) ?? DateTime.now(),
    );
  }

  MeterPhoto _rowToPhoto(Map<String, dynamic> row) => MeterPhoto(
    localPath:    row['local_path'] as String,
    hash:         row['photo_hash'] as String,
    gpsLat:       row['gps_lat'] as double,
    gpsLng:       row['gps_lng'] as double,
    gpsAccuracyM: row['gps_accuracy'] as double,
    capturedAt:   DateTime.parse(row['captured_at'] as String),
    withinFence:  (row['within_fence'] as int) == 1,
    remoteUrl:    row['remote_url'] as String?,
  );

  // ─── Illegal Connection Reports ──────────────────────────────────────────────

  /// Queue an illegal connection report for offline sync.
  Future<void> queueIllegalConnectionReport({
    required String connectionType,
    required String severity,
    required String description,
    required double latitude,
    required double longitude,
    required int photoCount,
    required List<String> photoHashes,
    String? jobId,
  }) async {
    final database = await db;
    final reportJson = jsonEncode({
      'connection_type': connectionType,
      'severity':        severity,
      'description':     description,
      'latitude':        latitude,
      'longitude':       longitude,
      'photo_count':     photoCount,
      'photo_hashes':    photoHashes,
      if (jobId != null) 'job_id': jobId,
      'reported_at':     DateTime.now().toIso8601String(),
    });
    await database.insert('pending_illegal_reports', {
      'id':          _uuid.v4(),
      'job_id':      jobId,
      'report_json': reportJson,
      'status':      'PENDING',
      'retry_count': 0,
      'created_at':  DateTime.now().toIso8601String(),
    });
  }

  // ─── Utility ───────────────────────────────────────────────────────────────

  /// Clear all cached data — used in tests and for logout.
  Future<void> clearAll() async {
    final database = await db;
    await database.delete('pending_illegal_reports');
    await database.delete('pending_outcomes');
    await database.delete('pending_submissions');
    await database.delete('offline_photos');
    await database.delete('offline_jobs');
  }
}

