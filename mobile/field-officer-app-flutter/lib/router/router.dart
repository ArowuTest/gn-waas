// GN-WAAS Field Officer App — Router

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../providers/providers.dart';
import '../screens/auth/login_screen.dart';
import '../screens/jobs/job_list_screen.dart';
import '../screens/jobs/job_detail_screen.dart';
import '../screens/capture/meter_capture_screen.dart';
import '../screens/outcome/outcome_recording_screen.dart';
import '../screens/sos/sos_screen.dart';
import '../screens/profile/profile_screen.dart';
import '../screens/reports/illegal_connection_screen.dart';

final routerProvider = Provider<GoRouter>((ref) {
  final authState = ref.watch(authProvider);

  return GoRouter(
    initialLocation: '/jobs',
    redirect: (context, state) {
      final isLoggedIn = authState.isAuthenticated;
      final isLoginPage = state.matchedLocation == '/login';

      if (!isLoggedIn && !isLoginPage) return '/login';
      if (isLoggedIn && isLoginPage) return '/jobs';
      return null;
    },
    routes: [
      GoRoute(
        path: '/login',
        name: 'login',
        builder: (context, state) => const LoginScreen(),
      ),
      GoRoute(
        path: '/jobs',
        name: 'jobs',
        builder: (context, state) => const JobListScreen(),
      ),
      GoRoute(
        path: '/jobs/:id',
        name: 'job-detail',
        builder: (context, state) => JobDetailScreen(
          jobId: state.pathParameters['id']!,
        ),
      ),
      GoRoute(
        path: '/capture',
        name: 'capture',
        builder: (context, state) => const MeterCaptureScreen(),
      ),
      // Outcome recording — reached after evidence submission or from job detail
      GoRoute(
        path: '/jobs/:id/outcome',
        name: 'record-outcome',
        builder: (context, state) => OutcomeRecordingScreen(
          jobId: state.pathParameters['id']!,
        ),
      ),
      GoRoute(
        path: '/sos',
        name: 'sos',
        builder: (context, state) => const SOSScreen(),
      ),
      GoRoute(
        path: '/profile',
        name: 'profile',
        builder: (context, state) => const ProfileScreen(),
      ),
      // Illegal connection report — linked from job detail or standalone
      GoRoute(
        path: '/report-illegal',
        name: 'report-illegal',
        builder: (context, state) {
          final jobId = state.uri.queryParameters['job_id'];
          return IllegalConnectionScreen(jobId: jobId);
        },
      ),
    ],
    errorBuilder: (context, state) => Scaffold(
      backgroundColor: const Color(0xFF0f172a),
      body: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.error_outline, color: Color(0xFF94A3B8), size: 48),
            const SizedBox(height: 16),
            Text(
              'Page not found',
              style: const TextStyle(
                color: Colors.white,
                fontSize: 18,
                fontWeight: FontWeight.w700,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              state.error?.toString() ?? 'Unknown error',
              style: const TextStyle(color: Color(0xFF94A3B8), fontSize: 13),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    ),
  );
});
