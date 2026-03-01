// GN-WAAS Flutter — Widget Tests: Login Screen

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:gnwaas_field_officer/screens/auth/login_screen.dart';
import 'package:gnwaas_field_officer/providers/providers.dart';
import 'package:gnwaas_field_officer/models/models.dart';
import 'package:gnwaas_field_officer/services/api_service.dart';

// ─── Fake Auth Notifier ───────────────────────────────────────────────────────

class FakeAuthNotifier extends AuthNotifier {
  bool loginCalled  = false;
  String? lastEmail;
  String? lastPassword;
  bool shouldFail   = false;

  FakeAuthNotifier() : super(ApiService());

  @override
  Future<void> restoreSession() async {
    // No-op in tests — don't try to read secure storage
  }

  @override
  Future<void> login(String email, String password) async {
    loginCalled  = true;
    lastEmail    = email;
    lastPassword = password;
    if (shouldFail) {
      state = state.copyWith(isLoading: false, error: 'Invalid credentials');
    } else {
      state = AuthState(
        user:  User(id: '1', email: email, fullName: 'Test Officer', role: 'FIELD_OFFICER'),
        token: 'fake-token',
      );
    }
  }
}

// ─── Test Helpers ─────────────────────────────────────────────────────────────

Widget buildLoginScreen({FakeAuthNotifier? fakeAuth}) {
  final notifier = fakeAuth ?? FakeAuthNotifier();
  final router = GoRouter(
    routes: [
      GoRoute(path: '/', builder: (_, __) => const LoginScreen()),
      GoRoute(path: '/jobs', builder: (_, __) => const Scaffold(body: Text('Jobs Screen'))),
    ],
  );

  return ProviderScope(
    overrides: [
      authProvider.overrideWith((ref) => notifier),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

// ─── Tests ────────────────────────────────────────────────────────────────────

void main() {
  group('LoginScreen', () {
    testWidgets('renders all key UI elements', (tester) async {
      await tester.pumpWidget(buildLoginScreen());
      await tester.pumpAndSettle();

      expect(find.text('GN-WAAS'),          findsOneWidget);
      expect(find.text('Field Officer App'), findsOneWidget);
      expect(find.byKey(const Key('email_field')),    findsOneWidget);
      expect(find.byKey(const Key('password_field')), findsOneWidget);
      expect(find.byKey(const Key('login_button')),   findsOneWidget);
    });

    testWidgets('shows validation error when email is empty', (tester) async {
      await tester.pumpWidget(buildLoginScreen());
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('login_button')));
      await tester.pumpAndSettle();

      expect(find.text('Email is required'), findsOneWidget);
    });

    testWidgets('shows validation error for invalid email', (tester) async {
      await tester.pumpWidget(buildLoginScreen());
      await tester.pumpAndSettle();

      await tester.enterText(find.byKey(const Key('email_field')), 'notanemail');
      await tester.tap(find.byKey(const Key('login_button')));
      await tester.pumpAndSettle();

      expect(find.text('Enter a valid email'), findsOneWidget);
    });

    testWidgets('shows validation error when password is empty', (tester) async {
      await tester.pumpWidget(buildLoginScreen());
      await tester.pumpAndSettle();

      await tester.enterText(
        find.byKey(const Key('email_field')), 'officer@gwl.gov.gh',
      );
      await tester.tap(find.byKey(const Key('login_button')));
      await tester.pumpAndSettle();

      expect(find.text('Password is required'), findsOneWidget);
    });

    testWidgets('shows validation error for short password', (tester) async {
      await tester.pumpWidget(buildLoginScreen());
      await tester.pumpAndSettle();

      await tester.enterText(find.byKey(const Key('email_field')), 'a@b.com');
      await tester.enterText(find.byKey(const Key('password_field')), '123');
      await tester.tap(find.byKey(const Key('login_button')));
      await tester.pumpAndSettle();

      expect(find.text('Password must be at least 6 characters'), findsOneWidget);
    });

    testWidgets('calls login with correct credentials', (tester) async {
      final fakeAuth = FakeAuthNotifier();
      await tester.pumpWidget(buildLoginScreen(fakeAuth: fakeAuth));
      await tester.pumpAndSettle();

      await tester.enterText(
        find.byKey(const Key('email_field')), 'officer@gwl.gov.gh',
      );
      await tester.enterText(
        find.byKey(const Key('password_field')), 'password123',
      );
      await tester.tap(find.byKey(const Key('login_button')));
      await tester.pumpAndSettle();

      expect(fakeAuth.loginCalled,  isTrue);
      expect(fakeAuth.lastEmail,    'officer@gwl.gov.gh');
      expect(fakeAuth.lastPassword, 'password123');
    });

    testWidgets('password field is obscured by default', (tester) async {
      await tester.pumpWidget(buildLoginScreen());
      await tester.pumpAndSettle();

      final passwordField = tester.widget<TextField>(
        find.descendant(
          of: find.byKey(const Key('password_field')),
          matching: find.byType(TextField),
        ),
      );
      expect(passwordField.obscureText, isTrue);
    });

    testWidgets('toggle visibility button shows/hides password', (tester) async {
      await tester.pumpWidget(buildLoginScreen());
      await tester.pumpAndSettle();

      var passwordField = tester.widget<TextField>(
        find.descendant(
          of: find.byKey(const Key('password_field')),
          matching: find.byType(TextField),
        ),
      );
      expect(passwordField.obscureText, isTrue);

      await tester.tap(find.byIcon(Icons.visibility_off));
      await tester.pumpAndSettle();

      passwordField = tester.widget<TextField>(
        find.descendant(
          of: find.byKey(const Key('password_field')),
          matching: find.byType(TextField),
        ),
      );
      expect(passwordField.obscureText, isFalse);
    });

    testWidgets('shows loading indicator while logging in', (tester) async {
      final fakeAuth = FakeAuthNotifier();
      await tester.pumpWidget(buildLoginScreen(fakeAuth: fakeAuth));
      await tester.pumpAndSettle();

      fakeAuth.state = const AuthState(isLoading: true);
      await tester.pump();

      expect(find.byType(CircularProgressIndicator), findsOneWidget);
    });
  });
}
