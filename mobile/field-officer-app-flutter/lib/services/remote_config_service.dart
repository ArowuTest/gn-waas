// GN-WAAS Field Officer App — Remote Config Service
// Fetches admin-controlled configuration from the API gateway.
// This allows the admin portal to control mobile app behaviour without
// requiring an app update (geofence radius, biometric requirement, etc.)

import 'package:dio/dio.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'dart:convert';
import '../config/app_config.dart';
import '../models/models.dart';

// MobileConfig is defined in models.dart — imported above.

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
