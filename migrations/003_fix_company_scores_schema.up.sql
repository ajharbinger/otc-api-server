-- Add missing columns to company_scores table
ALTER TABLE company_scores
ADD COLUMN qualified BOOLEAN DEFAULT false,
ADD COLUMN requirements_met BOOLEAN DEFAULT false;

-- Update existing records to have proper values
UPDATE company_scores
SET 
    qualified = (score >= 70),
    requirements_met = (score >= 50);

-- Make columns NOT NULL after setting values
ALTER TABLE company_scores
ALTER COLUMN qualified SET NOT NULL,
ALTER COLUMN requirements_met SET NOT NULL;

-- Add indexes for performance
CREATE INDEX idx_company_scores_qualified ON company_scores(qualified);
CREATE INDEX idx_company_scores_requirements_met ON company_scores(requirements_met);
CREATE INDEX idx_company_scores_scored_at ON company_scores(scored_at DESC);