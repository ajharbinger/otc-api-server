-- Insert default scoring models
INSERT INTO scoring_models (name, description, rules, is_active) VALUES
('Double Black Diamond', 'Companies in Expert Market needing services to regain eligibility', '{
  "must_have": [
    {"field": "market_tier", "operator": "equals", "value": "Expert Market"},
    {"field": "quote_status", "operator": "equals", "value": "Ineligible for solicited quotes"}
  ],
  "scoring_rules": [
    {"field": "delinquent_10k", "weight": 1, "condition": "last_10k_date > 15 months ago"},
    {"field": "delinquent_10q", "weight": 1, "condition": "last_10q_date > 6 months ago"},
    {"field": "no_verified_profile", "weight": 1, "condition": "profile_verified = false"},
    {"field": "pink_limited_or_expert", "weight": 1, "condition": "market_tier IN (\"Pink Limited\", \"Expert Market\")"},
    {"field": "no_recent_activity", "weight": 1, "condition": "last_filing_date > 12 months ago"},
    {"field": "reverse_merger_shell", "weight": 1, "condition": "description CONTAINS reverse merger OR shell"},
    {"field": "asian_management", "weight": 1, "condition": "officers.address CONTAINS TW OR HK OR CN"},
    {"field": "cannabis_or_crypto", "weight": 1, "condition": "description CONTAINS cannabis OR CBD OR blockchain OR crypto"},
    {"field": "holding_company_or_spac", "weight": 1, "condition": "description CONTAINS blank check OR SPAC OR holding company"},
    {"field": "active_transfer_agent", "weight": -1, "condition": "transfer_agent IS NOT NULL AND transfer_agent != \"\""},
    {"field": "domain_linked_to_company", "weight": -1, "condition": "website matches company_name"},
    {"field": "auditor_identified", "weight": -1, "condition": "auditor IS NOT NULL AND auditor != \"\""}
  ],
  "minimum_score": 3
}', true),
('Pink Market Opportunity', 'Active Pink sheet companies with potential compliance gaps', '{
  "must_have": [
    {"field": "market_tier", "operator": "equals", "value": "OTC Pink"},
    {"field": "trading_volume", "operator": "greater_than", "value": 0}
  ],
  "must_not": [
    {"field": "last_filing_date", "operator": "less_than", "value": "2023-01-01"}
  ],
  "scoring_rules": [
    {"field": "delinquent_10k", "weight": 1, "condition": "last_10k_date > 15 months ago"},
    {"field": "delinquent_10q", "weight": 1, "condition": "last_10q_date > 6 months ago"},
    {"field": "no_verified_profile", "weight": 1, "condition": "profile_verified = false"},
    {"field": "pink_limited_or_expert", "weight": 1, "condition": "market_tier IN (\"Pink Limited\", \"Expert Market\")"},
    {"field": "no_recent_activity", "weight": 1, "condition": "last_filing_date > 12 months ago"},
    {"field": "reverse_merger_shell", "weight": 1, "condition": "description CONTAINS reverse merger OR shell"},
    {"field": "asian_management", "weight": 1, "condition": "officers.address CONTAINS TW OR HK OR CN"},
    {"field": "cannabis_or_crypto", "weight": 1, "condition": "description CONTAINS cannabis OR CBD OR blockchain OR crypto"},
    {"field": "holding_company_or_spac", "weight": 1, "condition": "description CONTAINS blank check OR SPAC OR holding company"},
    {"field": "active_transfer_agent", "weight": -1, "condition": "transfer_agent IS NOT NULL AND transfer_agent != \"\""},
    {"field": "domain_linked_to_company", "weight": -1, "condition": "website matches company_name"},
    {"field": "auditor_identified", "weight": -1, "condition": "auditor IS NOT NULL AND auditor != \"\""}
  ],
  "minimum_score": 3
}', true);