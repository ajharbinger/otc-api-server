-- Drop triggers
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP TRIGGER IF EXISTS update_companies_updated_at ON companies;
DROP TRIGGER IF EXISTS update_scoring_models_updated_at ON scoring_models;
DROP TRIGGER IF EXISTS update_saved_views_updated_at ON saved_views;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables in reverse order
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS saved_views;
DROP TABLE IF EXISTS company_contacts;
DROP TABLE IF EXISTS company_scores;
DROP TABLE IF EXISTS scrape_jobs;
DROP TABLE IF EXISTS company_history;
DROP TABLE IF EXISTS companies;
DROP TABLE IF EXISTS scoring_models;
DROP TABLE IF EXISTS users;