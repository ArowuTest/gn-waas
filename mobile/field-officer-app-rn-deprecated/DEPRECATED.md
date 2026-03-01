# ⚠️ DEPRECATED — React Native App

This directory contains the **original React Native / Expo** implementation of the
GN-WAAS Field Officer mobile app.

**It has been fully replaced by the Flutter app at `../field-officer-app-flutter/`.**

## Why replaced?
- Flutter provides better offline-first SQLite support
- Stronger type safety with Dart
- Better camera + GPS + biometric plugin ecosystem
- Easier cross-platform (Android + iOS) build pipeline
- SHA-256 photo hashing via `package:crypto`

## Do not use this directory
All active development happens in `../field-officer-app-flutter/`.
This directory is retained for historical reference only and will be
removed in a future cleanup commit.
