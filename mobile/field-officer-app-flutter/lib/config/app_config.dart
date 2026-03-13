// GN-WAAS Field Officer App — App Configuration
// Reads from environment variables injected at build time

class AppConfig {
  static const String appName = 'GN-WAAS Field Officer';
  static const String appVersion = '1.0.0';

  // API base URL — override with --dart-define=API_BASE_URL=...
  static const String apiBaseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'https://api.gnwaas.nita.gov.gh/api/v1',
  );

  // Keycloak config
  static const String keycloakUrl = String.fromEnvironment(
    'KEYCLOAK_URL',
    defaultValue: 'https://auth.gnwaas.nita.gov.gh',
  );
  static const String keycloakRealm = String.fromEnvironment(
    'KEYCLOAK_REALM',
    defaultValue: 'gnwaas',
  );

  // GPS geofence radius in metres
  // Offline / pre-config-load fallback geofence radius.
  // The live value comes from system_config key field.gps_fence_radius_m (seeded as 5.0m).
  // 10m is used here so the GPS lock still has teeth if the config endpoint is
  // unreachable on first launch — 100m would effectively disable the GPS check.
  static const double geofenceRadiusMetres = 10.0;

  // Offline sync interval in seconds
  static const int syncIntervalSeconds = 30;

  // Max photo size in bytes (5 MB)
  static const int maxPhotoBytes = 5 * 1024 * 1024;

  // API timeout
  static const Duration apiTimeout = Duration(seconds: 30);

  // SQLite DB name
  static const String dbName = 'gnwaas_offline.db';
  static const int dbVersion = 3;
}
