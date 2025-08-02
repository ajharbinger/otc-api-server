-- Users table for authentication and authorization
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) NOT NULL CHECK (role IN ('admin', 'user')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Scoring models/ICPs configuration
CREATE TABLE scoring_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    rules JSONB NOT NULL,
    version INTEGER DEFAULT 1,
    is_active BOOLEAN DEFAULT true,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Companies master table
CREATE TABLE companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticker VARCHAR(10) UNIQUE NOT NULL,
    company_name VARCHAR(255),
    market_tier VARCHAR(50),
    quote_status VARCHAR(100),
    trading_volume BIGINT DEFAULT 0,
    website VARCHAR(255),
    description TEXT,
    officers JSONB,
    address JSONB,
    transfer_agent VARCHAR(255),
    auditor VARCHAR(255),
    last_10k_date DATE,
    last_10q_date DATE,
    last_filing_date DATE,
    profile_verified BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Company historical snapshots for tracking changes
CREATE TABLE company_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID REFERENCES companies(id) ON DELETE CASCADE,
    snapshot_data JSONB NOT NULL,
    scraped_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    scrape_job_id UUID
);

-- Scraping jobs tracking
CREATE TABLE scrape_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    total_tickers INTEGER DEFAULT 0,
    processed_tickers INTEGER DEFAULT 0,
    failed_tickers INTEGER DEFAULT 0,
    started_by UUID REFERENCES users(id),
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    error_message TEXT
);

-- Company scores from different models
CREATE TABLE company_scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID REFERENCES companies(id) ON DELETE CASCADE,
    scoring_model_id UUID REFERENCES scoring_models(id),
    score INTEGER NOT NULL,
    score_breakdown JSONB NOT NULL,
    scored_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(company_id, scoring_model_id)
);

-- Enriched contact data from DropContact
CREATE TABLE company_contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID REFERENCES companies(id) ON DELETE CASCADE,
    name VARCHAR(255),
    email VARCHAR(255),
    title VARCHAR(255),
    phone VARCHAR(50),
    linkedin_url VARCHAR(500),
    enriched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    enrichment_source VARCHAR(50) DEFAULT 'dropcontact'
);

-- Saved filter views
CREATE TABLE saved_views (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    filters JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- In-app notifications
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    company_id UUID REFERENCES companies(id) ON DELETE CASCADE,
    is_read BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance
CREATE INDEX idx_companies_ticker ON companies(ticker);
CREATE INDEX idx_companies_market_tier ON companies(market_tier);
CREATE INDEX idx_companies_updated_at ON companies(updated_at);
CREATE INDEX idx_company_scores_company_id ON company_scores(company_id);
CREATE INDEX idx_company_scores_scoring_model_id ON company_scores(scoring_model_id);
CREATE INDEX idx_company_history_company_id ON company_history(company_id);
CREATE INDEX idx_company_history_scraped_at ON company_history(scraped_at);
CREATE INDEX idx_notifications_user_id_unread ON notifications(user_id, is_read);
CREATE INDEX idx_scrape_jobs_status ON scrape_jobs(status);

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_companies_updated_at BEFORE UPDATE ON companies
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_scoring_models_updated_at BEFORE UPDATE ON scoring_models
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_saved_views_updated_at BEFORE UPDATE ON saved_views
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();