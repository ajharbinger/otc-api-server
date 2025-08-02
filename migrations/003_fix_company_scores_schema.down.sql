-- Drop indexes
DROP INDEX IF EXISTS idx_company_scores_qualified;
DROP INDEX IF EXISTS idx_company_scores_requirements_met;
DROP INDEX IF EXISTS idx_company_scores_scored_at;

-- Remove added columns
ALTER TABLE company_scores
DROP COLUMN IF EXISTS qualified,
DROP COLUMN IF EXISTS requirements_met;