-- ============================================================
-- Migration 041: Fix ocr_status enum — add CONFIRMED and MANUAL
-- ============================================================
-- The ocr_status enum (created in 001_extensions_and_types.sql) only had:
--   SUCCESS, FAILED, BLURRY, TAMPERED, CONFLICT, PENDING
--
-- Field officers submit evidence with ocr_status = 'CONFIRMED' (manually
-- confirmed reading) or 'MANUAL' (manually entered value). Without these
-- values the WriteEvidence UPDATE fails with an invalid enum cast error,
-- causing HTTP 500 on POST /field-jobs/:id/submit (evidence endpoint).
--
-- Fix: add CONFIRMED and MANUAL to the enum idempotently.
-- ============================================================

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_enum
        WHERE enumtypid = 'ocr_status'::regtype
          AND enumlabel = 'CONFIRMED'
    ) THEN
        ALTER TYPE ocr_status ADD VALUE 'CONFIRMED' AFTER 'SUCCESS';
    END IF;
END$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_enum
        WHERE enumtypid = 'ocr_status'::regtype
          AND enumlabel = 'MANUAL'
    ) THEN
        ALTER TYPE ocr_status ADD VALUE 'MANUAL' AFTER 'CONFIRMED';
    END IF;
END$$;

COMMENT ON TYPE ocr_status IS
    'OCR reading result status. '
    'SUCCESS=auto-read OK, CONFIRMED=field officer confirmed, MANUAL=manually entered, '
    'FAILED=unreadable, BLURRY=image quality issue, TAMPERED=evidence of tampering, '
    'CONFLICT=reading conflicts with billing data, PENDING=awaiting processing.';
