-- Add report language per repo and translated summaries
ALTER TABLE repos ADD COLUMN IF NOT EXISTS report_language VARCHAR(10) DEFAULT 'en';
ALTER TABLE analysis_results ADD COLUMN IF NOT EXISTS summary_translated TEXT DEFAULT '';
