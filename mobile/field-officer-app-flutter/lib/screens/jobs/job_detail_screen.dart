// GN-WAAS Field Officer App — Job Detail Screen

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../models/models.dart';
import '../../providers/providers.dart';

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
            content: Text('Status updated to ${newStatus.displayLabel}'),
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
    // Watch jobs so the UI rebuilds when outcome is recorded
    final jobsState = ref.watch(jobsProvider);
    final job = jobsState.jobs.where((j) => j.id == widget.jobId).firstOrNull
        ?? ref.read(activeJobProvider);

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
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              job.jobReference ?? 'Job Detail',
              style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 16),
            ),
            Text(
              job.status.displayLabel,
              style: const TextStyle(fontSize: 11, color: Color(0xFFA7F3D0)),
            ),
          ],
        ),
        actions: [
          IconButton(
            key: const Key('capture_button'),
            icon: const Icon(Icons.camera_alt_outlined),
            onPressed: () {
              ref.read(activeJobProvider.notifier).state = job;
              context.push('/capture');
            },
            tooltip: 'Capture Meter',
          ),
        ],
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // ── Revenue Leakage Banner ─────────────────────────────────────
            if (job.primaryLeakageGhs != null)
              _LeakageBanner(job: job),
            if (job.primaryLeakageGhs != null) const SizedBox(height: 12),

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
                  _InfoRow(
                    label: 'Description',
                    value: job.anomalyTypeDescription,
                  ),
                  if (job.leakageCategory != null)
                    _InfoRow(
                      label: 'Category',
                      value: job.leakageCategory!.displayLabel,
                    ),
                  if (job.monthlyLeakageGhs != null)
                    _InfoRow(
                      label: 'Monthly Loss',
                      value: '₵${job.monthlyLeakageGhs!.toStringAsFixed(2)}/mo',
                      valueColor: Colors.red.shade700,
                    ),
                  if (job.annualisedLeakageGhs != null)
                    _InfoRow(
                      label: 'Annual Loss',
                      value: '₵${job.annualisedLeakageGhs!.toStringAsFixed(2)}/yr',
                      valueColor: Colors.red.shade800,
                    ),
                  if (job.estimatedVarianceGhs != null && job.monthlyLeakageGhs == null)
                    _InfoRow(
                      label: 'Est. Variance',
                      value: '₵${job.estimatedVarianceGhs!.toStringAsFixed(2)}',
                      valueColor: Colors.red.shade700,
                    ),
                ],
              ),
            ),
            const SizedBox(height: 12),

            // ── Field Outcome (if recorded) ───────────────────────────────
            if (job.hasOutcome) ...[
              _InfoCard(
                title: 'Field Outcome',
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Icon(
                          job.outcome!.confirmsLeakage
                              ? Icons.warning_amber_rounded
                              : Icons.check_circle_outline,
                          color: job.outcome!.confirmsLeakage
                              ? const Color(0xFFDC2626)
                              : const Color(0xFF16A34A),
                          size: 18,
                        ),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            job.outcome!.displayLabel,
                            style: TextStyle(
                              fontWeight: FontWeight.w700,
                              color: job.outcome!.confirmsLeakage
                                  ? const Color(0xFFDC2626)
                                  : const Color(0xFF16A34A),
                            ),
                          ),
                        ),
                      ],
                    ),
                    if (job.meterFound != null) ...[
                      const SizedBox(height: 4),
                      _InfoRow(
                        label: 'Meter Found',
                        value: job.meterFound! ? 'Yes' : 'No',
                        valueColor: job.meterFound! ? Colors.green.shade700 : Colors.red.shade700,
                      ),
                    ],
                    if (job.addressConfirmed != null)
                      _InfoRow(
                        label: 'Address',
                        value: job.addressConfirmed! ? 'Confirmed' : 'Invalid',
                        valueColor: job.addressConfirmed! ? Colors.green.shade700 : Colors.red.shade700,
                      ),
                    if (job.outcomeNotes != null && job.outcomeNotes!.isNotEmpty)
                      _InfoRow(label: 'Notes', value: job.outcomeNotes!),
                    if (job.recommendedAction != null && job.recommendedAction!.isNotEmpty)
                      _InfoRow(label: 'Action', value: job.recommendedAction!),
                    if (job.outcomeRecordedAt != null)
                      _InfoRow(
                        label: 'Recorded',
                        value: _formatDateTime(job.outcomeRecordedAt!),
                      ),
                  ],
                ),
              ),
              const SizedBox(height: 12),
            ],

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

            // ── Capture Meter Button ──────────────────────────────────────
            SizedBox(
              width: double.infinity,
              child: ElevatedButton.icon(
                key: const Key('capture_meter_button'),
                onPressed: () {
                  ref.read(activeJobProvider.notifier).state = job;
                  context.push('/capture');
                },
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
            const SizedBox(height: 10),

            // ── Record Outcome Button (shown when outcome needed) ──────────
            if (job.needsOutcome) ...[
              SizedBox(
                width: double.infinity,
                child: ElevatedButton.icon(
                  key: const Key('record_outcome_button'),
                  onPressed: () => context.push('/jobs/${job.id}/outcome'),
                  icon: const Icon(Icons.assignment_turned_in_outlined),
                  label: const Text('Record Field Outcome'),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: const Color(0xFFD97706),
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(vertical: 14),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                ),
              ),
              const SizedBox(height: 10),
            ],

            // ── Report Illegal Connection ─────────────────────────────────
            SizedBox(
              width: double.infinity,
              child: OutlinedButton.icon(
                key: const Key('report_illegal_button'),
                onPressed: () => context.push(
                  '/report-illegal?job_id=${job.id}',
                ),
                icon: const Icon(Icons.warning_amber_rounded, color: Color(0xFFB45309)),
                label: const Text(
                  'Report Illegal Connection',
                  style: TextStyle(color: Color(0xFFB45309)),
                ),
                style: OutlinedButton.styleFrom(
                  side: const BorderSide(color: Color(0xFFB45309)),
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

  String _formatDateTime(DateTime dt) {
    return '${dt.day}/${dt.month}/${dt.year} ${dt.hour.toString().padLeft(2, '0')}:${dt.minute.toString().padLeft(2, '0')}';
  }
}

// ─── Revenue Leakage Banner ───────────────────────────────────────────────────

class _LeakageBanner extends StatelessWidget {
  final FieldJob job;
  const _LeakageBanner({required this.job});

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(14),
    decoration: BoxDecoration(
      gradient: const LinearGradient(
        colors: [Color(0xFF7F1D1D), Color(0xFF991B1B)],
        begin: Alignment.topLeft,
        end: Alignment.bottomRight,
      ),
      borderRadius: BorderRadius.circular(12),
    ),
    child: Row(
      children: [
        const Icon(Icons.trending_down, color: Colors.white, size: 28),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'Estimated Revenue Leakage',
                style: TextStyle(color: Color(0xFFFCA5A5), fontSize: 11),
              ),
              Text(
                '₵${job.primaryLeakageGhs!.toStringAsFixed(2)} / month',
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 20,
                  fontWeight: FontWeight.w900,
                ),
              ),
              if (job.annualisedLeakageGhs != null)
                Text(
                  '₵${job.annualisedLeakageGhs!.toStringAsFixed(2)} annualised',
                  style: const TextStyle(color: Color(0xFFFCA5A5), fontSize: 12),
                ),
            ],
          ),
        ),
        if (job.leakageCategory != null)
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
            decoration: BoxDecoration(
              color: Colors.white.withOpacity(0.15),
              borderRadius: BorderRadius.circular(6),
            ),
            child: Text(
              job.leakageCategory!.displayLabel,
              style: const TextStyle(
                color: Colors.white,
                fontSize: 10,
                fontWeight: FontWeight.w700,
              ),
            ),
          ),
      ],
    ),
  );
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
      case FieldJobStatus.queued:          return const Color(0xFF6B7280);
      case FieldJobStatus.assigned:        return const Color(0xFF0369A1);
      case FieldJobStatus.dispatched:      return const Color(0xFF2563EB);
      case FieldJobStatus.enRoute:         return const Color(0xFF7C3AED);
      case FieldJobStatus.onSite:          return const Color(0xFFD97706);
      case FieldJobStatus.completed:       return const Color(0xFF16A34A);
      case FieldJobStatus.failed:          return const Color(0xFFDC2626);
      case FieldJobStatus.cancelled:       return const Color(0xFF6B7280);
      case FieldJobStatus.escalated:       return const Color(0xFFB45309);
      case FieldJobStatus.sos:             return const Color(0xFFDC2626);
      case FieldJobStatus.outcomeRecorded: return const Color(0xFF0891B2);
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
      status.displayLabel,
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
