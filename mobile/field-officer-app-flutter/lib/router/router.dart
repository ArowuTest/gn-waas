// GN-WAAS Field Officer App — Router

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../providers/providers.dart';
import '../screens/auth/login_screen.dart';
import '../screens/jobs/job_list_screen.dart';
import '../screens/jobs/job_detail_screen.dart';
import '../screens/capture/meter_capture_screen.dart';
import '../screens/sos/sos_screen.dart';
import '../screens/profile/profile_screen.dart';
import '../screens/reports/illegal_connection_screen.dart'; // GAP-FIX-02

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
      // GAP-FIX-02: IllegalConnectionScreen was imported but had no route
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
      body: Center(
        child: Text('Page not found: ${state.error}'),
      ),
    ),
  );
});
