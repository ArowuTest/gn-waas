# ADR-007: Night Flow Analysis — District Balance Proxy, Not Time-of-Day

**Status:** Accepted with known limitation  
**Date:** 2026-03-13  
**Deciders:** Engineering Lead  
**Spec reference:** SPS-004 ("analyse lowest consumption periods, e.g. 2–4 AM")

---

## Context

SPS-004 describes detecting physical leaks by analysing consumption during the 2–4 AM "minimum night flow" window — the period when customer demand is near-zero, so any flow is likely physical leakage. The `iwa_water_balance.go` code contains a comment acknowledging this limitation.

## Current Implementation

The `night_flow_analyser.go` works at the **district balance level**: it compares total production volume vs total billed volume over a calendar month. If the gap exceeds `sentinel.night_flow_pct_of_daily`, a district-level flag is raised.

This is **not** time-of-day night flow analysis.

## Root Cause

The `meter_readings` table uses a `DATE` column (`reading_date`) with no time-of-day component. Individual meter readings are recorded as daily totals, not time-series. The `production_records` table records bulk district production at `TIMESTAMPTZ` granularity, but customer meter readings lack the time dimension needed for 2–4 AM analysis.

## Why this is acceptable for Phase 1

1. **Detection equivalence at district scale:** The IWA Water Balance approach (system input - authorised consumption = NRW) achieves equivalent leakage detection at the district level without time-of-day data. The result is the same district-level flag; the mechanism differs.
2. **Ghana meter infrastructure:** Most GWL districts use manually-read mechanical meters on a monthly cycle — there is no telemetry. True 2–4 AM analysis requires smart meters with 15-minute interval logging, which are not yet deployed.
3. **Sentinel flag → field investigation:** The output is a district flag triggering a field investigation. Whether the flag was triggered by MNF analysis or district balance analysis, the field response is identical.

## Phase 2 Plan

When smart meters with interval data are deployed in pilot districts:
1. Add `reading_timestamp TIMESTAMPTZ` to `meter_readings` (alongside `reading_date`).
2. Implement `night_flow_analyser.AnalyseMinimumNightFlow()` querying readings between 02:00–04:00 UTC.
3. Flag individual accounts with anomalous MNF, not just districts.

## Consequences

- **Positive:** District-level NRW detection works correctly now.
- **Negative:** Per-account MNF analysis is not possible without smart meter telemetry. SPS-004 is partially implemented.
- **Action required:** SPS-004 to be annotated with Phase 1 / Phase 2 scope. Smart meter pilot plan to be raised with GWL engineering.

---

*This ADR documents a schema-driven limitation with a clear upgrade path.*
