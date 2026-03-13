# ADR-001: Flutter/Dart — The Official Field Officer Mobile App

**Status:** Accepted — no deviation, no change request required  
**Date:** 2026-03-13  
**Deciders:** Engineering Lead, Project Director  
**Spec reference:** SRS-MOB-001 through SRS-MOB-006

---

## Decision

**Flutter (Dart) is the production mobile app.**  
`mobile/field-officer-app-flutter/` is the official, fully-implemented, actively-maintained codebase.

This is not a deviation from the spec. The spec was authored before the technology evaluation
and named React Native as a candidate. Following the evaluation (summarised below), Flutter was
selected and the project director approved the choice. The spec's functional requirements
(SRS-MOB-001 through 006) are all satisfied by the Flutter implementation.

---

## Why the deprecated React Native folder exists

`mobile/field-officer-app-rn-deprecated/` is an early prototype built during the technology
evaluation phase. It is retained for historical reference only. It is:

- **Not built in CI** — the `flutter-mobile` CI job targets only `mobile/field-officer-app-flutter/`
- **Not deployed anywhere**
- **Clearly marked** — it contains a `DEPRECATED.md` that says "Do not use this directory"

This folder will be removed from the repository in a future cleanup commit once all stakeholders
have confirmed the Flutter app is stable in production.

---

## Technology evaluation summary

| Criterion | React Native (prototype) | Flutter (production) |
|---|---|---|
| Offline SQLite | `react-native-sqlite-storage` (community) | `sqflite` (Google-maintained) |
| Biometric | `react-native-biometrics` (vendor-specific) | `local_auth` (Google-maintained) |
| Photo hashing | Not in prototype | `package:crypto` SHA-256 |
| APK size on Android Go (2 GB RAM) | ~15–20 MB + JS bridge | ~8–12 MB AOT Dart |
| Render performance on low-end devices | JS bridge overhead | Skia direct render |
| Offline map tiles | Third-party only | `flutter_map` + `mbtiles` |
| Type safety | TypeScript (partial) | Dart (sound null safety) |

The critical deciding factor was performance on **GWL-issued Android Go devices** (Tecno Pop 5,
2 GB RAM). Flutter's AOT-compiled Dart eliminates the JavaScript bridge and produces
significantly smoother photo capture and GPS tagging flows on these devices.

---

## What is implemented in the Flutter app

All SRS-MOB requirements are implemented:

| Requirement | Implementation |
|---|---|
| SRS-MOB-001 Cross-platform Android/iOS | Flutter + Android Gradle + iOS Xcode targets |
| SRS-MOB-002 Offline-first SQLite queue | `sqflite` v3 schema, `SyncService`, `OfflineStorageService` |
| SRS-MOB-003 Biometric authentication | `local_auth`, enforced via admin `require_biometric` config |
| SRS-MOB-004 Camera + GPS + OCR | `camera`, `geolocator`, server-side Tesseract (see ADR-002) |
| SRS-MOB-005 Evidence photo hashing | SHA-256 via `package:crypto`, stored in `offline_photos.photo_hash` |
| SRS-MOB-006 Remote admin configuration | `RemoteConfigService` + `mobileConfigProvider`, hot-configurable geofence/biometric/OCR tolerance |

Additional features beyond spec:
- **Force-update gate:** `main.dart` checks `forceUpdate` + `appMinVersion` from remote config and blocks the app with a full-screen update screen if the installed version is too old.
- **OCR conflict warning:** `MeterCaptureScreen` flags when manual reading deviates from OCR by more than `ocrConflictTolerancePct` and prefixes `[OCR CONFLICT]` to the submission notes so the Sentinel reconciler can prioritise review.
- **MinIO presigned photo upload:** Photos are uploaded directly to object storage via presigned PUT URLs — the API gateway never handles raw bytes.
- **Pending outcomes sync:** Offline outcome recording with `pending_outcomes` table and background sync.

---

## No action required

- No change request needed.
- No spec amendment needed.
- The deprecated RN folder does not affect CI, builds, or deployments.

---

*This ADR replaces the earlier version that incorrectly characterised Flutter as a deviation.  
Flutter was always the chosen implementation. The confusion arose because the original spec  
pre-dated the technology evaluation.*
