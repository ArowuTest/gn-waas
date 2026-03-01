// GN-WAAS Field Officer App — Job List Screen

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../models/models.dart';
import '../../providers/providers.dart';

class JobListScreen extends ConsumerWidget {
  const JobListScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final jobsState = ref.watch(jobsProvider);

    return Scaffold(
      backgroundColor: const Color(0xFFF9FAFB),
      appBar: AppBar(
        backgroundColor: const Color(0xFF166534),
        foregroundColor: Colors.white,
        title: const Text(
          'My Jobs',
          style: TextStyle(fontWeight: FontWeight.w800),
        ),
        actions: [
          if (jobsState.isRefreshing)
            const Padding(
              padding: EdgeInsets.all(16),
              child: SizedBox(
                width: 20, height: 20,
                child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white),
              ),
            )
          else
            IconButton(
              key: const Key('refresh_button'),
              icon: const Icon(Icons.refresh),
              onPressed: () => ref.read(jobsProvider.notifier).refresh(),
            ),
          IconButton(
            key: const Key('sync_button'),
            icon: Stack(
              children: [
                const Icon(Icons.sync),
                if ((jobsState.syncStats?.pendingSubmissions ?? 0) > 0)
                  Positioned(
                    right: 0, top: 0,
                    child: Container(
                      width: 8, height: 8,
                      decoration: const BoxDecoration(
                        color: Colors.orange,
                        shape: BoxShape.circle,
                      ),
                    ),
                  ),
              ],
            ),
            onPressed: () async {
              final synced = await ref.read(jobsProvider.notifier).syncPending();
              if (context.mounted) {
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(
                    content: Text(synced > 0
                        ? '$synced submission(s) synced successfully'
                        : 'No pending submissions to sync'),
                    backgroundColor: synced > 0 ? Colors.green.shade700 : null,
                  ),
                );
              }
            },
          ),
          IconButton(
            key: const Key('profile_button'),
            icon: const Icon(Icons.person_outline),
            onPressed: () => context.push('/profile'),
          ),
        ],
      ),
      body: Column(
        children: [
          // ── Offline Banner ──────────────────────────────────────────────
          if (!jobsState.isOnline)
            Container(
              width: double.infinity,
              color: const Color(0xFF92400e),
              padding: const EdgeInsets.symmetric(vertical: 8, horizontal: 16),
              child: Row(
                children: [
                  const Icon(Icons.wifi_off, color: Color(0xFFFEF3C7), size: 16),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Offline mode — showing cached jobs'
                      '${(jobsState.syncStats?.pendingSubmissions ?? 0) > 0
                          ? ' · ${jobsState.syncStats!.pendingSubmissions} pending sync'
                          : ''}',
                      style: const TextStyle(
                        color: Color(0xFFFEF3C7), fontSize: 12,
                      ),
                    ),
                  ),
                ],
              ),
            ),

          // ── Stats Row ───────────────────────────────────────────────────
          Container(
            color: const Color(0xFF166534),
            padding: const EdgeInsets.symmetric(vertical: 16, horizontal: 20),
            child: Row(
              children: [
                _StatBox(
                  value: jobsState.pendingJobs.length.toString(),
                  label: 'Pending',
                  color: Colors.white,
                ),
                _StatBox(
                  value: jobsState.completedJobs.length.toString(),
                  label: 'Completed',
                  color: const Color(0xFFA5D6A7),
                ),
                _StatBox(
                  value: jobsState.jobs.length.toString(),
                  label: 'Total',
                  color: Colors.white,
                ),
                if (!jobsState.isOnline &&
                    (jobsState.syncStats?.pendingSubmissions ?? 0) > 0)
                  _StatBox(
                    value: jobsState.syncStats!.pendingSubmissions.toString(),
                    label: 'Pending Sync',
                    color: const Color(0xFFFBBF24),
                  ),
              ],
            ),
          ),

          // ── Job List ────────────────────────────────────────────────────
          Expanded(
            child: jobsState.isLoading
                ? const Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        CircularProgressIndicator(color: Color(0xFF166534)),
                        SizedBox(height: 12),
                        Text('Loading jobs...', style: TextStyle(color: Color(0xFF6B7280))),
                      ],
                    ),
                  )
                : RefreshIndicator(
                    color: const Color(0xFF166534),
                    onRefresh: () => ref.read(jobsProvider.notifier).refresh(),
                    child: jobsState.pendingJobs.isEmpty
                        ? _EmptyState(isOnline: jobsState.isOnline)
                        : ListView.builder(
                            padding: const EdgeInsets.all(16),
                            itemCount: jobsState.pendingJobs.length,
                            itemBuilder: (context, index) {
                              final job = jobsState.pendingJobs[index];
                              return Padding(
                                padding: const EdgeInsets.only(bottom: 12),
                                child: JobCard(
                                  job: job,
                                  onTap: () {
                                    ref.read(activeJobProvider.notifier).state = job;
                                    context.push('/jobs/${job.id}');
                                  },
                                ),
                              );
                            },
                          ),
                  ),
          ),
        ],
      ),

      // ── SOS FAB ─────────────────────────────────────────────────────────
      floatingActionButton: FloatingActionButton.extended(
        key: const Key('sos_fab'),
        onPressed: () => context.push('/sos'),
        backgroundColor: Colors.red.shade700,
        icon: const Icon(Icons.emergency, color: Colors.white),
        label: const Text(
          'SOS',
          style: TextStyle(color: Colors.white, fontWeight: FontWeight.w900),
        ),
      ),
    );
  }
}

// ─── Job Card ─────────────────────────────────────────────────────────────────

class JobCard extends StatelessWidget {
  final FieldJob job;
  final VoidCallback onTap;

  const JobCard({super.key, required this.job, required this.onTap});

  Color get _alertColor {
    switch (job.alertLevel) {
      case AlertLevel.critical: return const Color(0xFFDC2626);
      case AlertLevel.high:     return const Color(0xFFEA580C);
      case AlertLevel.medium:   return const Color(0xFFD97706);
      case AlertLevel.low:      return const Color(0xFF16A34A);
      case AlertLevel.info:     return const Color(0xFF2563EB);
    }
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(14),
          boxShadow: [
            BoxShadow(
              color: Colors.black.withOpacity(0.06),
              blurRadius: 8,
              offset: const Offset(0, 2),
            ),
          ],
        ),
        child: Row(
          children: [
            // Alert level bar
            Container(
              width: 5,
              height: 90,
              decoration: BoxDecoration(
                color: _alertColor,
                borderRadius: const BorderRadius.only(
                  topLeft: Radius.circular(14),
                  bottomLeft: Radius.circular(14),
                ),
              ),
            ),
            Expanded(
              child: Padding(
                padding: const EdgeInsets.all(14),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            job.customerName,
                            style: const TextStyle(
                              fontSize: 15,
                              fontWeight: FontWeight.w700,
                              color: Color(0xFF111827),
                            ),
                          ),
                        ),
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 8, vertical: 3,
                          ),
                          decoration: BoxDecoration(
                            color: _alertColor.withOpacity(0.1),
                            borderRadius: BorderRadius.circular(20),
                          ),
                          child: Text(
                            job.alertLevel.toApiString(),
                            style: TextStyle(
                              fontSize: 10,
                              fontWeight: FontWeight.w700,
                              color: _alertColor,
                            ),
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 4),
                    Text(
                      job.accountNumber,
                      style: const TextStyle(fontSize: 12, color: Color(0xFF6B7280)),
                    ),
                    Text(
                      job.address,
                      style: const TextStyle(fontSize: 12, color: Color(0xFF6B7280)),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 8),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 8, vertical: 3,
                          ),
                          decoration: BoxDecoration(
                            color: const Color(0xFFF3F4F6),
                            borderRadius: BorderRadius.circular(6),
                          ),
                          child: Text(
                            job.anomalyType,
                            style: const TextStyle(
                              fontSize: 11, color: Color(0xFF374151),
                            ),
                          ),
                        ),
                        if (job.estimatedVarianceGhs != null)
                          Text(
                            '₵${job.estimatedVarianceGhs!.toStringAsFixed(2)}',
                            style: const TextStyle(
                              fontSize: 13,
                              fontWeight: FontWeight.w700,
                              color: Color(0xFFDC2626),
                            ),
                          ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ─── Stat Box ─────────────────────────────────────────────────────────────────

class _StatBox extends StatelessWidget {
  final String value;
  final String label;
  final Color color;

  const _StatBox({required this.value, required this.label, required this.color});

  @override
  Widget build(BuildContext context) => Expanded(
    child: Column(
      children: [
        Text(
          value,
          style: TextStyle(
            fontSize: 24, fontWeight: FontWeight.w900, color: color,
          ),
        ),
        Text(
          label,
          style: const TextStyle(fontSize: 11, color: Color(0xFFA5D6A7)),
        ),
      ],
    ),
  );
}

// ─── Empty State ──────────────────────────────────────────────────────────────

class _EmptyState extends StatelessWidget {
  final bool isOnline;
  const _EmptyState({required this.isOnline});

  @override
  Widget build(BuildContext context) => Center(
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        const Text('✅', style: TextStyle(fontSize: 48)),
        const SizedBox(height: 12),
        Text(
          isOnline ? 'All jobs completed for today!' : 'No cached jobs available',
          style: const TextStyle(fontSize: 16, color: Color(0xFF6B7280)),
        ),
        if (!isOnline) ...[
          const SizedBox(height: 8),
          const Text(
            'Connect to the internet to fetch new jobs',
            style: TextStyle(fontSize: 13, color: Color(0xFF9CA3AF)),
          ),
        ],
      ],
    ),
  );
}
