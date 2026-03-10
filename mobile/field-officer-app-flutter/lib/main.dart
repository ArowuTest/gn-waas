// GN-WAAS Field Officer App — Main Entry Point

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
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
    ref.watch(mobileConfigProvider);

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
    );
  }
}
