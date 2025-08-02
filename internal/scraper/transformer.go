package scraper

import (
	"fmt"
	"time"

	"github.com/ajharbinger/otc-oxy2-pipeline/internal/models"
)

// Transformer converts scraped data to company models
type Transformer struct{}

// NewTransformer creates a new transformer instance
func NewTransformer() *Transformer {
	return &Transformer{}
}

// TransformToCompany converts ScrapedData to a Company model
func (t *Transformer) TransformToCompany(scraped *models.ScrapedData) (*models.Company, error) {
	if scraped == nil {
		return nil, fmt.Errorf("scraped data is nil")
	}

	company := &models.Company{
		Ticker:    scraped.Ticker,
		UpdatedAt: time.Now(),
	}

	// Merge data from all three pages
	allData := make(map[string]interface{})
	
	// Overview data
	for k, v := range scraped.Overview {
		allData[k] = v
	}
	
	// Financials data
	for k, v := range scraped.Financials {
		allData[k] = v
	}
	
	// Disclosure data
	for k, v := range scraped.Disclosure {
		allData[k] = v
	}

	// Map fields to company struct
	if name, ok := allData["company_name"].(string); ok {
		company.CompanyName = name
	}

	if tier, ok := allData["market_tier"].(string); ok {
		company.MarketTier = tier
	}

	if status, ok := allData["quote_status"].(string); ok {
		company.QuoteStatus = status
	}

	if volume, ok := allData["trading_volume"].(int64); ok {
		company.TradingVolume = volume
	}

	if website, ok := allData["website"].(string); ok {
		company.Website = website
	}

	if description, ok := allData["description"].(string); ok {
		company.Description = description
	}

	if officers, ok := allData["officers"].([]models.Officer); ok {
		company.Officers = officers
	}

	if address, ok := allData["address"].(models.Address); ok {
		company.Address = address
	}

	if agent, ok := allData["transfer_agent"].(string); ok {
		company.TransferAgent = agent
	}

	if auditor, ok := allData["auditor"].(string); ok {
		company.Auditor = auditor
	}

	if date, ok := allData["last_10k_date"].(*time.Time); ok {
		company.Last10KDate = date
	}

	if date, ok := allData["last_10q_date"].(*time.Time); ok {
		company.Last10QDate = date
	}

	if date, ok := allData["last_filing_date"].(*time.Time); ok {
		company.LastFilingDate = date
	}

	if verified, ok := allData["profile_verified"].(bool); ok {
		company.ProfileVerified = verified
	}

	return company, nil
}

// ValidateCompanyData checks if the company data meets minimum requirements
func (t *Transformer) ValidateCompanyData(company *models.Company) []string {
	var errors []string

	if company.Ticker == "" {
		errors = append(errors, "ticker is required")
	}

	if company.CompanyName == "" {
		errors = append(errors, "company name is missing")
	}

	if company.MarketTier == "" {
		errors = append(errors, "market tier is missing")
	}

	return errors
}

// EnrichWithHistoricalContext adds historical context to company data
func (t *Transformer) EnrichWithHistoricalContext(company *models.Company, previousData *models.Company) {
	// Track if this is a new company or update
	if previousData == nil {
		return
	}

	// Add logic to detect significant changes
	// This can be used later for notifications and alerts
	
	// Example: Detect tier changes
	if previousData.MarketTier != company.MarketTier {
		// This change could trigger a notification
	}

	// Example: Detect filing updates
	if previousData.LastFilingDate != nil && company.LastFilingDate != nil {
		if company.LastFilingDate.After(*previousData.LastFilingDate) {
			// New filing detected
		}
	}
}