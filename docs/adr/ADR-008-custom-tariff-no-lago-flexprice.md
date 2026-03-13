# ADR-008: Custom Tariff Engine — No Lago/Flexprice Dependency

**Status:** Accepted — no action required  
**Date:** 2026-03-13  
**Deciders:** Project Director, Engineering Lead  
**Spec reference:** Master Spec §1.2 vs "Additional Solution Details" document

---

## Context

Two specification documents conflict on billing infrastructure:

- **"Additional Solution Details"** document mentions Lago and Flexprice as OSS billing modules.
- **Master Spec §1.2** explicitly states: *"This system is custom-built and does not clone or depend on third-party platforms like Flexprice or Lago."*

## Decision

**The master spec governs.** The tariff engine is entirely custom-built (`backend/tariff-engine/`). No Lago or Flexprice dependency exists anywhere in the codebase.

## Rationale

1. **PURC-specific tariff structure:** Ghana's PURC 2026 multi-tier residential tariff with lifeline exemptions, VAT treatment, and category-specific service charges does not map cleanly to Lago's subscription-billing model or Flexprice's dynamic pricing primitives.
2. **Regulatory auditability:** The PURC and GRA require the tariff calculation to be auditable against their published rate schedules. A custom engine with explicit rate tables (seeded from `tariff_rates` with `regulatory_ref` columns) is directly traceable. A third-party billing platform introduces an abstraction layer that complicates regulatory audit.
3. **Shadow billing:** The core requirement is a *shadow bill* computed independently of GWL's billing system for fraud detection. Lago/Flexprice are designed to be *the* billing system, not a shadow comparator.
4. **Data sovereignty:** NITA and GWL data governance policies restrict billing data from leaving NITA infrastructure. Third-party SaaS billing platforms would violate this policy.

## Consequences

- **No action required.** The codebase correctly follows the master spec.
- **The "Additional Solution Details" document should be archived** as superseded by the master spec. It should not be referenced in any formal audit or procurement documentation.

---

*This ADR resolves a document conflict in favour of the governing master spec.*
