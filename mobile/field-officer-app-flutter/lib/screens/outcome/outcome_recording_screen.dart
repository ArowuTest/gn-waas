// GN-WAAS Field Officer App — Outcome Recording Screen
//
// This screen is the critical missing link in the revenue recovery pipeline.
// After a field officer visits a site and submits meter evidence, they MUST
// record what they actually found on-site. This outcome drives:
//
//   1. Anomaly flag transition: OPEN → ACKNOWLEDGED (if leakage confirmed)
//   2. Revenue recovery event creation: PENDING → FIELD_VERIFIED
//   3. Escalation to management for fraudulent accounts
//   4. Back-billing calculation for unmetered consumption
//
// The screen is reached from:
//   a) MeterCaptureScreen "done" step — auto-navigated after evidence submission
//   b) JobDetailScreen "Record Outcome" button — for jobs already completed
//
// Offline support: if no network, the outcome is queued in pending_outcomes
// SQLite table and synced when connectivity returns.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../models/models.dart';
import '../../providers/providers.dart';

class OutcomeRecordingScreen extends ConsumerStatefulWidget {
  final String jobId;

  const OutcomeRecordingScreen({super.key, required this.jobId});

  @override
  ConsumerState<OutcomeRecordingScreen> createState() =>
      _OutcomeRecordingScreenState();
}

class _OutcomeRecordingScreenState
    extends ConsumerState<OutcomeRecordingScreen> {
  final _formKey = GlobalKey<FormState>();
  final _notesCtrl = TextEditingController();
  final _recommendedActionCtrl = TextEditingController();
  final _estimatedMonthlyM3Ctrl = TextEditingController();

  FieldJobOutcome? _selectedOutcome;
  bool? _meterFound;
  bool? _addressConfirmed;
  bool _isSubmitting = false;

  // Outcome groups for the picker — grouped by category for clarity
  static const _outcomeGroups = [
    _OutcomeGroup(
      label: 'Meter Status',
      icon: Icons.speed_outlined,
      color: Color(0xFF2563EB),
      outcomes: [
        FieldJobOutcome.meterFoundOk,
        FieldJobOutcome.meterFoundTampered,
        FieldJobOutcome.meterFoundFaulty,
        FieldJobOutcome.meterNotFoundInstall,
        FieldJobOutcome.duplicateMeter,
      ],
    ),
    _OutcomeGroup(
      label: 'Address / Account',
      icon: Icons.location_on_outlined,
      color: Color(0xFF7C3AED),
      outcomes: [
        FieldJobOutcome.addressValidUnregistered,
        FieldJobOutcome.addressInvalid,
        FieldJobOutcome.addressDemolished,
        FieldJobOutcome.accessDenied,
      ],
    ),
    _OutcomeGroup(
      label: 'Category / Usage',
      icon: Icons.category_outlined,
      color: Color(0xFFD97706),
      outcomes: [
        FieldJobOutcome.categoryConfirmedCorrect,
        FieldJobOutcome.categoryMismatchConfirmed,
      ],
    ),
    _OutcomeGroup(
      label: 'Illegal Activity',
      icon: Icons.warning_amber_outlined,
      color: Color(0xFFDC2626),
      outcomes: [
        FieldJobOutcome.illegalConnectionFound,
      ],
    ),
  ];

  @override
  void dispose() {
    _notesCtrl.dispose();
    _recommendedActionCtrl.dispose();
    _estimatedMonthlyM3Ctrl.dispose();
    super.dispose();
  }

  FieldJob? get _job {
    final jobs = ref.read(jobsProvider).jobs;
    try {
      return jobs.firstWhere((j) => j.id == widget.jobId);
    } catch (_) {
      return ref.read(activeJobProvider);
    }
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    if (_selectedOutcome == null) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Please select an outcome'),
          backgroundColor: Color(0xFFDC2626),
        ),
      );
      return;
    }

    setState(() => _isSubmitting = true);

    final request = FieldJobOutcomeRequest(
      outcome:             _selectedOutcome!,
      outcomeNotes:        _notesCtrl.text.trim(),
      meterFound:          _meterFound,
      addressConfirmed:    _addressConfirmed,
      recommendedAction:   _recommendedActionCtrl.text.trim(),
      estimatedMonthlyM3:  _estimatedMonthlyM3Ctrl.text.isNotEmpty
                             ? double.tryParse(_estimatedMonthlyM3Ctrl.text)
                             : null,
    );

    try {
      final api = ref.read(apiServiceProvider);
      final updatedJob = await api.recordFieldJobOutcome(widget.jobId, request);

      // Update local state
      ref.read(jobsProvider.notifier).updateJobFromServer(updatedJob);

      if (mounted) {
        _showSuccessAndNavigate();
      }
    } catch (e) {
      // Offline or server error — queue locally
      try {
        final storage = ref.read(offlineStorageProvider);
        await storage.queueOutcome(widget.jobId, request);

        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(
              content: Text(
                'Outcome saved offline — will sync when connected',
                style: TextStyle(color: Colors.white),
              ),
              backgroundColor: Color(0xFFD97706),
              duration: Duration(seconds: 3),
            ),
          );
          _showSuccessAndNavigate();
        }
      } catch (storageError) {
        if (mounted) {
          setState(() => _isSubmitting = false);
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text('Failed to save outcome: $e'),
              backgroundColor: const Color(0xFFDC2626),
            ),
          );
        }
      }
    }
  }

  void _showSuccessAndNavigate() {
    showDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (ctx) => AlertDialog(
        backgroundColor: const Color(0xFF1e293b),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const SizedBox(height: 8),
            Icon(
              _selectedOutcome!.confirmsLeakage
                  ? Icons.assignment_turned_in
                  : Icons.check_circle,
              color: _selectedOutcome!.confirmsLeakage
                  ? const Color(0xFFD97706)
                  : const Color(0xFF16A34A),
              size: 64,
            ),
            const SizedBox(height: 16),
            const Text(
              'Outcome Recorded',
              style: TextStyle(
                color: Colors.white,
                fontSize: 20,
                fontWeight: FontWeight.w800,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              _selectedOutcome!.confirmsLeakage
                  ? 'Revenue leakage confirmed. This case has been escalated to the audit pipeline.'
                  : 'Outcome recorded successfully.',
              textAlign: TextAlign.center,
              style: const TextStyle(color: Color(0xFF94A3B8), fontSize: 14),
            ),
            const SizedBox(height: 24),
            SizedBox(
              width: double.infinity,
              child: ElevatedButton(
                key: const Key('outcome_done_button'),
                onPressed: () {
                  Navigator.pop(ctx);
                  context.go('/jobs');
                },
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFF2563EB),
                  foregroundColor: Colors.white,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(10),
                  ),
                ),
                child: const Text('Back to Jobs'),
              ),
            ),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final job = _job;

    return Scaffold(
      backgroundColor: const Color(0xFF0f172a),
      appBar: AppBar(
        backgroundColor: const Color(0xFF0f172a),
        foregroundColor: Colors.white,
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text(
              'Record Outcome',
              style: TextStyle(fontWeight: FontWeight.w700, fontSize: 16),
            ),
            if (job != null)
              Text(
                job.jobReference ?? job.id,
                style: const TextStyle(fontSize: 11, color: Color(0xFF94A3B8)),
              ),
          ],
        ),
      ),
      body: Form(
        key: _formKey,
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // ── Job Summary Banner ─────────────────────────────────────
              if (job != null) _buildJobBanner(job),
              const SizedBox(height: 20),

              // ── Outcome Picker ─────────────────────────────────────────
              _buildSectionHeader('What did you find on-site?', required: true),
              const SizedBox(height: 12),
              ..._outcomeGroups.map((group) => _buildOutcomeGroup(group)),
              const SizedBox(height: 20),

              // ── Meter Found Toggle ─────────────────────────────────────
              _buildSectionHeader('Was the meter physically present?'),
              const SizedBox(height: 8),
              _buildToggleRow(
                trueLabel:  'Yes — Meter Found',
                falseLabel: 'No — Meter Missing',
                value:      _meterFound,
                onChanged:  (v) => setState(() => _meterFound = v),
                trueColor:  const Color(0xFF16A34A),
                falseColor: const Color(0xFFDC2626),
              ),
              const SizedBox(height: 20),

              // ── Address Confirmed Toggle ───────────────────────────────
              _buildSectionHeader('Was the address confirmed to exist?'),
              const SizedBox(height: 8),
              _buildToggleRow(
                trueLabel:  'Yes — Address Exists',
                falseLabel: 'No — Address Invalid',
                value:      _addressConfirmed,
                onChanged:  (v) => setState(() => _addressConfirmed = v),
                trueColor:  const Color(0xFF16A34A),
                falseColor: const Color(0xFFDC2626),
              ),
              const SizedBox(height: 20),

              // ── Estimated Monthly m³ (for unmetered consumption) ───────
              if (_selectedOutcome == FieldJobOutcome.addressValidUnregistered ||
                  _selectedOutcome == FieldJobOutcome.meterNotFoundInstall) ...[
                _buildSectionHeader('Estimated Monthly Consumption (m³)'),
                const SizedBox(height: 8),
                TextFormField(
                  key: const Key('estimated_monthly_m3_field'),
                  controller: _estimatedMonthlyM3Ctrl,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true),
                  style: const TextStyle(color: Colors.white),
                  decoration: _inputDecoration(
                    'e.g. 12.5',
                    Icons.water_drop_outlined,
                    hint: 'Estimate based on property type and visible usage',
                  ),
                  validator: (v) {
                    if (v != null && v.isNotEmpty) {
                      final parsed = double.tryParse(v);
                      if (parsed == null || parsed <= 0) {
                        return 'Enter a valid positive number';
                      }
                    }
                    return null;
                  },
                ),
                const SizedBox(height: 20),
              ],

              // ── Officer Notes ──────────────────────────────────────────
              _buildSectionHeader('Officer Notes'),
              const SizedBox(height: 8),
              TextFormField(
                key: const Key('outcome_notes_field'),
                controller: _notesCtrl,
                maxLines: 4,
                style: const TextStyle(color: Colors.white),
                decoration: _inputDecoration(
                  'Describe what you observed on-site...',
                  Icons.notes_outlined,
                ),
              ),
              const SizedBox(height: 20),

              // ── Recommended Action ─────────────────────────────────────
              _buildSectionHeader('Recommended Action'),
              const SizedBox(height: 8),
              TextFormField(
                key: const Key('recommended_action_field'),
                controller: _recommendedActionCtrl,
                maxLines: 2,
                style: const TextStyle(color: Colors.white),
                decoration: _inputDecoration(
                  'e.g. Install new meter, Disconnect illegal tap...',
                  Icons.build_outlined,
                ),
              ),
              const SizedBox(height: 32),

              // ── Submit Button ──────────────────────────────────────────
              SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  key: const Key('submit_outcome_button'),
                  onPressed: _isSubmitting ? null : _submit,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: const Color(0xFF16A34A),
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                    disabledBackgroundColor: const Color(0xFF374151),
                  ),
                  child: _isSubmitting
                      ? const SizedBox(
                          height: 20,
                          width: 20,
                          child: CircularProgressIndicator(
                            strokeWidth: 2,
                            color: Colors.white,
                          ),
                        )
                      : const Text(
                          'Submit Outcome',
                          style: TextStyle(
                            fontSize: 16,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                ),
              ),
              const SizedBox(height: 24),
            ],
          ),
        ),
      ),
    );
  }

  // ─── Job Banner ───────────────────────────────────────────────────────────

  Widget _buildJobBanner(FieldJob job) {
    final leakageGhs = job.primaryLeakageGhs;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: const Color(0xFF1e293b),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: const Color(0xFF334155)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Icon(Icons.person_outline, color: Color(0xFF94A3B8), size: 16),
              const SizedBox(width: 6),
              Expanded(
                child: Text(
                  job.customerName,
                  style: const TextStyle(
                    color: Colors.white,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
              _AlertBadge(level: job.alertLevel),
            ],
          ),
          const SizedBox(height: 6),
          Row(
            children: [
              const Icon(Icons.location_on_outlined, color: Color(0xFF94A3B8), size: 16),
              const SizedBox(width: 6),
              Expanded(
                child: Text(
                  job.address,
                  style: const TextStyle(color: Color(0xFF94A3B8), fontSize: 13),
                ),
              ),
            ],
          ),
          const SizedBox(height: 6),
          Row(
            children: [
              const Icon(Icons.info_outline, color: Color(0xFF94A3B8), size: 16),
              const SizedBox(width: 6),
              Expanded(
                child: Text(
                  job.anomalyTypeDescription,
                  style: const TextStyle(color: Color(0xFF94A3B8), fontSize: 12),
                ),
              ),
            ],
          ),
          if (leakageGhs != null) ...[
            const SizedBox(height: 8),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
              decoration: BoxDecoration(
                color: const Color(0xFF7F1D1D).withOpacity(0.3),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(color: const Color(0xFFDC2626).withOpacity(0.4)),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Icon(Icons.trending_down, color: Color(0xFFDC2626), size: 16),
                  const SizedBox(width: 6),
                  Text(
                    'Est. monthly leakage: ₵${leakageGhs.toStringAsFixed(2)}',
                    style: const TextStyle(
                      color: Color(0xFFDC2626),
                      fontWeight: FontWeight.w700,
                      fontSize: 13,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }

  // ─── Outcome Group ────────────────────────────────────────────────────────

  Widget _buildOutcomeGroup(_OutcomeGroup group) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(bottom: 8),
          child: Row(
            children: [
              Icon(group.icon, color: group.color, size: 16),
              const SizedBox(width: 6),
              Text(
                group.label,
                style: TextStyle(
                  color: group.color,
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                  letterSpacing: 0.5,
                ),
              ),
            ],
          ),
        ),
        ...group.outcomes.map((outcome) => _buildOutcomeTile(outcome, group.color)),
        const SizedBox(height: 12),
      ],
    );
  }

  Widget _buildOutcomeTile(FieldJobOutcome outcome, Color groupColor) {
    final isSelected = _selectedOutcome == outcome;
    return GestureDetector(
      onTap: () => setState(() => _selectedOutcome = outcome),
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 150),
        margin: const EdgeInsets.only(bottom: 6),
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
        decoration: BoxDecoration(
          color: isSelected
              ? groupColor.withOpacity(0.15)
              : const Color(0xFF1e293b),
          borderRadius: BorderRadius.circular(10),
          border: Border.all(
            color: isSelected ? groupColor : const Color(0xFF334155),
            width: isSelected ? 2 : 1,
          ),
        ),
        child: Row(
          children: [
            Icon(
              isSelected ? Icons.radio_button_checked : Icons.radio_button_unchecked,
              color: isSelected ? groupColor : const Color(0xFF475569),
              size: 20,
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                outcome.displayLabel,
                style: TextStyle(
                  color: isSelected ? Colors.white : const Color(0xFF94A3B8),
                  fontWeight: isSelected ? FontWeight.w600 : FontWeight.normal,
                  fontSize: 14,
                ),
              ),
            ),
            if (outcome.confirmsLeakage)
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                decoration: BoxDecoration(
                  color: const Color(0xFFDC2626).withOpacity(0.15),
                  borderRadius: BorderRadius.circular(4),
                ),
                child: const Text(
                  'LEAKAGE',
                  style: TextStyle(
                    color: Color(0xFFDC2626),
                    fontSize: 9,
                    fontWeight: FontWeight.w700,
                    letterSpacing: 0.5,
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  // ─── Toggle Row ───────────────────────────────────────────────────────────

  Widget _buildToggleRow({
    required String trueLabel,
    required String falseLabel,
    required bool? value,
    required ValueChanged<bool?> onChanged,
    required Color trueColor,
    required Color falseColor,
  }) {
    return Row(
      children: [
        Expanded(
          child: _ToggleButton(
            label: trueLabel,
            isSelected: value == true,
            color: trueColor,
            onTap: () => onChanged(value == true ? null : true),
          ),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: _ToggleButton(
            label: falseLabel,
            isSelected: value == false,
            color: falseColor,
            onTap: () => onChanged(value == false ? null : false),
          ),
        ),
      ],
    );
  }

  // ─── Section Header ───────────────────────────────────────────────────────

  Widget _buildSectionHeader(String title, {bool required = false}) => Row(
    children: [
      Text(
        title,
        style: const TextStyle(
          color: Colors.white,
          fontSize: 14,
          fontWeight: FontWeight.w600,
        ),
      ),
      if (required) ...[
        const SizedBox(width: 4),
        const Text(
          '*',
          style: TextStyle(color: Color(0xFFDC2626), fontSize: 14),
        ),
      ],
    ],
  );

  // ─── Input Decoration ─────────────────────────────────────────────────────

  InputDecoration _inputDecoration(
    String hintText,
    IconData icon, {
    String? hint,
  }) =>
      InputDecoration(
        hintText: hintText,
        hintStyle: const TextStyle(color: Color(0xFF475569)),
        helperText: hint,
        helperStyle: const TextStyle(color: Color(0xFF64748B), fontSize: 11),
        prefixIcon: Icon(icon, color: const Color(0xFF475569), size: 20),
        filled: true,
        fillColor: const Color(0xFF1e293b),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: BorderSide.none,
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: const BorderSide(color: Color(0xFF2563EB), width: 2),
        ),
        errorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: const BorderSide(color: Color(0xFFDC2626), width: 1),
        ),
        focusedErrorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(10),
          borderSide: const BorderSide(color: Color(0xFFDC2626), width: 2),
        ),
        contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      );
}

// ─── Supporting Widgets ───────────────────────────────────────────────────────

class _OutcomeGroup {
  final String label;
  final IconData icon;
  final Color color;
  final List<FieldJobOutcome> outcomes;

  const _OutcomeGroup({
    required this.label,
    required this.icon,
    required this.color,
    required this.outcomes,
  });
}

class _ToggleButton extends StatelessWidget {
  final String label;
  final bool isSelected;
  final Color color;
  final VoidCallback onTap;

  const _ToggleButton({
    required this.label,
    required this.isSelected,
    required this.color,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) => GestureDetector(
    onTap: onTap,
    child: AnimatedContainer(
      duration: const Duration(milliseconds: 150),
      padding: const EdgeInsets.symmetric(vertical: 12),
      decoration: BoxDecoration(
        color: isSelected ? color.withOpacity(0.15) : const Color(0xFF1e293b),
        borderRadius: BorderRadius.circular(10),
        border: Border.all(
          color: isSelected ? color : const Color(0xFF334155),
          width: isSelected ? 2 : 1,
        ),
      ),
      child: Center(
        child: Text(
          label,
          textAlign: TextAlign.center,
          style: TextStyle(
            color: isSelected ? color : const Color(0xFF94A3B8),
            fontWeight: isSelected ? FontWeight.w700 : FontWeight.normal,
            fontSize: 12,
          ),
        ),
      ),
    ),
  );
}

class _AlertBadge extends StatelessWidget {
  final AlertLevel level;
  const _AlertBadge({required this.level});

  Color get _color {
    switch (level) {
      case AlertLevel.critical: return const Color(0xFFDC2626);
      case AlertLevel.high:     return const Color(0xFFEA580C);
      case AlertLevel.medium:   return const Color(0xFFD97706);
      case AlertLevel.low:      return const Color(0xFF16A34A);
      case AlertLevel.info:     return const Color(0xFF2563EB);
    }
  }

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
    decoration: BoxDecoration(
      color: _color.withOpacity(0.15),
      borderRadius: BorderRadius.circular(6),
      border: Border.all(color: _color.withOpacity(0.4)),
    ),
    child: Text(
      level.toApiString(),
      style: TextStyle(
        color: _color,
        fontSize: 10,
        fontWeight: FontWeight.w700,
      ),
    ),
  );
}
