// GN-WAAS Field Officer App — SOS Screen

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../providers/providers.dart';

class SOSScreen extends ConsumerStatefulWidget {
  const SOSScreen({super.key});

  @override
  ConsumerState<SOSScreen> createState() => _SOSScreenState();
}

class _SOSScreenState extends ConsumerState<SOSScreen> {
  bool _triggered = false;
  bool _loading   = false;

  Future<void> _triggerSOS() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: const Color(0xFF1e293b),
        title: const Text(
          '🚨 Trigger SOS?',
          style: TextStyle(color: Colors.white),
        ),
        content: const Text(
          'This will immediately alert your supervisor and dispatch emergency support to your GPS location.',
          style: TextStyle(color: Color(0xFF94A3B8)),
        ),
        actions: [
          TextButton(
            key: const Key('dialog_cancel_button'),
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Cancel', style: TextStyle(color: Color(0xFF94A3B8))),
          ),
          ElevatedButton(
            key: const Key('confirm_sos_button'),
            onPressed: () => Navigator.pop(ctx, true),
            style: ElevatedButton.styleFrom(backgroundColor: Colors.red.shade700),
            child: const Text('TRIGGER SOS', style: TextStyle(color: Colors.white)),
          ),
        ],
      ),
    );

    if (confirmed != true) return;

    setState(() => _loading = true);
    try {
      final loc = ref.read(locationServiceProvider);
      final pos = await loc.getCurrentPosition();

      // Haptic feedback
      await HapticFeedback.heavyImpact();

      final api    = ref.read(apiServiceProvider);
      final job    = ref.read(activeJobProvider);
      final jobId  = job?.id ?? 'unknown';

      await api.triggerSOS(
        jobId,
        gpsLat:       pos.lat,
        gpsLng:       pos.lng,
        gpsAccuracyM: pos.accuracyM,
      );

      setState(() { _triggered = true; _loading = false; });
    } catch (e) {
      setState(() => _loading = false);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('SOS failed: $e'),
            backgroundColor: Colors.red.shade700,
          ),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFF0f172a),
      appBar: AppBar(
        backgroundColor: const Color(0xFF0f172a),
        foregroundColor: Colors.white,
        title: const Text('Emergency SOS', style: TextStyle(fontWeight: FontWeight.w700)),
      ),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: _triggered ? _buildTriggered() : _buildReady(),
        ),
      ),
    );
  }

  Widget _buildReady() => Column(
    mainAxisSize: MainAxisSize.min,
    children: [
      Container(
        width: 120, height: 120,
        decoration: BoxDecoration(
          color: Colors.red.shade900.withOpacity(0.3),
          shape: BoxShape.circle,
          border: Border.all(color: Colors.red.shade700, width: 3),
        ),
        child: const Center(
          child: Text('🚨', style: TextStyle(fontSize: 56)),
        ),
      ),
      const SizedBox(height: 24),
      const Text(
        'Emergency SOS',
        style: TextStyle(
          color: Colors.white, fontSize: 24, fontWeight: FontWeight.w900,
        ),
      ),
      const SizedBox(height: 8),
      const Text(
        'Use only in genuine emergencies.\nThis will alert your supervisor and dispatch support.',
        textAlign: TextAlign.center,
        style: TextStyle(color: Color(0xFF94A3B8), fontSize: 14),
      ),
      const SizedBox(height: 40),
      SizedBox(
        width: double.infinity,
        child: ElevatedButton(
          key: const Key('sos_trigger_button'),
          onPressed: _loading ? null : _triggerSOS,
          style: ElevatedButton.styleFrom(
            backgroundColor: Colors.red.shade700,
            foregroundColor: Colors.white,
            padding: const EdgeInsets.symmetric(vertical: 18),
            shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
          ),
          child: _loading
              ? const SizedBox(
                  width: 24, height: 24,
                  child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
                )
              : const Text(
                  'TRIGGER SOS',
                  style: TextStyle(fontSize: 18, fontWeight: FontWeight.w900, letterSpacing: 2),
                ),
        ),
      ),
      const SizedBox(height: 16),
      TextButton(
        onPressed: () => context.pop(),
        child: const Text('Cancel', style: TextStyle(color: Color(0xFF94A3B8))),
      ),
    ],
  );

  Widget _buildTriggered() => Column(
    mainAxisSize: MainAxisSize.min,
    children: [
      const Icon(Icons.check_circle, color: Color(0xFF16A34A), size: 80),
      const SizedBox(height: 16),
      const Text(
        'SOS Triggered!',
        style: TextStyle(
          color: Colors.white, fontSize: 24, fontWeight: FontWeight.w900,
        ),
      ),
      const SizedBox(height: 8),
      const Text(
        'Your supervisor has been alerted.\nHelp is on the way.',
        textAlign: TextAlign.center,
        style: TextStyle(color: Color(0xFF94A3B8)),
      ),
      const SizedBox(height: 32),
      ElevatedButton(
        key: const Key('sos_back_button'),
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
  );
}
