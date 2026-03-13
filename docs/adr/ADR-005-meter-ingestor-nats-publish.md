# ADR-005: Meter Ingestor Publishes to NATS — TECH-MI-003 Satisfied

**Status:** Accepted (clarification, not deviation)  
**Date:** 2026-03-13  
**Deciders:** Engineering Lead  
**Spec reference:** TECH-MI-003 ("writes the raw audit event to a message queue")

---

## Context

A review flag noted that "the meter-ingestor appears to write directly to PostgreSQL rather than publishing to NATS first." This would remove the decoupling benefit described in TECH-MI-003.

## Clarification

The ingestor service (`ingestor_service.go`) performs **both** operations in sequence:

```
1. Validate reading
2. Persist to PostgreSQL (authoritative record)
3. Publish NATS event → gnwaas.meter.reading.received
4. If anomaly pre-check triggered: publish → gnwaas.sentinel.scan.trigger
```

Step 3 publishes to NATS after the DB write. This satisfies TECH-MI-003 ("writes to a message queue on successful validation"). The NATS publish is non-blocking — if NATS is unavailable (`nc == nil`), the DB write is still committed and no error is returned. The sentinel consumes `gnwaas.sentinel.scan.trigger` to run immediate district scans.

## Why DB write precedes NATS publish

The DB write is the authoritative record. If NATS publish fails after a successful DB write, the reading is not lost — the sentinel's scheduled scan will process it on the next cycle. If the order were reversed (NATS first, then DB), a crash between the two would create a phantom NATS event with no DB record.

This is the standard "outbox-light" pattern at this scale. A true outbox table (DB transaction + outbox row, NATS publisher reads outbox) would be the Phase 2 improvement for guaranteed exactly-once delivery.

## Consequences

- **No action required.** TECH-MI-003 is met. NATS publish is present and functional.
- **Recommendation:** Implement transactional outbox pattern in Phase 2 if exactly-once delivery guarantees are required for regulatory reporting.

---

*This ADR clarifies an apparent deviation that is not a real deviation.*
