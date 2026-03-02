// GN-WAAS Field Officer App — API Service
// Handles all HTTP communication with the api-gateway

import 'dart:convert';
import 'dart:io';
import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import '../config/app_config.dart';
import '../models/models.dart';

class ApiService {
  late final Dio _dio;
  final FlutterSecureStorage _secureStorage;

  static const String _tokenKey        = 'gnwaas_token';
  static const String _userKey         = 'gnwaas_user';
  static const String _refreshTokenKey = 'gnwaas_refresh_token';

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

    // Backend returns either:
    //   { access_token, token_type, expires_in, user, refresh_token? }  (dev mode)
    //   { data: { access_token, user, refresh_token? } }                 (production wrapper)
    final body = res.data as Map<String, dynamic>;
    final data = body.containsKey('data')
        ? body['data'] as Map<String, dynamic>
        : body;

    final token = (data['access_token'] ?? data['token']) as String;
    await _secureStorage.write(key: _tokenKey, value: token);

    // Normalise user object (backend may return 'name' or 'full_name')
    final rawUser = data['user'] as Map<String, dynamic>;
    final normUser = _normaliseUser(rawUser);
    await _secureStorage.write(key: _userKey, value: jsonEncode(normUser));

    // Store refresh token for biometric login
    final refreshToken = data['refresh_token'] as String?;
    if (refreshToken != null) {
      await _secureStorage.write(key: _refreshTokenKey, value: refreshToken);
    }

    return {...data, 'access_token': token, 'user': normUser};
  }

  /// Exchange a stored refresh token for a fresh access token (used by biometric login)
  Future<Map<String, dynamic>> refreshToken() async {
    final storedRefresh = await _secureStorage.read(key: _refreshTokenKey);
    if (storedRefresh == null) throw Exception('No refresh token stored');

    final res = await _dio.post('/auth/refresh', data: {
      'refresh_token': storedRefresh,
      'grant_type':    'refresh_token',
    });

    final body = res.data as Map<String, dynamic>;
    final data = body.containsKey('data')
        ? body['data'] as Map<String, dynamic>
        : body;

    final token = (data['access_token'] ?? data['token']) as String;
    await _secureStorage.write(key: _tokenKey, value: token);

    final newRefresh = data['refresh_token'] as String?;
    if (newRefresh != null) {
      await _secureStorage.write(key: _refreshTokenKey, value: newRefresh);
    }

    final rawUser = data['user'] as Map<String, dynamic>?;
    if (rawUser != null) {
      final normUser = _normaliseUser(rawUser);
      await _secureStorage.write(key: _userKey, value: jsonEncode(normUser));
    }

    return data;
  }

  /// Normalise user object: backend may return 'name' instead of 'full_name'
  Map<String, dynamic> _normaliseUser(Map<String, dynamic> raw) => {
    'id':           raw['id'] ?? raw['sub'] ?? '',
    'email':        raw['email'] ?? '',
    'full_name':    raw['full_name'] ?? raw['name'] ?? raw['preferred_username'] ?? '',
    'role':         raw['role'] ?? raw['roles']?.first ?? 'FIELD_OFFICER',
    'badge_number': raw['badge_number'],
    'district_id':  raw['district_id'],
  };

  Future<void> logout() async {
    await _secureStorage.delete(key: _tokenKey);
    await _secureStorage.delete(key: _userKey);
    await _secureStorage.delete(key: _refreshTokenKey);
  }

  Future<String?> getStoredRefreshToken() => _secureStorage.read(key: _refreshTokenKey);

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

  // ─── Evidence Upload (MinIO presigned URL flow) ──────────────────────────
  //
  // Step 1: Call getUploadUrl() to get a presigned PUT URL from the backend.
  // Step 2: Call uploadPhotoToMinIO() to PUT the photo bytes directly to MinIO.
  // Step 3: Pass the returned objectKey in JobSubmission.photoUrls.
  //
  // This keeps the API gateway lightweight — it never handles raw photo bytes.

  /// Request a presigned PUT URL for uploading a meter photo to MinIO.
  /// Returns { object_key, upload_url, expires_in, storage_mode }
  Future<Map<String, dynamic>> getUploadUrl({
    required String jobId,
    required String filename,
    String contentType = 'image/jpeg',
  }) async {
    final res = await _dio.post('/evidence/upload-url', data: {
      'job_id':       jobId,
      'filename':     filename,
      'content_type': contentType,
    });
    final body = res.data as Map<String, dynamic>;
    // Response is wrapped: { success: true, data: { object_key, upload_url, ... } }
    return (body['data'] ?? body) as Map<String, dynamic>;
  }

  /// Upload a photo file directly to MinIO using the presigned PUT URL.
  /// Returns the object_key to store in the job submission.
  /// Falls back gracefully if MinIO is not configured (storage_mode == 'offline').
  Future<String?> uploadPhotoToMinIO({
    required String localPath,
    required String uploadUrl,
    required String objectKey,
    String contentType = 'image/jpeg',
  }) async {
    if (uploadUrl.isEmpty) {
      // MinIO not configured — offline mode, skip upload
      return null;
    }
    final file = File(localPath);
    if (!await file.exists()) return null;

    final bytes = await file.readAsBytes();

    // Use a plain Dio instance (no auth headers) for direct MinIO upload
    final minioDio = Dio();
    await minioDio.put(
      uploadUrl,
      data: Stream.fromIterable([bytes]),
      options: Options(
        headers: {
          'Content-Type':   contentType,
          'Content-Length': bytes.length,
        },
        sendTimeout:    const Duration(seconds: 60),
        receiveTimeout: const Duration(seconds: 30),
      ),
    );
    return objectKey;
  }

  Future<void> submitJobEvidence(JobSubmission submission) async {
    // POST /field-jobs/:id/submit — dedicated endpoint that writes OCR reading,
    // GPS, photo hashes to audit_events and marks the job COMPLETED.
    await _dio.post('/field-jobs/${submission.jobId}/submit', data: submission.toJson());
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

  /// Submit an illegal connection report (FIO-004)
  Future<void> submitIllegalConnectionReport(
    dynamic report,
    List<dynamic> photos,
  ) async {
    await _dio.post('/api/v1/field-jobs/illegal-connections', data: report.toJson());
  }
}
