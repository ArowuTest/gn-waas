// GN-WAAS Field Officer App — Main Entry Point

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:sqflite_common_ffi/sqflite_ffi.dart';
import 'package:flutter/foundation.dart';
import 'providers/providers.dart';
import 'router/router.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Initialise SQLite FFI for desktop/test environments
  if (!kIsWeb && (defaultTargetPlatform == TargetPlatform.linux ||
      defaultTargetPlatform == TargetPlatform.windows ||
      defaultTargetPlatform == TargetPlatform.macOS)) {
    sqfliteFfiInit();
    databaseFactory = databaseFactoryFfi;
  }

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
