// GN-WAAS Field Officer App — Remote Config Service
// Fetches admin-controlled configuration from the API gateway.
// This allows the admin portal to control mobile app behaviour without
// requiring an app update (geofence radius, biometric requirement, etc.)

import 'package:dio/dio.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'dart:convert';
import '../config/app_config.dart';

/// MobileConfig holds all admin-controlled settings for the mobile app.
/// Values are fetched from /api/v1/config/mobile on startup and cached locally.
class MobileConfig {
  final double geofenceRadiusM;
  final bool requireBiometric;
  final bool blindAuditDefault;
  final bool requireSurroundingsPhoto;
  final int maxPhotoAgeMinutes;
  final double ocrConflictTolerancePct;
  final int syncIntervalSeconds;
  final int maxJobsPerOfficer;
  final String appMinVersion;
  final String appLatestVersion;
  final bool forceUpdate;
  final bool maintenanceMode;
  final String maintenanceMessage;

  const MobileConfig({
    this.geofenceRadiusM          = 100.0,
    this.requireBiometric         = true,
    this.blindAuditDefault        = true,
    this.requireSurroundingsPhoto = true,
    this.maxPhotoAgeMinutes       = 5,
    this.ocrConflictTolerancePct  = 2.0,
    this.syncIntervalSeconds      = 30,
    this.maxJobsPerOfficer        = 5,
    this.appMinVersion            = '1.0.0',
    this.appLatestVersion         = '1.0.0',
    this.forceUpdate              = false,
    this.maintenanceMode          = false,
    this.maintenanceMessage       = '',
  });

  /// Default config used when offline and no cached config exists
  static const MobileConfig defaults = MobileConfig();

  factory MobileConfig.fromJson(Map<String, dynamic> json) => MobileConfig(
    geofenceRadiusM:          (json['geofence_radius_m']          as num?)?.toDouble() ?? 100.0,
    requireBiometric:         json['require_biometric']           as bool?  ?? true,
    blindAuditDefault:        json['blind_audit_default']         as bool?  ?? true,
    requireSurroundingsPhoto: json['require_surroundings_photo']  as bool?  ?? true,
    maxPhotoAgeMinutes:       json['max_photo_age_minutes']       as int?   ?? 5,
    ocrConflictTolerancePct:  (json['ocr_conflict_tolerance_pct'] as num?)?.toDouble() ?? 2.0,
    syncIntervalSeconds:      json['sync_interval_seconds']       as int?   ?? 30,
    maxJobsPerOfficer:        json['max_jobs_per_officer']        as int?   ?? 5,
    appMinVersion:            json['app_min_version']             as String? ?? '1.0.0',
    appLatestVersion:         json['app_latest_version']          as String? ?? '1.0.0',
    forceUpdate:              json['force_update']                as bool?  ?? false,
    maintenanceMode:          json['maintenance_mode']            as bool?  ?? false,
    maintenanceMessage:       json['maintenance_message']         as String? ?? '',
  );

  Map<String, dynamic> toJson() => {
    'geofence_radius_m':           geofenceRadiusM,
    'require_biometric':           requireBiometric,
    'blind_audit_default':         blindAuditDefault,
    'require_surroundings_photo':  requireSurroundingsPhoto,
    'max_photo_age_minutes':       maxPhotoAgeMinutes,
    'ocr_conflict_tolerance_pct':  ocrConflictTolerancePct,
    'sync_interval_seconds':       syncIntervalSeconds,
    'max_jobs_per_officer':        maxJobsPerOfficer,
    'app_min_version':             appMinVersion,
    'app_latest_version':          appLatestVersion,
    'force_update':                forceUpdate,
    'maintenance_mode':            maintenanceMode,
    'maintenance_message':         maintenanceMessage,
  };
}

/// RemoteConfigService fetches and caches admin-controlled mobile config.
/// On startup: fetch from API → cache in SharedPreferences.
/// On failure: use cached config → fall back to hardcoded defaults.
class RemoteConfigService {
  static const String _cacheKey = 'gnwaas_mobile_config';
  final Dio _dio;

  RemoteConfigService()
      : _dio = Dio(BaseOptions(
          baseUrl:        AppConfig.apiBaseUrl,
          connectTimeout: const Duration(seconds: 10),
          receiveTimeout: const Duration(seconds: 10),
        ));

  /// Fetch config from API. Returns cached or default if offline.
  Future<MobileConfig> fetchConfig() async {
    try {
      final res = await _dio.get('/config/mobile');
      final body = res.data as Map<String, dynamic>;
      final data = body.containsKey('data')
          ? body['data'] as Map<String, dynamic>
          : body;

      final config = MobileConfig.fromJson(data);
      await _cacheConfig(config);
      return config;
    } on DioException {
      // Offline or server error — use cached config
      return await _loadCachedConfig() ?? MobileConfig.defaults;
    } catch (_) {
      return await _loadCachedConfig() ?? MobileConfig.defaults;
    }
  }

  Future<void> _cacheConfig(MobileConfig config) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_cacheKey, jsonEncode(config.toJson()));
  }

  Future<MobileConfig?> _loadCachedConfig() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final raw = prefs.getString(_cacheKey);
      if (raw == null) return null;
      return MobileConfig.fromJson(jsonDecode(raw) as Map<String, dynamic>);
    } catch (_) {
      return null;
    }
  }
}
