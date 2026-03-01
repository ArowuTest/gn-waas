// GN-WAAS Field Officer App — Meter Capture Screen
// GPS check → Camera → OCR processing → Review → Notes → Submit

import 'dart:convert';
import 'dart:io';
import 'package:camera/camera.dart';
import 'package:crypto/crypto.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../models/models.dart';
import '../../providers/providers.dart';


enum CaptureStep { gpsCheck, camera, processing, review, notes, submitting, done }

class MeterCaptureScreen extends ConsumerStatefulWidget {
  const MeterCaptureScreen({super.key});

  @override
  ConsumerState<MeterCaptureScreen> createState() => _MeterCaptureScreenState();
}

class _MeterCaptureScreenState extends ConsumerState<MeterCaptureScreen> {
  CaptureStep _step = CaptureStep.gpsCheck;
  String _gpsStatus = 'checking'; // checking | ok | outside_fence | error
  MeterPhoto? _capturedPhoto;
  OcrResult? _ocrResult;
  double? _manualReading;
  final _notesCtrl = TextEditingController();
  final _manualCtrl = TextEditingController();
  CameraController? _cameraCtrl;
  List<CameraDescription> _cameras = [];
  bool _cameraReady = false;

  @override
  void initState() {
    super.initState();
    _checkGPS();
  }

  @override
  void dispose() {
    _cameraCtrl?.dispose();
    _notesCtrl.dispose();
    _manualCtrl.dispose();
    super.dispose();
  }

  // ─── Step 1: GPS Check ────────────────────────────────────────────────────

  Future<void> _checkGPS() async {
    setState(() => _gpsStatus = 'checking');
    final job = ref.read(activeJobProvider);
    if (job == null) {
      setState(() => _gpsStatus = 'error');
      return;
    }

    try {
      final loc = ref.read(locationServiceProvider);
      final pos = await loc.getCurrentPosition();
      // Use admin-controlled geofence radius from remote config
      final configAsync = ref.read(mobileConfigProvider);
      final geofenceRadius = configAsync.whenOrNull(data: (c) => c.geofenceRadiusM) ?? 100.0;
      final within = loc.isWithinFenceWithRadius(pos.lat, pos.lng, job.gpsLat, job.gpsLng, geofenceRadius);

      if (within) {
        setState(() => _gpsStatus = 'ok');
        await Future.delayed(const Duration(milliseconds: 800));
        await _initCamera();
        setState(() => _step = CaptureStep.camera);
      } else {
        setState(() => _gpsStatus = 'outside_fence');
      }
    } catch (e) {
      setState(() => _gpsStatus = 'error');
    }
  }

  // ─── Step 2: Camera ───────────────────────────────────────────────────────

  Future<void> _initCamera() async {
    _cameras = await availableCameras();
    if (_cameras.isEmpty) return;

    _cameraCtrl = CameraController(
      _cameras.first,
      ResolutionPreset.high,
      enableAudio: false,
    );
    await _cameraCtrl!.initialize();
    if (mounted) setState(() => _cameraReady = true);
  }

  Future<void> _capturePhoto() async {
    if (_cameraCtrl == null || !_cameraCtrl!.value.isInitialized) return;

    final job = ref.read(activeJobProvider);
    if (job == null) return;

    setState(() => _step = CaptureStep.processing);

    try {
      final file = await _cameraCtrl!.takePicture();
      final bytes = await File(file.path).readAsBytes();
      final hash  = sha256.convert(bytes).toString();

      final loc = ref.read(locationServiceProvider);
      final pos = await loc.getCurrentPosition();
      final within = loc.isWithinFence(pos.lat, pos.lng, job.gpsLat, job.gpsLng);

      _capturedPhoto = MeterPhoto(
        localPath:    file.path,
        hash:         hash,
        gpsLat:       pos.lat,
        gpsLng:       pos.lng,
        gpsAccuracyM: pos.accuracyM,
        capturedAt:   DateTime.now(),
        withinFence:  within,
      );

      // Save photo to offline storage
      final storage = ref.read(offlineStorageProvider);
      await storage.savePhoto(job.id, _capturedPhoto!);

      // Submit to OCR
      await _processOCR(bytes, job.id);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Capture failed: $e'), backgroundColor: Colors.red),
        );
        setState(() => _step = CaptureStep.camera);
      }
    }
  }

  // ─── Step 3: OCR Processing ───────────────────────────────────────────────

  Future<void> _processOCR(List<int> imageBytes, String jobId) async {
    try {
      final api = ref.read(apiServiceProvider);
      final base64Image = base64Encode(imageBytes);
      _ocrResult = await api.submitPhotoForOCR(base64Image, jobId);
      setState(() => _step = CaptureStep.review);
    } catch (_) {
      // OCR failed — allow manual entry
      setState(() => _step = CaptureStep.review);
    }
  }

  // ─── Step 5: Submit ───────────────────────────────────────────────────────

  Future<void> _submitEvidence() async {
    final job = ref.read(activeJobProvider);
    if (job == null || _capturedPhoto == null) return;

    final reading = _ocrResult?.readingM3 ?? _manualReading;
    if (reading == null) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Please enter a meter reading')),
      );
      return;
    }

    setState(() => _step = CaptureStep.submitting);

    final submission = JobSubmission(
      jobId:         job.id,
      ocrReadingM3:  reading,
      ocrConfidence: _ocrResult?.confidence ?? 0.0,
      ocrStatus:     _ocrResult?.status ?? OcrStatus.manual,
      officerNotes:  _notesCtrl.text,
      gpsLat:        _capturedPhoto!.gpsLat,
      gpsLng:        _capturedPhoto!.gpsLng,
      gpsAccuracyM:  _capturedPhoto!.gpsAccuracyM,
      photoUrls:     [],
      photoHashes:   [_capturedPhoto!.hash],
    );

    try {
      final online = await ref.read(syncServiceProvider).isOnline();
      if (online) {
        final api = ref.read(apiServiceProvider);

        // ── Step 1: Get presigned upload URL from backend ──────────────────
        String? uploadedObjectKey;
        try {
          final uploadMeta = await api.getUploadUrl(
            jobId:       job.id,
            filename:    'meter_${job.id}_${DateTime.now().millisecondsSinceEpoch}.jpg',
            contentType: 'image/jpeg',
          );
          final uploadUrl  = uploadMeta['upload_url']  as String? ?? '';
          final objectKey  = uploadMeta['object_key']  as String? ?? '';
          final storageMode = uploadMeta['storage_mode'] as String? ?? 'offline';

          // ── Step 2: Upload photo directly to MinIO ─────────────────────
          if (storageMode != 'offline' && uploadUrl.isNotEmpty) {
            uploadedObjectKey = await api.uploadPhotoToMinIO(
              localPath:   _capturedPhoto!.localPath,
              uploadUrl:   uploadUrl,
              objectKey:   objectKey,
              contentType: 'image/jpeg',
            );
          }
        } catch (uploadErr) {
          // Non-fatal: log and continue — photo will be in offline queue
          debugPrint('Photo upload to MinIO failed: $uploadErr');
        }

        // ── Step 3: Submit job evidence with MinIO object key ──────────────
        final submissionWithPhoto = JobSubmission(
          jobId:         submission.jobId,
          ocrReadingM3:  submission.ocrReadingM3,
          ocrConfidence: submission.ocrConfidence,
          ocrStatus:     submission.ocrStatus,
          officerNotes:  submission.officerNotes,
          gpsLat:        submission.gpsLat,
          gpsLng:        submission.gpsLng,
          gpsAccuracyM:  submission.gpsAccuracyM,
          photoUrls:     uploadedObjectKey != null ? [uploadedObjectKey] : [],
          photoHashes:   submission.photoHashes,
        );
        await api.submitJobEvidence(submissionWithPhoto);
        ref.read(jobsProvider.notifier).updateJobStatus(job.id, FieldJobStatus.completed);
      } else {
        // Queue for later sync
        final storage = ref.read(offlineStorageProvider);
        await storage.queueSubmission(job.id, submission, [_capturedPhoto!.localPath]);
        await storage.updateJobStatusLocally(job.id, FieldJobStatus.completed);
        ref.read(jobsProvider.notifier).updateJobStatus(job.id, FieldJobStatus.completed);
      }
      setState(() => _step = CaptureStep.done);
    } catch (e) {
      // Queue offline on error
      final storage = ref.read(offlineStorageProvider);
      await storage.queueSubmission(job.id, submission, [_capturedPhoto!.localPath]);
      setState(() => _step = CaptureStep.done);
    }
  }

  // ─── Build ────────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFF0f172a),
      appBar: AppBar(
        backgroundColor: const Color(0xFF0f172a),
        foregroundColor: Colors.white,
        title: Text(
          _stepTitle,
          style: const TextStyle(fontWeight: FontWeight.w700),
        ),
      ),
      body: _buildStep(),
    );
  }

  String get _stepTitle {
    switch (_step) {
      case CaptureStep.gpsCheck:    return 'GPS Verification';
      case CaptureStep.camera:      return 'Capture Meter';
      case CaptureStep.processing:  return 'Processing...';
      case CaptureStep.review:      return 'Review Reading';
      case CaptureStep.notes:       return 'Officer Notes';
      case CaptureStep.submitting:  return 'Submitting...';
      case CaptureStep.done:        return 'Submitted ✅';
    }
  }

  Widget _buildStep() {
    switch (_step) {
      case CaptureStep.gpsCheck:   return _buildGPSCheck();
      case CaptureStep.camera:     return _buildCamera();
      case CaptureStep.processing: return _buildProcessing();
      case CaptureStep.review:     return _buildReview();
      case CaptureStep.notes:      return _buildNotes();
      case CaptureStep.submitting: return _buildProcessing();
      case CaptureStep.done:       return _buildDone();
    }
  }

  Widget _buildGPSCheck() => Center(
    child: Padding(
      padding: const EdgeInsets.all(32),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (_gpsStatus == 'checking') ...[
            const CircularProgressIndicator(color: Color(0xFF2563EB)),
            const SizedBox(height: 16),
            const Text('Verifying GPS location...', style: TextStyle(color: Colors.white)),
          ] else if (_gpsStatus == 'ok') ...[
            const Icon(Icons.check_circle, color: Color(0xFF16A34A), size: 64),
            const SizedBox(height: 16),
            const Text('GPS verified ✅', style: TextStyle(color: Colors.white, fontSize: 18)),
          ] else if (_gpsStatus == 'outside_fence') ...[
            const Icon(Icons.location_off, color: Color(0xFFDC2626), size: 64),
            const SizedBox(height: 16),
            const Text(
              'You are outside the geofence.\nPlease move closer to the meter location.',
              textAlign: TextAlign.center,
              style: TextStyle(color: Colors.white),
            ),
            const SizedBox(height: 24),
            ElevatedButton(
              key: const Key('retry_gps_button'),
              onPressed: _checkGPS,
              child: const Text('Retry GPS'),
            ),
          ] else ...[
            const Icon(Icons.error, color: Color(0xFFDC2626), size: 64),
            const SizedBox(height: 16),
            const Text('GPS error. Please enable location services.',
                style: TextStyle(color: Colors.white)),
            const SizedBox(height: 24),
            ElevatedButton(onPressed: _checkGPS, child: const Text('Retry')),
          ],
        ],
      ),
    ),
  );

  Widget _buildCamera() {
    if (!_cameraReady || _cameraCtrl == null) {
      return const Center(
        child: CircularProgressIndicator(color: Color(0xFF2563EB)),
      );
    }
    return Stack(
      children: [
        CameraPreview(_cameraCtrl!),
        Positioned(
          bottom: 40,
          left: 0, right: 0,
          child: Center(
            child: GestureDetector(
              key: const Key('capture_shutter'),
              onTap: _capturePhoto,
              child: Container(
                width: 72, height: 72,
                decoration: BoxDecoration(
                  color: Colors.white,
                  shape: BoxShape.circle,
                  border: Border.all(color: Colors.white, width: 4),
                ),
                child: const Icon(Icons.camera_alt, size: 36, color: Color(0xFF166534)),
              ),
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildProcessing() => const Center(
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        CircularProgressIndicator(color: Color(0xFF2563EB)),
        SizedBox(height: 16),
        Text('Processing...', style: TextStyle(color: Colors.white)),
      ],
    ),
  );

  Widget _buildReview() => SingleChildScrollView(
    padding: const EdgeInsets.all(24),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (_capturedPhoto != null) ...[
          ClipRRect(
            borderRadius: BorderRadius.circular(12),
            child: Image.file(
              File(_capturedPhoto!.localPath),
              height: 200,
              width: double.infinity,
              fit: BoxFit.cover,
            ),
          ),
          const SizedBox(height: 16),
        ],

        if (_ocrResult != null) ...[
          _ReviewCard(
            title: 'OCR Reading',
            value: '${_ocrResult!.readingM3.toStringAsFixed(3)} m³',
            subtitle: 'Confidence: ${(_ocrResult!.confidence * 100).toStringAsFixed(1)}%',
            color: _ocrResult!.confidence > 0.8
                ? const Color(0xFF16A34A)
                : const Color(0xFFD97706),
          ),
          const SizedBox(height: 16),
        ],

        // Manual reading override
        const Text(
          'Manual Reading (override)',
          style: TextStyle(color: Color(0xFF94A3B8), fontSize: 13),
        ),
        const SizedBox(height: 8),
        TextField(
          key: const Key('manual_reading_field'),
          controller: _manualCtrl,
          keyboardType: const TextInputType.numberWithOptions(decimal: true),
          style: const TextStyle(color: Colors.white),
          decoration: InputDecoration(
            hintText: 'Enter reading in m³',
            hintStyle: const TextStyle(color: Color(0xFF475569)),
            filled: true,
            fillColor: const Color(0xFF1e293b),
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(10),
              borderSide: BorderSide.none,
            ),
            suffixText: 'm³',
            suffixStyle: const TextStyle(color: Color(0xFF94A3B8)),
          ),
          onChanged: (v) => _manualReading = double.tryParse(v),
        ),
        const SizedBox(height: 24),

        SizedBox(
          width: double.infinity,
          child: ElevatedButton(
            key: const Key('proceed_to_notes_button'),
            onPressed: () => setState(() => _step = CaptureStep.notes),
            style: ElevatedButton.styleFrom(
              backgroundColor: const Color(0xFF2563EB),
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(vertical: 14),
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
            ),
            child: const Text('Proceed to Notes'),
          ),
        ),
      ],
    ),
  );

  Widget _buildNotes() => SingleChildScrollView(
    padding: const EdgeInsets.all(24),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text(
          'Officer Notes',
          style: TextStyle(color: Colors.white, fontSize: 18, fontWeight: FontWeight.w700),
        ),
        const SizedBox(height: 8),
        TextField(
          key: const Key('notes_field'),
          controller: _notesCtrl,
          maxLines: 5,
          style: const TextStyle(color: Colors.white),
          decoration: InputDecoration(
            hintText: 'Add any observations, anomalies, or notes...',
            hintStyle: const TextStyle(color: Color(0xFF475569)),
            filled: true,
            fillColor: const Color(0xFF1e293b),
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(10),
              borderSide: BorderSide.none,
            ),
          ),
        ),
        const SizedBox(height: 24),
        SizedBox(
          width: double.infinity,
          child: ElevatedButton(
            key: const Key('submit_evidence_button'),
            onPressed: _submitEvidence,
            style: ElevatedButton.styleFrom(
              backgroundColor: const Color(0xFF16A34A),
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(vertical: 14),
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
            ),
            child: const Text(
              'Submit Evidence',
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700),
            ),
          ),
        ),
      ],
    ),
  );

  Widget _buildDone() => Center(
    child: Padding(
      padding: const EdgeInsets.all(32),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.check_circle, color: Color(0xFF16A34A), size: 80),
          const SizedBox(height: 16),
          const Text(
            'Evidence Submitted!',
            style: TextStyle(
              color: Colors.white, fontSize: 22, fontWeight: FontWeight.w800,
            ),
          ),
          const SizedBox(height: 8),
          const Text(
            'The meter reading and photos have been recorded.',
            textAlign: TextAlign.center,
            style: TextStyle(color: Color(0xFF94A3B8)),
          ),
          const SizedBox(height: 32),
          ElevatedButton(
            key: const Key('back_to_jobs_button'),
            onPressed: () => context.go('/jobs'),
            style: ElevatedButton.styleFrom(
              backgroundColor: const Color(0xFF2563EB),
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 14),
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
            ),
            child: const Text('Back to Jobs'),
          ),
        ],
      ),
    ),
  );
}

class _ReviewCard extends StatelessWidget {
  final String title;
  final String value;
  final String subtitle;
  final Color color;

  const _ReviewCard({
    required this.title,
    required this.value,
    required this.subtitle,
    required this.color,
  });

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(16),
    decoration: BoxDecoration(
      color: const Color(0xFF1e293b),
      borderRadius: BorderRadius.circular(12),
      border: Border.all(color: color.withOpacity(0.3)),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(title, style: const TextStyle(color: Color(0xFF94A3B8), fontSize: 12)),
        const SizedBox(height: 4),
        Text(value, style: TextStyle(color: color, fontSize: 28, fontWeight: FontWeight.w900)),
        Text(subtitle, style: const TextStyle(color: Color(0xFF64748B), fontSize: 12)),
      ],
    ),
  );
}
