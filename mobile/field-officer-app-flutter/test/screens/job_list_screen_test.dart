// GN-WAAS Flutter — Widget Tests: Job List Screen

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:gnwaas_field_officer/screens/jobs/job_list_screen.dart';
import 'package:gnwaas_field_officer/providers/providers.dart';
import 'package:gnwaas_field_officer/models/models.dart';
import 'package:gnwaas_field_officer/services/sync_service.dart';
import 'package:gnwaas_field_officer/services/api_service.dart';
import 'package:gnwaas_field_officer/services/offline_storage_service.dart';

// ─── Fake Jobs Notifier ───────────────────────────────────────────────────────

class FakeJobsNotifier extends JobsNotifier {
  bool refreshCalled = false;
  bool syncCalled    = false;

  FakeJobsNotifier(JobsState initial)
      : super(SyncService(
          api:     ApiService(),
          storage: OfflineStorageService(),
        )) {
    state = initial;
  }

  @override
  Future<void> loadJobs() async {
    // No-op — state is set in constructor
  }

  @override
  Future<void> refresh() async {
    refreshCalled = true;
  }

  @override
  Future<int> syncPending() async {
    syncCalled = true;
    return 0;
  }
}

// ─── Fake Auth Notifier ───────────────────────────────────────────────────────

class FakeAuthNotifier extends AuthNotifier {
  FakeAuthNotifier() : super(ApiService());

  @override
  Future<void> restoreSession() async {}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

FieldJob makeJob({
  String id = 'job-001',
  FieldJobStatus status = FieldJobStatus.queued,
  AlertLevel alertLevel = AlertLevel.medium,
  double? variance,
}) => FieldJob(
  id:            id,
  jobReference:  'GWL-$id',
  auditEventId:  'audit-$id',
  accountNumber: 'ACC-$id',
  customerName:  'Customer $id',
  address:       '1 Test St, Accra',
  gpsLat:        5.6037,
  gpsLng:        -0.1870,
  anomalyType:   'BILLING_VARIANCE',
  alertLevel:    alertLevel,
  status:        status,
  estimatedVarianceGhs: variance,
);

Widget buildJobListScreen(JobsState initialState) {
  final jobsNotifier = FakeJobsNotifier(initialState);
  final authNotifier = FakeAuthNotifier()
    ..state = AuthState(
      user:  User(id: '1', email: 'a@b.com', fullName: 'Test Officer', role: 'OFFICER'),
      token: 'token',
    );

  final router = GoRouter(
    routes: [
      GoRoute(path: '/', builder: (_, __) => const JobListScreen()),
      GoRoute(
        path: '/jobs/:id',
        builder: (_, state) => Scaffold(
          body: Text('Job Detail ${state.pathParameters['id']}'),
        ),
      ),
      GoRoute(path: '/sos',     builder: (_, __) => const Scaffold(body: Text('SOS Screen'))),
      GoRoute(path: '/profile', builder: (_, __) => const Scaffold(body: Text('Profile Screen'))),
    ],
  );

  return ProviderScope(
    overrides: [
      jobsProvider.overrideWith((ref) => jobsNotifier),
      authProvider.overrideWith((ref) => authNotifier),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

// ─── Tests ────────────────────────────────────────────────────────────────────

void main() {
  group('JobListScreen', () {
    testWidgets('renders app bar with title', (tester) async {
      await tester.pumpWidget(buildJobListScreen(const JobsState()));
      await tester.pumpAndSettle();

      expect(find.text('My Jobs'), findsOneWidget);
    });

    testWidgets('shows loading indicator when isLoading is true', (tester) async {
      await tester.pumpWidget(
        buildJobListScreen(const JobsState(isLoading: true)),
      );
      await tester.pump();

      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      expect(find.text('Loading jobs...'), findsOneWidget);
    });

    testWidgets('shows empty state when no pending jobs', (tester) async {
      await tester.pumpWidget(buildJobListScreen(const JobsState(isOnline: true)));
      await tester.pumpAndSettle();

      expect(find.text('All jobs completed for today!'), findsOneWidget);
    });

    testWidgets('shows offline empty state when offline', (tester) async {
      await tester.pumpWidget(
        buildJobListScreen(const JobsState(isOnline: false)),
      );
      await tester.pumpAndSettle();

      expect(find.text('No cached jobs available'), findsOneWidget);
    });

    testWidgets('shows offline banner when not online', (tester) async {
      await tester.pumpWidget(
        buildJobListScreen(const JobsState(isOnline: false)),
      );
      await tester.pumpAndSettle();

      expect(find.text('Offline mode — showing cached jobs'), findsOneWidget);
    });

    testWidgets('does not show offline banner when online', (tester) async {
      await tester.pumpWidget(
        buildJobListScreen(const JobsState(isOnline: true)),
      );
      await tester.pumpAndSettle();

      expect(find.text('Offline mode — showing cached jobs'), findsNothing);
    });

    testWidgets('renders job cards for pending jobs', (tester) async {
      final jobs = [
        makeJob(id: 'job-001', status: FieldJobStatus.queued),
        makeJob(id: 'job-002', status: FieldJobStatus.dispatched),
      ];
      await tester.pumpWidget(
        buildJobListScreen(JobsState(jobs: jobs, isOnline: true)),
      );
      await tester.pumpAndSettle();

      expect(find.text('Customer job-001'), findsOneWidget);
      expect(find.text('Customer job-002'), findsOneWidget);
    });

    testWidgets('does not show completed jobs in list', (tester) async {
      final jobs = [
        makeJob(id: 'job-001', status: FieldJobStatus.queued),
        makeJob(id: 'job-002', status: FieldJobStatus.completed),
      ];
      await tester.pumpWidget(
        buildJobListScreen(JobsState(jobs: jobs, isOnline: true)),
      );
      await tester.pumpAndSettle();

      expect(find.text('Customer job-001'), findsOneWidget);
      expect(find.text('Customer job-002'), findsNothing);
    });

    testWidgets('SOS FAB is visible', (tester) async {
      await tester.pumpWidget(buildJobListScreen(const JobsState()));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('sos_fab')), findsOneWidget);
      expect(find.text('SOS'), findsOneWidget);
    });

    testWidgets('shows variance amount on job card', (tester) async {
      final jobs = [makeJob(id: 'job-001', variance: 450.75)];
      await tester.pumpWidget(
        buildJobListScreen(JobsState(jobs: jobs, isOnline: true)),
      );
      await tester.pumpAndSettle();

      // JobCard now shows monthly leakage with /mo suffix
      expect(find.text('₵450.75/mo'), findsOneWidget);
    });
  });

  group('JobCard', () {
    testWidgets('renders customer name and account number', (tester) async {
      final job = makeJob(id: 'job-001');
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(body: JobCard(job: job, onTap: () {})),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Customer job-001'), findsOneWidget);
      expect(find.text('ACC-job-001'),      findsOneWidget);
    });

    testWidgets('calls onTap when tapped', (tester) async {
      bool tapped = false;
      final job = makeJob();
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(body: JobCard(job: job, onTap: () => tapped = true)),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.byType(JobCard));
      expect(tapped, isTrue);
    });

    testWidgets('shows alert level badge', (tester) async {
      final job = makeJob(alertLevel: AlertLevel.critical);
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(body: JobCard(job: job, onTap: () {})),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('CRITICAL'), findsOneWidget);
    });
  });
}
