# ADR-003: Timestamp-based CDC Polling instead of Debezium/WAL + Kafka

**Status:** Accepted  
**Date:** 2026-03-13  
**Deciders:** Engineering Lead, Infrastructure Lead  
**Spec reference:** TECH-DI-001, TECH-DI-002 (Debezium + PostgreSQL WAL + Kafka)

---

## Context

The spec (TECH-DI-001/002) describes Change Data Capture using Debezium monitoring the PostgreSQL Write-Ahead Log (WAL), publishing change events to a Kafka topic. The cdc-ingestor service would consume that topic.

The implemented `cdc-ingestor` uses **timestamp-based polling**: `WHERE updated_at > last_sync_time` against the GWL replica database, on a configurable interval (`cdc.sync_interval_minutes`).

## Decision

**Timestamp-based CDC polling is the production implementation.** Debezium/Kafka is not deployed.

## Rationale

### Infrastructure constraints

1. **NITA hosting:** The GWL replica database is hosted on NITA infrastructure. NITA did not grant permission to install Debezium connector plugins or modify `wal_level` from `replica` to `logical` (required for WAL CDC). This is a contractual constraint, not a technical one.
2. **Kafka:** Running a Kafka cluster (minimum 3 brokers for production reliability) requires dedicated VMs not within the Phase 1 infrastructure budget. NATS (already deployed) satisfies the internal event bus requirement.
3. **GWL DBA approval:** Enabling `wal_level = logical` requires a PostgreSQL restart and DBA sign-off from GWL. This was not obtained for Phase 1.

### Known limitations vs WAL-based CDC

| Issue | Impact | Mitigation |
|---|---|---|
| `updated_at` must be reliably set | If a GWL batch job updates rows without touching `updated_at`, changes are missed | All GWL tables under sync have `updated_at` with trigger-maintained DEFAULT NOW(). Monitored via `cdc.max_lag_minutes` alert. |
| Higher latency | Polling interval (default 5 min) vs near-realtime WAL | Acceptable for billing reconciliation; not for real-time alerts. Sentinel scans are triggered by NATS, not CDC lag. |
| DELETEs not captured | Deleted accounts invisible to GN-WAAS | GWL uses soft-delete (`is_active=false`). Physical DELETE is policy-prohibited on billing tables. |

### NATS vs Kafka

NATS JetStream provides at-least-once delivery, replay, and consumer groups — sufficient for the GN-WAAS internal event bus. Kafka's additional guarantees (total ordering across partitions, compaction) are not required at Phase 1 scale.

## Phase 2 Plan

When NITA grants `wal_level = logical` access:
1. Deploy Debezium PostgreSQL connector against GWL replica.
2. Replace polling loop in cdc-ingestor with Debezium event consumer.
3. Debezium → NATS JetStream bridge (retaining NATS as internal bus, no Kafka needed).

## Consequences

- **Positive:** No infrastructure dependency on NITA WAL access; simpler deployment.
- **Negative:** Deviates from TECH-DI-001/002. DELETEs are not captured (mitigated by soft-delete policy).
- **Action required:** TECH-DI-001/002 must be annotated with Phase 1 constraint. Infrastructure request for WAL access (IR-CDC-001) to be raised with NITA for Phase 2.

---

*This ADR was written to formally document a known deviation from spec driven by infrastructure constraints.*
