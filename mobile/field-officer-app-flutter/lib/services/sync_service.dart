// GN-WAAS Field Officer App — Sync Service
// Background sync: uploads pending submissions and outcomes when connectivity returns

import 'dart:convert';
import 'package:connectivity_plus/connectivity_plus.dart';
import '../models/models.dart';
import 'api_service.dart';
import 'offline_storage_service.dart';

class SyncService {
  final ApiService _api;
  final OfflineStorageService _storage;

  SyncService({required ApiService api, required OfflineStorageService storage})
      : _api = api,
        _storage = storage;

  /// Check if device has network connectivity
  Future<bool> isOnline() async {
    final results = await Connectivity().checkConnectivity();
    // connectivity_plus v6 returns List<ConnectivityResult>
    return results.isNotEmpty &&
           !results.contains(ConnectivityResult.none);
  }

  /// Sync all pending submissions to the server.
  /// Returns the number of successfully synced submissions.
  Future<int> syncPendingSubmissions() async {
    if (!await isOnline()) return 0;

    final pending = await _storage.getPendingSubmissions();
    if (pending.isEmpty) return 0;

    int synced = 0;
    for (final item in pending) {
      if (item.retryCount >= 5) continue;
      try {
        // ── Upload queued photos to MinIO before submitting ────────────────
        // When the device was offline, photos were stored locally.
        // Now that we're online, upload them and attach the object keys.
        final uploadedKeys = <String>[];
        for (final localPath in item.photoUris) {
          try {
            final uploadMeta = await _api.getUploadUrl(
              jobId:    item.jobId,
              filename: 'meter_${item.jobId}_sync.jpg',
            );
            final uploadUrl = uploadMeta['upload_url'] as String? ?? '';
            final objectKey = uploadMeta['object_key'] as String? ?? '';
            if (uploadUrl.isNotEmpty) {
              final key = await _api.uploadPhotoToMinIO(
                localPath:  localPath,
                uploadUrl:  uploadUrl,
                objectKey:  objectKey,
              );
              if (key != null) uploadedKeys.add(key);
            }
          } catch (_) {
            // Non-fatal: submit without this photo
          }
        }

        // Attach uploaded keys to submission
        final submissionWithPhotos = uploadedKeys.isEmpty
            ? item.submission
            : JobSubmission(
                jobId:         item.submission.jobId,
                ocrReadingM3:  item.submission.ocrReadingM3,
                ocrConfidence: item.submission.ocrConfidence,
                ocrStatus:     item.submission.ocrStatus,
                officerNotes:  item.submission.officerNotes,
                gpsLat:        item.submission.gpsLat,
                gpsLng:        item.submission.gpsLng,
                gpsAccuracyM:  item.submission.gpsAccuracyM,
                photoUrls:     uploadedKeys,
                photoHashes:   item.submission.photoHashes,
              );

        await _api.submitJobEvidence(submissionWithPhotos);
        await _storage.markSubmissionDone(item.id);
        await _storage.updateJobStatusLocally(item.jobId, FieldJobStatus.completed);
        synced++;
      } catch (e) {
        await _storage.markSubmissionFailed(item.id, e.toString());
      }
    }
    return synced;
  }

  /// Sync all pending outcomes to the server.
  /// Returns the number of successfully synced outcomes.
  Future<int> syncPendingOutcomes() async {
    if (!await isOnline()) return 0;

    final pending = await _storage.getPendingOutcomes();
    if (pending.isEmpty) return 0;

    int synced = 0;
    for (final item in pending) {
      if (item.retryCount >= 5) continue;
      try {
        await _api.recordFieldJobOutcome(item.jobId, item.outcomeRequest);
        await _storage.markOutcomeDone(item.id);
        synced++;
      } catch (e) {
        await _storage.markOutcomeFailed(item.id, e.toString());
      }
    }
    return synced;
  }

  /// MOB-03 fix: Sync pending illegal connection reports.
  /// Uses TIP_SUBMISSION action_type to match the backend sync_action_type enum.
  /// Returns the number of successfully synced reports.
  Future<int> syncPendingIllegalReports() async {
    if (!await isOnline()) return 0;

    final pending = await _storage.getPendingIllegalReports();
    if (pending.isEmpty) return 0;

    int synced = 0;
    for (final row in pending) {
      final id = row['id'] as String;
      final retryCount = (row['retry_count'] as int?) ?? 0;
      if (retryCount >= 5) continue;

      try {
        // Decode the stored report JSON
        final reportJson = row['report_json'] as String? ?? '{}';
        Map<String, dynamic> reportMap;
        try {
          reportMap = Map<String, dynamic>.from(
            jsonDecode(reportJson) as Map<String, dynamic>,
          );
        } catch (_) {
          reportMap = {};
        }

        // Submit via the API — photos were already uploaded at capture time
        // (or will be skipped gracefully if MinIO is unavailable).
        // The report JSON contains photo_hashes for chain-of-custody.
        await _api.submitIllegalConnectionReport(
          _MapReport(reportMap),
          const [],  // Photos already uploaded at capture; no re-upload needed
        );
        await _storage.markIllegalReportDone(id);
        synced++;
      } catch (e) {
        await _storage.markIllegalReportFailed(id, e.toString());
      }
    }
    return synced;
  }

  /// Sync all pending data (submissions + outcomes + illegal reports).
  /// Returns total number of items synced.
  Future<int> syncAll() async {
    if (!await isOnline()) return 0;
    final submissions = await syncPendingSubmissions();
    final outcomes    = await syncPendingOutcomes();
    final reports     = await syncPendingIllegalReports();
    return submissions + outcomes + reports;
  }

  /// Fetch fresh jobs from API and cache them
  Future<List<FieldJob>> refreshJobs() async {
    if (!await isOnline()) {
      return _storage.loadCachedJobs();
    }
    try {
      final jobs = await _api.getMyJobs();
      await _storage.cacheJobs(jobs);
      return jobs;
    } catch (_) {
      return _storage.loadCachedJobs();
    }
  }

  Future<SyncStats> getSyncStats() => _storage.getSyncStats();
}

/// Wraps a raw Map as a report object with toJson() for submitIllegalConnectionReport.
/// Used when syncing offline-queued illegal connection reports.
class _MapReport {
  final Map<String, dynamic> _data;
  const _MapReport(this._data);
  Map<String, dynamic> toJson() => _data;
}
