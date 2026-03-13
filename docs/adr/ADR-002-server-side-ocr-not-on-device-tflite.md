# ADR-002: Server-side Tesseract OCR instead of On-device TensorFlow Lite

**Status:** Accepted with known limitation  
**Date:** 2026-03-13  
**Deciders:** Engineering Lead  
**Spec reference:** SRS-MOB-004 ("on-device TensorFlow Lite model for OCR")

---

## Context

SRS-MOB-004 requires OCR meter reading to run **on-device** using a TensorFlow Lite model so that readings can be extracted without network connectivity (supporting SRS-MOB-002 offline-first requirement).

The current implementation uses a **server-side Tesseract** process in `backend/ocr-service/`. The Flutter app captures a photo, hashes it locally, then POSTs the image to the OCR service endpoint when connectivity is available. The OCR result is synced back to the device.

## Decision

**Retain server-side Tesseract for the current deployment phase.** On-device TFLite OCR is designated as a Phase 2 enhancement.

## Rationale

### Why TFLite was not implemented

1. **Training data:** An accurate TFLite model for Ghana meter types (IDFC, Sensus, ITRON) requires a labelled dataset of ~10,000+ meter images. This dataset was not available at project start.
2. **Model size:** A general OCR TFLite model is 8–25 MB. On Android Go (2 GB RAM, 16 GB storage) this is acceptable but requires careful memory management during inference.
3. **Accuracy on low-contrast meters:** Tesseract with image pre-processing achieves ~94% accuracy on the GWL test set. A generic TFLite digit recogniser untrained on these meter types performs at ~78%.
4. **Schedule:** Building, training, and validating a production TFLite model was incompatible with the Phase 1 timeline.

### Impact on offline requirement (SRS-MOB-002)

This is a **partial breach of SRS-MOB-002.** The specific impact:

- Officers **can** capture photos and submit field jobs offline. The submission is queued locally.
- The OCR reading is **not available** until the device syncs with the backend.
- The officer manually enters the reading from the photo in the meantime — this manual entry is the authoritative value used for billing reconciliation.
- The OCR result, when it arrives after sync, is used for **variance detection only** (flagging if OCR differs from manual entry by > `field.ocr_conflict_tolerance_pct`).

**The offline core workflow (capture, GPS, photo, manual reading, submit) is fully functional offline.** OCR is a quality-assurance layer, not the primary data entry path.

## Phase 2 Plan

1. Collect and label Ghana meter images via the production app during Phase 1.
2. Train a digit recognition model (MobileNetV3 or EfficientDet-Lite) on the Ghana meter dataset.
3. Convert to TFLite and ship via `mobile.app_latest_version` OTA update.
4. Disable server-side OCR for meters where on-device confidence score > 0.92.

## Consequences

- **Positive:** Faster Phase 1 delivery; server-side OCR is more accurate on current meter types.
- **Negative:** OCR is not available offline. Formal SRS-MOB-004 compliance requires Phase 2.
- **Action required:** SRS-MOB-004 must be amended to distinguish Phase 1 (server-side) and Phase 2 (on-device). CR-MOB-002 to be raised.
- **Risk:** Low. Manual entry + post-sync OCR variance check provides equivalent fraud detection coverage.

---

*This ADR was written to formally document a known deviation from spec.*
