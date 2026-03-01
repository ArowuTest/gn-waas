// GN-WAAS Field Officer App — API Service
// Handles all HTTP communication with the api-gateway

import 'dart:convert';
import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import '../config/app_config.dart';
import '../models/models.dart';

class ApiService {
  late final Dio _dio;
  final FlutterSecureStorage _secureStorage;

  static const String _tokenKey = 'gnwaas_token';
  static const String _userKey  = 'gnwaas_user';

  ApiService({FlutterSecureStorage? secureStorage})
      : _secureStorage = secureStorage ?? const FlutterSecureStorage() {
    _dio = Dio(BaseOptions(
      baseUrl:        AppConfig.apiBaseUrl,
      connectTimeout: AppConfig.apiTimeout,
      receiveTimeout: AppConfig.apiTimeout,
      headers: {'Content-Type': 'application/json'},
    ));

    // Request interceptor: attach JWT
    _dio.interceptors.add(InterceptorsWrapper(
      onRequest: (options, handler) async {
        final token = await _secureStorage.read(key: _tokenKey);
        if (token != null) {
          options.headers['Authorization'] = 'Bearer $token';
        }
        handler.next(options);
      },
      onError: (error, handler) async {
        if (error.response?.statusCode == 401) {
          await _secureStorage.delete(key: _tokenKey);
          await _secureStorage.delete(key: _userKey);
        }
        handler.next(error);
      },
    ));
  }

  // ─── Auth ──────────────────────────────────────────────────────────────────

  Future<Map<String, dynamic>> login(String email, String password) async {
    final res = await _dio.post('/auth/login', data: {
      'email':    email,
      'password': password,
    });
    final data = res.data['data'] as Map<String, dynamic>;
    await _secureStorage.write(key: _tokenKey, value: data['access_token'] as String);
    await _secureStorage.write(key: _userKey,  value: jsonEncode(data['user']));
    return data;
  }

  Future<void> logout() async {
    await _secureStorage.delete(key: _tokenKey);
    await _secureStorage.delete(key: _userKey);
  }

  Future<String?> getStoredToken() => _secureStorage.read(key: _tokenKey);

  Future<User?> getStoredUser() async {
    final raw = await _secureStorage.read(key: _userKey);
    if (raw == null) return null;
    return User.fromJson(jsonDecode(raw) as Map<String, dynamic>);
  }

  // ─── Field Jobs ────────────────────────────────────────────────────────────

  Future<List<FieldJob>> getMyJobs() async {
    final res = await _dio.get('/field-jobs/my-jobs');
    final list = res.data['data'] as List<dynamic>;
    return list.map((j) => FieldJob.fromJson(j as Map<String, dynamic>)).toList();
  }

  Future<void> updateJobStatus(
    String jobId,
    FieldJobStatus status, {
    double? gpsLat,
    double? gpsLng,
  }) async {
    await _dio.patch('/field-jobs/$jobId/status', data: {
      'status':  status.toApiString(),
      if (gpsLat != null) 'gps_lat': gpsLat,
      if (gpsLng != null) 'gps_lng': gpsLng,
    });
  }

  Future<void> triggerSOS(
    String jobId, {
    required double gpsLat,
    required double gpsLng,
    required double gpsAccuracyM,
  }) async {
    await _dio.post('/field-jobs/$jobId/sos', data: {
      'gps_lat':        gpsLat,
      'gps_lng':        gpsLng,
      'gps_accuracy_m': gpsAccuracyM,
      'notes':          'SOS triggered from Flutter mobile app',
    });
  }

  // ─── Audit / Submission ────────────────────────────────────────────────────

  Future<void> submitJobEvidence(JobSubmission submission) async {
    await _dio.post('/audits', data: submission.toJson());
  }

  // ─── OCR ──────────────────────────────────────────────────────────────────

  Future<OcrResult> submitPhotoForOCR(String base64Image, String jobId) async {
    final res = await _dio.post('/ocr/process', data: {
      'image_base64': base64Image,
      'job_id':       jobId,
    });
    return OcrResult.fromJson(res.data['data'] as Map<String, dynamic>);
  }

  // ─── User ─────────────────────────────────────────────────────────────────

  Future<User> getMe() async {
    final res = await _dio.get('/users/me');
    return User.fromJson(res.data['data'] as Map<String, dynamic>);
  }
}
