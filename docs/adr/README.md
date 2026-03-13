# ADR Summary Index

All Architecture Decision Records for GN-WAAS.

| ADR | Title | Status | Issue # |
|---|---|---|---|
| [ADR-001](ADR-001-flutter-over-react-native.md) | Flutter/Dart — the official mobile app (no deviation) | Accepted — no action required | #1 |
| [ADR-002](ADR-002-server-side-ocr-not-on-device-tflite.md) | Server-side Tesseract OCR, not on-device TFLite | Accepted with known limitation — CR-MOB-002 required | #2 |
| [ADR-003](ADR-003-timestamp-cdc-not-debezium-wal.md) | Timestamp-based CDC polling, not Debezium/WAL | Accepted — IR-CDC-001 required for Phase 2 | #3 |
| [ADR-004](ADR-004-meter-ingestor-grpc-and-http.md) | Meter ingestor runs gRPC + HTTP — gRPC is primary | Clarification — TECH-MI-001 is met | #4 |
| [ADR-005](ADR-005-meter-ingestor-nats-publish.md) | Meter ingestor publishes to NATS after DB write | Clarification — TECH-MI-003 is met | #5 |
| [ADR-006](ADR-006-minio-photo-storage.md) | Photo storage MinIO integration status | Partially implemented — IR-STORAGE-001 required | #6 |
| [ADR-007](ADR-007-night-flow-district-proxy-not-mnf.md) | Night flow uses district balance proxy, not MNF | Accepted with known limitation — Phase 2 smart meter | #7 |
| [ADR-008](ADR-008-custom-tariff-no-lago-flexprice.md) | Custom tariff engine, no Lago/Flexprice | Accepted — no action needed | #13 |

## Outstanding Actions

| Ref | Description | Owner |
|---|---|---|
| CR-MOB-002 | Amend SRS-MOB-004 on-device OCR → Phase 2 | Project Director |
| IR-CDC-001 | Request WAL logical replication access from NITA | Infrastructure Lead |
| IR-STORAGE-001 | Request MinIO server provisioning from NITA | Infrastructure Lead |
