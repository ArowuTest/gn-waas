// GN-WAAS Field Officer App — Sync Service
// Background sync: uploads pending submissions when connectivity returns

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

  /// Sync all pending submissions to the server
  Future<int> syncPendingSubmissions() async {
    if (!await isOnline()) return 0;

    final pending = await _storage.getPendingSubmissions();
    if (pending.isEmpty) return 0;

    int synced = 0;
    for (final item in pending) {
      if (item.retryCount >= 5) continue;
      try {
        await _api.submitJobEvidence(item.submission);
        await _storage.markSubmissionDone(item.id);
        await _storage.updateJobStatusLocally(item.jobId, FieldJobStatus.completed);
        synced++;
      } catch (e) {
        await _storage.markSubmissionFailed(item.id, e.toString());
      }
    }
    return synced;
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
