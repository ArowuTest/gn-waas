// GN-WAAS Flutter — Provider / State Tests

import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:gnwaas_field_officer/models/models.dart';
import 'package:gnwaas_field_officer/providers/providers.dart';

void main() {
  group('AuthState', () {
    test('isAuthenticated is false when user and token are null', () {
      const state = AuthState();
      expect(state.isAuthenticated, isFalse);
    });

    test('isAuthenticated is false when only user is set', () {
      final state = AuthState(
        user: User(id: '1', email: 'a@b.com', fullName: 'A', role: 'OFFICER'),
      );
      expect(state.isAuthenticated, isFalse);
    });

    test('isAuthenticated is false when only token is set', () {
      const state = AuthState(token: 'some-token');
      expect(state.isAuthenticated, isFalse);
    });

    test('isAuthenticated is true when both user and token are set', () {
      final state = AuthState(
        user:  User(id: '1', email: 'a@b.com', fullName: 'A', role: 'OFFICER'),
        token: 'valid-jwt-token',
      );
      expect(state.isAuthenticated, isTrue);
    });

    test('copyWith preserves unchanged fields', () {
      final original = AuthState(
        user:      User(id: '1', email: 'a@b.com', fullName: 'A', role: 'OFFICER'),
        token:     'token',
        isLoading: false,
      );
      final updated = original.copyWith(isLoading: true);
      expect(updated.user,      original.user);
      expect(updated.token,     original.token);
      expect(updated.isLoading, isTrue);
    });

    test('copyWith clearError removes error', () {
      const original = AuthState(error: 'Login failed');
      final updated = original.copyWith(clearError: true);
      expect(updated.error, isNull);
    });

    test('copyWith clearUser removes user and token', () {
      final original = AuthState(
        user:  User(id: '1', email: 'a@b.com', fullName: 'A', role: 'OFFICER'),
        token: 'token',
      );
      final updated = original.copyWith(clearUser: true);
      expect(updated.user,  isNull);
      expect(updated.token, isNull);
    });
  });

  group('JobsState', () {
    FieldJob makeJob(String id, FieldJobStatus status) => FieldJob(
      id:            id,
      auditEventId:  'audit-$id',
      accountNumber: 'ACC-$id',
      customerName:  'Customer $id',
      address:       'Address $id',
      gpsLat:        5.6037,
      gpsLng:        -0.1870,
      anomalyType:   'BILLING_VARIANCE',
      alertLevel:    AlertLevel.medium,
      status:        status,
    );

    test('pendingJobs excludes completed and failed', () {
      final state = JobsState(jobs: [
        makeJob('1', FieldJobStatus.queued),
        makeJob('2', FieldJobStatus.dispatched),
        makeJob('3', FieldJobStatus.completed),
        makeJob('4', FieldJobStatus.failed),
        makeJob('5', FieldJobStatus.onSite),
      ]);

      expect(state.pendingJobs.length, 3);
      expect(state.pendingJobs.map((j) => j.id).toList(), containsAll(['1', '2', '5']));
    });

    test('completedJobs includes only completed', () {
      final state = JobsState(jobs: [
        makeJob('1', FieldJobStatus.queued),
        makeJob('2', FieldJobStatus.completed),
        makeJob('3', FieldJobStatus.completed),
        makeJob('4', FieldJobStatus.failed),
      ]);

      expect(state.completedJobs.length, 2);
    });

    test('pendingJobs is empty when all jobs are completed', () {
      final state = JobsState(jobs: [
        makeJob('1', FieldJobStatus.completed),
        makeJob('2', FieldJobStatus.completed),
      ]);
      expect(state.pendingJobs, isEmpty);
    });

    test('copyWith preserves unchanged fields', () {
      final original = JobsState(
        jobs:     [makeJob('1', FieldJobStatus.queued)],
        isOnline: true,
      );
      final updated = original.copyWith(isLoading: true);
      expect(updated.jobs.length, 1);
      expect(updated.isOnline,    isTrue);
      expect(updated.isLoading,   isTrue);
    });

    test('copyWith clearError removes error', () {
      const original = JobsState(error: 'Network error');
      final updated = original.copyWith(clearError: true);
      expect(updated.error, isNull);
    });
  });

  group('activeJobProvider', () {
    test('initial value is null', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);

      final job = container.read(activeJobProvider);
      expect(job, isNull);
    });

    test('can be set and read', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);

      final testJob = FieldJob(
        id:            'job-test',
        auditEventId:  'audit-test',
        accountNumber: 'ACC-TEST',
        customerName:  'Test Customer',
        address:       'Test Address',
        gpsLat:        5.6037,
        gpsLng:        -0.1870,
        anomalyType:   'TEST',
        alertLevel:    AlertLevel.high,
        status:        FieldJobStatus.dispatched,
      );

      container.read(activeJobProvider.notifier).state = testJob;
      final retrieved = container.read(activeJobProvider);

      expect(retrieved, isNotNull);
      expect(retrieved!.id, 'job-test');
      expect(retrieved.alertLevel, AlertLevel.high);
    });

    test('can be cleared', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);

      final testJob = FieldJob(
        id:            'job-clear',
        auditEventId:  'audit-clear',
        accountNumber: 'ACC-CLEAR',
        customerName:  'Clear Customer',
        address:       'Clear Address',
        gpsLat:        5.0,
        gpsLng:        -1.0,
        anomalyType:   'TEST',
        alertLevel:    AlertLevel.low,
        status:        FieldJobStatus.queued,
      );

      container.read(activeJobProvider.notifier).state = testJob;
      expect(container.read(activeJobProvider), isNotNull);

      container.read(activeJobProvider.notifier).state = null;
      expect(container.read(activeJobProvider), isNull);
    });
  });
}
