// GN-WAAS Field Officer App — Location Service
// GPS capture with geofence validation

import 'dart:math';
import 'package:geolocator/geolocator.dart';
import '../config/app_config.dart';

class LocationResult {
  final double lat;
  final double lng;
  final double accuracyM;
  final DateTime timestamp;

  const LocationResult({
    required this.lat,
    required this.lng,
    required this.accuracyM,
    required this.timestamp,
  });
}

class LocationService {
  /// Request location permission and get current position
  Future<LocationResult> getCurrentPosition() async {
    bool serviceEnabled = await Geolocator.isLocationServiceEnabled();
    if (!serviceEnabled) {
      throw Exception('Location services are disabled. Please enable GPS.');
    }

    LocationPermission permission = await Geolocator.checkPermission();
    if (permission == LocationPermission.denied) {
      permission = await Geolocator.requestPermission();
      if (permission == LocationPermission.denied) {
        throw Exception('Location permission denied.');
      }
    }
    if (permission == LocationPermission.deniedForever) {
      throw Exception(
        'Location permission permanently denied. Please enable in Settings.',
      );
    }

    final position = await Geolocator.getCurrentPosition(
      desiredAccuracy: LocationAccuracy.high,
      timeLimit: const Duration(seconds: 15),
    );

    return LocationResult(
      lat:       position.latitude,
      lng:       position.longitude,
      accuracyM: position.accuracy,
      timestamp: position.timestamp,
    );
  }

  /// Check if a point is within the geofence radius of a target
  bool isWithinFence(
    double currentLat,
    double currentLng,
    double targetLat,
    double targetLng, {
    double? radiusMetres,
  }) {
    final radius = radiusMetres ?? AppConfig.geofenceRadiusMetres;
    final distanceM = Geolocator.distanceBetween(
      currentLat, currentLng,
      targetLat,  targetLng,
    );
    return distanceM <= radius;
  }

  /// Calculate distance in metres between two coordinates
  double distanceBetween(
    double lat1, double lng1,
    double lat2, double lng2,
  ) {
    return Geolocator.distanceBetween(lat1, lng1, lat2, lng2);
  }

  /// Pure Haversine calculation (no platform dependency — for testing)
  static double haversineDistance(
    double lat1, double lon1,
    double lat2, double lon2,
  ) {
    const r = 6371000.0; // Earth radius in metres
    final dLat = _toRad(lat2 - lat1);
    final dLon = _toRad(lon2 - lon1);
    final a = sin(dLat / 2) * sin(dLat / 2) +
        cos(_toRad(lat1)) * cos(_toRad(lat2)) *
        sin(dLon / 2) * sin(dLon / 2);
    final c = 2 * atan2(sqrt(a), sqrt(1 - a));
    return r * c;
  }

  static double _toRad(double deg) => deg * pi / 180;
}
