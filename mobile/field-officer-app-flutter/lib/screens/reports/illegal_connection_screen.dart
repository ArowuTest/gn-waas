import 'dart:convert';
import 'dart:io';
import 'package:crypto/crypto.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import '../../models/models.dart';
import '../../providers/providers.dart';
import '../../services/api_service.dart';
import '../../services/location_service.dart';
import '../../services/offline_storage_service.dart';

/// IllegalConnectionScreen
///
/// Allows field officers to report illegal water connections, meter bypasses,
/// and tampering incidents. This addresses FIO-004 — a key source of NRW.
///
/// Features:
/// - GPS-locked location capture (mandatory)
/// - Photo evidence (up to 5 photos, SHA-256 hashed)
/// - Connection type classification
/// - Estimated daily loss volume
/// - Offline-capable: queued locally if no network
class IllegalConnectionScreen extends ConsumerStatefulWidget {
  final String? jobId; // Optional: link to an existing field job

  const IllegalConnectionScreen({super.key, this.jobId});

  @override
  ConsumerState<IllegalConnectionScreen> createState() =>
      _IllegalConnectionScreenState();
}

class _IllegalConnectionScreenState
    extends ConsumerState<IllegalConnectionScreen> {
  final _formKey = GlobalKey<FormState>();
  final _descriptionController = TextEditingController();
  final _estimatedLossController = TextEditingController();
  final _addressController = TextEditingController();
  final _accountNumberController = TextEditingController();

  String _connectionType = 'BYPASS';
  String _severity = 'HIGH';
  bool _isSubmitting = false;
  bool _isCapturingLocation = false;
  LocationData? _capturedLocation;
  /// Each photo entry stores the File and its SHA-256 hash.
  /// The hash is computed immediately on capture to ensure chain of custody.
  final List<({File file, String hash})> _photos = [];
  final _picker = ImagePicker();

  static const _connectionTypes = [
    ('BYPASS', 'Meter Bypass'),
    ('ILLEGAL_TAP', 'Illegal Tap / Tee'),
    ('TAMPERED_METER', 'Tampered Meter'),
    ('REVERSED_METER', 'Reversed Meter'),
    ('SHARED_CONNECTION', 'Shared Connection (Unauthorised)'),
    ('BROKEN_SEAL', 'Broken Meter Seal'),
    ('OTHER', 'Other Tampering'),
  ];

  static const _severityLevels = [
    ('CRITICAL', 'Critical — Major loss (>500 L/day)', Colors.red),
    ('HIGH', 'High — Significant loss (100–500 L/day)', Colors.orange),
    ('MEDIUM', 'Medium — Moderate loss (20–100 L/day)', Colors.amber),
    ('LOW', 'Low — Minor loss (<20 L/day)', Colors.green),
  ];

  @override
  void dispose() {
    _descriptionController.dispose();
    _estimatedLossController.dispose();
    _addressController.dispose();
    _accountNumberController.dispose();
    super.dispose();
  }

  Future<void> _captureLocation() async {
    setState(() => _isCapturingLocation = true);
    try {
      final locationService = LocationService();
      final location = await locationService.getCurrentLocation();
      setState(() => _capturedLocation = location);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Location capture failed: $e'),
            backgroundColor: Colors.red,
          ),
        );
      }
    } finally {
      setState(() => _isCapturingLocation = false);
    }
  }

  Future<void> _addPhoto(ImageSource source) async {
    if (_photos.length >= 5) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Maximum 5 photos allowed')),
      );
      return;
    }
    try {
      final picked = await _picker.pickImage(
        source: source,
        imageQuality: 85,
        maxWidth: 1920,
      );
      if (picked != null) {
        final file = File(picked.path);
        // Compute SHA-256 hash immediately to establish chain of custody.
        // This matches the approach in meter_capture_screen.dart and satisfies
        // the FIO-004 tamper-evidence requirement.
        final bytes = await file.readAsBytes();
        final hash = sha256.convert(bytes).toString();
        setState(() => _photos.add((file: file, hash: hash)));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to capture photo: $e')),
        );
      }
    }
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    if (_capturedLocation == null) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('GPS location is required for illegal connection reports'),
          backgroundColor: Colors.red,
        ),
      );
      return;
    }
    if (_photos.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('At least one photo is required as evidence'),
          backgroundColor: Colors.orange,
        ),
      );
      return;
    }

    setState(() => _isSubmitting = true);

    try {
      final user = ref.read(authProvider).user;
      // Extract photo hashes for chain-of-custody evidence.
      // Hashes were computed at capture time (SHA-256 of raw bytes).
      final photoHashes = _photos.map((p) => p.hash).toList();
      final photoFiles = _photos.map((p) => p.file).toList();

      final report = IllegalConnectionReport(
        officerId: user?.id ?? '',
        jobId: widget.jobId,
        connectionType: _connectionType,
        severity: _severity,
        description: _descriptionController.text.trim(),
        estimatedDailyLossLitres: double.tryParse(_estimatedLossController.text) ?? 0,
        address: _addressController.text.trim(),
        accountNumber: _accountNumberController.text.trim().isEmpty
            ? null
            : _accountNumberController.text.trim(),
        latitude: _capturedLocation!.latitude,
        longitude: _capturedLocation!.longitude,
        gpsAccuracy: _capturedLocation!.accuracy,
        photoCount: _photos.length,
        photoHashes: photoHashes,
        reportedAt: DateTime.now().toUtc(),
      );

      final apiService = ref.read(apiServiceProvider);
      await apiService.submitIllegalConnectionReport(report, photoFiles);

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Illegal connection report submitted successfully'),
            backgroundColor: Colors.green,
          ),
        );
        Navigator.of(context).pop(true);
      }
    } catch (e) {
      // Offline fallback: queue locally
      try {
        final storage = ref.read(offlineStorageProvider);
        await storage.queueIllegalConnectionReport(
          connectionType: _connectionType,
          severity: _severity,
          description: _descriptionController.text.trim(),
          latitude: _capturedLocation!.latitude,
          longitude: _capturedLocation!.longitude,
          photoCount: _photos.length,
          photoHashes: _photos.map((p) => p.hash).toList(),
        );
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(
              content: Text('Saved offline — will sync when connected'),
              backgroundColor: Colors.blue,
            ),
          );
          Navigator.of(context).pop(true);
        }
      } catch (offlineErr) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text('Submission failed: $e'),
              backgroundColor: Colors.red,
            ),
          );
        }
      }
    } finally {
      if (mounted) setState(() => _isSubmitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Report Illegal Connection'),
        backgroundColor: Colors.red.shade700,
        foregroundColor: Colors.white,
        actions: [
          if (_isSubmitting)
            const Padding(
              padding: EdgeInsets.all(16),
              child: SizedBox(
                width: 20,
                height: 20,
                child: CircularProgressIndicator(
                  strokeWidth: 2,
                  color: Colors.white,
                ),
              ),
            ),
        ],
      ),
      body: Form(
        key: _formKey,
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            // Warning banner
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: Colors.red.shade50,
                borderRadius: BorderRadius.circular(8),
                border: Border.all(color: Colors.red.shade200),
              ),
              child: Row(
                children: [
                  Icon(Icons.warning_amber, color: Colors.red.shade700),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'This report will be submitted to the National Water Authority and GRA. '
                      'Ensure all evidence is accurate.',
                      style: TextStyle(
                        fontSize: 12,
                        color: Colors.red.shade800,
                      ),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 20),

            // Connection Type
            _SectionHeader(title: 'Connection Type', icon: Icons.plumbing),
            const SizedBox(height: 8),
            DropdownButtonFormField<String>(
              value: _connectionType,
              decoration: const InputDecoration(
                labelText: 'Type of Illegal Connection',
                border: OutlineInputBorder(),
              ),
              items: _connectionTypes
                  .map((t) => DropdownMenuItem(value: t.$1, child: Text(t.$2)))
                  .toList(),
              onChanged: (v) => setState(() => _connectionType = v!),
              validator: (v) => v == null ? 'Required' : null,
            ),
            const SizedBox(height: 16),

            // Severity
            _SectionHeader(title: 'Severity', icon: Icons.bar_chart),
            const SizedBox(height: 8),
            ..._severityLevels.map((s) => RadioListTile<String>(
              value: s.$1,
              groupValue: _severity,
              title: Text(s.$2, style: const TextStyle(fontSize: 14)),
              activeColor: s.$3,
              onChanged: (v) => setState(() => _severity = v!),
              contentPadding: EdgeInsets.zero,
              dense: true,
            )),
            const SizedBox(height: 16),

            // Account Number (optional)
            _SectionHeader(title: 'Account Details', icon: Icons.account_circle),
            const SizedBox(height: 8),
            TextFormField(
              controller: _accountNumberController,
              decoration: const InputDecoration(
                labelText: 'GWL Account Number (if known)',
                hintText: 'e.g. GWL-ACC-001234',
                border: OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 12),
            TextFormField(
              controller: _addressController,
              decoration: const InputDecoration(
                labelText: 'Location / Address',
                hintText: 'Street name, landmark, or description',
                border: OutlineInputBorder(),
              ),
              validator: (v) =>
                  (v == null || v.trim().isEmpty) ? 'Address is required' : null,
            ),
            const SizedBox(height: 12),
            TextFormField(
              controller: _estimatedLossController,
              keyboardType: TextInputType.number,
              decoration: const InputDecoration(
                labelText: 'Estimated Daily Water Loss (litres)',
                hintText: 'e.g. 250',
                border: OutlineInputBorder(),
                suffixText: 'L/day',
              ),
            ),
            const SizedBox(height: 16),

            // Description
            _SectionHeader(title: 'Description', icon: Icons.description),
            const SizedBox(height: 8),
            TextFormField(
              controller: _descriptionController,
              maxLines: 4,
              decoration: const InputDecoration(
                labelText: 'Describe the illegal connection',
                hintText: 'Describe what you observed, how the bypass is constructed, '
                    'any identifying features...',
                border: OutlineInputBorder(),
                alignLabelWithHint: true,
              ),
              validator: (v) => (v == null || v.trim().length < 20)
                  ? 'Please provide at least 20 characters of description'
                  : null,
            ),
            const SizedBox(height: 20),

            // GPS Location
            _SectionHeader(title: 'GPS Location', icon: Icons.location_on),
            const SizedBox(height: 8),
            if (_capturedLocation != null)
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: Colors.green.shade50,
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(color: Colors.green.shade300),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Icon(Icons.check_circle, color: Colors.green.shade700, size: 16),
                        const SizedBox(width: 6),
                        Text(
                          'Location captured',
                          style: TextStyle(
                            color: Colors.green.shade800,
                            fontWeight: FontWeight.w600,
                            fontSize: 13,
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 4),
                    Text(
                      'Lat: ${_capturedLocation!.latitude.toStringAsFixed(6)}, '
                      'Lng: ${_capturedLocation!.longitude.toStringAsFixed(6)}',
                      style: const TextStyle(fontSize: 12, fontFamily: 'monospace'),
                    ),
                    Text(
                      'Accuracy: ±${_capturedLocation!.accuracy.toStringAsFixed(1)}m',
                      style: TextStyle(fontSize: 11, color: Colors.green.shade700),
                    ),
                  ],
                ),
              )
            else
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: Colors.orange.shade50,
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(color: Colors.orange.shade300),
                ),
                child: Text(
                  'GPS location is required. Tap the button below to capture your current location.',
                  style: TextStyle(color: Colors.orange.shade800, fontSize: 13),
                ),
              ),
            const SizedBox(height: 8),
            ElevatedButton.icon(
              onPressed: _isCapturingLocation ? null : _captureLocation,
              icon: _isCapturingLocation
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.my_location),
              label: Text(_isCapturingLocation
                  ? 'Capturing GPS...'
                  : _capturedLocation != null
                      ? 'Re-capture Location'
                      : 'Capture GPS Location'),
              style: ElevatedButton.styleFrom(
                backgroundColor: Colors.blue.shade700,
                foregroundColor: Colors.white,
              ),
            ),
            const SizedBox(height: 20),

            // Photo Evidence
            _SectionHeader(
              title: 'Photo Evidence (${_photos.length}/5) — SHA-256 hashed',
              icon: Icons.camera_alt,
            ),
            const SizedBox(height: 8),
            if (_photos.isNotEmpty)
              SizedBox(
                height: 100,
                child: ListView.separated(
                  scrollDirection: Axis.horizontal,
                  itemCount: _photos.length,
                  separatorBuilder: (_, __) => const SizedBox(width: 8),
                  itemBuilder: (context, i) => Stack(
                    children: [
                      ClipRRect(
                        borderRadius: BorderRadius.circular(8),
                        child: Image.file(
                          _photos[i].file,
                          width: 100,
                          height: 100,
                          fit: BoxFit.cover,
                        ),
                      ),
                      Positioned(
                        top: 4,
                        right: 4,
                        child: GestureDetector(
                          onTap: () => setState(() => _photos.removeAt(i)),
                          child: Container(
                            padding: const EdgeInsets.all(2),
                            decoration: const BoxDecoration(
                              color: Colors.red,
                              shape: BoxShape.circle,
                            ),
                            child: const Icon(Icons.close, size: 14, color: Colors.white),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            const SizedBox(height: 8),
            Row(
              children: [
                Expanded(
                  child: OutlinedButton.icon(
                    onPressed: _photos.length >= 5
                        ? null
                        : () => _addPhoto(ImageSource.camera),
                    icon: const Icon(Icons.camera_alt),
                    label: const Text('Camera'),
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: OutlinedButton.icon(
                    onPressed: _photos.length >= 5
                        ? null
                        : () => _addPhoto(ImageSource.gallery),
                    icon: const Icon(Icons.photo_library),
                    label: const Text('Gallery'),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 32),

            // Submit
            ElevatedButton(
              onPressed: _isSubmitting ? null : _submit,
              style: ElevatedButton.styleFrom(
                backgroundColor: Colors.red.shade700,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(vertical: 16),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8),
                ),
              ),
              child: _isSubmitting
                  ? const Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        SizedBox(
                          width: 20,
                          height: 20,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: Colors.white,
                          ),
                        ),
                        SizedBox(width: 12),
                        Text('Submitting Report...'),
                      ],
                    )
                  : const Text(
                      'Submit Illegal Connection Report',
                      style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
                    ),
            ),
            const SizedBox(height: 24),
          ],
        ),
      ),
    );
  }
}

class _SectionHeader extends StatelessWidget {
  final String title;
  final IconData icon;

  const _SectionHeader({required this.title, required this.icon});

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Icon(icon, size: 18, color: Colors.grey.shade700),
        const SizedBox(width: 6),
        Text(
          title,
          style: const TextStyle(
            fontWeight: FontWeight.w600,
            fontSize: 15,
          ),
        ),
        const SizedBox(width: 8),
        Expanded(child: Divider(color: Colors.grey.shade300)),
      ],
    );
  }
}

/// Data model for an illegal connection report.
///
/// Photo hashes (SHA-256) are included to establish chain of custody.
/// This satisfies the FIO-004 tamper-evidence requirement.
class IllegalConnectionReport {
  final String officerId;
  final String? jobId;
  final String connectionType;
  final String severity;
  final String description;
  final double estimatedDailyLossLitres;
  final String address;
  final String? accountNumber;
  final double latitude;
  final double longitude;
  final double gpsAccuracy;
  final int photoCount;
  /// SHA-256 hashes of each photo, computed at capture time.
  /// Used to verify photo integrity and establish chain of custody.
  final List<String> photoHashes;
  final DateTime reportedAt;

  const IllegalConnectionReport({
    required this.officerId,
    this.jobId,
    required this.connectionType,
    required this.severity,
    required this.description,
    required this.estimatedDailyLossLitres,
    required this.address,
    this.accountNumber,
    required this.latitude,
    required this.longitude,
    required this.gpsAccuracy,
    required this.photoCount,
    required this.photoHashes,
    required this.reportedAt,
  });

  Map<String, dynamic> toJson() => {
    'officer_id': officerId,
    if (jobId != null) 'job_id': jobId,
    'connection_type': connectionType,
    'severity': severity,
    'description': description,
    'estimated_daily_loss_litres': estimatedDailyLossLitres,
    'address': address,
    if (accountNumber != null) 'account_number': accountNumber,
    'latitude': latitude,
    'longitude': longitude,
    'gps_accuracy': gpsAccuracy,
    'photo_count': photoCount,
    // SHA-256 hashes sent to backend for server-side verification
    'photo_hashes': photoHashes,
    'reported_at': reportedAt.toIso8601String(),
  };
}
