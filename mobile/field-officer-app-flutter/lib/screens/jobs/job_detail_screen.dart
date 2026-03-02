// GN-WAAS Field Officer App — Job Detail Screen

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../models/models.dart';
import '../../providers/providers.dart';
import '../reports/illegal_connection_screen.dart';

class JobDetailScreen extends ConsumerStatefulWidget {
  final String jobId;
  const JobDetailScreen({super.key, required this.jobId});

  @override
  ConsumerState<JobDetailScreen> createState() => _JobDetailScreenState();
}

class _JobDetailScreenState extends ConsumerState<JobDetailScreen> {
  bool _isUpdating = false;

  FieldJob? get _job {
    final jobs = ref.read(jobsProvider).jobs;
    try {
      return jobs.firstWhere((j) => j.id == widget.jobId);
    } catch (_) {
      return ref.read(activeJobProvider);
    }
  }

  Future<void> _updateStatus(FieldJobStatus newStatus) async {
    final job = _job;
    if (job == null) return;

    setState(() => _isUpdating = true);
    try {
      final api = ref.read(apiServiceProvider);
      await api.updateJobStatus(job.id, newStatus);
      ref.read(jobsProvider.notifier).updateJobStatus(job.id, newStatus);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Status updated to ${newStatus.toApiString()}'),
            backgroundColor: Colors.green.shade700,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Failed to update status: $e'),
            backgroundColor: Colors.red.shade700,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _isUpdating = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final job = _job;
    if (job == null) {
      return Scaffold(
        appBar: AppBar(title: const Text('Job Detail')),
        body: const Center(child: Text('Job not found')),
      );
    }

    return Scaffold(
      backgroundColor: const Color(0xFFF9FAFB),
      appBar: AppBar(
        backgroundColor: const Color(0xFF166534),
        foregroundColor: Colors.white,
        title: Text(
          job.jobReference ?? 'Job Detail',
          style: const TextStyle(fontWeight: FontWeight.w700),
        ),
        actions: [
          IconButton(
            key: const Key('capture_button'),
            icon: const Icon(Icons.camera_alt_outlined),
            onPressed: () => context.push('/capture'),
            tooltip: 'Capture Meter',
          ),
        ],
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // ── Status Card ───────────────────────────────────────────────
            _InfoCard(
              title: 'Status',
              child: Row(
                children: [
                  _StatusBadge(status: job.status),
                  const Spacer(),
                  if (_isUpdating)
                    const SizedBox(
                      width: 20, height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    ),
                ],
              ),
            ),
            const SizedBox(height: 12),

            // ── Customer Info ─────────────────────────────────────────────
            _InfoCard(
              title: 'Customer',
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _InfoRow(label: 'Name',    value: job.customerName),
                  _InfoRow(label: 'Account', value: job.accountNumber),
                  _InfoRow(label: 'Address', value: job.address),
                ],
              ),
            ),
            const SizedBox(height: 12),

            // ── Anomaly Info ──────────────────────────────────────────────
            _InfoCard(
              title: 'Anomaly Details',
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _InfoRow(label: 'Type',     value: job.anomalyType),
                  _InfoRow(label: 'Severity', value: job.alertLevel.toApiString()),
                  if (job.estimatedVarianceGhs != null)
                    _InfoRow(
                      label: 'Est. Variance',
                      value: '₵${job.estimatedVarianceGhs!.toStringAsFixed(2)}',
                      valueColor: Colors.red.shade700,
                    ),
                ],
              ),
            ),
            const SizedBox(height: 12),

            // ── GPS ───────────────────────────────────────────────────────
            _InfoCard(
              title: 'Location',
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _InfoRow(label: 'Latitude',  value: job.gpsLat.toStringAsFixed(6)),
                  _InfoRow(label: 'Longitude', value: job.gpsLng.toStringAsFixed(6)),
                ],
              ),
            ),
            const SizedBox(height: 24),

            // ── Action Buttons ────────────────────────────────────────────
            const Text(
              'Update Status',
              style: TextStyle(
                fontSize: 16, fontWeight: FontWeight.w700, color: Color(0xFF111827),
              ),
            ),
            const SizedBox(height: 12),
            Wrap(
              spacing: 8, runSpacing: 8,
              children: [
                _ActionButton(
                  key: const Key('btn_en_route'),
                  label: 'En Route',
                  icon: Icons.directions_car,
                  color: const Color(0xFF2563EB),
                  onPressed: _isUpdating
                      ? null
                      : () => _updateStatus(FieldJobStatus.enRoute),
                ),
                _ActionButton(
                  key: const Key('btn_on_site'),
                  label: 'On Site',
                  icon: Icons.location_on,
                  color: const Color(0xFFD97706),
                  onPressed: _isUpdating
                      ? null
                      : () => _updateStatus(FieldJobStatus.onSite),
                ),
                _ActionButton(
                  key: const Key('btn_completed'),
                  label: 'Complete',
                  icon: Icons.check_circle,
                  color: const Color(0xFF16A34A),
                  onPressed: _isUpdating
                      ? null
                      : () => _updateStatus(FieldJobStatus.completed),
                ),
              ],
            ),
            const SizedBox(height: 16),

            // ── Capture Button ────────────────────────────────────────────
            SizedBox(
              width: double.infinity,
              child: ElevatedButton.icon(
                key: const Key('capture_meter_button'),
                onPressed: () => context.push('/capture'),
                icon: const Icon(Icons.camera_alt),
                label: const Text('Capture Meter Reading'),
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFF166534),
                  foregroundColor: Colors.white,
                  padding: const EdgeInsets.symmetric(vertical: 14),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12),
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ─── Reusable Widgets ─────────────────────────────────────────────────────────

class _InfoCard extends StatelessWidget {
  final String title;
  final Widget child;
  const _InfoCard({required this.title, required this.child});

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(16),
    decoration: BoxDecoration(
      color: Colors.white,
      borderRadius: BorderRadius.circular(12),
      boxShadow: [
        BoxShadow(
          color: Colors.black.withOpacity(0.04),
          blurRadius: 6,
          offset: const Offset(0, 2),
        ),
      ],
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: const TextStyle(
            fontSize: 12,
            fontWeight: FontWeight.w600,
            color: Color(0xFF6B7280),
            letterSpacing: 0.5,
          ),
        ),
        const SizedBox(height: 8),
        child,
      ],
    ),
  );
}

class _InfoRow extends StatelessWidget {
  final String label;
  final String value;
  final Color? valueColor;
  const _InfoRow({required this.label, required this.value, this.valueColor});

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: 4),
    child: Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 90,
          child: Text(
            label,
            style: const TextStyle(fontSize: 13, color: Color(0xFF9CA3AF)),
          ),
        ),
        Expanded(
          child: Text(
            value,
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: valueColor ?? const Color(0xFF111827),
            ),
          ),
        ),
      ],
    ),
  );
}

class _StatusBadge extends StatelessWidget {
  final FieldJobStatus status;
  const _StatusBadge({required this.status});

  Color get _color {
    switch (status) {
      case FieldJobStatus.queued:     return const Color(0xFF6B7280);
      case FieldJobStatus.assigned:   return const Color(0xFF0369A1);
      case FieldJobStatus.dispatched: return const Color(0xFF2563EB);
      case FieldJobStatus.enRoute:    return const Color(0xFF7C3AED);
      case FieldJobStatus.onSite:     return const Color(0xFFD97706);
      case FieldJobStatus.completed:  return const Color(0xFF16A34A);
      case FieldJobStatus.failed:     return const Color(0xFFDC2626);
      case FieldJobStatus.cancelled:  return const Color(0xFF6B7280);
      case FieldJobStatus.escalated:  return const Color(0xFFB45309);
      case FieldJobStatus.sos:        return const Color(0xFFDC2626);
    }
  }

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
    decoration: BoxDecoration(
      color: _color.withOpacity(0.1),
      borderRadius: BorderRadius.circular(20),
      border: Border.all(color: _color.withOpacity(0.3)),
    ),
    child: Text(
      status.toApiString().replaceAll('_', ' '),
      style: TextStyle(
        fontSize: 12,
        fontWeight: FontWeight.w700,
        color: _color,
      ),
    ),
  );
}

class _ActionButton extends StatelessWidget {
  final String label;
  final IconData icon;
  final Color color;
  final VoidCallback? onPressed;

  const _ActionButton({
    super.key,
    required this.label,
    required this.icon,
    required this.color,
    this.onPressed,
  });

  @override
  Widget build(BuildContext context) => ElevatedButton.icon(
    onPressed: onPressed,
    icon: Icon(icon, size: 16),
    label: Text(label),
    style: ElevatedButton.styleFrom(
      backgroundColor: color,
      foregroundColor: Colors.white,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
    ),
  );
}
