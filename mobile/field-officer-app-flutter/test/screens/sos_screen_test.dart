// GN-WAAS Flutter — Widget Tests: SOS Screen

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:gnwaas_field_officer/screens/sos/sos_screen.dart';
import 'package:gnwaas_field_officer/providers/providers.dart';
import 'package:gnwaas_field_officer/models/models.dart';

Widget buildSOSScreen() {
  final router = GoRouter(
    routes: [
      GoRoute(path: '/', builder: (_, __) => const SOSScreen()),
      GoRoute(path: '/jobs', builder: (_, __) => const Scaffold(body: Text('Jobs'))),
    ],
  );

  return ProviderScope(
    overrides: [
      activeJobProvider.overrideWith((ref) => FieldJob(
        id:            'job-sos',
        auditEventId:  'audit-sos',
        accountNumber: 'ACC-SOS',
        customerName:  'SOS Customer',
        address:       'SOS Address',
        gpsLat:        5.6037,
        gpsLng:        -0.1870,
        anomalyType:   'TEST',
        alertLevel:    AlertLevel.critical,
        status:        FieldJobStatus.onSite,
      )),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

void main() {
  group('SOSScreen', () {
    testWidgets('renders SOS title and description', (tester) async {
      await tester.pumpWidget(buildSOSScreen());
      await tester.pumpAndSettle();

      expect(find.text('Emergency SOS'), findsWidgets);
      expect(find.byKey(const Key('sos_trigger_button')), findsOneWidget);
    });

    testWidgets('shows confirmation dialog when SOS button tapped', (tester) async {
      await tester.pumpWidget(buildSOSScreen());
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('sos_trigger_button')));
      await tester.pumpAndSettle();

      expect(find.text('🚨 Trigger SOS?'), findsOneWidget);
      expect(find.byKey(const Key('confirm_sos_button')), findsOneWidget);
      // Dialog has a Cancel button (use key to be specific)
      expect(find.byKey(const Key('dialog_cancel_button')), findsOneWidget);
    });

    testWidgets('dismisses dialog on Cancel', (tester) async {
      await tester.pumpWidget(buildSOSScreen());
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('sos_trigger_button')));
      await tester.pumpAndSettle();

      // Tap the dialog Cancel button specifically
      await tester.tap(find.byKey(const Key('dialog_cancel_button')));
      await tester.pumpAndSettle();

      // Dialog dismissed, still on SOS screen
      expect(find.text('🚨 Trigger SOS?'), findsNothing);
      expect(find.byKey(const Key('sos_trigger_button')), findsOneWidget);
    });

    testWidgets('shows emergency description text', (tester) async {
      await tester.pumpWidget(buildSOSScreen());
      await tester.pumpAndSettle();

      expect(
        find.text(
          'Use only in genuine emergencies.\nThis will alert your supervisor and dispatch support.',
        ),
        findsOneWidget,
      );
    });
  });
}
