# ADR-001: Flutter/Dart instead of React Native for Field Officer Mobile App

**Status:** Accepted  
**Date:** 2026-03-13  
**Deciders:** Engineering Lead, Project Director  
**Spec reference:** SRS-MOB-001 through SRS-MOB-006 (originally specified React Native)

---

## Context

The original SRS specified React Native (JavaScript/TypeScript) for the field officer mobile app (SRS-MOB-001). During the build phase the team evaluated both options against the concrete requirements of this deployment:

- Offline-first with SQLite-backed local queue (SRS-MOB-002)
- Biometric authentication (SRS-MOB-003)
- Camera access with GPS-tagged evidence capture (SRS-MOB-004/005)
- Low-bandwidth sync over GPRS/2G in rural Ghana districts
- Target devices: low-cost Android handsets (≥Android 8, 2 GB RAM)

## Decision

**Flutter (Dart) was adopted.** The deprecated React Native folder (`mobile/field-officer-app-rn-deprecated/`) represents the original prototype; `mobile/field-officer-app-flutter/` is the production implementation.

## Rationale

| Criterion | React Native | Flutter |
|---|---|---|
| Offline SQLite | `react-native-sqlite-storage` (community) | `sqflite` (Google-maintained) |
| Biometric | `react-native-biometrics` (varies by vendor) | `local_auth` (Google-maintained) |
| APK size on low RAM devices | ~15–20 MB JS bundle + native bridge | ~8–12 MB AOT-compiled Dart |
| Render performance on low-end Android | JS bridge overhead | Skia direct render, no bridge |
| Offline map tiles | Third-party only | `flutter_map` + `mbtiles` |
| NITA/GWL device standardisation | None specified | None specified |

The critical deciding factor was **APK performance on GWL-issued Android Go devices** (Tecno Pop 5, 2 GB RAM). Flutter's Skia-based rendering has no JavaScript bridge, reducing latency for the photo capture and GPS tagging flow.

## Consequences

- **Positive:** Better performance on target hardware; single codebase; strong offline support.
- **Negative:** Deviates from SRS-MOB-001 as written. Any formal spec audit against the original contract will flag this.
- **Action required:** The SRS must be formally amended (Change Request CR-MOB-001) to record React Native → Flutter. The project director must countersign before GRA final audit.
- **No functional regression:** All SRS-MOB-001 through 006 functional requirements are satisfied by the Flutter implementation.

---

*This ADR was written retrospectively to formalise an existing decision. It does not change the implementation.*
