# ADR-006: Photo Storage — MinIO Integration Status

**Status:** Partially implemented — Phase 2 completion required  
**Date:** 2026-03-13  
**Deciders:** Engineering Lead  
**Spec reference:** TECH-MI-003 ("store photos in an object storage service, e.g., MinIO on NITA servers")

---

## Context

TECH-MI-003 specifies storing meter evidence photos in MinIO (object storage) on NITA servers. The `docker-compose.yml` includes a MinIO service. A review flag noted that the API gateway's evidence handler "may use local filesystem or pre-signed URLs" and that "the full MinIO integration flow may not be completely wired up."

## Current State

| Component | Status |
|---|---|
| MinIO in docker-compose.yml | ✅ Present |
| MinIO environment variables in render.yaml | ✅ `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY` |
| Pre-signed URL generation for photo upload | ✅ Evidence handler generates S3-compatible pre-signed PUT URLs |
| Flutter app uploads directly to MinIO via pre-signed URL | ✅ Implemented in `evidence_upload_screen.dart` |
| Photo URL stored in `anomaly_flags.evidence_photo_url` | ✅ Written after upload confirmation |
| MinIO bucket lifecycle policy (retention) | ⚠️ Not configured — photos retained indefinitely |
| NITA MinIO server provisioning | ⚠️ Phase 1 uses Render object storage (S3-compatible); NITA MinIO pending NITA provisioning approval |

## Decision

**The MinIO integration flow is wired up for S3-compatible object storage.** The current Phase 1 deployment uses a Render-hosted S3-compatible bucket with the same interface. Migration to NITA-hosted MinIO requires NITA to provision the server and provide credentials — this is an infrastructure dependency, not a code change.

## What is not complete

1. **Bucket lifecycle policy:** Photos older than 7 years should be archived per OCS-004 retention policy. MinIO lifecycle rules need to be configured via `mc ilm set`.
2. **NITA MinIO provisioning:** Pending NITA infrastructure team. Tracked as IR-STORAGE-001.
3. **Offline photo queue:** Photos taken offline are stored locally on device and uploaded on next sync. The local queue uses Flutter's `path_provider` cache directory — not encrypted at rest. Phase 2 should encrypt the cache.

## Consequences

- **Positive:** No code changes needed for NITA MinIO migration — just credential swap.
- **Action required:** IR-STORAGE-001 (NITA MinIO provisioning). Configure bucket lifecycle policy before go-live. Add cache encryption in Phase 2.

---

*This ADR documents the current MinIO integration state and outstanding infrastructure items.*
