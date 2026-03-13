-- Migration 045: water_balance_records — add data_confidence_grade_awwa column
-- ============================================================================
--
-- Context (Issue #4 from code review):
--
--   Migration 023 added data_confidence_score INTEGER (0-100) to
--   water_balance_records.  The sentinel iwa_water_balance.go also computes a
--   DataConfidenceGrade (1-10, per AWWA / spec convention), but that value was
--   never persisted — only returned in the API response struct.
--
--   The column name uses the _awwa suffix to distinguish it clearly from the
--   0-100 score and to signal the grading standard.
--
-- After this migration:
--   data_confidence_score  INTEGER  — raw 0-100 score (migration 023)
--   data_confidence_grade_awwa SMALLINT — 1-10 AWWA grade (this migration)
--     grade = CEIL(score / 10), clamped to [1, 10]
--     1 = lowest confidence, 10 = highest confidence
--
-- The sentinel persist() function is updated in the same commit to write both
-- columns on every upsert.
-- ============================================================================

ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS data_confidence_grade_awwa SMALLINT
        CHECK (data_confidence_grade_awwa BETWEEN 1 AND 10);

COMMENT ON COLUMN water_balance_records.data_confidence_grade_awwa IS
    'AWWA Data Confidence Grade (1-10). Derived from data_confidence_score: '
    'grade = CEIL(score / 10), clamped to [1, 10]. '
    '1 = very low confidence, 10 = very high confidence. '
    'Written by sentinel iwa_water_balance.go alongside data_confidence_score.';

-- Back-fill existing rows using the same formula the sentinel uses.
-- Rows where data_confidence_score IS NULL will also have grade NULL (intentional).
UPDATE water_balance_records
SET data_confidence_grade_awwa = LEAST(10, GREATEST(1, CEIL(data_confidence_score::numeric / 10)))
WHERE data_confidence_score IS NOT NULL
  AND data_confidence_grade_awwa IS NULL;
