// GN-WAAS Field Officer App — Main Entry Point

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'config/app_config.dart';
import 'providers/providers.dart';
import 'router/router.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Note: sqflite_common_ffi (desktop SQLite FFI) is only used in test environments.
  // It is a dev_dependency and must NOT be imported in production code.
  // Desktop FFI initialisation is handled in test/helpers/test_setup.dart.

  runApp(
    const ProviderScope(
      child: GNWAASApp(),
    ),
  );
}

class GNWAASApp extends ConsumerWidget {
  const GNWAASApp({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final router = ref.watch(routerProvider);

    // Eagerly fetch remote config on startup (non-blocking)
    // This ensures admin-controlled settings are loaded before first job
    final configAsync = ref.watch(mobileConfigProvider);

    return MaterialApp.router(
      title: 'GN-WAAS Field Officer',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(
          seedColor: const Color(0xFF166534),
          brightness: Brightness.light,
        ),
        useMaterial3: true,
        fontFamily: 'Roboto',
        appBarTheme: const AppBarTheme(
          elevation: 0,
          centerTitle: false,
        ),
      ),
      routerConfig: router,
      // Overlay a force-update gate when the admin has flagged this version
      // as outdated.  The builder wraps the entire Navigator so the gate is
      // shown regardless of which route is active.
      builder: (context, child) {
        return configAsync.when(
          loading: () => child ?? const SizedBox.shrink(),
          error:   (_, __) => child ?? const SizedBox.shrink(),
          data: (config) {
            if (config.forceUpdate &&
                _isVersionBelow(AppConfig.appVersion, config.appMinVersion)) {
              return _ForceUpdateGate(minVersion: config.appMinVersion);
            }
            return child ?? const SizedBox.shrink();
          },
        );
      },
    );
  }

  /// Returns true if [current] is strictly less than [required].
  /// Compares semver-style strings: "1.0.0" < "1.1.0" → true.
  bool _isVersionBelow(String current, String required) {
    final cur = current.split('.').map(int.tryParse).toList();
    final req = required.split('.').map(int.tryParse).toList();
    // Pad shorter version with zeros
    while (cur.length < req.length) cur.add(0);
    while (req.length < cur.length) req.add(0);
    for (int i = 0; i < cur.length; i++) {
      final c = cur[i] ?? 0;
      final r = req[i] ?? 0;
      if (c < r) return true;
      if (c > r) return false;
    }
    return false; // equal
  }
}

/// Full-screen blocking gate shown when the admin sets force_update=true and
/// the installed app version is below app_min_version.
///
/// The officer cannot dismiss this screen — they must update the app via the
/// GWL MDM system or the Play Store.  An admin can disable the gate by
/// setting force_update=false in the admin portal → System Config.
class _ForceUpdateGate extends StatelessWidget {
  final String minVersion;
  const _ForceUpdateGate({required this.minVersion});

  @override
  Widget build(BuildContext context) {
    return Material(
      color: const Color(0xFF0f172a),
      child: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              const Icon(Icons.system_update_alt, color: Color(0xFF2563eb), size: 72),
              const SizedBox(height: 24),
              const Text(
                'Update Required',
                style: TextStyle(
                  color: Colors.white,
                  fontSize: 24,
                  fontWeight: FontWeight.w900,
                ),
              ),
              const SizedBox(height: 12),
              Text(
                'This version of the GN-WAAS Field Officer App is no longer '
                'supported.\n\nPlease update to version $minVersion or later '
                'to continue.',
                textAlign: TextAlign.center,
                style: const TextStyle(color: Color(0xFF94a3b8), fontSize: 15),
              ),
              const SizedBox(height: 32),
              // Informational only — update is pushed via MDM or Play Store
              Container(
                padding: const EdgeInsets.all(16),
                decoration: BoxDecoration(
                  color: const Color(0xFF1e293b),
                  borderRadius: BorderRadius.circular(12),
                ),
                child: const Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(Icons.info_outline, color: Color(0xFF64748b), size: 18),
                    SizedBox(width: 10),
                    Flexible(
                      child: Text(
                        'Contact your GWL supervisor or IT department for the '
                        'latest APK.',
                        style: TextStyle(color: Color(0xFF64748b), fontSize: 13),
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 16),
              Text(
                'Installed: ${AppConfig.appVersion}  ·  Required: $minVersion',
                style: const TextStyle(color: Color(0xFF475569), fontSize: 12),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
