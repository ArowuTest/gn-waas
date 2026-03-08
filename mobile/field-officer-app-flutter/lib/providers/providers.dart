// GN-WAAS Field Officer App — Riverpod Providers
// Central state management for auth, jobs, sync

import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/models.dart';
import '../services/api_service.dart';
import '../services/offline_storage_service.dart';
import '../services/sync_service.dart';
import '../services/location_service.dart';
import '../services/biometric_service.dart';
import '../services/remote_config_service.dart';

// ─── Service Providers ────────────────────────────────────────────────────────

final apiServiceProvider = Provider<ApiService>((ref) => ApiService());

final offlineStorageProvider = Provider<OfflineStorageService>(
  (ref) => OfflineStorageService(),
);

final syncServiceProvider = Provider<SyncService>((ref) => SyncService(
  api:     ref.read(apiServiceProvider),
  storage: ref.read(offlineStorageProvider),
));

final locationServiceProvider = Provider<LocationService>(
  (ref) => LocationService(),
);

final biometricServiceProvider = Provider<BiometricService>(
  (ref) => BiometricService(),
);

final remoteConfigServiceProvider = Provider<RemoteConfigService>(
  (ref) => RemoteConfigService(),
);

// Async provider that fetches config on app startup
final mobileConfigProvider = FutureProvider<MobileConfig>((ref) async {
  final svc = ref.read(remoteConfigServiceProvider);
  return svc.fetchConfig();
});

// ─── Auth State ───────────────────────────────────────────────────────────────

class AuthState {
  final User? user;
  final String? token;
  final bool isLoading;
  final String? error;

  const AuthState({
    this.user,
    this.token,
    this.isLoading = false,
    this.error,
  });

  bool get isAuthenticated => user != null && token != null;

  AuthState copyWith({
    User? user,
    String? token,
    bool? isLoading,
    String? error,
    bool clearError = false,
    bool clearUser  = false,
  }) => AuthState(
    user:      clearUser  ? null : (user      ?? this.user),
    token:     clearUser  ? null : (token     ?? this.token),
    isLoading: isLoading  ?? this.isLoading,
    error:     clearError ? null : (error     ?? this.error),
  );
}

class AuthNotifier extends StateNotifier<AuthState> {
  final ApiService _api;

  AuthNotifier(this._api) : super(const AuthState()) {
    restoreSession();
  }

  Future<void> restoreSession() async {
    final user  = await _api.getStoredUser();
    final token = await _api.getStoredToken();
    if (user != null && token != null) {
      state = AuthState(user: user, token: token);
    }
  }

  Future<void> login(String email, String password) async {
    state = state.copyWith(isLoading: true, clearError: true);
    try {
      final data  = await _api.login(email, password);
      final user  = User.fromJson(data['user'] as Map<String, dynamic>);
      final token = data['access_token'] as String;
      state = AuthState(user: user, token: token);
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: _parseError(e),
      );
    }
  }

  Future<void> logout() async {
    await _api.logout();
    state = const AuthState();
  }

  /// Called after successful biometric authentication — sets state directly
  Future<void> loginWithToken(String token, User user) async {
    state = AuthState(user: user, token: token);
  }

  String _parseError(Object e) {
    if (e is Exception) return e.toString().replaceAll('Exception: ', '');
    return 'Login failed. Please try again.';
  }
}

final authProvider = StateNotifierProvider<AuthNotifier, AuthState>(
  (ref) => AuthNotifier(ref.read(apiServiceProvider)),
);

// ─── Jobs State ───────────────────────────────────────────────────────────────

class JobsState {
  final List<FieldJob> jobs;
  final bool isLoading;
  final bool isRefreshing;
  final String? error;
  final bool isOnline;
  final SyncStats? syncStats;

  const JobsState({
    this.jobs        = const [],
    this.isLoading   = false,
    this.isRefreshing = false,
    this.error,
    this.isOnline    = true,
    this.syncStats,
  });

  List<FieldJob> get pendingJobs => jobs
      .where((j) => j.status != FieldJobStatus.completed && j.status != FieldJobStatus.failed)
      .toList();

  List<FieldJob> get completedJobs => jobs
      .where((j) => j.status == FieldJobStatus.completed)
      .toList();

  JobsState copyWith({
    List<FieldJob>? jobs,
    bool? isLoading,
    bool? isRefreshing,
    String? error,
    bool? isOnline,
    SyncStats? syncStats,
    bool clearError = false,
  }) => JobsState(
    jobs:         jobs         ?? this.jobs,
    isLoading:    isLoading    ?? this.isLoading,
    isRefreshing: isRefreshing ?? this.isRefreshing,
    error:        clearError   ? null : (error ?? this.error),
    isOnline:     isOnline     ?? this.isOnline,
    syncStats:    syncStats    ?? this.syncStats,
  );
}

class JobsNotifier extends StateNotifier<JobsState> {
  final SyncService _sync;

  JobsNotifier(this._sync) : super(const JobsState()) {
    loadJobs();
  }

  Future<void> loadJobs() async {
    state = state.copyWith(isLoading: true, clearError: true);
    try {
      final online = await _sync.isOnline();
      final jobs   = await _sync.refreshJobs();
      final stats  = await _sync.getSyncStats();
      state = state.copyWith(
        jobs:      jobs,
        isLoading: false,
        isOnline:  online,
        syncStats: stats,
      );
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: e.toString(),
      );
    }
  }

  Future<void> refresh() async {
    state = state.copyWith(isRefreshing: true, clearError: true);
    try {
      final online = await _sync.isOnline();
      final jobs   = await _sync.refreshJobs();
      final stats  = await _sync.getSyncStats();
      state = state.copyWith(
        jobs:         jobs,
        isRefreshing: false,
        isOnline:     online,
        syncStats:    stats,
      );
    } catch (e) {
      state = state.copyWith(isRefreshing: false, error: e.toString());
    }
  }

  Future<int> syncPending() async {
    final synced = await _sync.syncPendingSubmissions();
    if (synced > 0) await refresh();
    return synced;
  }

  void updateJobStatus(String jobId, FieldJobStatus status) {
    final updated = state.jobs.map((j) {
      if (j.id == jobId) {
        j.status = status;
        return j;
      }
      return j;
    }).toList();
    state = state.copyWith(jobs: updated);
  }

  /// Replace a job in the list with a server-returned updated version.
  /// Used after recording an outcome to reflect the new status and outcome fields.
  void updateJobFromServer(FieldJob updatedJob) {
    final updated = state.jobs.map((j) {
      if (j.id == updatedJob.id) return updatedJob;
      return j;
    }).toList();
    state = state.copyWith(jobs: updated);
  }

  /// Sync all pending data (submissions + outcomes) and refresh job list.
  Future<int> syncAll() async {
    final synced = await _sync.syncAll();
    if (synced > 0) await refresh();
    return synced;
  }
}

final jobsProvider = StateNotifierProvider<JobsNotifier, JobsState>(
  (ref) => JobsNotifier(ref.read(syncServiceProvider)),
);

// ─── Active Job ───────────────────────────────────────────────────────────────

final activeJobProvider = StateProvider<FieldJob?>((ref) => null);

// ─── Connectivity ─────────────────────────────────────────────────────────────

final isOnlineProvider = FutureProvider<bool>((ref) async {
  return ref.read(syncServiceProvider).isOnline();
});

