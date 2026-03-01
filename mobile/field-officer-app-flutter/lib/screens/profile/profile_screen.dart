// GN-WAAS Field Officer App — Profile Screen

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../providers/providers.dart';

class ProfileScreen extends ConsumerWidget {
  const ProfileScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auth = ref.watch(authProvider);
    final user = auth.user;

    return Scaffold(
      backgroundColor: const Color(0xFFF9FAFB),
      appBar: AppBar(
        backgroundColor: const Color(0xFF166534),
        foregroundColor: Colors.white,
        title: const Text('Profile', style: TextStyle(fontWeight: FontWeight.w700)),
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(24),
        child: Column(
          children: [
            // ── Avatar ────────────────────────────────────────────────────
            Container(
              width: 80, height: 80,
              decoration: BoxDecoration(
                color: const Color(0xFF166534),
                shape: BoxShape.circle,
              ),
              child: Center(
                child: Text(
                  user?.fullName.isNotEmpty == true
                      ? user!.fullName[0].toUpperCase()
                      : '?',
                  style: const TextStyle(
                    color: Colors.white, fontSize: 32, fontWeight: FontWeight.w900,
                  ),
                ),
              ),
            ),
            const SizedBox(height: 12),
            Text(
              user?.fullName ?? 'Unknown Officer',
              style: const TextStyle(
                fontSize: 20, fontWeight: FontWeight.w800, color: Color(0xFF111827),
              ),
            ),
            Text(
              user?.email ?? '',
              style: const TextStyle(color: Color(0xFF6B7280)),
            ),
            const SizedBox(height: 24),

            // ── Info Cards ────────────────────────────────────────────────
            _ProfileCard(
              items: [
                _ProfileItem(label: 'Role',         value: user?.role ?? '-'),
                _ProfileItem(label: 'Badge Number', value: user?.badgeNumber ?? '-'),
                _ProfileItem(label: 'District',     value: user?.districtId ?? '-'),
              ],
            ),
            const SizedBox(height: 16),

            // ── Sync Stats ────────────────────────────────────────────────
            Consumer(
              builder: (context, ref, _) {
                final jobsState = ref.watch(jobsProvider);
                final stats = jobsState.syncStats;
                return _ProfileCard(
                  title: 'Sync Status',
                  items: [
                    _ProfileItem(
                      label: 'Cached Jobs',
                      value: stats?.cachedJobs.toString() ?? '0',
                    ),
                    _ProfileItem(
                      label: 'Pending Sync',
                      value: stats?.pendingSubmissions.toString() ?? '0',
                      valueColor: (stats?.pendingSubmissions ?? 0) > 0
                          ? Colors.orange
                          : null,
                    ),
                    _ProfileItem(
                      label: 'Last Sync',
                      value: stats?.lastSyncAt != null
                          ? _formatTime(stats!.lastSyncAt!)
                          : 'Never',
                    ),
                  ],
                );
              },
            ),
            const SizedBox(height: 32),

            // ── Logout ────────────────────────────────────────────────────
            SizedBox(
              width: double.infinity,
              child: ElevatedButton.icon(
                key: const Key('logout_button'),
                onPressed: () async {
                  final confirmed = await showDialog<bool>(
                    context: context,
                    builder: (ctx) => AlertDialog(
                      title: const Text('Sign Out'),
                      content: const Text('Are you sure you want to sign out?'),
                      actions: [
                        TextButton(
                          onPressed: () => Navigator.pop(ctx, false),
                          child: const Text('Cancel'),
                        ),
                        ElevatedButton(
                          onPressed: () => Navigator.pop(ctx, true),
                          style: ElevatedButton.styleFrom(
                            backgroundColor: Colors.red.shade700,
                          ),
                          child: const Text(
                            'Sign Out',
                            style: TextStyle(color: Colors.white),
                          ),
                        ),
                      ],
                    ),
                  );
                  if (confirmed == true && context.mounted) {
                    await ref.read(authProvider.notifier).logout();
                    if (context.mounted) context.go('/login');
                  }
                },
                icon: const Icon(Icons.logout),
                label: const Text('Sign Out'),
                style: ElevatedButton.styleFrom(
                  backgroundColor: Colors.red.shade700,
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

  String _formatTime(DateTime dt) {
    final now = DateTime.now();
    final diff = now.difference(dt);
    if (diff.inMinutes < 1) return 'Just now';
    if (diff.inMinutes < 60) return '${diff.inMinutes}m ago';
    if (diff.inHours < 24) return '${diff.inHours}h ago';
    return '${diff.inDays}d ago';
  }
}

class _ProfileCard extends StatelessWidget {
  final String? title;
  final List<_ProfileItem> items;
  const _ProfileCard({this.title, required this.items});

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
        if (title != null) ...[
          Text(
            title!,
            style: const TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: Color(0xFF6B7280),
              letterSpacing: 0.5,
            ),
          ),
          const SizedBox(height: 8),
        ],
        ...items.map((item) => Padding(
          padding: const EdgeInsets.only(bottom: 8),
          child: Row(
            children: [
              SizedBox(
                width: 110,
                child: Text(
                  item.label,
                  style: const TextStyle(fontSize: 13, color: Color(0xFF9CA3AF)),
                ),
              ),
              Expanded(
                child: Text(
                  item.value,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: item.valueColor ?? const Color(0xFF111827),
                  ),
                ),
              ),
            ],
          ),
        )),
      ],
    ),
  );
}

class _ProfileItem {
  final String label;
  final String value;
  final Color? valueColor;
  const _ProfileItem({required this.label, required this.value, this.valueColor});
}
