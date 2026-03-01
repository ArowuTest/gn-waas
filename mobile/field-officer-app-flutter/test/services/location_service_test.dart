// GN-WAAS Flutter — Location Service Unit Tests
// Tests the pure Haversine calculation (no platform dependency)

import 'package:flutter_test/flutter_test.dart';
import 'package:gnwaas_field_officer/services/location_service.dart';

void main() {
  group('LocationService.haversineDistance', () {
    test('returns 0 for identical coordinates', () {
      final dist = LocationService.haversineDistance(5.6037, -0.1870, 5.6037, -0.1870);
      expect(dist, closeTo(0.0, 0.001));
    });

    test('calculates known distance between Accra and Kumasi (~200 km straight-line)', () {
      // Accra: 5.6037, -0.1870
      // Kumasi: 6.6885, -1.6244
      // Straight-line (haversine) distance is ~199.5 km
      final dist = LocationService.haversineDistance(5.6037, -0.1870, 6.6885, -1.6244);
      expect(dist, greaterThan(190000));
      expect(dist, lessThan(210000));
    });

    test('calculates short distance correctly (within geofence)', () {
      // Two points ~50 metres apart
      final dist = LocationService.haversineDistance(
        5.6037, -0.1870,
        5.6037, -0.18694, // ~6m longitude shift at equator ≈ ~6m
      );
      expect(dist, lessThan(100)); // well within 100m geofence
    });

    test('calculates distance outside geofence (>100m)', () {
      // ~200 metres apart
      final dist = LocationService.haversineDistance(
        5.6037, -0.1870,
        5.6055, -0.1870,
      );
      expect(dist, greaterThan(100));
    });

    test('is symmetric (A→B == B→A)', () {
      final d1 = LocationService.haversineDistance(5.6037, -0.1870, 6.6885, -1.6244);
      final d2 = LocationService.haversineDistance(6.6885, -1.6244, 5.6037, -0.1870);
      expect(d1, closeTo(d2, 0.001));
    });

    test('handles negative latitudes (southern hemisphere)', () {
      // Johannesburg: -26.2041, 28.0473
      // Cape Town: -33.9249, 18.4241
      final dist = LocationService.haversineDistance(
        -26.2041, 28.0473,
        -33.9249, 18.4241,
      );
      // ~1261 km straight-line
      expect(dist, greaterThan(1200000));
      expect(dist, lessThan(1350000));
    });

    test('handles crossing the prime meridian', () {
      final dist = LocationService.haversineDistance(
        51.5074, -0.1278, // London
        48.8566,  2.3522, // Paris
      );
      // ~343 km straight-line
      expect(dist, greaterThan(330000));
      expect(dist, lessThan(360000));
    });
  });

  group('LocationService geofence logic', () {
    // We test the pure math version since we can't call Geolocator in unit tests
    test('point within 100m is inside geofence', () {
      const targetLat = 5.6037;
      const targetLng = -0.1870;
      // ~50m away
      const currentLat = 5.6037;
      const currentLng = -0.18694;

      final dist = LocationService.haversineDistance(
        currentLat, currentLng, targetLat, targetLng,
      );
      expect(dist <= 100.0, isTrue);
    });

    test('point 200m away is outside geofence', () {
      const targetLat = 5.6037;
      const targetLng = -0.1870;
      // ~200m away
      const currentLat = 5.6055;
      const currentLng = -0.1870;

      final dist = LocationService.haversineDistance(
        currentLat, currentLng, targetLat, targetLng,
      );
      expect(dist > 100.0, isTrue);
    });
  });
}
